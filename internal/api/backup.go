package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/version"
)

const backupVersion = "1"

// BackupDocument is the full HSS provisioning backup. Runtime state
// (serving_apn, emergency_subscriber) and the TAC device database are excluded.
type BackupDocument struct {
	Version              string                       `json:"version"               doc:"Backup format version — must be \"1\""`
	ExportedAt           time.Time                    `json:"exported_at"           doc:"UTC timestamp of the export"`
	HSSVersion           string                       `json:"hss_version"           doc:"HSS application version at export time"`
	AlgorithmProfiles    []models.AlgorithmProfile    `json:"algorithm_profiles"`
	APNs                 []models.APN                 `json:"apns"`
	ChargingRules        []models.ChargingRule        `json:"charging_rules"`
	TFTs                 []models.TFT                 `json:"tfts"`
	IFCProfiles          []models.IFCProfile          `json:"ifc_profiles"`
	EIR                  []models.EIR                 `json:"eir"`
	RoamingRules         []models.RoamingRules        `json:"roaming_rules"`
	AUCs                 []models.AUC                 `json:"aucs"`
	Subscribers          []models.Subscriber          `json:"subscribers"`
	SubscriberRouting    []models.SubscriberRouting   `json:"subscriber_routing"`
	SubscriberAttributes []models.SubscriberAttribute `json:"subscriber_attributes"`
	IMSSubscribers       []models.IMSSubscriber       `json:"ims_subscribers"`
}

// BackupResponse wraps BackupDocument with a Content-Disposition download header.
type BackupResponse struct {
	ContentDisposition string `header:"Content-Disposition"`
	Body               BackupDocument
}

// RestoreTableResult reports the delete and insert counts for one table.
type RestoreTableResult struct {
	Table    string `json:"table"    doc:"Table name"`
	Deleted  int64  `json:"deleted"  doc:"Rows wiped"`
	Inserted int    `json:"inserted" doc:"Rows inserted from backup"`
}

// RestoreResponse summarises the outcome of a restore operation.
type RestoreResponse struct {
	Body struct {
		Tables []RestoreTableResult `json:"tables"`
	}
}

func registerBackupRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "export-backup",
		Method:      http.MethodGet,
		Path:        "/oam/backup",
		Summary:     "Export database backup",
		Description: "Exports all HSS provisioning data as a downloadable JSON file. " +
			"Excludes runtime state (serving APN, emergency subscribers) and the TAC device database.",
		Tags: []string{"OAM"},
	}, func(ctx context.Context, _ *struct{}) (*BackupResponse, error) {
		doc, err := buildBackup(s.db)
		if err != nil {
			s.log.Error("backup: export failed", zap.Error(err))
			return nil, huma.Error500InternalServerError("export failed", err)
		}
		filename := fmt.Sprintf("hss-backup-%s.json", doc.ExportedAt.Format("2006-01-02T15-04-05Z"))
		return &BackupResponse{
			ContentDisposition: fmt.Sprintf(`attachment; filename="%s"`, filename),
			Body:               *doc,
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "import-restore",
		Method:        http.MethodPost,
		Path:          "/oam/restore",
		Summary:       "Import database backup",
		Description:   "Wipes all HSS provisioning data and restores from the supplied backup document in a single transaction. Runtime state and TAC data are not affected.",
		Tags:          []string{"OAM"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, in *struct{ Body BackupDocument }) (*RestoreResponse, error) {
		if in.Body.Version != backupVersion {
			return nil, huma.Error422UnprocessableEntity(
				fmt.Sprintf("unsupported backup version %q (expected %q)", in.Body.Version, backupVersion), nil)
		}
		results, err := applyRestore(s.db, &in.Body)
		if err != nil {
			s.log.Error("backup: restore failed", zap.Error(err))
			return nil, huma.Error500InternalServerError("restore failed", err)
		}
		if s.cache != nil {
			for _, sub := range in.Body.Subscribers {
				s.cache.InvalidateCache(sub.IMSI)
			}
		}
		s.log.Info("backup: restore complete",
			zap.Int("subscribers", len(in.Body.Subscribers)),
			zap.Int("aucs", len(in.Body.AUCs)),
		)
		resp := &RestoreResponse{}
		resp.Body.Tables = results
		return resp, nil
	})
}

// buildBackup reads all provisioning tables into a BackupDocument.
func buildBackup(db *gorm.DB) (*BackupDocument, error) {
	doc := &BackupDocument{
		Version:    backupVersion,
		ExportedAt: time.Now().UTC(),
		HSSVersion: version.AppVersion,
	}
	for _, step := range []struct {
		label string
		dest  interface{}
	}{
		{"algorithm_profiles", &doc.AlgorithmProfiles},
		{"apns", &doc.APNs},
		{"charging_rules", &doc.ChargingRules},
		{"tfts", &doc.TFTs},
		{"ifc_profiles", &doc.IFCProfiles},
		{"eir", &doc.EIR},
		{"roaming_rules", &doc.RoamingRules},
		{"aucs", &doc.AUCs},
		{"subscribers", &doc.Subscribers},
		{"subscriber_routing", &doc.SubscriberRouting},
		{"subscriber_attributes", &doc.SubscriberAttributes},
		{"ims_subscribers", &doc.IMSSubscribers},
	} {
		if err := db.Find(step.dest).Error; err != nil {
			return nil, fmt.Errorf("%s: %w", step.label, err)
		}
	}
	return doc, nil
}

// applyRestore wipes all provisioning tables and reloads from doc in a single
// database transaction. On PostgreSQL the autoIncrement sequences are reset so
// subsequent inserts via the API continue from the correct value.
func applyRestore(db *gorm.DB, doc *BackupDocument) ([]RestoreTableResult, error) {
	var results []RestoreTableResult

	err := db.Transaction(func(tx *gorm.DB) error {
		sess := tx.Session(&gorm.Session{AllowGlobalUpdate: true})

		// Wipe in reverse FK dependency order.
		type wipeStep struct {
			label string
			model interface{}
		}
		for _, w := range []wipeStep{
			{"subscriber_attributes", &models.SubscriberAttribute{}},
			{"subscriber_routing", &models.SubscriberRouting{}},
			{"ims_subscriber", &models.IMSSubscriber{}},
			{"subscriber", &models.Subscriber{}},
			{"auc", &models.AUC{}},
			{"tft", &models.TFT{}},
			{"charging_rule", &models.ChargingRule{}},
			{"eir", &models.EIR{}},
			{"roaming_rules", &models.RoamingRules{}},
			{"apn", &models.APN{}},
			{"ifc_profile", &models.IFCProfile{}},
			{"algorithm_profile", &models.AlgorithmProfile{}},
		} {
			res := sess.Delete(w.model)
			if res.Error != nil {
				return fmt.Errorf("wipe %s: %w", w.label, res.Error)
			}
			results = append(results, RestoreTableResult{Table: w.label, Deleted: res.RowsAffected})
		}

		// Insert in FK-safe order. CreateInBatches preserves explicit IDs.
		type insertStep struct {
			label string
			rows  interface{}
			count int
		}
		for _, step := range []insertStep{
			{"algorithm_profile", &doc.AlgorithmProfiles, len(doc.AlgorithmProfiles)},
			{"ifc_profile", &doc.IFCProfiles, len(doc.IFCProfiles)},
			{"apn", &doc.APNs, len(doc.APNs)},
			{"charging_rule", &doc.ChargingRules, len(doc.ChargingRules)},
			{"tft", &doc.TFTs, len(doc.TFTs)},
			{"eir", &doc.EIR, len(doc.EIR)},
			{"roaming_rules", &doc.RoamingRules, len(doc.RoamingRules)},
			{"auc", &doc.AUCs, len(doc.AUCs)},
			{"subscriber", &doc.Subscribers, len(doc.Subscribers)},
			{"subscriber_routing", &doc.SubscriberRouting, len(doc.SubscriberRouting)},
			{"subscriber_attributes", &doc.SubscriberAttributes, len(doc.SubscriberAttributes)},
			{"ims_subscriber", &doc.IMSSubscribers, len(doc.IMSSubscribers)},
		} {
			if step.count > 0 {
				if err := tx.CreateInBatches(step.rows, 500).Error; err != nil {
					return fmt.Errorf("insert %s: %w", step.label, err)
				}
			}
			for i := range results {
				if results[i].Table == step.label {
					results[i].Inserted = step.count
					break
				}
			}
		}

		// PostgreSQL only: reset sequences so post-restore API inserts don't
		// collide with the restored primary key values.
		if tx.Dialector.Name() == "postgres" {
			for _, st := range []struct{ table, col string }{
				{"algorithm_profile", "algorithm_profile_id"},
				{"apn", "apn_id"},
				{"charging_rule", "charging_rule_id"},
				{"tft", "tft_id"},
				{"ifc_profile", "ifc_profile_id"},
				{"eir", "eir_id"},
				{"roaming_rules", "roaming_rule_id"},
				{"auc", "auc_id"},
				{"subscriber", "subscriber_id"},
				{"subscriber_routing", "subscriber_routing_id"},
				{"subscriber_attributes", "subscriber_attributes_id"},
				{"ims_subscriber", "ims_subscriber_id"},
			} {
				sql := fmt.Sprintf(
					`SELECT setval(pg_get_serial_sequence('%s','%s'), COALESCE((SELECT MAX(%s) FROM %s), 1))`,
					st.table, st.col, st.col, st.table,
				)
				if err := tx.Exec(sql).Error; err != nil {
					return fmt.Errorf("seq reset %s.%s: %w", st.table, st.col, err)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}
