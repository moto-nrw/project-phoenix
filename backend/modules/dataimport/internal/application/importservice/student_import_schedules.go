package importpkg

import (
	"context"
	"fmt"
	"strings"
	"time"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func parseWallClock(value string) (time.Time, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return time.Time{}, err
	}
	// Use a valid reference date: time.Parse("15:04") produces year 0000,
	// which PostgreSQL rejects.
	return timezone.NormalizeWallClock(parsed), nil
}

// createArrivalSchedules creates weekly arrival schedule records for a student.
func (c *StudentImportConfig) createArrivalSchedules(ctx context.Context, studentID int64, schedules []importModels.ArrivalScheduleImportData) error {
	if len(schedules) == 0 {
		return nil
	}
	for i, sched := range schedules {
		arrival, err := parseWallClock(sched.ExpectedArrival)
		if err != nil {
			return fmt.Errorf("arrival schedule %d: invalid time '%s': %w", i+1, sched.ExpectedArrival, err)
		}
		if _, err := c.Schedules.CreateArrivalSchedule(ctx, ports.ArrivalSchedule{
			StudentID: studentID, Weekday: sched.Weekday, ExpectedArrival: arrival,
			Notes: importModels.OptionalCell(sched.Notes), CreatedBy: ImporterIDFromContext(ctx),
		}); err != nil {
			return fmt.Errorf("create arrival schedule (weekday %d): %w", sched.Weekday, err)
		}
	}
	return nil
}

// createPickupSchedules creates weekly pickup schedule records for a student.
func (c *StudentImportConfig) createPickupSchedules(ctx context.Context, studentID int64, schedules []importModels.PickupScheduleImportData) error {
	if len(schedules) == 0 {
		return nil
	}
	for i, sched := range schedules {
		pickup, err := parseWallClock(sched.PickupTime)
		if err != nil {
			return fmt.Errorf("pickup schedule %d: invalid time '%s': %w", i+1, sched.PickupTime, err)
		}
		if _, err := c.Schedules.CreatePickupSchedule(ctx, ports.PickupSchedule{
			StudentID: studentID, Weekday: sched.Weekday, PickupTime: pickup,
			Notes: importModels.OptionalCell(sched.Notes), CreatedBy: ImporterIDFromContext(ctx), Source: ports.ScheduleSourceStaff,
		}); err != nil {
			return fmt.Errorf("create pickup schedule (weekday %d): %w", sched.Weekday, err)
		}
	}
	return nil
}

func (c *StudentImportConfig) upsertArrivalSchedules(ctx context.Context, studentID int64, schedules []importModels.ArrivalScheduleImportData) error {
	if len(schedules) == 0 {
		return nil
	}
	for _, sched := range schedules {
		if err := c.upsertArrivalSchedule(ctx, studentID, sched); err != nil {
			return err
		}
	}
	return nil
}

func (c *StudentImportConfig) upsertPickupSchedules(ctx context.Context, studentID int64, schedules []importModels.PickupScheduleImportData) error {
	if len(schedules) == 0 {
		return nil
	}
	for _, sched := range schedules {
		if err := c.upsertPickupSchedule(ctx, studentID, sched); err != nil {
			return err
		}
	}
	return nil
}

func (c *StudentImportConfig) upsertArrivalSchedule(ctx context.Context, studentID int64, sched importModels.ArrivalScheduleImportData) error {
	existing, err := c.Schedules.ListArrivalSchedules(ctx, ports.StudentScheduleFilter{StudentIDs: []int64{studentID}, Weekday: sched.Weekday})
	if err != nil {
		return copyErrorf("Ankunftszeit für Wochentag %d laden: %w", sched.Weekday, err)
	}
	if len(existing) == 0 {
		if err := c.createArrivalSchedules(ctx, studentID, []importModels.ArrivalScheduleImportData{sched}); err != nil {
			return err
		}
		return nil
	}
	arrival, err := parseWallClock(sched.ExpectedArrival)
	if err != nil {
		return copyErrorf("Ungültige Ankunftszeit '%s': %w", sched.ExpectedArrival, err)
	}
	current := existing[0]
	current.ExpectedArrival = arrival
	if strings.TrimSpace(sched.Notes) != "" {
		current.Notes = importModels.OptionalCell(sched.Notes)
	}
	if err := c.Schedules.UpdateArrivalSchedule(ctx, current); err != nil {
		return copyErrorf("Ankunftszeit für Wochentag %d aktualisieren: %w", sched.Weekday, err)
	}
	return nil
}

func (c *StudentImportConfig) upsertPickupSchedule(ctx context.Context, studentID int64, sched importModels.PickupScheduleImportData) error {
	existing, err := c.Schedules.ListPickupSchedules(ctx, ports.StudentScheduleFilter{StudentIDs: []int64{studentID}, Weekday: sched.Weekday})
	if err != nil {
		return copyErrorf("Abholzeit für Wochentag %d laden: %w", sched.Weekday, err)
	}
	if len(existing) == 0 {
		if err := c.createPickupSchedules(ctx, studentID, []importModels.PickupScheduleImportData{sched}); err != nil {
			return err
		}
		return nil
	}
	pickup, err := parseWallClock(sched.PickupTime)
	if err != nil {
		return copyErrorf("Ungültige Abholzeit '%s': %w", sched.PickupTime, err)
	}
	current := existing[0]
	current.PickupTime = pickup
	if strings.TrimSpace(sched.Notes) != "" {
		current.Notes = importModels.OptionalCell(sched.Notes)
	}
	if err := c.Schedules.UpdatePickupSchedule(ctx, current); err != nil {
		return copyErrorf("Abholzeit für Wochentag %d aktualisieren: %w", sched.Weekday, err)
	}
	return nil
}
