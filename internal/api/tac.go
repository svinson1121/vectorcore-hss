package api

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/svinson1121/vectorcore-hss/internal/metrics"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/taccache"
)

// ── I/O types ─────────────────────────────────────────────────────────────────

type TACListOutput struct{ Body []models.TACModel }
type TACOutput struct{ Body *models.TACModel }
type TACCodeInput struct{ TAC string `path:"tac"` }
type TACIMEIInput struct{ IMEI string `path:"imei"` }

type TACListInput struct {
	Make  string `query:"make"  doc:"Filter by manufacturer (case-insensitive substring)"`
	Model string `query:"model" doc:"Filter by model name (case-insensitive substring)"`
	Limit int    `query:"limit" default:"100" minimum:"1" maximum:"10000" doc:"Maximum number of records to return"`
}

type TACCreateInput struct{ Body *models.TACModel }
type TACUpdateInput struct {
	TAC  string `path:"tac"`
	Body *struct {
		Make  string `json:"make"  required:"true"`
		Model string `json:"model" required:"true"`
	}
}

type TACImportInput struct {
	Body *struct {
		// CSV text with one TAC record per line.  The first three columns must
		// be: TAC (8 digits), Make, Model.  Header rows and comment lines are
		// skipped automatically — any row whose first field is not all digits
		// is ignored.
		CSV string `json:"csv" required:"true" doc:"CSV text (TAC, Make, Model columns; additional columns ignored; header and comment rows skipped automatically)"`
	}
}

type TACImportResult struct {
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
	Errors   int `json:"errors"`
}
type TACImportOutput struct{ Body *TACImportResult }

// TACLookupOutput is returned by the IMEI lookup endpoint.
type TACLookupResult struct {
	TAC   string `json:"tac"`
	Make  string `json:"make"`
	Model string `json:"model"`
}
type TACLookupOutput struct{ Body *TACLookupResult }

// ── Route registration ────────────────────────────────────────────────────────

func registerTACRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-tac", Method: http.MethodGet,
		Path: "/eir/tac", Summary: "List TAC entries", Tags: []string{"EIR"},
	}, s.listTAC)
	huma.Register(api, huma.Operation{
		OperationID: "create-tac", Method: http.MethodPost,
		Path: "/eir/tac", Summary: "Create TAC entry", Tags: []string{"EIR"},
		DefaultStatus: http.StatusCreated,
	}, s.createTAC)
	huma.Register(api, huma.Operation{
		OperationID:  "import-tac", Method: http.MethodPost,
		Path:         "/eir/tac/import", Summary: "Bulk import TAC records from CSV", Tags: []string{"EIR"},
		MaxBodyBytes: 50 * 1024 * 1024, // 50MB — full GSMA TAC DB is ~3MB CSV / ~15MB JSON-encoded
	}, s.importTAC)
	huma.Register(api, huma.Operation{
		OperationID: "export-tac", Method: http.MethodGet,
		Path:    "/eir/tac/export",
		Summary: "Export TAC database as CSV",
		Description: "Downloads the full TAC device database as a CSV file (TAC, Make, Model). " +
			"Compatible with the bulk import endpoint — the exported file can be re-imported directly.",
		Tags: []string{"EIR"},
	}, s.exportTAC)
	huma.Register(api, huma.Operation{
		OperationID: "lookup-tac-by-imei", Method: http.MethodGet,
		Path: "/eir/tac/imei/{imei}", Summary: "Lookup device by IMEI", Tags: []string{"EIR"},
	}, s.lookupTACByIMEI)
	huma.Register(api, huma.Operation{
		OperationID: "get-tac", Method: http.MethodGet,
		Path: "/eir/tac/{tac}", Summary: "Get TAC entry by code", Tags: []string{"EIR"},
	}, s.getTAC)
	huma.Register(api, huma.Operation{
		OperationID: "update-tac", Method: http.MethodPut,
		Path: "/eir/tac/{tac}", Summary: "Update TAC entry", Tags: []string{"EIR"},
	}, s.updateTAC)
	huma.Register(api, huma.Operation{
		OperationID: "delete-tac", Method: http.MethodDelete,
		Path: "/eir/tac/{tac}", Summary: "Delete TAC entry", Tags: []string{"EIR"},
	}, s.deleteTAC)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (s *Server) listTAC(ctx context.Context, input *TACListInput) (*TACListOutput, error) {
	q := s.db.WithContext(ctx).Model(&models.TACModel{})
	if input.Make != "" {
		q = q.Where("LOWER(make) LIKE ?", "%"+strings.ToLower(input.Make)+"%")
	}
	if input.Model != "" {
		q = q.Where("LOWER(model) LIKE ?", "%"+strings.ToLower(input.Model)+"%")
	}
	var items []models.TACModel
	if err := q.Limit(input.Limit).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &TACListOutput{Body: items}, nil
}

func (s *Server) createTAC(ctx context.Context, input *TACCreateInput) (*TACOutput, error) {
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.tac != nil {
		s.tac.Set(input.Body.TAC, input.Body.Make, input.Body.Model)
	}
	return &TACOutput{Body: input.Body}, nil
}

func (s *Server) getTAC(ctx context.Context, input *TACCodeInput) (*TACOutput, error) {
	var item models.TACModel
	if err := s.db.WithContext(ctx).Where("tac = ?", input.TAC).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &TACOutput{Body: &item}, nil
}

func (s *Server) lookupTACByIMEI(ctx context.Context, input *TACIMEIInput) (*TACLookupOutput, error) {
	imei := input.IMEI
	// Try 8-digit TAC first (modern), then 6-digit (legacy pre-2004).
	var item models.TACModel
	var found bool
	if len(imei) >= 8 {
		err := s.db.WithContext(ctx).Where("tac = ?", imei[:8]).First(&item).Error
		if err == nil {
			found = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error500InternalServerError("db error", err)
		}
	}
	if !found && len(imei) >= 6 {
		err := s.db.WithContext(ctx).Where("tac = ?", imei[:6]).First(&item).Error
		if err == nil {
			found = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error500InternalServerError("db error", err)
		}
	}
	if !found {
		return nil, huma.Error404NotFound("no TAC entry for this IMEI", nil)
	}
	return &TACLookupOutput{Body: &TACLookupResult{TAC: item.TAC, Make: item.Make, Model: item.Model}}, nil
}

func (s *Server) updateTAC(ctx context.Context, input *TACUpdateInput) (*TACOutput, error) {
	var item models.TACModel
	if err := s.db.WithContext(ctx).Where("tac = ?", input.TAC).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	item.Make = input.Body.Make
	item.Model = input.Body.Model
	item.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Save(&item).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.tac != nil {
		s.tac.Set(item.TAC, item.Make, item.Model)
	}
	return &TACOutput{Body: &item}, nil
}

func (s *Server) deleteTAC(ctx context.Context, input *TACCodeInput) (*struct{}, error) {
	if err := s.db.WithContext(ctx).Where("tac = ?", input.TAC).Delete(&models.TACModel{}).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.tac != nil {
		s.tac.Delete(input.TAC)
	}
	return nil, nil
}

func (s *Server) importTAC(ctx context.Context, input *TACImportInput) (*TACImportOutput, error) {
	r := csv.NewReader(strings.NewReader(input.Body.CSV))
	r.FieldsPerRecord = -1 // variable — TAC DB can have any number of columns
	r.LazyQuotes = true

	rows, err := r.ReadAll()
	if err != nil {
		return nil, huma.Error422UnprocessableEntity("invalid CSV", err)
	}

	result := &TACImportResult{}
	const batchSize = 500
	batch := make([]models.TACModel, 0, batchSize)
	now := time.Now().UTC().Format(time.RFC3339)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		res := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tac"}},
			DoUpdates: clause.AssignmentColumns([]string{"make", "model", "last_modified"}),
		}).Create(&batch)
		if res.Error != nil {
			return res.Error
		}
		// GORM doesn't distinguish inserted vs updated on upsert;
		// count rows written and record metric.
		written := int(res.RowsAffected)
		metrics.TACImportedTotal.Add(float64(written))
		// Heuristic: rows affected == 1 per INSERT (new), 2 per UPDATE (PostgreSQL).
		// On SQLite RowsAffected is just 1 for both, so we report all as inserted.
		updated := written / 2
		inserted := written - updated
		result.Inserted += inserted
		result.Updated += updated
		batch = batch[:0]
		return nil
	}

	for _, row := range rows {
		if len(row) < 3 {
			result.Skipped++
			continue
		}
		tac := strings.TrimSpace(row[0])
		make_ := strings.TrimSpace(row[1])
		model := strings.TrimSpace(row[2])

		// Skip header/comment rows — TAC must be all digits.
		if !allDigits(tac) {
			result.Skipped++
			continue
		}
		if make_ == "" || model == "" {
			result.Skipped++
			continue
		}

		batch = append(batch, models.TACModel{
			TAC:          tac,
			Make:         make_,
			Model:        model,
			LastModified: now,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				result.Errors++
				batch = batch[:0]
			}
		}
	}
	if err := flush(); err != nil {
		result.Errors++
	}

	// Reload the full cache from DB so every entry is consistent.
	if s.tac != nil {
		if err := s.reloadTACCache(ctx); err != nil {
			s.log.Warn("tac: cache reload after import failed", zap.Error(err))
		}
	}

	return &TACImportOutput{Body: result}, nil
}

// reloadTACCache reads all rows from the tac table and rebuilds the in-memory map.
func (s *Server) reloadTACCache(ctx context.Context) error {
	var rows []models.TACModel
	if err := s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return err
	}
	entries := make(map[string]taccache.Entry, len(rows))
	for _, r := range rows {
		entries[r.TAC] = taccache.Entry{Make: r.Make, Model: r.Model}
	}
	s.tac.Load(entries)
	return nil
}

func (s *Server) exportTAC(_ context.Context, _ *struct{}) (*huma.StreamResponse, error) {
	var rows []models.TACModel
	if err := s.db.Order("tac asc").Find(&rows).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	filename := fmt.Sprintf("hss-tac-%s.csv", time.Now().UTC().Format("2006-01-02T15-04-05Z"))
	return &huma.StreamResponse{
		Body: func(ctx huma.Context) {
			ctx.SetHeader("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			ctx.SetHeader("Content-Type", "text/csv")
			ctx.SetStatus(http.StatusOK)
			w := csv.NewWriter(ctx.BodyWriter())
			_ = w.Write([]string{"TAC", "Make", "Model"})
			for i := range rows {
				_ = w.Write([]string{rows[i].TAC, rows[i].Make, rows[i].Model})
			}
			w.Flush()
		},
	}, nil
}

// allDigits returns true when s is non-empty and contains only ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

