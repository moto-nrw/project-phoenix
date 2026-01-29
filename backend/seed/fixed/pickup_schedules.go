package fixed

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Typical pickup times for school children (afternoon)
var pickupTimes = []string{
	"14:00", "14:30", "15:00", "15:30", "16:00", "16:30", "17:00",
}

// Notes that might be added to pickup schedules
var scheduleNotes = []string{
	"Mit Schwester",
	"Mit Bruder",
	"Oma holt ab",
	"Opa holt ab",
	"Tante holt ab",
	"Nachbarin holt ab",
	"Fahrgemeinschaft",
	"Geht alleine",
	"Hort bis 16 Uhr",
	"",
	"",
	"", // Empty notes are more common
}

// Reasons for pickup exceptions
var exceptionReasons = []string{
	"Arzttermin",
	"Zahnarzt",
	"Therapie",
	"Familientermin",
	"Geburtstag",
	"Ausflug",
	"Früher abholen wegen Termin",
	"Oma kommt zu Besuch",
	"Geschwisterkind hat Termin",
}

// seedPickupSchedules creates weekly pickup schedules and exceptions for students
func (s *Seeder) seedPickupSchedules(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	createdBy := s.result.AdminAccount.ID

	scheduleCount := 0
	exceptionCount := 0

	for _, student := range s.result.Students {
		existingCount, err := s.countExistingSchedules(ctx, student.ID)
		if err != nil {
			return err
		}

		if existingCount > 0 {
			scheduleCount += existingCount
			continue
		}

		// 90% of students have pickup schedules
		if rng.Float32() > 0.90 {
			continue
		}

		count, err := s.seedStudentWeeklySchedules(ctx, rng, student, createdBy)
		if err != nil {
			return err
		}
		scheduleCount += count

		count, err = s.seedStudentExceptions(ctx, rng, student, createdBy)
		if err != nil {
			return err
		}
		exceptionCount += count
	}

	if s.verbose {
		log.Printf("Created %d pickup schedules and %d exceptions for students",
			scheduleCount, exceptionCount)
	}

	return nil
}

// countExistingSchedules returns the number of existing pickup schedules for a student
func (s *Seeder) countExistingSchedules(ctx context.Context, studentID int64) (int, error) {
	count, err := s.tx.NewSelect().
		Table("schedule.student_pickup_schedules").
		Where("student_id = ?", studentID).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to check existing schedules for student %d: %w", studentID, err)
	}
	return count, nil
}

// seedStudentWeeklySchedules creates pickup schedules for each weekday
func (s *Seeder) seedStudentWeeklySchedules(ctx context.Context, rng *rand.Rand, student *users.Student, createdBy int64) (int, error) {
	baseTimeIdx := rng.Intn(len(pickupTimes))
	count := 0

	for weekday := schedule.WeekdayMonday; weekday <= schedule.WeekdayFriday; weekday++ {
		// 85% chance to have a schedule for each day
		if rng.Float32() > 0.85 {
			continue
		}

		if err := s.createDaySchedule(ctx, rng, student.ID, weekday, baseTimeIdx, createdBy); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// createDaySchedule creates a single pickup schedule for a specific weekday
func (s *Seeder) createDaySchedule(ctx context.Context, rng *rand.Rand, studentID int64, weekday int, baseTimeIdx int, createdBy int64) error {
	timeIdx := baseTimeIdx + rng.Intn(3) - 1
	timeIdx = max(0, min(timeIdx, len(pickupTimes)-1))

	pickupTime, err := parseTimeOnly(pickupTimes[timeIdx])
	if err != nil {
		return fmt.Errorf("failed to parse pickup time: %w", err)
	}

	pickupSchedule := &schedule.StudentPickupSchedule{
		StudentID:  studentID,
		Weekday:    weekday,
		PickupTime: pickupTime,
		Notes:      generateRandomNote(rng),
		CreatedBy:  createdBy,
	}
	pickupSchedule.CreatedAt = time.Now()
	pickupSchedule.UpdatedAt = time.Now()

	_, err = s.tx.NewInsert().
		Model(pickupSchedule).
		ModelTableExpr("schedule.student_pickup_schedules").
		Returning(SQLBaseColumns).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create pickup schedule for student %d: %w", studentID, err)
	}

	return nil
}

// generateRandomNote returns a random note with 30% probability
func generateRandomNote(rng *rand.Rand) *string {
	if rng.Float32() >= 0.30 {
		return nil
	}
	note := scheduleNotes[rng.Intn(len(scheduleNotes))]
	if note == "" {
		return nil
	}
	return &note
}

// seedStudentExceptions creates pickup exceptions for a student (40% chance)
func (s *Seeder) seedStudentExceptions(ctx context.Context, rng *rand.Rand, student *users.Student, createdBy int64) (int, error) {
	// 40% of students with schedules have exceptions
	if rng.Float32() >= 0.40 {
		return 0, nil
	}

	numExceptions := 1
	if rng.Float32() < 0.3 {
		numExceptions = 2
	}

	count := 0
	for i := 0; i < numExceptions; i++ {
		created, err := s.createException(ctx, rng, student.ID, createdBy)
		if err != nil {
			return count, err
		}
		if created {
			count++
		}
	}

	return count, nil
}

// createException creates a single pickup exception for a student
func (s *Seeder) createException(ctx context.Context, rng *rand.Rand, studentID int64, createdBy int64) (bool, error) {
	exceptionDate := generateExceptionDate(rng)

	exists, _ := s.tx.NewSelect().
		Table("schedule.student_pickup_exceptions").
		Where("student_id = ?", studentID).
		Where("exception_date = ?", exceptionDate.Format("2006-01-02")).
		Exists(ctx)
	if exists {
		return false, nil
	}

	exceptionTimeIdx := rng.Intn(4) // Earlier times (14:00-15:30)
	exceptionTime, err := parseTimeOnly(pickupTimes[exceptionTimeIdx])
	if err != nil {
		return false, fmt.Errorf("failed to parse exception time: %w", err)
	}

	reason := exceptionReasons[rng.Intn(len(exceptionReasons))]
	exception := &schedule.StudentPickupException{
		StudentID:     studentID,
		ExceptionDate: exceptionDate,
		PickupTime:    &exceptionTime,
		Reason:        &reason,
		CreatedBy:     createdBy,
	}
	exception.CreatedAt = time.Now()
	exception.UpdatedAt = time.Now()

	_, err = s.tx.NewInsert().
		Model(exception).
		ModelTableExpr("schedule.student_pickup_exceptions").
		Returning(SQLBaseColumns).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to create pickup exception for student %d: %w", studentID, err)
	}

	return true, nil
}

// generateExceptionDate generates a random weekday within the next 2 weeks
// Returns UTC midnight of the date in Berlin timezone for consistent DB storage
func generateExceptionDate(rng *rand.Rand) time.Time {
	daysFromNow := rng.Intn(14) + 1
	// Use Berlin timezone to ensure consistent date calculation
	date := timezone.Now().AddDate(0, 0, daysFromNow)

	// Skip weekends
	for date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		date = date.AddDate(0, 0, 1)
	}

	// Return as UTC midnight for consistent DB storage
	return timezone.DateOfUTC(date)
}

// parseTimeOnly parses a time string (HH:MM) into a time.Time with only hour/minute set
func parseTimeOnly(timeStr string) (time.Time, error) {
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, err
	}
	// Return time with a valid date (year 2000, only time portion matters for storage)
	return time.Date(2000, 1, 1, t.Hour(), t.Minute(), 0, 0, time.UTC), nil
}
