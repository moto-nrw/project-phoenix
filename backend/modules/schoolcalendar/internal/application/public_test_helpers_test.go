package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"
)

type seamRecorder interface {
	FindCalendarPeriod(context.Context, int64) (schoolcalendar.CalendarPeriod, error)
	ListCalendarPeriods(context.Context, schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, error)
	CreateCalendarPeriod(ctx context.Context, input schoolcalendar.CreateCalendarPeriod, ifAbsent bool) (schoolcalendar.CalendarPeriod, bool, error)
	UpdateCalendarPeriod(context.Context, schoolcalendar.UpdateCalendarPeriod) (schoolcalendar.CalendarPeriod, error)
	DeleteCalendarPeriod(context.Context, int64) error

	FindClosingDay(context.Context, int64) (schoolcalendar.ClosingDay, error)
	ListClosingDays(context.Context, schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error)
	CreateClosingDay(context.Context, schoolcalendar.CreateClosingDay) (schoolcalendar.ClosingDay, error)
	UpdateClosingDay(context.Context, schoolcalendar.UpdateClosingDay) (schoolcalendar.ClosingDay, error)
	DeleteClosingDay(context.Context, int64) error

	FindDateframe(context.Context, int64) (schoolcalendar.Dateframe, error)
	ListDateframes(context.Context, schoolcalendar.DateframeFilter) ([]schoolcalendar.Dateframe, error)
	CreateDateframe(context.Context, schoolcalendar.CreateDateframe) (schoolcalendar.Dateframe, error)
	UpdateDateframe(context.Context, schoolcalendar.UpdateDateframe) (schoolcalendar.Dateframe, error)
	DeleteDateframe(context.Context, int64) error

	ValidHolidayRegion(string) bool
	ListHolidays(context.Context, string, string, string) ([]schoolcalendar.Holiday, error)
	HolidayDates(context.Context, string, string, string) (map[string]bool, error)
	RenderCalendar(context.Context, string, []schoolcalendar.CalendarEvent) (string, error)
	RenderCalendarObject(context.Context, schoolcalendar.CalendarEvent) (string, error)

	// FederalState resolves the caller's tenant federal-state ISO code; the
	// composition binds it to the settings platform.
	FederalState(context.Context) (string, error)
}

type seamStore struct {
	e              seamRecorder
	administrating bool
}

func newTestModule(e seamRecorder) *Service {
	if e == nil {
		return New(nil, func(ports.Observation) {}, Administration{}, e)
	}
	store := &seamStore{e: e}
	return New(store, func(ports.Observation) {}, Administration{Transaction: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }, RecurrenceGate: func(context.Context) error { store.administrating = true; return nil }, Today: func() string { return "2030-09-01" }, FederalState: e.FederalState}, e)
}
func (s *seamStore) FindCalendarPeriod(ctx context.Context, id int64) (schoolcalendar.CalendarPeriod, bool, domain.OperationStats, error) {
	v, err := s.e.FindCalendarPeriod(ctx, id)
	return v, err == nil, domain.OperationStats{}, err
}
func (s *seamStore) ListCalendarPeriods(ctx context.Context, f schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, domain.OperationStats, error) {
	if s.administrating {
		return nil, domain.OperationStats{}, nil
	}
	v, err := s.e.ListCalendarPeriods(ctx, f)
	return v, domain.OperationStats{}, err
}
func (s *seamStore) CreateCalendarPeriod(ctx context.Context, f schoolcalendar.CalendarPeriodFields, ifAbsent bool) (schoolcalendar.CalendarPeriod, bool, domain.OperationStats, error) {
	v, created, err := s.e.CreateCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: f}, ifAbsent)
	return v, created, domain.OperationStats{}, err
}
func (s *seamStore) UpdateCalendarPeriod(ctx context.Context, id int64, f schoolcalendar.CalendarPeriodFields) (schoolcalendar.CalendarPeriod, domain.OperationStats, error) {
	v, err := s.e.UpdateCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: id, CalendarPeriodFields: f})
	return v, domain.OperationStats{}, err
}
func (s *seamStore) DeleteCalendarPeriod(ctx context.Context, id int64) (domain.OperationStats, error) {
	return domain.OperationStats{}, s.e.DeleteCalendarPeriod(ctx, id)
}
func (s *seamStore) FindClosingDay(ctx context.Context, id int64) (schoolcalendar.ClosingDay, bool, domain.OperationStats, error) {
	v, err := s.e.FindClosingDay(ctx, id)
	return v, err == nil, domain.OperationStats{}, err
}
func (s *seamStore) ListClosingDays(ctx context.Context, f schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, domain.OperationStats, error) {
	v, err := s.e.ListClosingDays(ctx, f)
	return v, domain.OperationStats{}, err
}
func (s *seamStore) CreateClosingDay(ctx context.Context, f schoolcalendar.ClosingDayFields) (schoolcalendar.ClosingDay, domain.OperationStats, error) {
	v, err := s.e.CreateClosingDay(ctx, schoolcalendar.CreateClosingDay{ClosingDayFields: f})
	return v, domain.OperationStats{}, err
}
func (s *seamStore) UpdateClosingDay(ctx context.Context, id int64, f schoolcalendar.ClosingDayFields) (schoolcalendar.ClosingDay, domain.OperationStats, error) {
	v, err := s.e.UpdateClosingDay(ctx, schoolcalendar.UpdateClosingDay{ID: id, ClosingDayFields: f})
	return v, domain.OperationStats{}, err
}
func (s *seamStore) DeleteClosingDay(ctx context.Context, id int64) (domain.OperationStats, error) {
	return domain.OperationStats{}, s.e.DeleteClosingDay(ctx, id)
}
func (s *seamStore) FindDateframe(ctx context.Context, id int64) (schoolcalendar.Dateframe, bool, domain.OperationStats, error) {
	v, err := s.e.FindDateframe(ctx, id)
	return v, err == nil, domain.OperationStats{}, err
}
func (s *seamStore) ListDateframes(ctx context.Context, f schoolcalendar.DateframeFilter) ([]schoolcalendar.Dateframe, domain.OperationStats, error) {
	v, err := s.e.ListDateframes(ctx, f)
	return v, domain.OperationStats{}, err
}
func (s *seamStore) CreateDateframe(ctx context.Context, f schoolcalendar.DateframeFields) (schoolcalendar.Dateframe, domain.OperationStats, error) {
	v, err := s.e.CreateDateframe(ctx, schoolcalendar.CreateDateframe{DateframeFields: f})
	return v, domain.OperationStats{}, err
}
func (s *seamStore) UpdateDateframe(ctx context.Context, id int64, f schoolcalendar.DateframeFields) (schoolcalendar.Dateframe, domain.OperationStats, error) {
	v, err := s.e.UpdateDateframe(ctx, schoolcalendar.UpdateDateframe{ID: id, DateframeFields: f})
	return v, domain.OperationStats{}, err
}
func (s *seamStore) DeleteDateframe(ctx context.Context, id int64) (domain.OperationStats, error) {
	return domain.OperationStats{}, s.e.DeleteDateframe(ctx, id)
}

var _ schoolcalendar.Days = (*Service)(nil)
var _ schoolcalendar.Calendar = (*Service)(nil)
