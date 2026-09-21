package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const opBulkUpsertArrivalSchedules = "bulk upsert arrival schedules"

func (s *arrivalScheduleService) BulkUpsertArrivalSchedules(ctx context.Context, filter careplan.ArrivalScheduleBulkFilter, inputs []careplan.ArrivalScheduleInput, createdBy int64) (*careplan.BulkUpsertResult, error) {
	filter.SchoolClass = strings.TrimSpace(filter.SchoolClass)
	if err := validateArrivalSelection(filter, inputs); err != nil {
		return nil, &careplan.ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: err}
	}
	if filter.SchoolClass != "" && s.classes != nil {
		return s.upsertClassArrivalTimes(ctx, filter.SchoolClass, inputs, createdBy, filter.Authorize)
	}
	students, kind, value, err := s.selectArrivalStudents(ctx, filter)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: err}
	}
	if len(students) == 0 {
		return &careplan.BulkUpsertResult{}, nil
	}
	parsed, err := parseArrivalTimes(inputs)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: err}
	}
	result := &careplan.BulkUpsertResult{OverwrittenStudents: make([]careplan.OverwriteWarning, 0)}
	err = s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		var lockErr error
		students, lockErr = s.lockArrivalSelection(txCtx, students, filter)
		if lockErr != nil {
			return lockErr
		}
		for _, student := range students {
			if err := s.patchArrivalWeek(txCtx, student, inputs, parsed, createdBy, result); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, &careplan.ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: err}
	}
	s.logger.Info("bulk upsert arrival schedules",
		"filter_type", kind,
		"filter_value", value,
		"students_affected", len(students),
		"weekdays_set", len(inputs),
		"overwrites", len(result.OverwrittenStudents))
	result.StudentsAffected = len(students)
	result.AffectedStudentIDs = make([]int64, 0, len(students))
	for _, student := range students {
		result.AffectedStudentIDs = append(result.AffectedStudentIDs, student.ID)
	}
	return result, nil
}

func validateArrivalSelection(filter careplan.ArrivalScheduleBulkFilter, inputs []careplan.ArrivalScheduleInput) error {
	count := 0
	if filter.SchoolClass != "" {
		count++
	}
	if filter.GroupID != 0 {
		count++
	}
	if len(filter.StudentIDs) > 0 {
		count++
	}
	if count != 1 {
		return errors.New("exactly one bulk filter is required: school_class, group_id, or student_ids")
	}
	if filter.GroupID < 0 {
		return errors.New("group_id must be positive")
	}
	if len(inputs) == 0 {
		return errors.New("schedules cannot be empty")
	}
	if len(filter.StudentIDs) > 500 {
		return errors.New("student_ids cannot exceed 500 items")
	}
	return nil
}

func (s *arrivalScheduleService) selectArrivalStudents(ctx context.Context, filter careplan.ArrivalScheduleBulkFilter) ([]ports.ArrivalBulkStudent, string, string, error) {
	var rows []ports.ArrivalBulkStudent
	var kind, value string
	var err error
	if filter.SchoolClass != "" {
		kind, value = "school_class", filter.SchoolClass
		rows, err = s.students.ByClass(ctx, filter.SchoolClass, calendar.TodayDate())
		rows = dropDepartedArrivalStudents(rows)
	} else if filter.GroupID != 0 {
		kind, value = "group_id", strconv.FormatInt(filter.GroupID, 10)
		rows, err = s.students.ByGroup(ctx, filter.GroupID, calendar.TodayDate())
		rows = dropDepartedArrivalStudents(rows)
	} else {
		kind, value = "student_ids", strconv.Itoa(len(filter.StudentIDs))
		var byID map[int64]ports.ArrivalBulkStudent
		byID, err = s.students.ByIDs(ctx, filter.StudentIDs, calendar.TodayDate())
		if err == nil {
			rows, err = explicitArrivalStudents(filter.StudentIDs, byID)
		}
		if errors.Is(err, careplan.ErrBulkStudentNotFound) {
			return nil, kind, value, err
		}
	}
	if err != nil {
		return nil, kind, value, fmt.Errorf("failed to find students for %s %s: %w", kind, value, err)
	}
	return rows, kind, value, nil
}

func explicitArrivalStudents(ids []int64, byID map[int64]ports.ArrivalBulkStudent) ([]ports.ArrivalBulkStudent, error) {
	rows := make([]ports.ArrivalBulkStudent, 0, len(ids))
	for _, id := range ids {
		row, ok := byID[id]
		if !ok || row.Alumnus || row.CareEnded {
			return nil, fmt.Errorf("%w: student %d", careplan.ErrBulkStudentNotFound, id)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func dropDepartedArrivalStudents(rows []ports.ArrivalBulkStudent) []ports.ArrivalBulkStudent {
	kept := make([]ports.ArrivalBulkStudent, 0, len(rows))
	for _, row := range rows {
		if !row.CareEnded {
			kept = append(kept, row)
		}
	}
	return kept
}

func parseArrivalTimes(inputs []careplan.ArrivalScheduleInput) (map[int]time.Time, error) {
	parsed := make(map[int]time.Time, len(inputs))
	for _, input := range inputs {
		if input.Weekday < 1 || input.Weekday > 5 {
			return nil, fmt.Errorf("invalid weekday %d", input.Weekday)
		}
		if _, duplicate := parsed[input.Weekday]; duplicate {
			return nil, fmt.Errorf("duplicate weekday %d", input.Weekday)
		}
		if strings.TrimSpace(input.ArrivalTime) == "" {
			return nil, fmt.Errorf("expected_arrival is required for weekday %d unless school_class is selected", input.Weekday)
		}
		clock, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+input.ArrivalTime)
		if err != nil {
			return nil, fmt.Errorf("invalid expected_arrival %q for weekday %d: %w", input.ArrivalTime, input.Weekday, err)
		}
		parsed[input.Weekday] = clock
	}
	return parsed, nil
}

func (s *arrivalScheduleService) lockArrivalSelection(ctx context.Context, students []ports.ArrivalBulkStudent, filter careplan.ArrivalScheduleBulkFilter) ([]ports.ArrivalBulkStudent, error) {
	sort.Slice(students, func(i, j int) bool { return students[i].ID < students[j].ID })
	locked := make([]ports.ArrivalBulkStudent, 0, len(students))
	for _, selected := range students {
		fresh, err := s.students.LockByID(ctx, selected.ID, calendar.TodayDate())
		if errors.Is(err, careplan.ErrBulkStudentNotFound) {
			return nil, fmt.Errorf("%w: student %d", careplan.ErrBulkStudentNotFound, selected.ID)
		}
		if err != nil {
			return nil, fmt.Errorf("lock selected student %d: %w", selected.ID, err)
		}
		if fresh.Alumnus || fresh.CareEnded {
			if len(filter.StudentIDs) == 0 {
				continue
			}
			return nil, fmt.Errorf("%w: student %d", careplan.ErrBulkStudentNotFound, fresh.ID)
		}
		if err := validateLockedArrivalStudent(ctx, fresh, filter); err != nil {
			return nil, err
		}
		locked = append(locked, fresh)
	}
	return locked, nil
}

func validateLockedArrivalStudent(ctx context.Context, row ports.ArrivalBulkStudent, filter careplan.ArrivalScheduleBulkFilter) error {
	if filter.SchoolClass != "" && !strings.EqualFold(strings.TrimSpace(row.SchoolClass), filter.SchoolClass) {
		return fmt.Errorf("%w: student %d left school class", careplan.ErrBulkStudentNotFound, row.ID)
	}
	if filter.GroupID != 0 && (row.GroupID == nil || *row.GroupID != filter.GroupID) {
		return fmt.Errorf("%w: student %d left group", careplan.ErrBulkStudentNotFound, row.ID)
	}
	return authorizeArrivalStudent(ctx, row, filter.Authorize)
}

func authorizeArrivalStudent(ctx context.Context, row ports.ArrivalBulkStudent, authorize func(context.Context, careplan.ScheduleStudent) (bool, error)) error {
	if authorize == nil {
		return nil
	}
	allowed, err := authorize(ctx, row.ScheduleStudent)
	if err != nil || !allowed {
		return fmt.Errorf("%w: student %d", careplan.ErrBulkStudentUnauthorized, row.ID)
	}
	return nil
}

func (s *arrivalScheduleService) patchArrivalWeek(ctx context.Context, student ports.ArrivalBulkStudent, inputs []careplan.ArrivalScheduleInput, parsed map[int]time.Time, createdBy int64, result *careplan.BulkUpsertResult) error {
	existing, err := s.scheduleRepo.FindByStudentID(ctx, student.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch existing schedules for student %d: %w", student.ID, err)
	}
	byDay := make(map[int]*careplan.ArrivalSchedule, len(existing))
	for _, row := range existing {
		byDay[row.Weekday] = row
	}
	for _, input := range inputs {
		clock := parsed[input.Weekday]
		row := &careplan.ArrivalSchedule{StudentID: student.ID, Weekday: input.Weekday, ExpectedArrival: clock, CreatedBy: createdBy}
		if previous := byDay[input.Weekday]; previous != nil {
			row.Notes = previous.Notes
			if previous.ExpectedArrival.Format("15:04") != clock.Format("15:04") {
				result.OverwrittenStudents = append(result.OverwrittenStudents, careplan.OverwriteWarning{StudentID: student.ID, StudentName: s.arrivalStudentName(ctx, student), Weekday: input.Weekday,
					WeekdayName: domain.WeekdayName(input.Weekday), PreviousTime: previous.ExpectedArrival.Format("15:04"), NewTime: clock.Format("15:04")})
			}
		}
		row.SetTenantID(s.tx.TenantID(ctx))
		if err := s.scheduleRepo.UpsertSchedule(ctx, row); err != nil {
			return fmt.Errorf("failed to upsert schedule for student %d weekday %d: %w", student.ID, input.Weekday, err)
		}
	}
	return nil
}

func (s *arrivalScheduleService) arrivalStudentName(ctx context.Context, student ports.ArrivalBulkStudent) string {
	name, err := s.students.Name(ctx, student)
	if err != nil {
		return fmt.Sprintf("Student %d", student.ID)
	}
	return name
}
