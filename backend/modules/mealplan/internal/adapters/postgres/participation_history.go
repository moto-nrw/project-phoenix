package postgres

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/mealplan"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// InsertParticipationHistory uses the existing registration's author and
// tenant, and preserves existing overrides on retries, including cancellations.
func (s *Store) InsertParticipationHistory(ctx context.Context, days []mealplan.PastMealDay) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	ids, dates := make([]int64, len(days)), make([]string, len(days))
	for i, day := range days {
		ids[i], dates[i] = day.StudentID, day.Date.String()
	}
	_, err = db.NewRaw(`INSERT INTO schedule.meal_participation_overrides
 (tenant_id,student_id,date,participating,changed_by_account_id)
 SELECT ?, day.student_id, day.date::date, true, registration.changed_by_account_id
 FROM unnest(?::bigint[], ?::text[]) AS day(student_id,date)
 JOIN LATERAL (SELECT changed_by_account_id FROM schedule.meal_participation_schedules
 WHERE tenant_id = ? AND student_id = day.student_id ORDER BY effective_from LIMIT 1) AS registration ON true
 ON CONFLICT (tenant_id,student_id,date) DO NOTHING`, tenantID, pgdialect.Array(ids), pgdialect.Array(dates), tenantID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("meal plan: record past registrations: %w", err)
	}
	return nil
}
