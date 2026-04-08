package api

import (
	"context"
	"fmt"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

func conflictInUse(entity, label, refType, refLabel string) error {
	if refLabel == "" {
		return huma.Error409Conflict(fmt.Sprintf("%s %s is still used by %s", entity, label, refType), nil)
	}
	return huma.Error409Conflict(fmt.Sprintf("%s %s is still used by %s %q", entity, label, refType, refLabel), nil)
}

func csvContainsID(column string, id int) (string, string) {
	token := "%," + strconv.Itoa(id) + ",%"
	return "(',' || COALESCE(" + column + ", '') || ',') LIKE ?", token
}

func firstString(ctx context.Context, db *gorm.DB, model interface{}, column, where string, args ...interface{}) (string, error) {
	var row struct {
		Value string
	}
	if err := db.WithContext(ctx).Model(model).Select(column+" AS value").Where(where, args...).Take(&row).Error; err != nil {
		return "", err
	}
	return row.Value, nil
}
