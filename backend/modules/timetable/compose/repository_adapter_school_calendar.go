package compose

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The retained calendar period, closing day and dateframe repositories serve
// the legacy schedule contracts over the School Calendar owner (#2666). They
// name the schedule models, so they live with the Timetable composition that
// owns those models (#2743).

// NewPlanningCalendarPeriodUsage composes the School Calendar usage read from the
// two owners that hold calendar-period references: Enrollment (phases) and
// Timetable (planning tables), one statement each (#3124).
func NewPlanningCalendarPeriodUsage(enrollment EnrollmentPhaseQueries, planning timetable.Capability) *CalendarPeriodUsageRepository {
	return NewCalendarPeriodUsageRepository(enrollment, func(ctx context.Context) (map[int64]CalendarPeriodReferences, error) {
		references, err := planning.CountCalendarPeriodReferences(ctx)
		if err != nil {
			return nil, err
		}
		counts := make(map[int64]CalendarPeriodReferences, len(references))
		for id, entry := range references {
			counts[id] = CalendarPeriodReferences(entry)
		}
		return counts, nil
	})
}

// SchoolCalendarRepositories are the retained calendar period, closing day
// and dateframe repositories served by the School Calendar owner.
type SchoolCalendarRepositories struct {
	CalendarPeriod scheduleModels.CalendarPeriodRepository
	ClosingDay     scheduleModels.ClosingDayRepository
	Dateframe      scheduleModels.DateframeRepository
}

// NewSchoolCalendarRepositories builds the retained calendar repositories
// over the School Calendar owner; usage answers the period reference counts.
func NewSchoolCalendarRepositories(calendar schoolcalendar.Capability, usage *CalendarPeriodUsageRepository) SchoolCalendarRepositories {
	return SchoolCalendarRepositories{
		CalendarPeriod: NewCalendarPeriodRepository(calendar, usage),
		ClosingDay:     NewClosingDayRepository(calendar),
		Dateframe:      NewDateframeRepository(calendar),
	}
}

// NewCalendarPeriodRepository serves the legacy calendar period repository
// over the School Calendar owner; the reference counts come from usage
// (#2666, #3124).
func NewCalendarPeriodRepository(calendar schoolcalendar.Capability, usage *CalendarPeriodUsageRepository) scheduleModels.CalendarPeriodRepository {
	return newCalendarPeriodCalendarRepository(calendar, usage)
}

// NewClosingDayRepository serves the legacy closing day repository over the
// School Calendar owner (#2666).
func NewClosingDayRepository(calendar schoolcalendar.Capability) scheduleModels.ClosingDayRepository {
	return newClosingDayCalendarRepository(calendar)
}

// NewDateframeRepository serves the legacy dateframe repository over the
// School Calendar owner (#2666).
func NewDateframeRepository(calendar schoolcalendar.Capability) scheduleModels.DateframeRepository {
	return newDateframeCalendarRepository(calendar)
}

const opFindByID = "find by id"

// calendarError maps an owner error onto the legacy repository error shape:
// a missing row keeps the sql.ErrNoRows callers classify on, a name
// collision surfaces the domain sentinel with its driver cause intact.
func calendarError(op string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound), errors.Is(err, schoolcalendar.ErrClosingDayNotFound),
		errors.Is(err, schoolcalendar.ErrDateframeNotFound):
		return calendarNotFoundError(op)
	case errors.Is(err, schoolcalendar.ErrCalendarPeriodNameConflict):
		return calendarWrapError(op, fmt.Errorf("%w: %w", scheduleModels.ErrCalendarPeriodNameConflict, err))
	default:
		return calendarWrapError(op, err)
	}
}

// The legacy models carry the repository calendar-date type; these helpers
// move YYYY-MM-DD strings in and out of it by shape, so the composition never
// names the type itself.
func setOptionalDate[D ~string](target **D, value string) {
	if value == "" {
		*target = nil
		return
	}
	date := D(value)
	*target = &date
}

func optionalDateString[D ~string](value *D) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

// --- calendar periods ---

// calendarPeriodCalendarRepository serves the legacy
// schedule.CalendarPeriodRepository over the School Calendar capability
// (#2666). schedule.calendar_periods belongs to that owner; the reference
// counts stay with the planning owner's usage repository.
type calendarPeriodCalendarRepository struct {
	UnfilteredListing[*scheduleModels.CalendarPeriod]
	CalendarPeriodOverlapListing
	calendar schoolcalendar.Capability
	usage    *CalendarPeriodUsageRepository
}

var _ scheduleModels.CalendarPeriodRepository = calendarPeriodCalendarRepository{}

func newCalendarPeriodCalendarRepository(capability schoolcalendar.Capability, usage *CalendarPeriodUsageRepository) calendarPeriodCalendarRepository {
	repo := calendarPeriodCalendarRepository{calendar: capability, usage: usage}
	repo.UnfilteredListing.Source = repo.FindByTenantID
	repo.CalendarPeriodOverlapListing.Source = repo.listActiveOverlapping
	return repo
}

func calendarPeriodFieldsFromLegacy(period *scheduleModels.CalendarPeriod) schoolcalendar.CalendarPeriodFields {
	return schoolcalendar.CalendarPeriodFields{
		Name:            period.Name,
		PeriodType:      period.PeriodType,
		StartDate:       string(period.StartDate),
		EndDate:         string(period.EndDate),
		WeekCycleLength: period.WeekCycleLength,
		WeekCycleAnchor: optionalDateString(period.WeekCycleAnchor),
		IsActive:        period.IsActive,
	}
}

func applyCalendarPeriodToLegacy(target *scheduleModels.CalendarPeriod, value schoolcalendar.CalendarPeriod) {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.SetTenantID(value.TenantID)
	target.Name = value.Name
	target.PeriodType = value.PeriodType
	setDate(&target.StartDate, value.StartDate)
	setDate(&target.EndDate, value.EndDate)
	target.WeekCycleLength = value.WeekCycleLength
	setOptionalDate(&target.WeekCycleAnchor, value.WeekCycleAnchor)
	target.IsActive = value.IsActive
}

func toLegacyCalendarPeriod(value schoolcalendar.CalendarPeriod) *scheduleModels.CalendarPeriod {
	period := new(scheduleModels.CalendarPeriod)
	applyCalendarPeriodToLegacy(period, value)
	return period
}

func toLegacyCalendarPeriods(values []schoolcalendar.CalendarPeriod) []*scheduleModels.CalendarPeriod {
	result := make([]*scheduleModels.CalendarPeriod, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacyCalendarPeriod(value))
	}
	return result
}

// Create keeps the base repository's nil and validation gates in front of
// the owner so the messages callers assert on are unchanged.
func (r calendarPeriodCalendarRepository) Create(ctx context.Context, period *scheduleModels.CalendarPeriod) error {
	if period == nil {
		return errors.New("CalendarPeriod cannot be nil or zero value")
	}
	if err := period.Validate(); err != nil {
		return err
	}
	created, err := r.calendar.CreateCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{
		CalendarPeriodFields: calendarPeriodFieldsFromLegacy(period),
	})
	if err != nil {
		return calendarError("create", err)
	}
	applyCalendarPeriodToLegacy(period, created)
	return nil
}

func (r calendarPeriodCalendarRepository) FindByID(ctx context.Context, id any) (*scheduleModels.CalendarPeriod, error) {
	periodID, err := membershipID(id)
	if err != nil {
		return nil, calendarWrapError(opFindByID, err)
	}
	value, err := r.calendar.FindCalendarPeriod(ctx, periodID)
	if err != nil {
		return nil, calendarError(opFindByID, err)
	}
	return toLegacyCalendarPeriod(value), nil
}

func (r calendarPeriodCalendarRepository) Update(ctx context.Context, period *scheduleModels.CalendarPeriod) error {
	if period == nil {
		return errors.New("CalendarPeriod cannot be nil or zero value")
	}
	if err := period.Validate(); err != nil {
		return err
	}
	updated, err := r.calendar.UpdateCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{
		ID: period.ID, CalendarPeriodFields: calendarPeriodFieldsFromLegacy(period),
	})
	if err != nil {
		return calendarError("update", err)
	}
	applyCalendarPeriodToLegacy(period, updated)
	return nil
}

// Delete stays idempotent like the base repository it replaces; a period
// still referenced by a non-nullable foreign key keeps its driver error.
func (r calendarPeriodCalendarRepository) Delete(ctx context.Context, id any) error {
	periodID, err := membershipID(id)
	if err != nil {
		return calendarWrapError("delete", err)
	}
	err = r.calendar.DeleteCalendarPeriod(ctx, periodID)
	if errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound) {
		return nil
	}
	return calendarError("delete", err)
}

func (r calendarPeriodCalendarRepository) FindByTenantID(ctx context.Context) ([]*scheduleModels.CalendarPeriod, error) {
	values, err := r.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{})
	if err != nil {
		return nil, calendarError("find by tenant id", err)
	}
	return toLegacyCalendarPeriods(values), nil
}

func (r calendarPeriodCalendarRepository) FindActiveByTenantID(ctx context.Context) ([]*scheduleModels.CalendarPeriod, error) {
	values, err := r.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{ActiveOnly: true})
	if err != nil {
		return nil, calendarError("find active by tenant id", err)
	}
	return toLegacyCalendarPeriods(values), nil
}

// FindByName keeps the legacy contract of a nil period without error when
// the name is unknown.
func (r calendarPeriodCalendarRepository) FindByName(ctx context.Context, name string) (*scheduleModels.CalendarPeriod, error) {
	values, err := r.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{Name: name})
	if err != nil {
		return nil, calendarError("find by name", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return toLegacyCalendarPeriod(values[0]), nil
}

func (r calendarPeriodCalendarRepository) CreateIfAbsent(ctx context.Context, period *scheduleModels.CalendarPeriod) (bool, error) {
	if period == nil {
		return false, errors.New("calendar period cannot be nil")
	}
	if err := period.Validate(); err != nil {
		return false, err
	}
	created, inserted, err := r.calendar.CreateCalendarPeriodIfAbsent(ctx, schoolcalendar.CreateCalendarPeriod{
		CalendarPeriodFields: calendarPeriodFieldsFromLegacy(period),
	})
	if err != nil {
		return false, calendarError("create if absent", err)
	}
	if inserted {
		applyCalendarPeriodToLegacy(period, created)
	}
	return inserted, nil
}

func (r calendarPeriodCalendarRepository) listActiveOverlapping(ctx context.Context, periodType, start, end string, excludeID int64) ([]*scheduleModels.CalendarPeriod, error) {
	op := "find active overlapping"
	if periodType != "" {
		op = "find active overlapping by type"
	}
	values, err := r.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		ActiveOnly: true, PeriodType: periodType, OverlappingFrom: start, OverlappingTo: end, ExcludeID: excludeID,
	})
	if err != nil {
		return nil, calendarError(op, err)
	}
	return toLegacyCalendarPeriods(values), nil
}

func (r calendarPeriodCalendarRepository) UsageCounts(ctx context.Context) (map[int64]scheduleModels.CalendarPeriodUsage, error) {
	return r.usage.UsageCounts(ctx)
}

// --- closing days ---

// closingDayCalendarRepository serves the legacy schedule.ClosingDayRepository
// over the School Calendar capability (#2666).
type closingDayCalendarRepository struct {
	UnfilteredListing[*scheduleModels.ClosingDay]
	ClosingDayRangeListing
	calendar schoolcalendar.Capability
}

var _ scheduleModels.ClosingDayRepository = closingDayCalendarRepository{}

func newClosingDayCalendarRepository(capability schoolcalendar.Capability) closingDayCalendarRepository {
	repo := closingDayCalendarRepository{calendar: capability}
	repo.UnfilteredListing.Source = repo.FindByTenantID
	repo.ClosingDayRangeListing.Source = repo.listOverlappingRange
	return repo
}

func closingDayFieldsFromLegacy(day *scheduleModels.ClosingDay) schoolcalendar.ClosingDayFields {
	return schoolcalendar.ClosingDayFields{
		StartDate: string(day.StartDate), EndDate: string(day.EndDate), Reason: day.Reason,
	}
}

func applyClosingDayToLegacy(target *scheduleModels.ClosingDay, value schoolcalendar.ClosingDay) {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.SetTenantID(value.TenantID)
	setDate(&target.StartDate, value.StartDate)
	setDate(&target.EndDate, value.EndDate)
	target.Reason = value.Reason
}

func toLegacyClosingDays(values []schoolcalendar.ClosingDay) []*scheduleModels.ClosingDay {
	result := make([]*scheduleModels.ClosingDay, 0, len(values))
	for _, value := range values {
		day := new(scheduleModels.ClosingDay)
		applyClosingDayToLegacy(day, value)
		result = append(result, day)
	}
	return result
}

func (r closingDayCalendarRepository) Create(ctx context.Context, day *scheduleModels.ClosingDay) error {
	if day == nil {
		return errors.New("ClosingDay cannot be nil or zero value")
	}
	if err := day.Validate(); err != nil {
		return err
	}
	created, err := r.calendar.CreateClosingDay(ctx, schoolcalendar.CreateClosingDay{ClosingDayFields: closingDayFieldsFromLegacy(day)})
	if err != nil {
		return calendarError("create", err)
	}
	applyClosingDayToLegacy(day, created)
	return nil
}

func (r closingDayCalendarRepository) FindByID(ctx context.Context, id any) (*scheduleModels.ClosingDay, error) {
	dayID, err := membershipID(id)
	if err != nil {
		return nil, calendarWrapError(opFindByID, err)
	}
	value, err := r.calendar.FindClosingDay(ctx, dayID)
	if err != nil {
		return nil, calendarError(opFindByID, err)
	}
	day := new(scheduleModels.ClosingDay)
	applyClosingDayToLegacy(day, value)
	return day, nil
}

func (r closingDayCalendarRepository) Update(ctx context.Context, day *scheduleModels.ClosingDay) error {
	if day == nil {
		return errors.New("ClosingDay cannot be nil or zero value")
	}
	if err := day.Validate(); err != nil {
		return err
	}
	updated, err := r.calendar.UpdateClosingDay(ctx, schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: closingDayFieldsFromLegacy(day)})
	if err != nil {
		return calendarError("update", err)
	}
	applyClosingDayToLegacy(day, updated)
	return nil
}

// Delete stays idempotent like the base repository it replaces.
func (r closingDayCalendarRepository) Delete(ctx context.Context, id any) error {
	dayID, err := membershipID(id)
	if err != nil {
		return calendarWrapError("delete", err)
	}
	err = r.calendar.DeleteClosingDay(ctx, dayID)
	if errors.Is(err, schoolcalendar.ErrClosingDayNotFound) {
		return nil
	}
	return calendarError("delete", err)
}

func (r closingDayCalendarRepository) FindByTenantID(ctx context.Context) ([]*scheduleModels.ClosingDay, error) {
	values, err := r.calendar.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{})
	if err != nil {
		return nil, calendarError("find by tenant id", err)
	}
	return toLegacyClosingDays(values), nil
}

func (r closingDayCalendarRepository) listOverlappingRange(ctx context.Context, from, to string) ([]*scheduleModels.ClosingDay, error) {
	values, err := r.calendar.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: from, OverlappingTo: to})
	if err != nil {
		return nil, calendarError("find overlapping range", err)
	}
	return toLegacyClosingDays(values), nil
}

// --- dateframes ---

// dateframeCalendarRepository serves the legacy schedule.DateframeRepository
// over the School Calendar capability (#2666). Dateframes are instants, so
// the legacy midnight normalisation of the lookups stays here.
type dateframeCalendarRepository struct {
	DateframeOptionsListing
	calendar schoolcalendar.Capability
}

var _ scheduleModels.DateframeRepository = dateframeCalendarRepository{}

func newDateframeCalendarRepository(capability schoolcalendar.Capability) dateframeCalendarRepository {
	repo := dateframeCalendarRepository{calendar: capability}
	repo.Source = repo.list
	return repo
}

func dateframeFieldsFromLegacy(dateframe *scheduleModels.Dateframe) schoolcalendar.DateframeFields {
	return schoolcalendar.DateframeFields{
		StartDate: dateframe.StartDate, EndDate: dateframe.EndDate,
		Name: dateframe.Name, Description: dateframe.Description,
	}
}

func applyDateframeToLegacy(target *scheduleModels.Dateframe, value schoolcalendar.Dateframe) {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.SetTenantID(value.TenantID)
	target.StartDate = value.StartDate
	target.EndDate = value.EndDate
	target.Name = value.Name
	target.Description = value.Description
}

func toLegacyDateframes(values []schoolcalendar.Dateframe) []*scheduleModels.Dateframe {
	result := make([]*scheduleModels.Dateframe, 0, len(values))
	for _, value := range values {
		dateframe := new(scheduleModels.Dateframe)
		applyDateframeToLegacy(dateframe, value)
		result = append(result, dateframe)
	}
	return result
}

func (r dateframeCalendarRepository) Create(ctx context.Context, dateframe *scheduleModels.Dateframe) error {
	if dateframe == nil {
		return fmt.Errorf("%s cannot be nil or zero value", "Dateframe")
	}
	if err := dateframe.Validate(); err != nil {
		return err
	}
	created, err := r.calendar.CreateDateframe(ctx, schoolcalendar.CreateDateframe{DateframeFields: dateframeFieldsFromLegacy(dateframe)})
	if err != nil {
		return calendarError("create", err)
	}
	applyDateframeToLegacy(dateframe, created)
	return nil
}

func (r dateframeCalendarRepository) FindByID(ctx context.Context, id any) (*scheduleModels.Dateframe, error) {
	dateframeID, err := membershipID(id)
	if err != nil {
		return nil, calendarWrapError(opFindByID, err)
	}
	value, err := r.calendar.FindDateframe(ctx, dateframeID)
	if err != nil {
		return nil, calendarError(opFindByID, err)
	}
	dateframe := new(scheduleModels.Dateframe)
	applyDateframeToLegacy(dateframe, value)
	return dateframe, nil
}

func (r dateframeCalendarRepository) Update(ctx context.Context, dateframe *scheduleModels.Dateframe) error {
	if dateframe == nil {
		return fmt.Errorf("%s cannot be nil or zero value", "Dateframe")
	}
	if err := dateframe.Validate(); err != nil {
		return err
	}
	updated, err := r.calendar.UpdateDateframe(ctx, schoolcalendar.UpdateDateframe{ID: dateframe.ID, DateframeFields: dateframeFieldsFromLegacy(dateframe)})
	if err != nil {
		return calendarError("update", err)
	}
	applyDateframeToLegacy(dateframe, updated)
	return nil
}

// Delete stays idempotent like the base repository it replaces.
func (r dateframeCalendarRepository) Delete(ctx context.Context, id any) error {
	dateframeID, err := membershipID(id)
	if err != nil {
		return calendarWrapError("delete", err)
	}
	err = r.calendar.DeleteDateframe(ctx, dateframeID)
	if errors.Is(err, schoolcalendar.ErrDateframeNotFound) {
		return nil
	}
	return calendarError("delete", err)
}

func (r dateframeCalendarRepository) list(ctx context.Context, listing DateframeListing) ([]*scheduleModels.Dateframe, error) {
	filter := schoolcalendar.DateframeFilter{
		Name: listing.Name, NamePattern: listing.NamePattern, Limit: listing.Limit, Offset: listing.Offset,
	}
	for _, field := range listing.Sort {
		filter.Sort = append(filter.Sort, schoolcalendar.DateframeSort{Field: field.Field, Descending: field.Descending})
	}
	values, err := r.calendar.ListDateframes(ctx, filter)
	if err != nil {
		return nil, calendarError("list", err)
	}
	return toLegacyDateframes(values), nil
}

// FindByName keeps the legacy not-found error shape for an unknown name.
func (r dateframeCalendarRepository) FindByName(ctx context.Context, name string) (*scheduleModels.Dateframe, error) {
	values, err := r.calendar.ListDateframes(ctx, schoolcalendar.DateframeFilter{NameFold: name, Limit: 1})
	if err != nil {
		return nil, calendarError("find by name", err)
	}
	if len(values) == 0 {
		return nil, calendarNotFoundError("find by name")
	}
	dateframe := new(scheduleModels.Dateframe)
	applyDateframeToLegacy(dateframe, values[0])
	return dateframe, nil
}

// calendarNotFoundError is the retained not-found shape: a repository error
// that matches both the base not-found sentinel and sql.ErrNoRows.
func calendarNotFoundError(op string) error {
	return &modelBase.DatabaseError{Op: op, Err: translateNotFound(sql.ErrNoRows)}
}

// calendarWrapError wraps err in the retained repository error shape,
// preserving the chain.
func calendarWrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &modelBase.DatabaseError{Op: op, Err: err}
}

func membershipID(id any) (int64, error) {
	switch value := id.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", id)
	}
}
