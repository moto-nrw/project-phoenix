package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar/traegercalendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Closing days of a Träger (#4702). The caller runs every operation in the
// Träger transaction of organizationID; the store refuses any other unit.
// The database decides whether a named school belongs to the Träger.

const (
	traegerClosingDayReasonMaxLength = 120
	traegerClosingDayMaxDays         = 366
	// traegerCalendarMaxDays bounds the window of the overview: a school
	// year and a little more.
	traegerCalendarMaxDays = 400
)

var errTraegerRequired = errors.New("school calendar: the Träger is required")

// TraegerClosingDayStore persists the closing days of one Träger.
type TraegerClosingDayStore interface {
	ListTraegerClosingDays(ctx context.Context, organizationID int64, from, to calendar.Date) ([]domain.TraegerClosingDay, error)
	InsertTraegerClosingDay(ctx context.Context, organizationID int64, input domain.TraegerClosingDayFields) (domain.TraegerClosingDay, error)
	UpdateTraegerClosingDay(ctx context.Context, organizationID, id int64, input domain.TraegerClosingDayFields) (domain.TraegerClosingDay, error)
	DeleteTraegerClosingDay(ctx context.Context, organizationID, id int64) error
	LockTraegerClosingDay(ctx context.Context, organizationID, id int64) (domain.TraegerClosingDay, error)
}

// TraegerClosingDays serves traegercalendar.ClosingDays.
type TraegerClosingDays struct {
	store   TraegerClosingDayStore
	schools traegercalendar.Schools
	notices traegercalendar.HolidayNotices
}

var _ traegercalendar.ClosingDays = (*TraegerClosingDays)(nil)

// NewTraegerClosingDays builds the Träger's closing days over store; schools
// names the Träger's schools with their federal state, notices tells the
// parents of the holiday bookings a saved closing day affects (#6434).
func NewTraegerClosingDays(store TraegerClosingDayStore, schools traegercalendar.Schools, notices traegercalendar.HolidayNotices) *TraegerClosingDays {
	if store == nil || schools == nil || notices == nil {
		panic("school calendar: the Träger closing day store, the Träger's schools and the holiday notices are required")
	}
	return &TraegerClosingDays{store: store, schools: schools, notices: notices}
}

// Calendar lists the Träger's closing days in [from, to], its schools
// with their federal state and the statutory holidays of each state.
func (t *TraegerClosingDays) Calendar(ctx context.Context, organizationID int64, from, to calendar.Date) (traegercalendar.Calendar, error) {
	if organizationID <= 0 {
		return traegercalendar.Calendar{}, errTraegerRequired
	}
	if from.IsZero() || to.IsZero() || to.Before(from) || from.DaysUntil(to) > traegerCalendarMaxDays {
		return traegercalendar.Calendar{}, invalidTraegerClosingDay(fmt.Sprintf("the window needs from <= to and at most %d days", traegerCalendarMaxDays))
	}
	days, err := t.store.ListTraegerClosingDays(ctx, organizationID, from, to)
	if err != nil {
		return traegercalendar.Calendar{}, err
	}
	schools, err := t.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return traegercalendar.Calendar{}, err
	}
	result := traegercalendar.Calendar{
		ClosingDays: make([]traegercalendar.ClosingDay, 0, len(days)), Schools: make([]traegercalendar.School, 0, len(schools)),
		Holidays: []traegercalendar.Holidays{},
	}
	for _, day := range days {
		result.ClosingDays = append(result.ClosingDays, traegerClosingDay(day))
	}
	seen := map[string]bool{}
	for _, school := range schools {
		region := schoolcalendar.HolidayRegion{Code: school.FederalState, Name: HolidayRegionName(school.FederalState)}
		result.Schools = append(result.Schools, traegercalendar.School{ID: school.ID, Name: school.Name, FederalState: region})
		if seen[region.Code] || !ValidHolidayRegion(region.Code) {
			continue
		}
		seen[region.Code] = true
		holidays, err := HolidaysInRange(region.Code, from.String(), to.String())
		if err != nil {
			return traegercalendar.Calendar{}, err
		}
		result.Holidays = append(result.Holidays, traegercalendar.Holidays{Region: region, Holidays: holidays})
	}
	sort.Slice(result.Holidays, func(i, j int) bool { return result.Holidays[i].Region.Code < result.Holidays[j].Region.Code })
	return result, nil
}

// CreateClosingDay adds a closing day for the named schools.
func (t *TraegerClosingDays) CreateClosingDay(ctx context.Context, organizationID int64, input traegercalendar.ClosingDayInput) (traegercalendar.ClosingDay, error) {
	return t.save(ctx, organizationID, 0, input)
}

// UpdateClosingDay replaces a closing day of the Träger.
func (t *TraegerClosingDays) UpdateClosingDay(ctx context.Context, organizationID, id int64, input traegercalendar.ClosingDayInput) (traegercalendar.ClosingDay, error) {
	if id <= 0 {
		return traegercalendar.ClosingDay{}, traegercalendar.ErrClosingDayNotFound
	}
	return t.save(ctx, organizationID, id, input)
}

// save checks input and writes it as a new closing day (id 0) or over the
// closing day id of the Träger, then tells the parents of the holiday
// bookings it affects in the same Träger transaction (#6434). A refused
// notice fails the save, so the caller's transaction rolls the write back.
func (t *TraegerClosingDays) save(ctx context.Context, organizationID, id int64, input traegercalendar.ClosingDayInput) (traegercalendar.ClosingDay, error) {
	if organizationID <= 0 {
		return traegercalendar.ClosingDay{}, errTraegerRequired
	}
	input, err := t.checkedInput(ctx, organizationID, input)
	if err != nil {
		return traegercalendar.ClosingDay{}, err
	}
	fields := domain.TraegerClosingDayFields{StartDate: input.StartDate, EndDate: input.EndDate, Reason: input.Reason, SchoolIDs: input.SchoolIDs}
	var day traegercalendar.ClosingDay
	if id == 0 {
		day, err = storedTraegerClosingDay(t.store.InsertTraegerClosingDay(ctx, organizationID, fields))
	} else {
		previous, lockErr := t.store.LockTraegerClosingDay(ctx, organizationID, id)
		if lockErr != nil {
			return traegercalendar.ClosingDay{}, traegerClosingDayError(lockErr)
		}
		input.ClosingDayID = id
		input.Previous = &traegercalendar.ClosingDaySpan{StartDate: previous.StartDate, EndDate: previous.EndDate, SchoolIDs: previous.SchoolIDs}
		day, err = storedTraegerClosingDay(t.store.UpdateTraegerClosingDay(ctx, organizationID, id, fields))
	}
	if err != nil {
		return traegercalendar.ClosingDay{}, err
	}
	if day.NotifiedFamilies, err = t.notices.NotifyHolidayBookings(ctx, organizationID, input); err != nil {
		return traegercalendar.ClosingDay{}, err
	}
	return day, nil
}

// DeleteClosingDay removes a closing day of the Träger. The named schools
// count it no longer from the next read on; the parents of the holiday
// bookings whose days open again learn it in the same Träger transaction
// (#6457), so a refused notice rolls the deletion back.
func (t *TraegerClosingDays) DeleteClosingDay(ctx context.Context, organizationID, id int64, holidayBookingsToken string) (int, error) {
	if organizationID <= 0 {
		return 0, errTraegerRequired
	}
	if id <= 0 {
		return 0, traegercalendar.ErrClosingDayNotFound
	}
	previous, err := t.store.LockTraegerClosingDay(ctx, organizationID, id)
	if err != nil {
		return 0, traegerClosingDayError(err)
	}
	if err := traegerClosingDayError(t.store.DeleteTraegerClosingDay(ctx, organizationID, id)); err != nil {
		return 0, err
	}
	return t.notices.NotifyHolidayBookings(ctx, organizationID, traegercalendar.ClosingDayInput{
		Reason: previous.Reason, HolidayBookingsToken: holidayBookingsToken, ClosingDayID: id, Deleted: true,
		Previous: &traegercalendar.ClosingDaySpan{StartDate: previous.StartDate, EndDate: previous.EndDate, SchoolIDs: previous.SchoolIDs},
	})
}

func traegerClosingDay(day domain.TraegerClosingDay) traegercalendar.ClosingDay {
	return traegercalendar.ClosingDay{ID: day.ID, StartDate: day.StartDate, EndDate: day.EndDate, Reason: day.Reason, SchoolIDs: day.SchoolIDs, UpdatedAt: day.UpdatedAt}
}

func storedTraegerClosingDay(day domain.TraegerClosingDay, err error) (traegercalendar.ClosingDay, error) {
	if err != nil {
		return traegercalendar.ClosingDay{}, traegerClosingDayError(err)
	}
	return traegerClosingDay(day), nil
}

// traegerClosingDayError names the store's refusals in the contract.
func traegerClosingDayError(err error) error {
	switch {
	case errors.Is(err, domain.ErrTraegerClosingDayNotFound):
		return traegercalendar.ErrClosingDayNotFound
	case errors.Is(err, domain.ErrTraegerClosingDaySchoolOutside):
		return traegercalendar.ErrSchoolOutside
	}
	return err
}

// checkedInput normalizes input and refuses a school that is not an
// undeleted school of the Träger before anything is written, so a refusal
// leaves the Träger transaction intact. The database trigger keeps the same
// rule for a school deleted in between.
func (t *TraegerClosingDays) checkedInput(ctx context.Context, organizationID int64, input traegercalendar.ClosingDayInput) (traegercalendar.ClosingDayInput, error) {
	input, err := normalizeTraegerClosingDay(input)
	if err != nil {
		return input, err
	}
	schools, err := t.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return input, err
	}
	for _, id := range input.SchoolIDs {
		if !slices.ContainsFunc(schools, func(school traegercalendar.SchoolState) bool { return school.ID == id }) {
			return input, traegercalendar.ErrSchoolOutside
		}
	}
	return input, nil
}

// normalizeTraegerClosingDay trims the reason, orders and deduplicates the
// schools and checks the fields.
func normalizeTraegerClosingDay(input traegercalendar.ClosingDayInput) (traegercalendar.ClosingDayInput, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	switch {
	case input.StartDate.IsZero() || input.EndDate.IsZero():
		return input, invalidTraegerClosingDay("start and end date are required")
	case input.EndDate.Before(input.StartDate):
		return input, invalidTraegerClosingDay("the end date is before the start date")
	case input.StartDate.DaysUntil(input.EndDate) >= traegerClosingDayMaxDays:
		return input, invalidTraegerClosingDay(fmt.Sprintf("a closing day range has at most %d days", traegerClosingDayMaxDays))
	case input.Reason == "":
		return input, invalidTraegerClosingDay("the reason is required")
	case utf8.RuneCountInString(input.Reason) > traegerClosingDayReasonMaxLength:
		return input, invalidTraegerClosingDay(fmt.Sprintf("the reason is longer than %d characters", traegerClosingDayReasonMaxLength))
	}
	schools := make([]int64, 0, len(input.SchoolIDs))
	for _, id := range input.SchoolIDs {
		if id <= 0 {
			return input, traegercalendar.ErrSchoolOutside
		}
		schools = append(schools, id)
	}
	slices.Sort(schools)
	input.SchoolIDs = slices.Compact(schools)
	if len(input.SchoolIDs) == 0 {
		return input, invalidTraegerClosingDay("at least one school is required")
	}
	return input, nil
}

func invalidTraegerClosingDay(detail string) error {
	return fmt.Errorf("%w: %s", traegercalendar.ErrInvalidClosingDay, detail)
}
