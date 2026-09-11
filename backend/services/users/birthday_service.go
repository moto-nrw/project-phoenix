package users

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// BirthdayService resolves who is celebrating a birthday and who may see it
// (#1542).
//
// It is deliberately its own service rather than another method group on
// PersonService: the read spans two populations (children and staff), it is
// governed by two tenant settings plus a per-person opt-out, and it is polled
// by every open dashboard. Keeping that policy in one place is what stops the
// next caller from querying the tables directly and skipping the opt-out.
type BirthdayService interface {
	// Overview returns the birthdays the dashboard may show for the caller,
	// already filtered by the school's settings, the personal opt-outs, and the
	// caller's student data scope. It is safe to render as-is.
	//
	// visibility carries the caller's student-data decision (admin or verified
	// staff, #2329), resolved by the handler via common.DetermineStudentAccess. A nil visibility means "no child is
	// visible": the display must fail closed rather than publish the whole
	// school to a caller nobody classified.
	Overview(ctx context.Context, visibility StudentVisibility) (*BirthdayOverview, error)

	// GetOptOut reports whether the staff member behind the given account has
	// removed themselves from the display.
	GetOptOut(ctx context.Context, accountID int64) (bool, error)

	// SetOptOut sets the display opt-out for the staff member behind the given
	// account. Self-service only: the caller's own account id comes from the
	// JWT, never from the request body.
	SetOptOut(ctx context.Context, accountID int64, optOut bool) error

	// ListStaffBirthdays returns every staff member with a stored birth date,
	// restricted to the given birth months (empty = all months), sorted as a
	// calendar. Backs the administrative Geburtstagsliste export.
	ListStaffBirthdays(ctx context.Context, months map[time.Month]bool) ([]userModels.BirthdayEntry, error)
}

// StudentVisibility decides whether the caller may see children at all
// (admin or verified staff, #2329). It is the method set of
// usercontext.StudentAccessContext, declared here as a local interface because
// services/usercontext transitively imports this package — the policy still
// lives in exactly one place, this is only how it travels.
type StudentVisibility interface {
	HasFullAccess() bool
}

// BirthdayCelebration is one person celebrating on one concrete calendar day.
// Date is the day the celebration is SHOWN on, which is not always the stored
// birth date: a Monday carries the weekend's birthdays, and 29 February falls
// on 1 March in a common year.
type BirthdayCelebration struct {
	Kind        userModels.BirthdayKind
	ID          int64
	Name        string
	GroupName   string
	SchoolClass string
	Date        timezone.Date
	// Age is the age reached, in completed years. Only children carry it: a
	// child turning seven is what the group celebrates, while publishing a
	// colleague's age to the whole team is exactly what the opt-out exists to
	// prevent. Zero means "not disclosed".
	Age int
	// IsToday separates today's birthdays from the weekend ones the Monday
	// view carries along, so the UI can label them without re-deriving it.
	IsToday bool
}

// BirthdayOverview is the dashboard payload.
type BirthdayOverview struct {
	// Enabled mirrors the school setting. A disabled display returns an empty,
	// enabled=false overview instead of an error, so the dashboard simply
	// renders nothing.
	Enabled      bool
	IncludeStaff bool
	Today        timezone.Date
	Celebrations []BirthdayCelebration
}

// BirthdayServiceDependencies wires the service.
type BirthdayServiceDependencies struct {
	StudentRepo     userModels.StudentRepository
	StaffRepo       userModels.StaffRepository
	PersonRepo      userModels.PersonRepository
	SettingsService config.SettingsService
	Logger          *slog.Logger
	// Now is injectable so tests can pin a weekday; nil means time.Now.
	Now func() time.Time
}

type birthdayService struct {
	BirthdayServiceDependencies
}

// NewBirthdayService creates the birthday service.
func NewBirthdayService(deps BirthdayServiceDependencies) BirthdayService {
	return &birthdayService{BirthdayServiceDependencies: deps}
}

func (s *birthdayService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *birthdayService) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *birthdayService) Overview(ctx context.Context, visibility StudentVisibility) (*BirthdayOverview, error) {
	today := timezone.DateFromTime(s.now())
	overview := &BirthdayOverview{Today: today}

	enabled, err := s.SettingsService.ResolveBool(ctx, configModels.KeyBirthdayDisplayEnabled)
	if err != nil {
		return nil, &UsersError{Op: "resolve birthday display setting", Err: err}
	}
	overview.Enabled = enabled
	if !enabled {
		return overview, nil
	}

	includeStaff, err := s.SettingsService.ResolveBool(ctx, configModels.KeyBirthdayDisplayIncludeStaff)
	if err != nil {
		return nil, &UsersError{Op: "resolve staff birthday setting", Err: err}
	}
	overview.IncludeStaff = includeStaff

	dates := celebrationDates(today)
	byMonthDay := make(map[userModels.MonthDay]timezone.Date, len(dates)*2)
	days := make([]userModels.MonthDay, 0, len(dates)*2)
	for _, date := range dates {
		for _, day := range monthDaysFor(date) {
			if _, seen := byMonthDay[day]; seen {
				continue
			}
			byMonthDay[day] = date
			days = append(days, day)
		}
	}

	students, err := s.StudentRepo.FindBirthdaysOn(ctx, days)
	if err != nil {
		return nil, &UsersError{Op: "find student birthdays", Err: err}
	}
	// A birthday row is student data: name, group, class and the age reached.
	// It therefore obeys the same read boundary as every other child list —
	// admin or verified staff (#2329) — instead of handing the whole school to
	// anyone holding users:read.
	entries := visibleStudents(students, visibility)

	if includeStaff {
		staff, err := s.StaffRepo.FindBirthdaysOn(ctx, days)
		if err != nil {
			return nil, &UsersError{Op: "find staff birthdays", Err: err}
		}
		entries = append(entries, staff...)
	}

	overview.Celebrations = buildCelebrations(entries, byMonthDay, today)

	s.logger().Debug("birthday overview resolved",
		"date", today.String(),
		"include_staff", includeStaff,
		"count", len(overview.Celebrations),
	)

	return overview, nil
}

// visibleStudents drops every child the caller may not see. A nil visibility
// removes all of them: an unclassified caller gets an empty card, never the
// whole school.
func visibleStudents(entries []userModels.BirthdayEntry, visibility StudentVisibility) []userModels.BirthdayEntry {
	visible := make([]userModels.BirthdayEntry, 0, len(entries))
	if visibility == nil || !visibility.HasFullAccess() {
		return visible
	}
	return append(visible, entries...)
}

// celebrationDates are the calendar days one dashboard view speaks for.
//
// Any day speaks for itself. A Monday additionally carries the Saturday and
// Sunday before it: nobody sees the dashboard on the weekend, so without this
// a child born on a Saturday would silently never be celebrated (#1542).
func celebrationDates(today timezone.Date) []timezone.Date {
	dates := []timezone.Date{today}
	if today.Weekday() == time.Monday {
		dates = append(dates, today.AddDays(-2), today.AddDays(-1))
	}
	return dates
}

// monthDaysFor maps a shown date to the recurring birth days it stands for.
//
// Normally that is the date's own (month, day). The exception is 1 March in a
// common year, which also stands for 29 February — otherwise a person born on
// a leap day would appear on the dashboard once every four years. 1 March
// follows the BGB convention (the day after the last day of February), not the
// "28 February" reading some calendars use.
func monthDaysFor(date timezone.Date) []userModels.MonthDay {
	days := []userModels.MonthDay{{Month: date.Month(), Day: date.Day()}}
	if date.Month() == time.March && date.Day() == 1 && !isLeapYear(date.Year()) {
		days = append(days, userModels.MonthDay{Month: time.February, Day: 29})
	}
	return days
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func buildCelebrations(entries []userModels.BirthdayEntry, byMonthDay map[userModels.MonthDay]timezone.Date, today timezone.Date) []BirthdayCelebration {
	celebrations := make([]BirthdayCelebration, 0, len(entries))
	for _, entry := range entries {
		shownOn, ok := byMonthDay[userModels.MonthDayOf(entry.Birthday)]
		if !ok {
			// The query only returns the days we asked for; a miss would mean
			// a mismatch between filter and mapping, and inventing a date here
			// would hide it.
			continue
		}
		celebration := BirthdayCelebration{
			Kind:        entry.Kind,
			ID:          entry.ID,
			Name:        entry.FullName(),
			GroupName:   entry.GroupName,
			SchoolClass: entry.SchoolClass,
			Date:        shownOn,
			IsToday:     shownOn == today,
		}
		if entry.Kind == userModels.BirthdayKindStudent {
			celebration.Age = shownOn.Year() - entry.Birthday.Year()
		}
		celebrations = append(celebrations, celebration)
	}

	sort.SliceStable(celebrations, func(i, j int) bool {
		a, b := celebrations[i], celebrations[j]
		if a.Date != b.Date {
			// Today first, then the weekend days in calendar order behind it.
			return a.Date.After(b.Date)
		}
		if a.Kind != b.Kind {
			return a.Kind == userModels.BirthdayKindStudent
		}
		return a.Name < b.Name
	})

	return celebrations
}

func (s *birthdayService) GetOptOut(ctx context.Context, accountID int64) (bool, error) {
	staff, err := s.resolveOwnStaff(ctx, accountID)
	if err != nil {
		return false, err
	}
	return staff.BirthdayDisplayOptOut, nil
}

func (s *birthdayService) SetOptOut(ctx context.Context, accountID int64, optOut bool) error {
	staff, err := s.resolveOwnStaff(ctx, accountID)
	if err != nil {
		return err
	}
	if staff.BirthdayDisplayOptOut == optOut {
		return nil
	}
	if err := s.StaffRepo.SetBirthdayDisplayOptOut(ctx, staff.ID, optOut); err != nil {
		return &UsersError{Op: "set birthday display opt-out", Err: err}
	}
	return nil
}

// resolveOwnStaff maps the caller's JWT account to their staff row. Callers
// pass only their own account id, so the person → staff hop is the entire
// authorization: an account without a staff row in this tenant has nothing to
// opt out of.
func (s *birthdayService) resolveOwnStaff(ctx context.Context, accountID int64) (*userModels.Staff, error) {
	person, err := s.PersonRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, &UsersError{Op: "resolve person for account", Err: ErrStaffNotFound}
		}
		return nil, &UsersError{Op: "resolve person for account", Err: err}
	}
	if person == nil {
		return nil, &UsersError{Op: "resolve person for account", Err: ErrStaffNotFound}
	}
	staff, err := s.StaffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, &UsersError{Op: "resolve staff for person", Err: ErrStaffNotFound}
		}
		return nil, &UsersError{Op: "resolve staff for person", Err: err}
	}
	if staff == nil {
		return nil, &UsersError{Op: "resolve staff for person", Err: ErrStaffNotFound}
	}
	return staff, nil
}

func (s *birthdayService) ListStaffBirthdays(ctx context.Context, months map[time.Month]bool) ([]userModels.BirthdayEntry, error) {
	entries, err := s.StaffRepo.ListBirthdaysForExport(ctx)
	if err != nil {
		return nil, &UsersError{Op: "list staff birthdays", Err: err}
	}

	filtered := entries[:0]
	for _, entry := range entries {
		if len(months) > 0 && !months[entry.Birthday.Month()] {
			continue
		}
		filtered = append(filtered, entry)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		if a.Birthday.Month() != b.Birthday.Month() {
			return a.Birthday.Month() < b.Birthday.Month()
		}
		if a.Birthday.Day() != b.Birthday.Day() {
			return a.Birthday.Day() < b.Birthday.Day()
		}
		return a.FullName() < b.FullName()
	})

	return filtered, nil
}
