package fixed

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
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

	// Get admin account ID for created_by
	createdBy := s.result.AdminAccount.ID

	scheduleCount := 0
	exceptionCount := 0

	for _, student := range s.result.Students {
		// Check if student already has pickup schedules
		existingCount, err := s.tx.NewSelect().
			Table("schedule.student_pickup_schedules").
			Where("student_id = ?", student.ID).
			Count(ctx)
		if err != nil {
			return fmt.Errorf("failed to check existing schedules for student %d: %w", student.ID, err)
		}

		// Skip if student already has schedules
		if existingCount > 0 {
			scheduleCount += existingCount
			continue
		}

		// 90% of students have pickup schedules
		if rng.Float32() > 0.90 {
			continue
		}

		// Generate a base pickup time for this student (varies slightly by day)
		baseTimeIdx := rng.Intn(len(pickupTimes))

		// Create schedule for each weekday (Mon-Fri)
		for weekday := schedule.WeekdayMonday; weekday <= schedule.WeekdayFriday; weekday++ {
			// 85% chance to have a schedule for each day
			if rng.Float32() > 0.85 {
				continue
			}

			// Vary the time slightly from base (+/- 1 slot)
			timeIdx := baseTimeIdx + rng.Intn(3) - 1
			timeIdx = max(0, min(timeIdx, len(pickupTimes)-1))

			pickupTime, err := parseTimeOnly(pickupTimes[timeIdx])
			if err != nil {
				return fmt.Errorf("failed to parse pickup time: %w", err)
			}

			// Maybe add a note (30% chance)
			var notes *string
			if rng.Float32() < 0.30 {
				note := scheduleNotes[rng.Intn(len(scheduleNotes))]
				if note != "" {
					notes = &note
				}
			}

			pickupSchedule := &schedule.StudentPickupSchedule{
				StudentID:  student.ID,
				Weekday:    weekday,
				PickupTime: pickupTime,
				Notes:      notes,
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
				return fmt.Errorf("failed to create pickup schedule for student %d: %w", student.ID, err)
			}

			scheduleCount++
		}

		// 40% of students with schedules also have 1-2 exceptions
		if rng.Float32() < 0.40 {
			numExceptions := 1
			if rng.Float32() < 0.3 {
				numExceptions = 2
			}

			for i := 0; i < numExceptions; i++ {
				// Exception date: within next 2 weeks
				daysFromNow := rng.Intn(14) + 1
				exceptionDate := time.Now().AddDate(0, 0, daysFromNow)

				// Skip weekends
				for exceptionDate.Weekday() == time.Saturday || exceptionDate.Weekday() == time.Sunday {
					exceptionDate = exceptionDate.AddDate(0, 0, 1)
				}

				// Check if exception already exists for this date
				existingException, _ := s.tx.NewSelect().
					Table("schedule.student_pickup_exceptions").
					Where("student_id = ?", student.ID).
					Where("exception_date = ?", exceptionDate.Format("2006-01-02")).
					Exists(ctx)
				if existingException {
					continue
				}

				// Exception pickup time (usually earlier than regular)
				exceptionTimeIdx := rng.Intn(4) // Earlier times (14:00-15:30)
				exceptionTime, err := parseTimeOnly(pickupTimes[exceptionTimeIdx])
				if err != nil {
					return fmt.Errorf("failed to parse exception time: %w", err)
				}

				reason := exceptionReasons[rng.Intn(len(exceptionReasons))]

				exception := &schedule.StudentPickupException{
					StudentID:     student.ID,
					ExceptionDate: exceptionDate,
					PickupTime:    &exceptionTime,
					Reason:        reason,
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
					return fmt.Errorf("failed to create pickup exception for student %d: %w", student.ID, err)
				}

				exceptionCount++
			}
		}
	}

	if s.verbose {
		log.Printf("Created %d pickup schedules and %d exceptions for students",
			scheduleCount, exceptionCount)
	}

	return nil
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
