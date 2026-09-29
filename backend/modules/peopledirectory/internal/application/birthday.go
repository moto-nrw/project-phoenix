package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/userscontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BirthdayOverview is the dashboard payload of the birthday display.
type BirthdayOverview struct {
	Enabled      bool
	IncludeStaff bool
	Today        calendar.Date
	Celebrations []domain.BirthdayCelebration
}

// BirthdayService resolves who is celebrating a birthday and who may see it.
type BirthdayService struct {
	store    ports.BirthdayStore
	settings ports.BirthdaySettings
	logger   *slog.Logger
	now      func() time.Time
}

// NewBirthdayService creates the birthday service. now is injectable so tests
// can pin a weekday; nil means time.Now.
func NewBirthdayService(store ports.BirthdayStore, settings ports.BirthdaySettings, logger *slog.Logger, now func() time.Time) *BirthdayService {
	if store == nil || settings == nil {
		panic("people directory birthdays: store and settings are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return &BirthdayService{store: store, settings: settings, logger: logger, now: now}
}

func (s *BirthdayService) Overview(ctx context.Context, visibility domain.BirthdayVisibility) (BirthdayOverview, error) {
	today := calendar.DateFromTime(s.now())
	overview := BirthdayOverview{Today: today}

	enabled, err := s.settings.BirthdayDisplayEnabled(ctx)
	if err != nil {
		return BirthdayOverview{}, &userscontract.UsersError{Op: "resolve birthday display setting", Err: err}
	}
	overview.Enabled = enabled
	if !enabled {
		return overview, nil
	}
	includeStaff, err := s.settings.BirthdayDisplayIncludesStaff(ctx)
	if err != nil {
		return BirthdayOverview{}, &userscontract.UsersError{Op: "resolve staff birthday setting", Err: err}
	}
	overview.IncludeStaff = includeStaff

	days, byMonthDay := domain.BirthdayWindow(today)
	students, err := s.store.StudentBirthdaysOn(ctx, days)
	if err != nil {
		return BirthdayOverview{}, &userscontract.UsersError{Op: "find student birthdays", Err: err}
	}
	// A birthday row is student data and obeys the same read boundary as every
	// other child list (#2329).
	entries := domain.VisibleBirthdayStudents(students, visibility)
	if includeStaff {
		staff, err := s.store.StaffBirthdaysOn(ctx, days)
		if err != nil {
			return BirthdayOverview{}, &userscontract.UsersError{Op: "find staff birthdays", Err: err}
		}
		entries = append(entries, staff...)
	}
	overview.Celebrations = domain.BuildCelebrations(entries, byMonthDay, today)

	s.logger.Debug("birthday overview resolved",
		"date", today.String(),
		"include_staff", includeStaff,
		"count", len(overview.Celebrations),
	)
	return overview, nil
}

func (s *BirthdayService) GetOptOut(ctx context.Context, accountID int64) (bool, error) {
	staff, err := s.resolveOwnStaff(ctx, accountID)
	if err != nil {
		return false, err
	}
	return staff.OptOut, nil
}

func (s *BirthdayService) SetOptOut(ctx context.Context, accountID int64, optOut bool) error {
	staff, err := s.resolveOwnStaff(ctx, accountID)
	if err != nil {
		return err
	}
	if staff.OptOut == optOut {
		return nil
	}
	if err := s.store.SetStaffBirthdayOptOut(ctx, staff.ID, optOut); err != nil {
		return &userscontract.UsersError{Op: "set birthday display opt-out", Err: err}
	}
	s.logger.Debug("birthday display opt-out changed",
		"staff_id", staff.ID,
		"opt_out", optOut,
	)
	return nil
}

// Failures carry the users.<op> wrapper the retained service always used: the
// HTTP layer renders the error text of an unexpected failure as-is, so it is
// part of the response contract.

// resolveOwnStaff maps the caller's account to their staff row. Callers pass
// only their own account id, so the person to staff hop is the entire
// authorization.
func (s *BirthdayService) resolveOwnStaff(ctx context.Context, accountID int64) (domain.BirthdayStaff, error) {
	personID, found, err := s.store.FindPersonByAccount(ctx, accountID)
	if err != nil {
		return domain.BirthdayStaff{}, &userscontract.UsersError{Op: "resolve person for account", Err: err}
	}
	if !found {
		return domain.BirthdayStaff{}, &userscontract.UsersError{Op: "resolve person for account", Err: domain.ErrBirthdayStaffNotFound}
	}
	staff, found, err := s.store.FindStaffByPerson(ctx, personID)
	if err != nil {
		return domain.BirthdayStaff{}, &userscontract.UsersError{Op: "resolve staff for person", Err: err}
	}
	if !found {
		return domain.BirthdayStaff{}, &userscontract.UsersError{Op: "resolve staff for person", Err: domain.ErrBirthdayStaffNotFound}
	}
	return staff, nil
}

func (s *BirthdayService) ListStaffBirthdays(ctx context.Context, months map[time.Month]bool) ([]domain.BirthdayEntry, error) {
	persons, err := s.store.StaffBirthdaysForExport(ctx)
	if err != nil {
		return nil, &userscontract.UsersError{Op: "list staff birthdays", Err: err}
	}
	list := domain.StaffBirthdayList(persons, months)
	s.logger.Debug("staff birthday list resolved",
		"months", len(months),
		"count", len(list),
	)
	return list, nil
}
