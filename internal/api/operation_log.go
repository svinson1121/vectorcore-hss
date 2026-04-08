package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

type OperationLogListOutput struct{ Body []models.OperationLog }
type OperationLogOutput struct{ Body *models.OperationLog }
type OperationLogIDInput struct{ ID int `path:"id"` }

func registerOperationLogRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-operation-log", Method: http.MethodGet, Path: "/oam/operation_log", Summary: "List Operation Logs", Tags: []string{"OAM"}}, s.listOperationLogs)
	huma.Register(api, huma.Operation{OperationID: "get-operation-log", Method: http.MethodGet, Path: "/oam/operation_log/{id}", Summary: "Get Operation Log", Tags: []string{"OAM"}}, s.getOperationLog)
	huma.Register(api, huma.Operation{OperationID: "rollback-operation", Method: http.MethodPost, Path: "/oam/operation_log/{id}/rollback", Summary: "Rollback Operation", Tags: []string{"OAM"}}, s.rollbackOperation)
}

func (s *Server) listOperationLogs(ctx context.Context, _ *struct{}) (*OperationLogListOutput, error) {
	var items []models.OperationLog
	if err := s.db.WithContext(ctx).Order("timestamp desc").Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &OperationLogListOutput{Body: items}, nil
}

func (s *Server) getOperationLog(ctx context.Context, input *OperationLogIDInput) (*OperationLogOutput, error) {
	var item models.OperationLog
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &OperationLogOutput{Body: &item}, nil
}

// rollbackOperation reverses a previously logged create, update, or delete.
//
//   - create rollback → DELETE the created record
//   - update rollback → UPDATE all columns back to the before snapshot
//   - delete rollback → INSERT the original record with its original primary key
//
// Raw SQL is used so that the DB driver handles type coercion (JSON numbers
// unmarshal as float64 in Go, which GORM's ORM layer rejects for integer columns).
func (s *Server) rollbackOperation(ctx context.Context, input *OperationLogIDInput) (*struct{}, error) {
	var entry models.OperationLog
	if err := s.db.WithContext(ctx).First(&entry, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}

	if entry.Changes == nil {
		return nil, huma.Error422UnprocessableEntity("no change data recorded for this operation", nil)
	}

	var cr changeRecord
	if err := json.Unmarshal([]byte(*entry.Changes), &cr); err != nil {
		return nil, huma.Error500InternalServerError("failed to parse change record", err)
	}

	pkCol := cr.PKColumn
	if pkCol == "" {
		pkCol = "id"
	}

	db := s.db.WithContext(ctx)
	table := entry.DBTableName

	switch entry.Operation {
	case "create":
		if cr.After == nil {
			return nil, huma.Error422UnprocessableEntity("no after-state recorded; cannot rollback", nil)
		}
		sql := fmt.Sprintf(`DELETE FROM "%s" WHERE "%s" = ?`, table, pkCol)
		if err := db.Exec(sql, entry.ItemID).Error; err != nil {
			return nil, huma.Error500InternalServerError("rollback delete failed", err)
		}

	case "update":
		if cr.Before == nil {
			return nil, huma.Error422UnprocessableEntity("no before-state recorded; cannot rollback", nil)
		}
		setParts, vals := buildSetClause(cr.Before, pkCol)
		vals = append(vals, entry.ItemID)
		sql := fmt.Sprintf(`UPDATE "%s" SET %s WHERE "%s" = ?`, table, setParts, pkCol)
		if err := db.Exec(sql, vals...).Error; err != nil {
			return nil, huma.Error500InternalServerError("rollback update failed", err)
		}

	case "delete":
		if cr.Before == nil {
			return nil, huma.Error422UnprocessableEntity("no before-state recorded; cannot rollback", nil)
		}
		colSQL, placeholders, vals := buildInsertClause(cr.Before)
		sql := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`, table, colSQL, placeholders)
		if err := db.Exec(sql, vals...).Error; err != nil {
			return nil, huma.Error500InternalServerError("rollback insert failed", err)
		}

	default:
		return nil, huma.Error422UnprocessableEntity("unknown operation type: "+entry.Operation, nil)
	}

	return nil, nil
}

// buildSetClause builds "col1" = ?, "col2" = ?, ... (excluding the PK column)
// and returns the clause string plus the ordered values slice.
func buildSetClause(m map[string]interface{}, pkCol string) (string, []interface{}) {
	keys := sortedKeys(m)
	parts := make([]string, 0, len(keys))
	vals := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		if k == pkCol {
			continue
		}
		parts = append(parts, fmt.Sprintf(`"%s" = ?`, k))
		vals = append(vals, normalizeJSON(m[k]))
	}
	return strings.Join(parts, ", "), vals
}

// buildInsertClause builds the column list, placeholders, and values for INSERT.
func buildInsertClause(m map[string]interface{}) (string, string, []interface{}) {
	keys := sortedKeys(m)
	cols := make([]string, 0, len(keys))
	placeholders := make([]string, 0, len(keys))
	vals := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		cols = append(cols, fmt.Sprintf(`"%s"`, k))
		placeholders = append(placeholders, "?")
		vals = append(vals, normalizeJSON(m[k]))
	}
	return strings.Join(cols, ", "), strings.Join(placeholders, ", "), vals
}

// normalizeJSON converts float64 values (the default JSON number type in Go)
// to int64 when they represent whole numbers, so DB drivers accept them for
// integer columns.
func normalizeJSON(v interface{}) interface{} {
	f, ok := v.(float64)
	if !ok {
		return v
	}
	if f == math.Trunc(f) && !math.IsInf(f, 0) {
		return int64(f)
	}
	return f
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
