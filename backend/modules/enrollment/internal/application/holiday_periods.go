package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Holiday periods of a Träger (#4659). The caller runs every operation in the
// Träger transaction of organizationID; the store refuses any other unit. The
// calendar binding authorizes host schools before reading their calendars;
// the database also enforces membership on writes.

// Limits of one period. They bound the request, not the business: a holiday
// season has a handful of weeks with two or three time models each.
const (
	holidayNameMaxLength          = 120
	holidayNotesMaxLength         = 2000
	holidayTimeModelNameMaxLength = 80
	holidayMaxWeeks               = 26
	holidayMaxTimeModelsPerWeek   = 10
	holidayMaxWeekDays            = 7
	holidayMaxPriceCents          = 1_000_000
)

var errHolidayPeriodRequired = errors.New("enrollment: the Träger is required")

// HolidayPeriodStore persists the periods of one Träger.
type HolidayPeriodStore interface {
	ListHolidayPeriods(ctx context.Context, organizationID int64) ([]enrollment.HolidayPeriod, error)
	// FindHolidayPeriod reads one period; lock holds its row for an update.
	FindHolidayPeriod(ctx context.Context, organizationID, periodID int64, lock bool) (enrollment.HolidayPeriod, bool, error)
	InsertHolidayPeriod(ctx context.Context, organizationID int64, input enrollment.HolidayPeriodInput) (int64, error)
	// UpdateHolidayPeriod writes the period and makes its weeks and time
	// models those of input: kept by ID, added without one, removed when
	// left out.
	UpdateHolidayPeriod(ctx context.Context, organizationID, periodID int64, input enrollment.HolidayPeriodInput, effectiveOn calendar.Date) error
	PublishHolidayPeriod(ctx context.Context, organizationID, periodID int64) error
	// HolidayWeeksBookedTogether lists the pairs of the period's weeks one
	// child has booked, those of a cancelled registration included: reopening
	// it brings its bookings back.
	HolidayWeeksBookedTogether(ctx context.Context, organizationID, periodID int64) ([][2]int64, error)
	// HolidayChildBookings lists the bookings of each child of the period
	// whose registration is not cancelled, one list per child (#6506). A
	// cancellation is final, so its bookings take no care day.
	HolidayChildBookings(ctx context.Context, organizationID, periodID int64) ([][]enrollment.HolidayBookingInput, error)
}

// HolidayPeriods serves enrollment.HolidayPeriods.
type HolidayPeriods struct {
	store HolidayPeriodStore
	forms HolidayFormStore
	today func() calendar.Date
	// calendar names the host schools' days without care (#4702). Without
	// it the periods are read unmarked and no week is written.
	calendar enrollment.HolidaySchoolCalendar
}

var _ enrollment.HolidayPeriods = (*HolidayPeriods)(nil)

// NewHolidayPeriods builds the holiday period commands over store; forms
// receives the first version of a new period's form (#5198), schoolCalendar
// names the days without care of the host schools (#4702).
func NewHolidayPeriods(store HolidayPeriodStore, forms HolidayFormStore, today func() calendar.Date, schoolCalendar enrollment.HolidaySchoolCalendar) *HolidayPeriods {
	if store == nil || forms == nil {
		panic("enrollment: the holiday period store and the form store are required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &HolidayPeriods{store: store, forms: forms, today: today, calendar: schoolCalendar}
}

// ListHolidayPeriods returns every period of the Träger, newest first.
func (h *HolidayPeriods) ListHolidayPeriods(ctx context.Context, organizationID int64) ([]enrollment.HolidayPeriod, error) {
	if organizationID <= 0 {
		return nil, errHolidayPeriodRequired
	}
	periods, err := h.store.ListHolidayPeriods(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("enrollment: list holiday periods: %w", err)
	}
	return periods, nil
}

// HolidayPeriod returns one period of the Träger.
func (h *HolidayPeriods) HolidayPeriod(ctx context.Context, organizationID, periodID int64) (enrollment.HolidayPeriod, error) {
	if err := holidayPeriodKey(organizationID, periodID); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	return h.markedPeriod(ctx, organizationID, periodID)
}

// markedPeriod reads the stored period with its days without care.
func (h *HolidayPeriods) markedPeriod(ctx context.Context, organizationID, periodID int64) (enrollment.HolidayPeriod, error) {
	period, err := foundHolidayPeriod(h.store.FindHolidayPeriod(ctx, organizationID, periodID, false))
	if err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	return withHolidayNoCareDays(ctx, h.calendar, period)
}

// CreateHolidayPeriod adds a draft period with version 1 of its form, the
// standard template, and returns it as stored.
func (h *HolidayPeriods) CreateHolidayPeriod(ctx context.Context, organizationID int64, input enrollment.HolidayPeriodInput) (enrollment.HolidayPeriod, error) {
	if organizationID <= 0 {
		return enrollment.HolidayPeriod{}, errHolidayPeriodRequired
	}
	input = normalizeHolidayPeriod(input)
	if err := validateHolidayPeriod(input, enrollment.HolidayPeriod{}); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	if err := refuseHolidayWeeksWithoutCare(ctx, h.calendar, input.Weeks, enrollment.HolidayPeriod{}); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	periodID, err := h.store.InsertHolidayPeriod(ctx, organizationID, input)
	if err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	if _, err := h.forms.InsertHolidayPeriodForm(ctx, organizationID, periodID, 0, enrollment.HolidayFormTemplate(), nil, enrollment.HolidayAddressOff); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	return h.markedPeriod(ctx, organizationID, periodID)
}

// UpdateHolidayPeriod replaces the period's fields, weeks and time models
// under the lock of its row and returns it as stored. A published period
// stays published.
func (h *HolidayPeriods) UpdateHolidayPeriod(ctx context.Context, organizationID, periodID int64, input enrollment.HolidayPeriodInput) (enrollment.HolidayPeriod, error) {
	if err := holidayPeriodKey(organizationID, periodID); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	current, err := foundHolidayPeriod(h.store.FindHolidayPeriod(ctx, organizationID, periodID, true))
	if err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	input = keepArchivedHolidayTimeModels(normalizeHolidayPeriod(input), current)
	input = keepHolidayBookingUnits(input, current)
	if err := validateHolidayPeriod(input, current); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	if current.Status == enrollment.HolidayPeriodPublished && !holidayInputComplete(input) {
		return enrollment.HolidayPeriod{}, enrollment.ErrHolidayPeriodIncomplete
	}
	if err := refuseBookedUnitChange(input.Weeks, current); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	if err := h.refuseBookedTogether(ctx, organizationID, periodID, input.Weeks); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	if err := refuseHolidayWeeksWithoutCare(ctx, h.calendar, input.Weeks, current); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	if err := h.refuseRaisedDayLimit(ctx, organizationID, periodID, input, current); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	if err := h.store.UpdateHolidayPeriod(ctx, organizationID, periodID, input, h.today()); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	return h.markedPeriod(ctx, organizationID, periodID)
}

// refuseBookedTogether keeps a child from ending up in two weeks that share a
// day: a stored week may not move onto a week the same child has booked. The
// caller holds the lock of the period.
func (h *HolidayPeriods) refuseBookedTogether(ctx context.Context, organizationID, periodID int64, weeks []enrollment.HolidayWeekInput) error {
	overlapping := map[[2]int64]int{}
	for i, week := range weeks {
		for _, other := range weeks[:i] {
			if week.ID != 0 && other.ID != 0 && holidayWeeksOverlap(week, other) {
				overlapping[holidayWeekPair(week.ID, other.ID)] = i
			}
		}
	}
	if len(overlapping) == 0 {
		return nil
	}
	together, err := h.store.HolidayWeeksBookedTogether(ctx, organizationID, periodID)
	if err != nil {
		return fmt.Errorf("enrollment: read weeks booked together: %w", err)
	}
	for _, pair := range together {
		if index, found := overlapping[holidayWeekPair(pair[0], pair[1])]; found {
			return &enrollment.HolidayWeeksBookedTogetherRefusal{Week: index}
		}
	}
	return nil
}

func holidayWeekPair(a, b int64) [2]int64 {
	if b < a {
		return [2]int64{b, a}
	}
	return [2]int64{a, b}
}

// PublishHolidayPeriod offers a complete period for registration. Publishing
// a published period changes nothing.
func (h *HolidayPeriods) PublishHolidayPeriod(ctx context.Context, organizationID, periodID int64) (enrollment.HolidayPeriod, error) {
	if err := holidayPeriodKey(organizationID, periodID); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	current, err := foundHolidayPeriod(h.store.FindHolidayPeriod(ctx, organizationID, periodID, true))
	if err != nil || current.Status == enrollment.HolidayPeriodPublished {
		return current, err
	}
	if !holidayPeriodComplete(current) {
		return enrollment.HolidayPeriod{}, enrollment.ErrHolidayPeriodIncomplete
	}
	if err := h.store.PublishHolidayPeriod(ctx, organizationID, periodID); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	return h.markedPeriod(ctx, organizationID, periodID)
}

// holidayPeriodKey checks the Träger and the period a caller names.
func holidayPeriodKey(organizationID, periodID int64) error {
	if organizationID <= 0 {
		return errHolidayPeriodRequired
	}
	if periodID <= 0 {
		return enrollment.ErrHolidayPeriodNotFound
	}
	return nil
}

// foundHolidayPeriod turns the store's answer into the period or its error.
func foundHolidayPeriod(period enrollment.HolidayPeriod, found bool, err error) (enrollment.HolidayPeriod, error) {
	if err != nil {
		return enrollment.HolidayPeriod{}, fmt.Errorf("enrollment: read holiday period: %w", err)
	}
	if !found {
		return enrollment.HolidayPeriod{}, enrollment.ErrHolidayPeriodNotFound
	}
	return period, nil
}

// holidayPeriodComplete reports a period with at least one week, each with
// at least one time model that is not archived.
func holidayPeriodComplete(period enrollment.HolidayPeriod) bool {
	if len(period.Weeks) == 0 {
		return false
	}
	for _, week := range period.Weeks {
		if week.BookingUnit == enrollment.HolidayBookingUnitDay && len(enrollment.HolidayWeekCareDays(week.StartsOn, week.EndsOn)) == 0 {
			return false
		}
		if !slices.ContainsFunc(week.TimeModels, func(model enrollment.HolidayTimeModel) bool { return !model.Archived }) {
			return false
		}
	}
	return true
}

// holidayInputComplete applies holidayPeriodComplete to a written period: a
// published period stays complete through every change.
func holidayInputComplete(input enrollment.HolidayPeriodInput) bool {
	if len(input.Weeks) == 0 {
		return false
	}
	for _, week := range input.Weeks {
		if !slices.ContainsFunc(week.TimeModels, func(model enrollment.HolidayTimeModelInput) bool { return !model.Archived }) {
			return false
		}
	}
	return true
}

// keepArchivedHolidayTimeModels marks every written time model the period
// already archived as archived: an archived time model is never restored and
// counts for no week (#5994).
func keepArchivedHolidayTimeModels(input enrollment.HolidayPeriodInput, current enrollment.HolidayPeriod) enrollment.HolidayPeriodInput {
	archived := map[int64]bool{}
	for _, week := range current.Weeks {
		for _, model := range week.TimeModels {
			archived[model.ID] = model.Archived
		}
	}
	for _, week := range input.Weeks {
		for j := range week.TimeModels {
			if archived[week.TimeModels[j].ID] {
				week.TimeModels[j].Archived = true
			}
		}
	}
	return input
}

func normalizeHolidayPeriod(input enrollment.HolidayPeriodInput) enrollment.HolidayPeriodInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Notes = strings.TrimSpace(input.Notes)
	weeks := make([]enrollment.HolidayWeekInput, len(input.Weeks))
	for i, week := range input.Weeks {
		models := make([]enrollment.HolidayTimeModelInput, len(week.TimeModels))
		for j, model := range week.TimeModels {
			model.Name = strings.TrimSpace(model.Name)
			model.StartsAt = normalizeWallClock(model.StartsAt)
			model.EndsAt = normalizeWallClock(model.EndsAt)
			models[j] = model
		}
		week.TimeModels = models
		weeks[i] = week
	}
	input.Weeks = weeks
	return input
}

// normalizeWallClock turns "8:00" or "08:00" into "08:00"; anything else
// stays as given and fails validation.
func normalizeWallClock(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return value
	}
	return parsed.Format("15:04")
}

// validateHolidayPeriod checks input against the rules of a period. current
// is the stored period on an update and the zero period on a create: only
// its weeks and time models may be named by ID.
func validateHolidayPeriod(input enrollment.HolidayPeriodInput, current enrollment.HolidayPeriod) error {
	errs := validation.Errors{}
	if input.Name == "" {
		errs["name"] = errors.New("name is required")
	} else if utf8.RuneCountInString(input.Name) > holidayNameMaxLength {
		errs["name"] = fmt.Errorf("name is longer than %d characters", holidayNameMaxLength)
	}
	if utf8.RuneCountInString(input.Notes) > holidayNotesMaxLength {
		errs["notes"] = fmt.Errorf("notes are longer than %d characters", holidayNotesMaxLength)
	}
	validateRegistrationWindow(input, errs)
	if limit := input.MaxDaysPerChild; limit != nil && (*limit < 1 || *limit > enrollment.HolidayMaxDaysPerChildMax) {
		errs["max_days_per_child"] = fmt.Errorf("must be between 1 and %d", enrollment.HolidayMaxDaysPerChildMax)
	}
	if len(input.Weeks) > holidayMaxWeeks {
		errs["weeks"] = fmt.Errorf("a period has at most %d weeks", holidayMaxWeeks)
	} else if weekErrs := validateHolidayWeeks(input.Weeks, current); len(weekErrs) > 0 {
		errs["weeks"] = weekErrs
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func validateRegistrationWindow(input enrollment.HolidayPeriodInput, errs validation.Errors) {
	if input.RegistrationOpensOn.IsZero() {
		errs["registration_opens_on"] = errors.New("registration start is required")
	}
	if input.RegistrationClosesOn.IsZero() {
		errs["registration_closes_on"] = errors.New("registration end is required")
	}
	if !input.RegistrationOpensOn.IsZero() && !input.RegistrationClosesOn.IsZero() &&
		input.RegistrationClosesOn.Before(input.RegistrationOpensOn) {
		errs["registration_closes_on"] = errors.New("registration end is before its start")
	}
}

func validateHolidayWeeks(weeks []enrollment.HolidayWeekInput, current enrollment.HolidayPeriod) validation.Errors {
	errs := validation.Errors{}
	known := map[int64]enrollment.HolidayWeek{}
	for _, week := range current.Weeks {
		known[week.ID] = week
	}
	seen := map[int64]bool{}
	overlaps := overlappingHolidayWeeks(weeks)
	for i, week := range weeks {
		weekErrs := validateHolidayWeek(week)
		if j, overlapping := overlaps[i]; overlapping {
			weekErrs["starts_on"] = fmt.Errorf("the week overlaps week %d", j+1)
		}
		if week.ID != 0 {
			if _, ok := known[week.ID]; !ok || seen[week.ID] {
				weekErrs["id"] = errors.New("the week is not a week of this period")
			}
			seen[week.ID] = true
		}
		if modelErrs := validateHolidayTimeModels(week.TimeModels, known[week.ID]); len(modelErrs) > 0 {
			weekErrs["time_models"] = modelErrs
		}
		if len(weekErrs) > 0 {
			errs[fmt.Sprint(i)] = weekErrs
		}
	}
	return errs
}

func validateHolidayWeek(week enrollment.HolidayWeekInput) validation.Errors {
	errs := validation.Errors{}
	if week.HostSchoolID <= 0 {
		errs["host_school_id"] = errors.New("the host school is required")
	}
	if week.StartsOn.IsZero() {
		errs["starts_on"] = errors.New("the first day is required")
	}
	if week.EndsOn.IsZero() {
		errs["ends_on"] = errors.New("the last day is required")
	}
	if week.StartsOn.IsZero() || week.EndsOn.IsZero() {
		return errs
	}
	if week.EndsOn.Before(week.StartsOn) {
		errs["ends_on"] = errors.New("the last day is before the first day")
	} else if week.StartsOn.DaysUntil(week.EndsOn) >= holidayMaxWeekDays {
		errs["ends_on"] = fmt.Errorf("a week has at most %d days", holidayMaxWeekDays)
	}
	if week.BookingUnit == enrollment.HolidayBookingUnitDay && len(enrollment.HolidayWeekCareDays(week.StartsOn, week.EndsOn)) == 0 {
		errs["ends_on"] = errors.New("a week booked by days needs at least one weekday")
	}
	if len(week.TimeModels) > holidayMaxTimeModelsPerWeek {
		errs["time_models"] = fmt.Errorf("a week has at most %d time models", holidayMaxTimeModelsPerWeek)
	}
	switch week.BookingUnit {
	case "", enrollment.HolidayBookingUnitWeek, enrollment.HolidayBookingUnitDay:
	default:
		errs["booking_unit"] = errors.New("the booking unit is week or day")
	}
	return errs
}

// refuseBookedUnitChange keeps the bookings of a week valid (#4701): a week
// that bookings name keeps its booking unit, and a week by days keeps its
// dates, which its bookings' days lie in. A week is booked when a booking
// or a rebooking names one of its time models.
func refuseBookedUnitChange(weeks []enrollment.HolidayWeekInput, current enrollment.HolidayPeriod) error {
	stored := map[int64]enrollment.HolidayWeek{}
	for _, week := range current.Weeks {
		stored[week.ID] = week
	}
	for _, week := range weeks {
		before, ok := stored[week.ID]
		if week.ID == 0 || !ok || !slices.ContainsFunc(before.TimeModels, func(model enrollment.HolidayTimeModel) bool { return model.Protected }) {
			continue
		}
		unit := week.BookingUnit
		if unit == "" {
			unit = enrollment.HolidayBookingUnitWeek
		}
		moved := before.StartsOn != week.StartsOn || before.EndsOn != week.EndsOn
		if unit != before.BookingUnit || (unit == enrollment.HolidayBookingUnitDay && moved) {
			return fmt.Errorf("%w: %w", enrollment.ErrHolidayWeekBooked, enrollment.ErrHolidayBookingUnitBooked)
		}
	}
	return nil
}

// overlappingHolidayWeeks maps the index of each valid week that shares a day
// with an earlier valid week of the same school to that earlier week's index.
// Weeks at different schools may run in parallel (#5689).
func overlappingHolidayWeeks(weeks []enrollment.HolidayWeekInput) map[int]int {
	overlaps := map[int]int{}
	for i, week := range weeks {
		if !holidayWeekDatesValid(week) {
			continue
		}
		for j := range i {
			other := weeks[j]
			if week.HostSchoolID == other.HostSchoolID && holidayWeekDatesValid(other) && holidayWeeksOverlap(week, other) {
				overlaps[i] = j
				break
			}
		}
	}
	return overlaps
}

func holidayWeeksOverlap(a, b enrollment.HolidayWeekInput) bool {
	return holidayDaysOverlap(a.StartsOn, a.EndsOn, b.StartsOn, b.EndsOn)
}

func holidayDaysOverlap(startsOn, endsOn, otherStartsOn, otherEndsOn calendar.Date) bool {
	return !startsOn.After(otherEndsOn) && !otherStartsOn.After(endsOn)
}

func holidayWeekDatesValid(week enrollment.HolidayWeekInput) bool {
	return !week.StartsOn.IsZero() && !week.EndsOn.IsZero() && !week.EndsOn.Before(week.StartsOn)
}

func validateHolidayTimeModels(models []enrollment.HolidayTimeModelInput, current enrollment.HolidayWeek) validation.Errors {
	errs := validation.Errors{}
	known := map[int64]bool{}
	for _, model := range current.TimeModels {
		known[model.ID] = true
	}
	seenIDs := map[int64]bool{}
	var names []string
	for i, model := range models {
		modelErrs := validateHolidayTimeModel(model)
		if model.ID != 0 && (!known[model.ID] || seenIDs[model.ID]) {
			modelErrs["id"] = errors.New("the time model is not a time model of this week")
		}
		if model.ID == 0 && model.Archived {
			modelErrs["archived"] = errors.New("only a stored time model can be archived")
		}
		seenIDs[model.ID] = true
		name := strings.ToLower(model.Name)
		if name != "" && slices.Contains(names, name) {
			modelErrs["name"] = errors.New("the week already has a time model with this name")
		}
		names = append(names, name)
		if len(modelErrs) > 0 {
			errs[fmt.Sprint(i)] = modelErrs
		}
	}
	return errs
}

func validateHolidayTimeModel(model enrollment.HolidayTimeModelInput) validation.Errors {
	errs := validation.Errors{}
	if model.Name == "" {
		errs["name"] = errors.New("name is required")
	} else if utf8.RuneCountInString(model.Name) > holidayTimeModelNameMaxLength {
		errs["name"] = fmt.Errorf("name is longer than %d characters", holidayTimeModelNameMaxLength)
	}
	starts, startErr := time.Parse("15:04", model.StartsAt)
	if startErr != nil {
		errs["starts_at"] = errors.New("start time must be HH:MM")
	}
	ends, endErr := time.Parse("15:04", model.EndsAt)
	if endErr != nil {
		errs["ends_at"] = errors.New("end time must be HH:MM")
	}
	if startErr == nil && endErr == nil && !starts.Before(ends) {
		errs["ends_at"] = errors.New("end time is not after the start time")
	}
	if model.PriceCents < 0 || model.PriceCents > holidayMaxPriceCents {
		errs["price_cents"] = fmt.Errorf("price must be between 0 and %d cent", holidayMaxPriceCents)
	}
	return errs
}

// keepHolidayBookingUnits keeps the booking unit of a stored week that the
// written week does not name, so a client that does not know the unit
// leaves it as it is (#4701).
func keepHolidayBookingUnits(input enrollment.HolidayPeriodInput, current enrollment.HolidayPeriod) enrollment.HolidayPeriodInput {
	stored := map[int64]string{}
	for _, week := range current.Weeks {
		stored[week.ID] = week.BookingUnit
	}
	for i, week := range input.Weeks {
		if unit, ok := stored[week.ID]; ok && week.ID != 0 && week.BookingUnit == "" {
			input.Weeks[i].BookingUnit = unit
		}
	}
	return input
}
