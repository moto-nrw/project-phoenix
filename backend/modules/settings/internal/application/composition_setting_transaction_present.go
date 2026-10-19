package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func AttachSettingUnit[U any](ctx context.Context, unit *U, attach func(context.Context, U) context.Context) context.Context {
	if unit != nil {
		return attach(ctx, *unit)
	}
	return ctx
}
func SettingContextCallback[T any](fn func(context.Context) error) func(context.Context, T) error {
	return func(ctx context.Context, _ T) error { return fn(ctx) }
}
func SettingTransactionPresent(ctx context.Context, find func(context.Context) (any, bool)) bool {
	_, ok := find(ctx)
	return ok
}
func LockSettingStaffBalance(ctx context.Context, staffID int64, tenantID func(context.Context) int64, acquire func(context.Context, string, bool) error) error {
	if staffID <= 0 {
		return errors.New("staff id is required")
	}
	school := tenantID(ctx)
	if school <= 0 {
		return errors.New("tenant id is required")
	}
	if err := acquire(ctx, fmt.Sprintf("staff-balance:%d:%d", school, staffID), false); err != nil {
		return fmt.Errorf("lock staff balance writes: %w", err)
	}
	return nil
}
func OperatorSettingSchema(ctx context.Context, schoolID int64, execute func(context.Context, int64, func(context.Context) error) error, source func() configModel.SettingsService) (json.RawMessage, error) {
	var schema *configModel.SettingsSchema
	err := execute(ctx, schoolID, func(ctx context.Context) error {
		var err error
		schema, err = source().GetSchemaForOperator(ctx, nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(schema)
}
func OperatorSettingReveal(ctx context.Context, schoolID int64, key string, execute func(context.Context, int64, func(context.Context) error) error, source func() configModel.SettingsService) (any, error) {
	var value any
	err := execute(ctx, schoolID, func(ctx context.Context) error { var err error; value, err = source().Resolve(ctx, key); return err })
	return value, err
}
func OperatorSettingToday() configModel.CalendarDate {
	today := calendar.TodayDate()
	return configModel.NewCalendarDate(today.Year(), today.Month(), today.Day())
}
func NewAnalyseFreigabeReader(reader interface {
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
}, logger *slog.Logger, key string) func(context.Context, int64) bool {
	return func(ctx context.Context, schoolID int64) bool {
		freigabe, err := reader.ResolveBoolForTenant(ctx, schoolID, key)
		if err != nil {
			logger.WarnContext(ctx, "analyse-freigabe unresolved, core action stays anonymous",
				slog.Int64("school_id", schoolID),
				slog.String("error", err.Error()),
			)
			return false
		}
		return freigabe
	}
}
