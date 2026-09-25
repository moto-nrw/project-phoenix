package presence_test

import (
	"context"

	active "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
)

// presenceSettingsStub answers the presence settings questions with scripted
// values, so a test names the answers it depends on instead of the registry
// keys they are stored under. Unset answers resolve to the zero value; an
// error field fails every question, which is how the tests drive the
// settings-unavailable paths.
type presenceSettingsStub struct {
	presenceMode             string
	sickClearMode            string
	excusedClearMode         string
	attendanceEditScope      string
	operationalOverviewScope string
	inactivityTimeoutMinutes int
	// webParticipantLimitEnforced is the negated #3632 setting; the zero value
	// is the registry default (exceeding the limit on the web is allowed).
	webParticipantLimitEnforced bool

	presenceModeErr             error
	sickClearModeErr            error
	excusedClearModeErr         error
	attendanceEditScopeErr      error
	operationalOverviewScopeErr error
	webParticipantLimitErr      error

	// Answers that depend on what the test has already done take a function
	// instead of a fixed value.
	presenceModeFn     func() (string, error)
	sickClearModeFn    func() (string, error)
	excusedClearModeFn func() (string, error)
}

var _ active.SettingsResolver = presenceSettingsStub{}

func (s presenceSettingsStub) PresenceMode(context.Context) (string, error) {
	if s.presenceModeFn != nil {
		return s.presenceModeFn()
	}
	return s.presenceMode, s.presenceModeErr
}

func (s presenceSettingsStub) SickClearMode(context.Context) (string, error) {
	if s.sickClearModeFn != nil {
		return s.sickClearModeFn()
	}
	return s.sickClearMode, s.sickClearModeErr
}

func (s presenceSettingsStub) ExcusedClearMode(context.Context) (string, error) {
	if s.excusedClearModeFn != nil {
		return s.excusedClearModeFn()
	}
	return s.excusedClearMode, s.excusedClearModeErr
}

func (s presenceSettingsStub) SessionInactivityTimeoutMinutes(context.Context) (int, error) {
	return s.inactivityTimeoutMinutes, nil
}

func (s presenceSettingsStub) AttendanceEditScope(context.Context) (string, error) {
	return s.attendanceEditScope, s.attendanceEditScopeErr
}

func (s presenceSettingsStub) OperationalOverviewScope(context.Context) (string, error) {
	return s.operationalOverviewScope, s.operationalOverviewScopeErr
}

func (s presenceSettingsStub) WebParticipantLimitEnforced(context.Context) (bool, error) {
	return s.webParticipantLimitEnforced, s.webParticipantLimitErr
}

// defaultPresenceSettings answers every question with the value an
// unconfigured tenant gets. That these are the registry defaults is asserted
// where the settings ports are bound; tests that only need "a tenant that
// changed nothing" use this.
func defaultPresenceSettings() presenceSettingsStub {
	return presenceSettingsStub{
		presenceMode:             active.PresenceModeDetailed,
		sickClearMode:            active.ClearModeNextCheckin,
		excusedClearMode:         "end_of_day",
		attendanceEditScope:      active.AttendanceEditScopeOwn,
		operationalOverviewScope: active.OverviewScopeAllStaff,
		inactivityTimeoutMinutes: 30,
	}
}
