package services

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

// reminderClock derives both evaluation inputs from the same Berlin instant.
func reminderClock(clocks ...func() time.Time) func() (string, int) {
	now := timezone.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return func() (string, int) {
		instant := now().In(timezone.Berlin)
		return timezone.DateFromTime(instant).String(), instant.Hour()*60 + instant.Minute()
	}
}

type reminderAttendanceReader struct {
	source interface {
		ListOpenStudentIDsForDate(context.Context, timezone.Date) ([]int64, error)
	}
}

func (r reminderAttendanceReader) ListOpenStudentIDsForDate(ctx context.Context, date string) ([]int64, error) {
	return r.source.ListOpenStudentIDsForDate(ctx, timezone.Date(date))
}

type reminderPickupReader struct {
	source interface {
		GetBulkEffectivePickupTimesForDate(context.Context, []int64, timezone.Date) (map[int64]*schedule.EffectivePickupTime, error)
	}
}

func (r reminderPickupReader) GetBulkEffectivePickupTimesForDate(ctx context.Context, ids []int64, date string) (map[int64]*ports.EffectivePickupTime, error) {
	values, err := r.source.GetBulkEffectivePickupTimesForDate(ctx, ids, timezone.Date(date))
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*ports.EffectivePickupTime, len(values))
	for id, value := range values {
		if value == nil {
			result[id] = nil
			continue
		}
		result[id] = &ports.EffectivePickupTime{PickupTime: value.PickupTime}
	}
	return result, nil
}

type reminderSettings struct {
	config.SettingsService
}

func (r reminderSettings) Snapshot(ctx context.Context) (context.Context, error) {
	batch, ok := r.SettingsService.(interface {
		ResolveMany(context.Context, []string) (*config.SettingsSnapshot, error)
	})
	if !ok {
		return ctx, nil
	}
	snapshot, err := batch.ResolveMany(ctx, []string{
		configModel.KeyRemindersPickupUpcomingEnabled,
		configModel.KeyRemindersPickupOverdueEnabled,
		configModel.KeyRemindersActivityStartEnabled,
		configModel.KeyRemindersActivityOverdueEnabled,
		configModel.KeyRemindersPickupUpcomingLeadMinutes,
		configModel.KeyRemindersActivityStartLeadMinutes,
		configModel.KeyTimetableOverdueThresholdMinutes,
		configModel.KeyPresenceMode,
	})
	if err != nil {
		return ctx, err
	}
	return config.WithSettingsSnapshot(ctx, snapshot), nil
}

func (r reminderSettings) PickupUpcomingEnabled(ctx context.Context) (bool, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersPickupUpcomingEnabled, r.ResolveBool)
}

func (r reminderSettings) PickupOverdueEnabled(ctx context.Context) (bool, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersPickupOverdueEnabled, r.ResolveBool)
}

func (r reminderSettings) ActivityStartEnabled(ctx context.Context) (bool, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersActivityStartEnabled, r.ResolveBool)
}

func (r reminderSettings) ActivityOverdueEnabled(ctx context.Context) (bool, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersActivityOverdueEnabled, r.ResolveBool)
}

func (r reminderSettings) PickupLeadMinutes(ctx context.Context) (int, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes, r.ResolveInt)
}

func (r reminderSettings) ActivityLeadMinutes(ctx context.Context) (int, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersActivityStartLeadMinutes, r.ResolveInt)
}

func (r reminderSettings) OverdueThresholdMinutes(ctx context.Context) (int, error) {
	return reminderSettingValue(ctx, configModel.KeyTimetableOverdueThresholdMinutes, r.ResolveInt)
}

func (r reminderSettings) BinaryPresence(ctx context.Context) (bool, error) {
	v, err := reminderSettingValue(ctx, configModel.KeyPresenceMode, r.ResolveString)
	return v == configModel.PresenceModeBinary, err
}

func reminderSettingValue[T any](ctx context.Context, key string, resolve func(context.Context, string) (T, error)) (T, error) {
	value, err := resolve(ctx, key)
	if err != nil {
		return value, fmt.Errorf("resolve %s: %w", key, err)
	}
	return value, nil
}
