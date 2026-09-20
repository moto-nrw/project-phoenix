package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// upsertClassArrivalTimes writes the Unterrichtsschluss of one class. Weekdays
// the request does not mention keep whatever the class had, mirroring the
// "empty fields stay unchanged" contract the bulk screen has always had.
// Existing child rows are deliberately untouched: an own time remains the
// higher-priority deviation until a person resets it explicitly (ADR 0005).
func (s *arrivalScheduleService) upsertClassArrivalTimes(
	ctx context.Context,
	schoolClass string,
	schedules []careplan.ArrivalScheduleInput,
	updatedBy int64,
	authorize func(context.Context, careplan.ScheduleStudent) (bool, error),
) (*careplan.BulkUpsertResult, error) {
	touched, err := classArrivalTimeChanges(schedules)
	if err != nil {
		return nil, err
	}

	students, err := s.students.ByClass(ctx, schoolClass, timezone.TodayDate())
	if err != nil {
		return nil, &careplan.ScheduleError{
			Op:  opBulkUpsertArrivalSchedules,
			Err: fmt.Errorf("failed to find students for school class %s: %w", schoolClass, err),
		}
	}
	students = dropDepartedArrivalStudents(students)

	students, err = s.writeClassArrivalTimes(ctx, schoolClass, touched, updatedBy, students, authorize)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: err}
	}
	result := &careplan.BulkUpsertResult{OverwrittenStudents: make([]careplan.OverwriteWarning, 0)}
	result.StudentsAffected = len(students)
	for _, student := range students {
		result.AffectedStudentIDs = append(result.AffectedStudentIDs, student.ID)
	}
	return result, nil
}

func classArrivalTimeChanges(schedules []careplan.ArrivalScheduleInput) (map[int]string, error) {
	touched := make(map[int]string, len(schedules))
	for _, input := range schedules {
		if input.Weekday < 1 || input.Weekday > 5 {
			return nil, &careplan.ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: fmt.Errorf("invalid weekday %d", input.Weekday)}
		}
		if _, duplicate := touched[input.Weekday]; duplicate {
			return nil, &careplan.ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: fmt.Errorf("duplicate weekday %d", input.Weekday)}
		}
		touched[input.Weekday] = strings.TrimSpace(input.ArrivalTime)
	}
	return touched, nil
}

func (s *arrivalScheduleService) writeClassArrivalTimes(
	ctx context.Context,
	schoolClass string,
	touched map[int]string,
	updatedBy int64,
	students []ports.ArrivalBulkStudent,
	authorize func(context.Context, careplan.ScheduleStudent) (bool, error),
) ([]ports.ArrivalBulkStudent, error) {
	err := s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		var lockErr error
		students, lockErr = s.lockClassArrivalStudents(txCtx, schoolClass, students, authorize)
		if lockErr != nil {
			return lockErr
		}
		// The row may not exist yet, so its transaction advisory lock covers
		// concurrent first inserts as well as updates.
		if lockErr := s.classes.LockClass(txCtx, schoolClass); lockErr != nil {
			return fmt.Errorf("lock class arrival times for %s: %w", schoolClass, lockErr)
		}
		row, mergeErr := s.mergedClassRow(txCtx, schoolClass, touched, updatedBy)
		if mergeErr != nil {
			return mergeErr
		}
		if upsertErr := s.classes.Upsert(txCtx, row); upsertErr != nil {
			return fmt.Errorf("upsert class arrival times for %s: %w", schoolClass, upsertErr)
		}
		return nil
	})
	return students, err
}

func (s *arrivalScheduleService) lockClassArrivalStudents(
	ctx context.Context,
	schoolClass string,
	students []ports.ArrivalBulkStudent,
	authorize func(context.Context, careplan.ScheduleStudent) (bool, error),
) ([]ports.ArrivalBulkStudent, error) {
	// A class row affects every matched child, so authorize the complete class.
	sort.Slice(students, func(i, j int) bool { return students[i].ID < students[j].ID })
	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
	}
	lockedByID, err := s.students.LockByIDs(ctx, studentIDs, timezone.TodayDate())
	if err != nil {
		return nil, fmt.Errorf("lock selected students: %w", err)
	}
	locked := make([]ports.ArrivalBulkStudent, 0, len(students))
	for _, selected := range students {
		fresh, exists := lockedByID[selected.ID]
		if !exists {
			return nil, fmt.Errorf("%w: student %d", careplan.ErrBulkStudentNotFound, selected.ID)
		}
		if fresh.CareEnded {
			continue
		}
		if fresh.Alumnus || !strings.EqualFold(strings.TrimSpace(fresh.SchoolClass), schoolClass) {
			return nil, fmt.Errorf("%w: student %d", careplan.ErrBulkStudentNotFound, fresh.ID)
		}
		if err := authorizeArrivalStudent(ctx, fresh, authorize); err != nil {
			return nil, err
		}
		locked = append(locked, fresh)
	}
	return locked, nil
}

// mergedClassRow folds the touched weekdays into whatever the class already
// carries. An empty time clears that weekday.
func (s *arrivalScheduleService) mergedClassRow(
	ctx context.Context,
	schoolClass string,
	touched map[int]string,
	updatedBy int64,
) (*ports.ArrivalClassPlan, error) {
	existing, err := s.classes.FindByClasses(ctx, []string{schoolClass})
	if err != nil {
		return nil, fmt.Errorf("load class arrival times for %s: %w", schoolClass, err)
	}
	row := &ports.ArrivalClassPlan{SchoolClass: schoolClass, ArrivalTimes: map[string]string{}}
	if len(existing) > 0 && existing[0] != nil {
		row = existing[0]
		row.SchoolClass = schoolClass
		if row.ArrivalTimes == nil {
			row.ArrivalTimes = map[string]string{}
		}
	}
	for weekday, hhmm := range touched {
		day, ok := domain.ArrivalWeekdayKey(weekday)
		if !ok {
			continue
		}
		if hhmm == "" {
			delete(row.ArrivalTimes, day)
			continue
		}
		row.ArrivalTimes[day] = hhmm
	}
	if updatedBy > 0 {
		row.UpdatedBy = &updatedBy
	}
	normalized, err := domain.NormalizeClassArrivalTimes(row.ArrivalTimes)
	if err != nil {
		return nil, err
	}
	row.ArrivalTimes = normalized
	if strings.TrimSpace(row.SchoolClass) == "" {
		return nil, errors.New("school class is required")
	}
	return row, nil
}

// GetClassArrivalTimes returns what a class currently carries, so the
// maintenance screen shows the present state instead of empty fields, and can
// name when it was last touched (ADR 0005, "Bekannte Grenze").
func (s *arrivalScheduleService) GetClassArrivalTimes(
	ctx context.Context,
	schoolClass string,
) (*careplan.ClassArrivalTimes, error) {
	class := strings.TrimSpace(schoolClass)
	result := &careplan.ClassArrivalTimes{SchoolClass: class, Times: map[string]string{}}
	if class == "" || s.classes == nil {
		return result, nil
	}
	rows, err := s.classes.FindByClasses(ctx, []string{class})
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "get class arrival times", Err: err}
	}
	if len(rows) == 0 || rows[0] == nil {
		return result, nil
	}
	result.SchoolClass = rows[0].SchoolClass
	for day, hhmm := range rows[0].ArrivalTimes {
		result.Times[day] = hhmm
	}
	if !rows[0].UpdatedAt.IsZero() {
		updated := rows[0].UpdatedAt
		result.UpdatedAt = &updated
	}
	return result, nil
}
