package fixed

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
)

// seedWorkSessions creates realistic historical work session data for the WeekChart.
// Generates sessions for the previous week through yesterday (skipping weekends and today).
func (s *Seeder) seedWorkSessions(ctx context.Context) error {
	today := time.Now().Truncate(24 * time.Hour)

	// Find Monday of PREVIOUS week to get ~2 weeks of data
	weekday := today.Weekday()
	mondayOffset := int(weekday - time.Monday)
	if weekday == time.Sunday {
		mondayOffset = 6
	}
	monday := today.AddDate(0, 0, -mondayOffset-7)

	staffCount := min(5, len(s.result.Staff))
	if staffCount == 0 {
		if s.verbose {
			log.Printf("No staff found, skipping work sessions")
		}
		return nil
	}

	for staffIdx, staff := range s.result.Staff[:staffCount] {
		for d := monday; d.Before(today); d = d.AddDate(0, 0, 1) {
			if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
				continue
			}

			dayIdx := int(d.Weekday()) - 1 // Mon=0, Tue=1, ...

			checkIn := d.Add(7*time.Hour + 30*time.Minute +
				time.Duration(staffIdx*15)*time.Minute)
			checkOut := d.Add(15*time.Hour + 30*time.Minute +
				time.Duration(staffIdx*20+dayIdx*10)*time.Minute)

			// Default 30min break, bump to 45min when net work > 9h (§4 ArbZG)
			breakMins := 30
			if checkOut.Sub(checkIn).Minutes()-30 > 540 {
				breakMins = 45
			}

			status := active.WorkSessionStatusPresent
			if staffIdx%3 == 1 && dayIdx%2 == 0 {
				status = active.WorkSessionStatusHomeOffice
			}

			session := &active.WorkSession{
				StaffID:      staff.ID,
				Date:         d,
				Status:       status,
				CheckInTime:  checkIn,
				CheckOutTime: &checkOut,
				BreakMinutes: breakMins,
				CreatedBy:    staff.ID,
			}
			session.CreatedAt = time.Now()
			session.UpdatedAt = time.Now()

			_, err := s.tx.NewInsert().Model(session).
				ModelTableExpr("active.work_sessions").
				On("CONFLICT (staff_id, date) DO UPDATE").
				Set("check_in_time = EXCLUDED.check_in_time").
				Set("check_out_time = EXCLUDED.check_out_time").
				Set("break_minutes = EXCLUDED.break_minutes").
				Set("status = EXCLUDED.status").
				Set(SQLExcludedUpdatedAt).
				Returning(SQLBaseColumns).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to upsert work session for staff %d on %s: %w",
					staff.ID, d.Format("2006-01-02"), err)
			}

			s.result.WorkSessions = append(s.result.WorkSessions, session)
		}
	}

	if s.verbose {
		log.Printf("Created %d work sessions", len(s.result.WorkSessions))
	}
	return nil
}
