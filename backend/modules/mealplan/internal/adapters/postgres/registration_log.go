package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

type registrationChangeRow struct {
	bun.BaseModel `bun:"table:meal_registration_changes,alias:meal_registration_change"`
	TenantID      int64         `bun:"tenant_id"`
	StudentID     int64         `bun:"student_id"`
	AccountID     int64         `bun:"account_id"`
	BySchool      bool          `bun:"by_school"`
	Kind          string        `bun:"kind"`
	Date          calendar.Date `bun:"date,type:date"`
	Participating *bool         `bun:"participating"`
	Weekdays      []int16       `bun:"weekdays,array"`
}

// InsertRegistrationChanges appends entries to the registration log in one
// statement (#3873). The log only grows; it goes with the child.
func (s *Store) InsertRegistrationChanges(ctx context.Context, changes []domain.RegistrationChange) (domain.OperationStats, error) {
	if len(changes) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	rows := make([]registrationChangeRow, 0, len(changes))
	for _, change := range changes {
		row := registrationChangeRow{TenantID: tenantID, StudentID: change.StudentID, AccountID: change.AccountID, BySchool: change.BySchool, Kind: string(change.Kind), Date: change.Date, Participating: change.Participating}
		for _, weekday := range change.Weekdays {
			row.Weekdays = append(row.Weekdays, int16(weekday))
		}
		rows = append(rows, row)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewInsert().Model(&rows).ModelTableExpr(`schedule.meal_registration_changes`).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: log registration changes: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}
