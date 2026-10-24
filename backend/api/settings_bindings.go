package api

import (
	"context"
	"fmt"
	"log/slog"

	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The bindings below read the Settings Platform's keys for owners whose
// compositions take a narrow, key-free port (#4004, #4085): the owner names
// what it needs, the root resolves it from the settings.

// classDayWriteScopeSettings is the slice of the settings service the class
// day writes resolve operations.school_portal_write_scope and the morning
// attendance switch with.
type classDayWriteScopeSettings interface {
	Resolve(ctx context.Context, key string) (any, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// ClassDayWriteScope applies operations.school_portal_write_scope: only a
// ticked class arrival exception opens that write of the school.
type ClassDayWriteScope struct{ Settings classDayWriteScopeSettings }

func (b ClassDayWriteScope) SchoolMayWriteClassArrivalExceptions(ctx context.Context) (bool, error) {
	scope, err := b.Settings.Resolve(ctx, configModels.KeySchoolPortalWriteScope)
	if err != nil {
		return false, err
	}
	return configModels.SchoolPortalWriteScopeIncludes(scope, configModels.SchoolPortalWriteScopeClassArrivalExceptions), nil
}

// SchoolMayReportClassTrips reports whether a Lehrkraft may report a class
// trip through the school portal (#4799).
func (b ClassDayWriteScope) SchoolMayReportClassTrips(ctx context.Context) (bool, error) {
	return schoolPortalWriteScopeIncludes(ctx, b.Settings, configModels.SchoolPortalWriteScopeClassTrips)
}

// schoolPortalWriteScopeIncludes resolves the multi-select write scope
// (#4794) and reports whether it ticks scope.
func schoolPortalWriteScopeIncludes(ctx context.Context, settings classDayWriteScopeSettings, scope string) (bool, error) {
	value, err := settings.Resolve(ctx, configModels.KeySchoolPortalWriteScope)
	if err != nil {
		return false, err
	}
	return configModels.SchoolPortalWriteScopeIncludes(value, scope), nil
}

// SchoolMayWriteDayHints reports whether a Lehrkraft may leave today's hint
// for the OGS through the school portal (#4795).
func (b ClassDayWriteScope) SchoolMayWriteDayHints(ctx context.Context) (bool, error) {
	return schoolPortalWriteScopeIncludes(ctx, b.Settings, configModels.SchoolPortalWriteScopeDayHints)
}

// newClassDayWriteScope binds the write scope; nil settings leave it unbound.
func newClassDayWriteScope(settings classDayWriteScopeSettings) classdayCompose.SchoolWriteScope {
	if settings == nil {
		return nil
	}
	return ClassDayWriteScope{Settings: settings}
}

// ClassDaySlotListReader is the slice of the settings service the slot lists
// read.
type ClassDaySlotListReader interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	// LockSlotListCutoffPairShared acquires the SHARED advisory lock guarding
	// the Ganztag pickup-cutoff pair so both cutoffs come from one consistent
	// snapshot (#1565 review pass 12).
	LockSlotListCutoffPairShared(ctx context.Context) error
}

// ClassDaySlotListSettings resolves the slot lists' timetable flag and
// Ganztag pickup cutoffs.
type ClassDaySlotListSettings struct{ Settings ClassDaySlotListReader }

func (b ClassDaySlotListSettings) TimetableEnabled(ctx context.Context) (bool, error) {
	return b.Settings.ResolveBool(ctx, configModels.KeyTimetableEnabled)
}

// PickupCutoffs resolves both Ganztag cutoffs under the SHARED variant of the
// advisory lock the settings writer takes exclusively. Without it the two
// reads run as separate statements under READ COMMITTED, and an administrator
// lowering BOTH boundaries through two individually valid writes could commit
// between them and hand the reader an inverted pair (#1565 review pass 12).
func (b ClassDaySlotListSettings) PickupCutoffs(ctx context.Context) (string, string, error) {
	if err := b.Settings.LockSlotListCutoffPairShared(ctx); err != nil {
		return "", "", err
	}
	short, err := b.Settings.ResolveString(ctx, configModels.KeySlotListShortDayCutoff)
	if err != nil {
		return "", "", fmt.Errorf("resolve short-day pickup cutoff: %w", err)
	}
	long, err := b.Settings.ResolveString(ctx, configModels.KeySlotListLongDayCutoff)
	if err != nil {
		return "", "", fmt.Errorf("resolve long-day pickup cutoff: %w", err)
	}
	return short, long, nil
}

// deviceScanSettingsReader is the slice of the settings service the device
// scans read.
type deviceScanSettingsReader interface {
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// DeviceScanSettings resolves the tenant settings the device scans read.
type DeviceScanSettings struct{ Settings deviceScanSettingsReader }

func (s DeviceScanSettings) FeedbackEnabled(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyFeedbackEnabled)
}

func (s DeviceScanSettings) RoomCapacityDetailsDisclosed(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyCheckinRoomCapacityDetailsEnabled)
}

func (s DeviceScanSettings) ActivityCapacityDetailsDisclosed(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyCheckinActivityCapacityDetailsEnabled)
}

// DailyCheckoutTime resolves the tenant override or registry default.
func (s DeviceScanSettings) DailyCheckoutTime(ctx context.Context) (string, error) {
	return s.Settings.ResolveString(ctx, configModels.KeyStudentDailyCheckoutTime)
}

func (s DeviceScanSettings) PerStudentCheckoutEnabled(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyPerStudentCheckoutEnabled)
}

func (s DeviceScanSettings) PerStudentCheckoutDeltaMinutes(ctx context.Context) (int, error) {
	return s.Settings.ResolveInt(ctx, configModels.KeyPerStudentCheckoutDeltaMinutes)
}

func (s DeviceScanSettings) DailyCheckoutFromAllRoomsEnabled(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyCheckoutDailyFromAllRoomsEnabled)
}

// newDeviceScanSettings binds the scan settings; nil settings leave them
// unbound.
func newDeviceScanSettings(settings deviceScanSettingsReader) devicescanCompose.ScanSettings {
	if settings == nil {
		return nil
	}
	return DeviceScanSettings{Settings: settings}
}

// deviceConfigurationResolver is the slice of the settings service the
// device configuration resolves one tenant's values with.
type deviceConfigurationResolver interface {
	ResolveStringForTenant(context.Context, int64, string) (string, error)
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
}

// DeviceConfiguration loads one tenant's device configuration from the
// settings, in one snapshot when the settings service batches.
type DeviceConfiguration struct{ Settings deviceConfigurationResolver }

func (rs DeviceConfiguration) LoadConfiguration(ctx context.Context, tenantID int64) (devicescan.Configuration, error) {
	var response devicescan.Configuration
	settingsCtx, err := rs.settingsContext(ctx, tenantID)
	if err != nil {
		return response, err
	}
	rawTime, err := rs.Settings.ResolveStringForTenant(settingsCtx, tenantID, configModels.KeyStudentDailyCheckoutTime)
	if err != nil {
		return response, err
	}
	if rawTime != "" {
		response.Checkout.DailyCheckoutTime = &rawTime
	}
	for _, setting := range []struct {
		key    string
		target *bool
	}{
		{configModels.KeyCheckoutRaumwechselEnabled, &response.Checkout.RaumwechselEnabled},
		{configModels.KeyCheckoutSchulhofEnabled, &response.Checkout.SchulhofEnabled},
		{configModels.KeyCheckoutWCEnabled, &response.Checkout.WCEnabled},
		{configModels.KeyFeedbackEnabled, &response.Feedback.Enabled},
	} {
		value, resolveErr := rs.Settings.ResolveBoolForTenant(settingsCtx, tenantID, setting.key)
		if resolveErr != nil {
			return response, resolveErr
		}
		*setting.target = value
	}
	response.PresenceMode, err = rs.Settings.ResolveStringForTenant(settingsCtx, tenantID, configModels.KeyPresenceMode)
	return response, err
}

func (rs DeviceConfiguration) settingsContext(ctx context.Context, tenantID int64) (context.Context, error) {
	if rs.Settings == nil {
		return ctx, fmt.Errorf("device configuration requires settings service")
	}
	batch, ok := rs.Settings.(interface {
		ResolveManyForTenant(context.Context, int64, []string) (*settingsCompose.SettingsSnapshot, error)
	})
	if !ok {
		return ctx, nil
	}
	snapshot, err := batch.ResolveManyForTenant(ctx, tenantID, []string{
		configModels.KeyCheckoutRaumwechselEnabled,
		configModels.KeyCheckoutSchulhofEnabled,
		configModels.KeyCheckoutWCEnabled,
		configModels.KeyFeedbackEnabled,
		configModels.KeyStudentDailyCheckoutTime,
		configModels.KeyPresenceMode,
	})
	if err != nil {
		return ctx, err
	}
	if snapshot == nil {
		return ctx, nil
	}
	settingsCtx := tenant.WithTenantID(ctx, tenantID)
	return settingsCompose.WithSettingsSnapshot(settingsCtx, snapshot), nil
}

// roomHistorySettingsReader is the slice of the settings service the room
// history resolves its toggle and window with.
type roomHistorySettingsReader interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// NewRoomHistorySettings resolves whether the room history is shown and how
// many days it spans, prefetching both settings in one read.
func NewRoomHistorySettings(settings roomHistorySettingsReader, logger *slog.Logger, prefetch func(context.Context, any, ...string) context.Context) func(context.Context) (context.Context, bool, int, error) {
	if settings == nil {
		panic("room history: tenant settings are required")
	}
	return func(ctx context.Context) (context.Context, bool, int, error) {
		ctx = prefetch(ctx, settings, configModels.KeyAttendanceLogEnabled, configModels.KeyRoomDetailVisibleDays)
		enabled, err := settings.ResolveBool(ctx, configModels.KeyAttendanceLogEnabled)
		if err != nil {
			return ctx, false, 0, err
		}
		days, err := settings.ResolveInt(ctx, configModels.KeyRoomDetailVisibleDays)
		return ctx, enabled, days, err
	}
}

// SchoolSetupSettings resolves the two settings the school setup steps
// depend on in the request's tenant.
type SchoolSetupSettings struct {
	Settings interface {
		ResolveString(ctx context.Context, key string) (string, error)
	}
}

func (s SchoolSetupSettings) PresenceMode(ctx context.Context) (string, error) {
	return s.Settings.ResolveString(ctx, configModels.KeyPresenceMode)
}

func (s SchoolSetupSettings) GroupMode(ctx context.Context) (string, error) {
	return s.Settings.ResolveString(ctx, configModels.KeyGroupMode)
}
