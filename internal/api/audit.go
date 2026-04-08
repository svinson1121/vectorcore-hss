package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

// auditKey is the context key used to mark DB operations as API-driven.
// Only writes whose context carries this key are recorded in operation_log.
// Diameter-driven DB writes use contexts that never pass through API middleware,
// so they are never audited.
type auditKeyType struct{}

var auditKey = auditKeyType{}

// WithAudit returns a context that will cause GORM operations to be recorded
// in operation_log. Injected by apiAuditMiddleware for every HTTP request.
func WithAudit(ctx context.Context) context.Context {
	return context.WithValue(ctx, auditKey, true)
}

func isAuditContext(ctx context.Context) bool {
	v, _ := ctx.Value(auditKey).(bool)
	return v
}

// changeRecord is the JSON stored in OperationLog.Changes.
// It holds full record snapshots so any operation can be reversed.
//
//	create → Before=nil, After=new record
//	update → Before=old record, After=new record
//	delete → Before=old record, After=nil
type changeRecord struct {
	PKColumn string                 `json:"pk_column"`
	Before   map[string]interface{} `json:"before,omitempty"`
	After    map[string]interface{} `json:"after,omitempty"`
}

const auditBeforeKey = "audit:before_state"

// registerAuditCallbacks installs GORM hooks that record a full before/after
// snapshot in operation_log for every successful create, update, or delete.
func registerAuditCallbacks(db *gorm.DB) {
	db.Callback().Update().Before("gorm:update").Register("audit:before_update", captureBeforeState)
	db.Callback().Delete().Before("gorm:delete").Register("audit:before_delete", captureBeforeState)

	db.Callback().Create().After("gorm:create").Register("audit:after_create", afterCreate)
	db.Callback().Update().After("gorm:update").Register("audit:after_update", afterUpdate)
	db.Callback().Delete().After("gorm:delete").Register("audit:after_delete", afterDelete)
}

// captureBeforeState fetches the current record from the DB and stashes it in
// the GORM statement instance so the After hook can build the diff.
//
// GORM's BeforeDelete callback fires before SQL is compiled, so two strategies
// are needed depending on how the delete was called:
//
//  1. Reflected struct has the PK set (Save/Update pattern) → use it directly.
//  2. Delete(&T{}, id) pattern → GORM stores a clause.IN with the "~~~py~~~"
//     primary-key sentinel; extract the value and query by actual column name.
func captureBeforeState(db *gorm.DB) {
	if db.Statement == nil || db.Statement.Table == "operation_log" {
		return
	}
	if !isAuditContext(db.Statement.Context) {
		return
	}
	if db.Statement.Schema == nil || db.Statement.Schema.PrioritizedPrimaryField == nil {
		return
	}
	pkCol := db.Statement.Schema.PrioritizedPrimaryField.DBName

	// Strategy 1: PK is set on the reflected model struct (update with Save).
	id := primaryKeyInt(db)
	if id != 0 {
		stashCurrentRecord(db, pkCol, id)
		return
	}

	// Strategy 2: Delete(&T{}, id) — GORM adds a clause.IN with the primary-key
	// sentinel column before the BeforeDelete hook fires.
	if whereC, ok := db.Statement.Clauses["WHERE"]; ok {
		if where, ok := whereC.Expression.(clause.Where); ok {
			for _, expr := range where.Exprs {
				in, ok := expr.(clause.IN)
				if !ok {
					continue
				}
				col, ok := in.Column.(clause.Column)
				if !ok {
					continue
				}
				if (col.Name == clause.PrimaryKey || col.Name == pkCol) && len(in.Values) == 1 {
					if id = anyToInt(in.Values[0]); id != 0 {
						stashCurrentRecord(db, pkCol, id)
						return
					}
				}
			}
		}
	}
}

func stashCurrentRecord(db *gorm.DB, pkCol string, id int) {
	var current map[string]interface{}
	if err := db.Session(&gorm.Session{NewDB: true}).
		Table(db.Statement.Table).
		Where(pkCol+" = ?", id).
		Take(&current).Error; err != nil {
		return
	}
	db.InstanceSet(auditBeforeKey, current)
}

func afterCreate(db *gorm.DB) {
	if db.Error != nil || db.Statement == nil || db.Statement.Table == "operation_log" {
		return
	}
	if !isAuditContext(db.Statement.Context) {
		return
	}

	after := currentRecordMap(db)
	// currentRecordMap returns nil for batch creates or when the row cannot be
	// reloaded by primary key. Nothing meaningful to log in that case.
	if after == nil {
		return
	}
	pkCol := pkColumnName(db)

	writeLog(db, "create", pkCol, nil, after)
}

func afterUpdate(db *gorm.DB) {
	if db.Error != nil || db.Statement == nil || db.Statement.Table == "operation_log" {
		return
	}
	if !isAuditContext(db.Statement.Context) {
		return
	}

	before := instanceBefore(db)
	after := currentRecordMap(db)
	pkCol := pkColumnName(db)

	writeLog(db, "update", pkCol, before, after)
}

func afterDelete(db *gorm.DB) {
	if db.Error != nil || db.Statement == nil || db.Statement.Table == "operation_log" {
		return
	}
	if !isAuditContext(db.Statement.Context) {
		return
	}

	before := instanceBefore(db)
	pkCol := pkColumnName(db)

	writeLog(db, "delete", pkCol, before, nil)
}

const maxOperationLogRows = 100

func writeLog(db *gorm.DB, op, pkCol string, before, after map[string]interface{}) {
	cr := changeRecord{PKColumn: pkCol, Before: before, After: after}
	data, _ := json.Marshal(cr)
	s := string(data)

	entry := models.OperationLog{
		ItemID:       primaryKeyInt(db),
		OperationID:  newOperationID(),
		Operation:    op,
		Changes:      &s,
		LastModified: time.Now().UTC().Format(time.RFC3339),
		DBTableName:  db.Statement.Table,
	}
	sess := db.Session(&gorm.Session{NewDB: true})
	sess.Create(&entry)

	// Prune to keep only the most recent maxOperationLogRows rows.
	sess.Exec(
		`DELETE FROM "operation_log" WHERE id NOT IN `+
			`(SELECT id FROM "operation_log" ORDER BY id DESC LIMIT ?)`,
		maxOperationLogRows,
	)
}

// instanceBefore retrieves the before-state stashed by captureBeforeState.
func instanceBefore(db *gorm.DB) map[string]interface{} {
	val, ok := db.InstanceGet(auditBeforeKey)
	if !ok {
		return nil
	}
	m, _ := val.(map[string]interface{})
	return m
}

// currentRecordMap reloads the current DB row into a generic map. This avoids
// losing zero values like nam=0 to json `omitempty` tags on model structs.
func currentRecordMap(db *gorm.DB) map[string]interface{} {
	if db.Statement == nil || db.Statement.Table == "" {
		return nil
	}
	pkCol := pkColumnName(db)
	id := primaryKeyInt(db)
	if pkCol == "" || id == 0 {
		return nil
	}
	var current map[string]interface{}
	if err := db.Session(&gorm.Session{NewDB: true}).
		Table(db.Statement.Table).
		Where(pkCol+" = ?", id).
		Take(&current).Error; err != nil {
		return nil
	}
	return current
}

// pkColumnName returns the primary key DB column name for the current statement.
func pkColumnName(db *gorm.DB) string {
	if db.Statement != nil && db.Statement.Schema != nil && db.Statement.Schema.PrioritizedPrimaryField != nil {
		return db.Statement.Schema.PrioritizedPrimaryField.DBName
	}
	return "id"
}

// primaryKeyInt extracts the integer primary key from the statement using
// three strategies in order:
//  1. Reflected struct — works for create, update, and delete with a populated model
//  2. WHERE clause expressions — needed for Delete(&T{}, id) before Vars are built
//  3. Statement Vars — fallback for after-hooks once SQL has been compiled
func primaryKeyInt(db *gorm.DB) int {
	if db.Statement == nil {
		return 0
	}

	// 1. Reflected struct value (id is set on the model).
	if db.Statement.Schema != nil {
		pk := db.Statement.Schema.PrioritizedPrimaryField
		if pk != nil && db.Statement.ReflectValue.IsValid() {
			rv := db.Statement.ReflectValue
			if rv.Kind() == reflect.Ptr {
				rv = rv.Elem()
			}
			if rv.Kind() == reflect.Struct {
				val, zero := pk.ValueOf(context.Background(), rv)
				if !zero {
					return anyToInt(val)
				}
			}
		}
	}

	// 2. WHERE clause — for Delete(&T{}, id), GORM stores the condition as a
	// clause.Eq expression before the callback fires; Vars are not yet built.
	if id := pkFromWhereClauses(db); id != 0 {
		return id
	}

	// 3. Compiled bind variables (populated after SQL is built in after-hooks).
	if len(db.Statement.Vars) > 0 {
		return anyToInt(db.Statement.Vars[0])
	}

	return 0
}

// pkFromWhereClauses scans the WHERE clause for an Eq condition on the primary
// key column and returns its value.  GORM uses the real column name (e.g.
// "roaming_rule_id") when the schema is known, or the "@primary_key" sentinel
// when it is not.
func pkFromWhereClauses(db *gorm.DB) int {
	if db.Statement == nil || db.Statement.Schema == nil {
		return 0
	}
	pk := db.Statement.Schema.PrioritizedPrimaryField
	if pk == nil {
		return 0
	}
	pkName := pk.DBName

	where, ok := db.Statement.Clauses["WHERE"]
	if !ok {
		return 0
	}
	whereExpr, ok := where.Expression.(clause.Where)
	if !ok {
		return 0
	}
	for _, expr := range whereExpr.Exprs {
		eq, ok := expr.(clause.Eq)
		if !ok {
			continue
		}
		col, ok := eq.Column.(clause.Column)
		if ok && (col.Name == pkName || col.Name == clause.PrimaryKey) {
			return anyToInt(eq.Value)
		}
	}
	return 0
}

func anyToInt(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case uint:
		return int(x)
	case uint64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}

func newOperationID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
