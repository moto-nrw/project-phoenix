package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Closing days of a Träger (#4702). The table carries organization_id and the
// Träger policy of ADR 0105: the Träger-Büro reads and writes it only in the
// Träger transaction of the Träger it names, and every statement names that
// Träger once more in its own filter. A school reads the rows naming it in
// school_ids through its own tenant transaction (see ListClosingDays).

// traegerClosingDaySchoolConstraint is the name the school trigger of
// schedule.traeger_closing_days raises for a school outside the Träger.
const traegerClosingDaySchoolConstraint = "chk_traeger_closing_days_school_member"

// TraegerClosingDayStore persists the closing days of a Träger.
type TraegerClosingDayStore struct{}

// NewTraegerClosingDayStore builds the store.
func NewTraegerClosingDayStore() *TraegerClosingDayStore { return &TraegerClosingDayStore{} }

type traegerClosingDayRow struct {
	bun.BaseModel `bun:"table:schedule.traeger_closing_days"`

	ID        int64         `bun:"id"`
	StartDate calendar.Date `bun:"start_date"`
	EndDate   calendar.Date `bun:"end_date"`
	Reason    string        `bun:"reason"`
	SchoolIDs []int64       `bun:"school_ids,array"`
	UpdatedAt time.Time     `bun:"updated_at"`
}

const (
	traegerClosingDayColumns    = `id, start_date, end_date, reason, school_ids, updated_at`
	listTraegerClosingDaysQuery = `SELECT ` + traegerClosingDayColumns + ` FROM schedule.traeger_closing_days
		WHERE organization_id = ? AND start_date <= ? AND end_date >= ?
		ORDER BY start_date, id`
	insertTraegerClosingDayQuery = `INSERT INTO schedule.traeger_closing_days (organization_id, start_date, end_date, reason, school_ids)
		VALUES (?, ?, ?, ?, ?) RETURNING ` + traegerClosingDayColumns
	updateTraegerClosingDayQuery = `UPDATE schedule.traeger_closing_days
		SET start_date = ?, end_date = ?, reason = ?, school_ids = ?
		WHERE organization_id = ? AND id = ? RETURNING ` + traegerClosingDayColumns
	deleteTraegerClosingDayQuery = `DELETE FROM schedule.traeger_closing_days WHERE organization_id = ? AND id = ?`
)

// traegerDB returns the Träger transaction of organizationID. A school,
// administrative or foreign Träger transaction is refused.
func traegerDB(ctx context.Context, organizationID int64) (bun.IDB, error) {
	if organizationID <= 0 || tenant.TraegerTransactionFromContext(ctx) != organizationID {
		return nil, errors.New("school calendar postgres: the Träger transaction of this Träger is required")
	}
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("school calendar postgres: transaction is required")
	}
	tx, ok := raw.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("school calendar postgres: unsupported transaction %T", raw)
	}
	return tx, nil
}

// ListTraegerClosingDays lists the Träger's closing days touching [from, to].
func (s *TraegerClosingDayStore) ListTraegerClosingDays(ctx context.Context, organizationID int64, from, to calendar.Date) ([]domain.TraegerClosingDay, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []traegerClosingDayRow
	if err := db.NewRaw(listTraegerClosingDaysQuery, organizationID, to, from).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school calendar postgres: list Träger closing days: %w", err)
	}
	result := make([]domain.TraegerClosingDay, 0, len(rows))
	for _, row := range rows {
		result = append(result, traegerClosingDayToDomain(row))
	}
	return result, nil
}

// InsertTraegerClosingDay adds a closing day of the Träger.
func (s *TraegerClosingDayStore) InsertTraegerClosingDay(ctx context.Context, organizationID int64, input domain.TraegerClosingDayFields) (domain.TraegerClosingDay, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return domain.TraegerClosingDay{}, err
	}
	var row traegerClosingDayRow
	err = db.NewRaw(insertTraegerClosingDayQuery, organizationID, input.StartDate, input.EndDate, input.Reason,
		pgdialect.Array(input.SchoolIDs)).Scan(ctx, &row)
	if err != nil {
		return domain.TraegerClosingDay{}, traegerClosingDayWriteError("insert", err)
	}
	return traegerClosingDayToDomain(row), nil
}

// UpdateTraegerClosingDay replaces a closing day of the Träger.
func (s *TraegerClosingDayStore) UpdateTraegerClosingDay(ctx context.Context, organizationID, id int64, input domain.TraegerClosingDayFields) (domain.TraegerClosingDay, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return domain.TraegerClosingDay{}, err
	}
	var row traegerClosingDayRow
	err = db.NewRaw(updateTraegerClosingDayQuery, input.StartDate, input.EndDate, input.Reason,
		pgdialect.Array(input.SchoolIDs), organizationID, id).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TraegerClosingDay{}, domain.ErrTraegerClosingDayNotFound
	}
	if err != nil {
		return domain.TraegerClosingDay{}, traegerClosingDayWriteError("update", err)
	}
	return traegerClosingDayToDomain(row), nil
}

// DeleteTraegerClosingDay removes a closing day of the Träger.
func (s *TraegerClosingDayStore) DeleteTraegerClosingDay(ctx context.Context, organizationID, id int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.NewRaw(deleteTraegerClosingDayQuery, organizationID, id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("school calendar postgres: delete Träger closing day: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("school calendar postgres: delete Träger closing day: count rows: %w", err)
	}
	if rows != 1 {
		return domain.ErrTraegerClosingDayNotFound
	}
	return nil
}

func traegerClosingDayWriteError(operation string, err error) error {
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == traegerClosingDaySchoolConstraint {
		return domain.ErrTraegerClosingDaySchoolOutside
	}
	return fmt.Errorf("school calendar postgres: %s Träger closing day: %w", operation, err)
}

func traegerClosingDayToDomain(row traegerClosingDayRow) domain.TraegerClosingDay {
	schoolIDs := row.SchoolIDs
	if schoolIDs == nil {
		schoolIDs = []int64{}
	}
	return domain.TraegerClosingDay{
		ID: row.ID, StartDate: row.StartDate, EndDate: row.EndDate, Reason: row.Reason, SchoolIDs: schoolIDs, UpdatedAt: row.UpdatedAt,
	}
}
