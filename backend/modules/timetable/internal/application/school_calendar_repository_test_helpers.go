//go:build testhooks

package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// NewClosingDayRepository serves the legacy closing day repository over the
// School Calendar owner (#2666).
func NewClosingDayRepository(calendar schoolcalendar.Capability) ports.ClosingDayRecords {
	return newClosingDayCalendarRepository(calendar)
}

// NewDateframeRepository serves the legacy dateframe repository over the
// School Calendar owner (#2666).
func NewDateframeRepository(calendar schoolcalendar.Capability) ports.DateframeRecords {
	return newDateframeCalendarRepository(calendar)
}

// closingDayCalendarRepository serves the legacy schedule.ClosingDayRecords
// over the School Calendar capability (#2666).
type closingDayCalendarRepository struct {
	ClosingDayRangeListing
	calendar schoolcalendar.Capability
}

func newClosingDayCalendarRepository(capability schoolcalendar.Capability) closingDayCalendarRepository {
	repo := closingDayCalendarRepository{calendar: capability}
	repo.Source = repo.listOverlappingRange
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
	SetDate(&target.StartDate, value.StartDate)
	SetDate(&target.EndDate, value.EndDate)
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

func (r closingDayCalendarRepository) FindByID(ctx context.Context, nativeRecordID int64) (*scheduleModels.ClosingDay, error) {
	var id any = nativeRecordID
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
func (r closingDayCalendarRepository) Delete(ctx context.Context, nativeRecordID int64) error {
	var id any = nativeRecordID
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

// dateframeCalendarRepository serves the legacy schedule.DateframeRecords
// over the School Calendar capability (#2666). Dateframes are instants, so
// the legacy midnight normalisation of the lookups stays here.
type dateframeCalendarRepository struct {
	calendar schoolcalendar.Capability
}

func newDateframeCalendarRepository(capability schoolcalendar.Capability) dateframeCalendarRepository {
	repo := dateframeCalendarRepository{calendar: capability}
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

func (r dateframeCalendarRepository) FindByID(ctx context.Context, nativeRecordID int64) (*scheduleModels.Dateframe, error) {
	var id any = nativeRecordID
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
func (r dateframeCalendarRepository) Delete(ctx context.Context, nativeRecordID int64) error {
	var id any = nativeRecordID
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

var (
	_ ports.ClosingDayRecords = closingDayCalendarRepository{}
	_ ports.DateframeRecords  = dateframeCalendarRepository{}
)

func (r dateframeCalendarRepository) ListDateframes(ctx context.Context, filter schoolcalendar.DateframeFilter) ([]*scheduleModels.Dateframe, error) {
	values, err := r.calendar.ListDateframes(ctx, filter)
	if err != nil {
		return nil, calendarError("list", err)
	}
	return toLegacyDateframes(values), nil
}
