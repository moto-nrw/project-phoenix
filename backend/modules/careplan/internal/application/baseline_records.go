package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The adapters in this file serve the baseline, approval and excusal
// services from the owner's value-based records (#4084).

// ArrivalBaselineDependencies are the tenant-safe projections the arrival
// baselines read; the composition converts its root-facing dependencies.
type ArrivalBaselineDependencies struct {
	Records         carePlanPorts.ArrivalBaselineRecords
	Bookings        careplan.ApprovedBookingReader
	BookingMode     func(context.Context) (bool, error)
	StudentClasses  func(context.Context, []int64) (map[int64]string, error)
	ClassPlans      func(context.Context, []string) ([]*ports.ArrivalClassPlan, error)
	ClassExceptions func(context.Context, []string, calendar.Date, calendar.Date) (map[string]careplan.ClassArrivalExceptionsByDate, error)
}

// NewArrivalBaselines projects the arrival baselines from deps.
func NewArrivalBaselines(deps ArrivalBaselineDependencies) careplan.ArrivalBaselineReader {
	return NewArrivalBaselineQueries(arrivalBaselineSource{deps.Bookings, deps})
}

// arrivalBaselineSource reads the booking rows straight from the approved
// booking reader.
type arrivalBaselineSource struct {
	careplan.ApprovedBookingReader
	deps ArrivalBaselineDependencies
}

func (s arrivalBaselineSource) StoredArrivalRows(ctx context.Context, ids []int64) ([]*careplan.ArrivalSchedule, error) {
	if len(ids) == 0 {
		return []*careplan.ArrivalSchedule{}, nil
	}
	rows, err := s.deps.Records.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load stored schedules: %w", err)
	}
	out := make([]*careplan.ArrivalSchedule, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

func (s arrivalBaselineSource) StudentClasses(ctx context.Context, ids []int64) (map[int64]string, error) {
	rows, err := s.deps.StudentClasses(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load students: %w", err)
	}
	out := make(map[int64]string, len(rows))
	for id, class := range rows {
		if key := strings.ToLower(strings.TrimSpace(class)); key != "" {
			out[id] = key
		}
	}
	return out, nil
}

func (s arrivalBaselineSource) ClassArrivalTimes(ctx context.Context, classes map[int64]string) (map[string]*domain.ClassArrivalBaseline, error) {
	if len(classes) == 0 {
		return nil, nil
	}
	rows, err := s.deps.ClassPlans(ctx, arrivalBaselineClasses(classes))
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load class arrival times: %w", err)
	}
	out := make(map[string]*domain.ClassArrivalBaseline, len(rows))
	for _, row := range rows {
		if row == nil || len(row.ArrivalTimes) == 0 {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(row.SchoolClass))] = domain.ClassArrivalBaselineFromTimes(row.SchoolClass, row.ArrivalTimes)
	}
	return out, nil
}

func (s arrivalBaselineSource) ClassArrivalExceptions(ctx context.Context, classes map[int64]string, from, to calendar.Date) (map[string]careplan.ClassArrivalExceptionsByDate, error) {
	if len(classes) == 0 {
		return nil, nil
	}
	rows, err := s.deps.ClassExceptions(ctx, arrivalBaselineClasses(classes), from, to)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load class arrival exceptions: %w", err)
	}
	return rows, nil
}

func arrivalBaselineClasses(classes map[int64]string) []string {
	out := make([]string, 0, len(classes))
	for _, class := range classes {
		out = append(out, class)
	}
	return out
}

func (s arrivalBaselineSource) BookingsAuthoritative(ctx context.Context) (bool, error) {
	value, err := s.deps.BookingMode(ctx)
	if err != nil {
		return false, fmt.Errorf("project arrival baselines: resolve booking mode: %w", err)
	}
	return value, nil
}

func (s arrivalBaselineSource) OfferingRows(ctx context.Context, links []*careplan.ApprovedBooking) (map[int64]*careplan.CareOffering, error) {
	rows, err := baselineOfferingRows(ctx, s.deps.Records, links)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load care offerings: %w", err)
	}
	return rows, nil
}

// NewPickupBaselines projects the pickup baselines from the stored schedules,
// the approved bookings and the booking mode.
func NewPickupBaselines(records carePlanPorts.PickupBaselineRecords, links careplan.ApprovedBookingReader, authoritative func(context.Context) (bool, error)) careplan.PickupBaselineReader {
	return NewPickupBaselineQueries(pickupBaselineSource{links, records, authoritative})
}

// pickupBaselineSource reads the booking rows straight from the approved
// booking reader.
type pickupBaselineSource struct {
	careplan.ApprovedBookingReader
	records       carePlanPorts.PickupBaselineRecords
	authoritative func(context.Context) (bool, error)
}

func (s pickupBaselineSource) StoredPickupRows(ctx context.Context, ids []int64) ([]*careplan.PickupSchedule, error) {
	if len(ids) == 0 {
		return []*careplan.PickupSchedule{}, nil
	}
	rows, err := s.records.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
	if err != nil {
		return nil, fmt.Errorf("project pickup baselines: load stored schedules: %w", err)
	}
	out := make([]*careplan.PickupSchedule, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

func (s pickupBaselineSource) BookingsAuthoritative(ctx context.Context) (bool, error) {
	result, err := s.authoritative(ctx)
	if err != nil {
		return false, fmt.Errorf("project pickup baselines: resolve booking mode: %w", err)
	}
	return result, nil
}

func (s pickupBaselineSource) OfferingRows(ctx context.Context, links []*careplan.ApprovedBooking) (map[int64]*careplan.CareOffering, error) {
	rows, err := baselineOfferingRows(ctx, s.records, links)
	if err != nil {
		return nil, fmt.Errorf("project pickup baselines: load care offerings: %w", err)
	}
	return rows, nil
}

func baselineOfferingRows(ctx context.Context, records interface {
	ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
}, links []*careplan.ApprovedBooking) (map[int64]*careplan.CareOffering, error) {
	ids := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))
	for _, entry := range links {
		if entry != nil && entry.Link != nil && entry.Link.CareOfferingID > 0 && !seen[entry.Link.CareOfferingID] {
			ids = append(ids, entry.Link.CareOfferingID)
			seen[entry.Link.CareOfferingID] = true
		}
	}
	if len(ids) == 0 {
		return map[int64]*careplan.CareOffering{}, nil
	}
	rows, err := records.ListCareOfferings(ctx, careplan.CareOfferingFilter{IDs: ids})
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*careplan.CareOffering, len(rows))
	for i := range rows {
		out[rows[i].ID] = &rows[i]
	}
	return out, nil
}

// NewPickupApprovalExceptions serves the approval exceptions from records;
// isConflict recognises the store's unique violation of a pickup exception.
func NewPickupApprovalExceptions(records carePlanPorts.PickupApprovalRecords, isConflict func(error) bool) ports.PickupApprovalExceptions {
	return pickupApprovalExceptions{records: records, isConflict: isConflict}
}

type pickupApprovalExceptions struct {
	records    carePlanPorts.PickupApprovalRecords
	isConflict func(error) bool
}

func (a pickupApprovalExceptions) FindForDate(ctx context.Context, id int64, date calendar.Date) (*careplan.PickupException, error) {
	rows, err := a.records.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: careplan.Date(date)})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (a pickupApprovalExceptions) Create(ctx context.Context, value careplan.PickupException) (int64, error) {
	if err := value.Validate(); err != nil {
		return 0, err
	}
	row, err := a.records.CreatePickupException(ctx, value)
	return row.ID, err
}

func (a pickupApprovalExceptions) Update(ctx context.Context, value careplan.PickupException) error {
	if err := value.Validate(); err != nil {
		return err
	}
	return a.records.UpdatePickupException(ctx, value)
}

func (a pickupApprovalExceptions) IsUniqueViolation(err error) bool {
	return a.isConflict(err)
}

func (a pickupApprovalExceptions) FindDepartureForDate(ctx context.Context, id int64, date calendar.Date) (*careplan.DepartureException, error) {
	rows, err := a.records.ListDepartureExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: careplan.Date(date)})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (a pickupApprovalExceptions) SaveDeparture(ctx context.Context, value careplan.DepartureException) (int64, error) {
	row, err := a.records.SaveDepartureException(ctx, value)
	return row.ID, err
}

// NewPickupExcusalRows serves the pointer-based excusal rows from records.
func NewPickupExcusalRows(records carePlanPorts.PickupExcusalRecords) PickupExcusalRows {
	return PickupExcusalRows{records}
}

// PickupExcusalRows are the excusal commands' pointer-based pickup exception
// rows over the owner's records.
type PickupExcusalRows struct {
	carePlanPorts.PickupExcusalRecords
}

func (s PickupExcusalRows) FindByID(ctx context.Context, id int64) (*careplan.PickupException, error) {
	row, err := s.FindPickupException(ctx, id, false)
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return nil, careplan.ErrPartialAbsenceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (s PickupExcusalRows) FindByStudentIDAndDate(ctx context.Context, id int64, date calendar.Date) (*careplan.PickupException, error) {
	rows, err := s.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: careplan.Date(date)})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
func (s PickupExcusalRows) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*careplan.PickupException, error) {
	return s.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: careplan.Date(calendar.TodayDate())})
}
func (s PickupExcusalRows) FindByStudentIDAndDateRange(ctx context.Context, id int64, from, to calendar.Date) ([]*careplan.PickupException, error) {
	return s.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, From: careplan.Date(from), To: careplan.Date(to)})
}
func (s PickupExcusalRows) list(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*careplan.PickupException, error) {
	rows, err := s.ListPickupExceptions(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]*careplan.PickupException, 0, len(rows))
	for i := range rows {
		out = append(out, &rows[i])
	}
	return out, nil
}
func (s PickupExcusalRows) Create(ctx context.Context, row *careplan.PickupException) error {
	row.NormalizeWallClockTimes()
	if err := row.Validate(); err != nil {
		return err
	}
	result, err := s.CreatePickupException(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (s PickupExcusalRows) Update(ctx context.Context, row *careplan.PickupException) error {
	row.NormalizeWallClockTimes()
	if err := row.Validate(); err != nil {
		return err
	}
	return s.UpdatePickupException(ctx, *row)
}
func (s PickupExcusalRows) Delete(ctx context.Context, id int64) error {
	return s.DeletePickupException(ctx, id)
}
func (s PickupExcusalRows) HasFullDayStatus(ctx context.Context, id int64, date calendar.Date) (bool, error) {
	rows, err := s.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		StudentIDs: []int64{id}, From: careplan.Date(date), To: careplan.Date(date), ActiveOnly: true,
	})
	return len(rows) > 0, err
}
func (s PickupExcusalRows) PendingExcusedDates(ctx context.Context, id int64) ([]calendar.Date, error) {
	rows, err := s.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{StudentID: id, Statuses: []string{"pending"}})
	if err != nil {
		return nil, err
	}
	var dates []calendar.Date
	for _, row := range rows {
		for _, date := range row.Dates {
			dates = append(dates, calendar.Date(date))
		}
	}
	return dates, nil
}
