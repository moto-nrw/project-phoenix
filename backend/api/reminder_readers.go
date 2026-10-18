package api

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

// ReminderClock derives both evaluation inputs from the same Berlin instant.
func ReminderClock(clocks ...func() time.Time) func() (string, int) {
	now := timezone.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return func() (string, int) {
		instant := now().In(timezone.Berlin)
		return timezone.DateFromTime(instant).String(), instant.Hour()*60 + instant.Minute()
	}
}

type ReminderVisitReader struct {
	Source interface {
		ListOpenVisitRooms(context.Context, int64) ([]studentpresence.OpenVisitRoom, error)
	}
}

func (r ReminderVisitReader) ListOpenVisitStudentIDsByRoom(ctx context.Context) (map[int64][]int64, error) {
	rows, err := r.Source.ListOpenVisitRooms(ctx, 0)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]int64)
	for _, row := range rows {
		result[row.RoomID] = append(result[row.RoomID], row.StudentID)
	}
	return result, nil
}

type ReminderPickupReader struct {
	Source interface {
		GetBulkEffectivePickupTimesForDate(context.Context, []int64, timezone.Date) (map[int64]*careplan.EffectivePickupTime, error)
	}
}

func (r ReminderPickupReader) GetBulkEffectivePickupTimesForDate(ctx context.Context, ids []int64, date string) (map[int64]*ports.EffectivePickupTime, error) {
	values, err := r.Source.GetBulkEffectivePickupTimesForDate(ctx, ids, timezone.Date(date))
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

type ReminderSettings struct {
	settingsCompose.SettingsService
}

func (r ReminderSettings) Snapshot(ctx context.Context) (context.Context, error) {
	batch, ok := r.SettingsService.(interface {
		ResolveMany(context.Context, []string) (*settingsCompose.SettingsSnapshot, error)
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
	return settingsCompose.WithSettingsSnapshot(ctx, snapshot), nil
}

func (r ReminderSettings) PickupUpcomingEnabled(ctx context.Context) (bool, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersPickupUpcomingEnabled, r.ResolveBool)
}

func (r ReminderSettings) PickupOverdueEnabled(ctx context.Context) (bool, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersPickupOverdueEnabled, r.ResolveBool)
}

func (r ReminderSettings) ActivityStartEnabled(ctx context.Context) (bool, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersActivityStartEnabled, r.ResolveBool)
}

func (r ReminderSettings) ActivityOverdueEnabled(ctx context.Context) (bool, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersActivityOverdueEnabled, r.ResolveBool)
}

func (r ReminderSettings) PickupLeadMinutes(ctx context.Context) (int, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes, r.ResolveInt)
}

func (r ReminderSettings) ActivityLeadMinutes(ctx context.Context) (int, error) {
	return reminderSettingValue(ctx, configModel.KeyRemindersActivityStartLeadMinutes, r.ResolveInt)
}

func (r ReminderSettings) OverdueThresholdMinutes(ctx context.Context) (int, error) {
	return reminderSettingValue(ctx, configModel.KeyTimetableOverdueThresholdMinutes, r.ResolveInt)
}

func (r ReminderSettings) BinaryPresence(ctx context.Context) (bool, error) {
	v, err := reminderSettingValue(ctx, configModel.KeyPresenceMode, r.ResolveString)
	return configModel.IsBinaryPresence(v), err
}

func reminderSettingValue[T any](ctx context.Context, key string, resolve func(context.Context, string) (T, error)) (T, error) {
	value, err := resolve(ctx, key)
	if err != nil {
		return value, fmt.Errorf("resolve %s: %w", key, err)
	}
	return value, nil
}
