package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/uptrace/bun"
)

// NewSchoolCalendar composes the calendar owner behind the legacy composition
// seam for graphs that do not record observations. Production roots compose
// the module themselves (api/base.go) so runtime evidence is kept.
func NewSchoolCalendar(db *bun.DB) (schoolcalendar.Capability, error) {
	return schoolCalendarCompose.New(schoolCalendarCompose.Dependencies{
		DB:      db,
		Observe: func(schoolCalendarCompose.Observation) {},
	})
}

// BindSchoolCalendar replaces the calendar period, closing day and dateframe
// adapters, and the repositories that used to read schedule.calendar_periods
// themselves, with compositions over the observed School Calendar capability
// (#2666). NewFactory already composes an unobserved module, so this binding
// is about runtime evidence, not about correctness.
func (f *Factory) BindSchoolCalendar(capability schoolcalendar.Capability) {
	if capability == nil {
		panic("repository factory: school calendar capability is required")
	}
	if f.schoolCalendarBound {
		return
	}
	f.schoolCalendarBound = true
	f.bindSchoolCalendarAdapters(capability)
}

// SchoolCalendar returns the capability the calendar adapters read through.
func (f *Factory) SchoolCalendar() schoolcalendar.Capability { return f.schoolCalendar }

// bindSchoolCalendarAdapters points every calendar-derived repository at the
// given capability. The raw activities and users repositories are reached
// through their bind methods, so the binding survives the person, school and
// group wrappers layered on top of them.
func (f *Factory) bindSchoolCalendarAdapters(capability schoolcalendar.Capability) {
	f.schoolCalendar = capability
	f.CalendarPeriod = newCalendarPeriodCalendarRepository(capability, scheduleRepo.NewCalendarPeriodUsageRepository(f.db))
	f.ClosingDay = newClosingDayCalendarRepository(capability)
	f.Dateframe = newDateframeCalendarRepository(capability)
	if repo, ok := f.ActivityGroup.(interface {
		BindCalendarPeriods(activitiesRepo.CalendarPeriodSource)
	}); ok {
		repo.BindCalendarPeriods(capacityCalendarPeriods{calendar: capability})
	}
	if repo, ok := f.CareExitCleanup.(interface {
		BindCalendarPeriods(usersRepo.CalendarPeriodDirectory)
	}); ok {
		repo.BindCalendarPeriods(careExitCalendarPeriods{calendar: capability})
	}
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
		return usersRepo.NotFoundError(op)
	case errors.Is(err, schoolcalendar.ErrCalendarPeriodNameConflict):
		return usersRepo.WrapError(op, fmt.Errorf("%w: %w", scheduleModels.ErrCalendarPeriodNameConflict, err))
	default:
		return usersRepo.WrapError(op, err)
	}
}

// The legacy models carry the repository calendar-date type; these helpers
// move YYYY-MM-DD strings in and out of it by shape, so the composition never
// names the type itself.
func setDate[D ~string](target *D, value string) { *target = D(value) }

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
	scheduleRepo.UnfilteredListing[*scheduleModels.CalendarPeriod]
	scheduleRepo.CalendarPeriodOverlapListing
	calendar schoolcalendar.Capability
	usage    *scheduleRepo.CalendarPeriodUsageRepository
}

var _ scheduleModels.CalendarPeriodRepository = calendarPeriodCalendarRepository{}

func newCalendarPeriodCalendarRepository(capability schoolcalendar.Capability, usage *scheduleRepo.CalendarPeriodUsageRepository) calendarPeriodCalendarRepository {
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
		return nil, usersRepo.WrapError(opFindByID, err)
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
		return usersRepo.WrapError("delete", err)
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
	scheduleRepo.UnfilteredListing[*scheduleModels.ClosingDay]
	scheduleRepo.ClosingDayRangeListing
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
		return nil, usersRepo.WrapError(opFindByID, err)
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
		return usersRepo.WrapError("delete", err)
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
	scheduleRepo.DateframeOptionsListing
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
		return nil, usersRepo.WrapError(opFindByID, err)
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
		return usersRepo.WrapError("delete", err)
	}
	err = r.calendar.DeleteDateframe(ctx, dateframeID)
	if errors.Is(err, schoolcalendar.ErrDateframeNotFound) {
		return nil
	}
	return calendarError("delete", err)
}

func (r dateframeCalendarRepository) list(ctx context.Context, listing scheduleRepo.DateframeListing) ([]*scheduleModels.Dateframe, error) {
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
		return nil, usersRepo.NotFoundError("find by name")
	}
	dateframe := new(scheduleModels.Dateframe)
	applyDateframeToLegacy(dateframe, values[0])
	return dateframe, nil
}

func (r dateframeCalendarRepository) FindByDate(ctx context.Context, date time.Time) ([]*scheduleModels.Dateframe, error) {
	instant := dateframeMidnight(date)
	values, err := r.calendar.ListDateframes(ctx, schoolcalendar.DateframeFilter{Contains: &instant})
	if err != nil {
		return nil, calendarError("find by date", err)
	}
	return toLegacyDateframes(values), nil
}

func (r dateframeCalendarRepository) FindOverlapping(ctx context.Context, startDate, endDate time.Time) ([]*scheduleModels.Dateframe, error) {
	from, to := dateframeMidnight(startDate), dateframeMidnight(endDate)
	values, err := r.calendar.ListDateframes(ctx, schoolcalendar.DateframeFilter{OverlappingFrom: &from, OverlappingTo: &to})
	if err != nil {
		return nil, calendarError("find overlapping", err)
	}
	return toLegacyDateframes(values), nil
}

// dateframeMidnight drops the clock in the instant's own location, the
// normalisation the legacy lookups applied before comparing.
func dateframeMidnight(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

// --- owner queries for the raw repositories ---

// capacityCalendarPeriods projects the owner's active periods onto the shape
// the capacity query unnests.
type capacityCalendarPeriods struct {
	calendar schoolcalendar.Query
}

func (p capacityCalendarPeriods) ListActiveCalendarPeriods(ctx context.Context) ([]activitiesRepo.CapacityCalendarPeriod, error) {
	values, err := p.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{ActiveOnly: true})
	if err != nil {
		return nil, err
	}
	result := make([]activitiesRepo.CapacityCalendarPeriod, 0, len(values))
	for _, value := range values {
		period := activitiesRepo.CapacityCalendarPeriod{ID: value.ID, TenantID: value.TenantID, WeekCycleLength: value.WeekCycleLength}
		setDate(&period.StartDate, value.StartDate)
		setDate(&period.EndDate, value.EndDate)
		setOptionalDate(&period.WeekCycleAnchor, value.WeekCycleAnchor)
		result = append(result, period)
	}
	return result, nil
}

// careExitCalendarPeriods answers the booking restore with the ids of every
// period the tenant still has.
type careExitCalendarPeriods struct {
	calendar schoolcalendar.Query
}

func (p careExitCalendarPeriods) ListCalendarPeriodIDs(ctx context.Context) ([]int64, error) {
	values, err := p.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return ids, nil
}
