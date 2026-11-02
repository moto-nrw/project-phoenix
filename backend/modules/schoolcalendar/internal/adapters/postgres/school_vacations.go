package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// SchoolVacationStore persists the school vacations of every federal state
// and the schools' confirmations of planned care (#5345). It always works in
// the transaction of its context: a tenant transaction for the school reads
// and confirmations, the administrative one for the sync.
type SchoolVacationStore struct{}

func NewSchoolVacationStore() *SchoolVacationStore { return &SchoolVacationStore{} }

const schoolVacationTable = `calendar.school_vacations AS "school_vacation"`

type schoolVacationRow struct {
	bun.BaseModel `bun:"table:calendar.school_vacations,alias:school_vacation"`
	ID            int64         `bun:"id,pk,autoincrement"`
	RegionCode    string        `bun:"region_code,notnull"`
	Name          string        `bun:"name,notnull"`
	StartDate     calendar.Date `bun:"start_date,notnull,type:date"`
	EndDate       calendar.Date `bun:"end_date,notnull,type:date"`
	SyncedAt      time.Time     `bun:"synced_at,nullzero,notnull,default:current_timestamp"`
}

type schoolVacationAcknowledgementRow struct {
	bun.BaseModel           `bun:"table:calendar.school_vacation_acknowledgements,alias:school_vacation_acknowledgement"`
	ID                      int64         `bun:"id,pk,autoincrement"`
	TenantID                int64         `bun:"tenant_id,notnull"`
	RegionCode              string        `bun:"region_code,notnull"`
	StartDate               calendar.Date `bun:"start_date,notnull,type:date"`
	EndDate                 calendar.Date `bun:"end_date,notnull,type:date"`
	AcknowledgedByAccountID *int64        `bun:"acknowledged_by_account_id"`
	AcknowledgedAt          time.Time     `bun:"acknowledged_at,nullzero,notnull,default:current_timestamp"`
}

type schoolVacationListRow struct {
	Name         string        `bun:"name"`
	StartDate    calendar.Date `bun:"start_date"`
	EndDate      calendar.Date `bun:"end_date"`
	Acknowledged bool          `bun:"acknowledged"`
}

func (s *SchoolVacationStore) ListSchoolVacations(ctx context.Context, region, from, to string) ([]schoolcalendar.SchoolVacation, error) {
	db, schoolID, err := tenantTransaction(ctx)
	if err != nil {
		return nil, err
	}
	var rows []schoolVacationListRow
	err = db.NewSelect().
		TableExpr(`calendar.school_vacations AS "school_vacation"`).
		ColumnExpr(`"school_vacation".name, "school_vacation".start_date, "school_vacation".end_date`).
		ColumnExpr(`EXISTS (SELECT 1 FROM calendar.school_vacation_acknowledgements AS "ack"
			WHERE "ack".tenant_id = ? AND "ack".region_code = "school_vacation".region_code
			AND "ack".start_date = "school_vacation".start_date AND "ack".end_date = "school_vacation".end_date) AS acknowledged`, schoolID).
		Where(`"school_vacation".region_code = ?`, region).
		Where(`"school_vacation".end_date >= ?`, calendar.Date(from)).
		Where(`"school_vacation".start_date <= ?`, calendar.Date(to)).
		OrderExpr(`"school_vacation".start_date ASC, "school_vacation".end_date ASC`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("list school vacations: %w", err)
	}
	result := make([]schoolcalendar.SchoolVacation, 0, len(rows))
	for _, row := range rows {
		result = append(result, schoolcalendar.SchoolVacation{
			Name: row.Name, StartDate: row.StartDate.String(), EndDate: row.EndDate.String(), Acknowledged: row.Acknowledged,
		})
	}
	return result, nil
}

func (s *SchoolVacationStore) LastSchoolVacationSync(ctx context.Context, region string) (*time.Time, error) {
	db, err := tenant.CurrentTransaction[bun.IDB](ctx)
	if err != nil {
		return nil, fmt.Errorf("school vacations: %w", err)
	}
	var synced *time.Time
	err = db.NewSelect().Model((*schoolVacationRow)(nil)).ModelTableExpr(schoolVacationTable).
		ColumnExpr(`MAX("school_vacation".synced_at)`).
		Where(`"school_vacation".region_code = ?`, region).
		Scan(ctx, &synced)
	if err != nil {
		return nil, fmt.Errorf("read school vacation sync: %w", err)
	}
	return synced, nil
}

func (s *SchoolVacationStore) HasSchoolVacation(ctx context.Context, region, start, end string) (bool, error) {
	db, err := tenant.CurrentTransaction[bun.IDB](ctx)
	if err != nil {
		return false, fmt.Errorf("school vacations: %w", err)
	}
	exists, err := db.NewSelect().Model((*schoolVacationRow)(nil)).ModelTableExpr(schoolVacationTable).
		Where(`"school_vacation".region_code = ?`, region).
		Where(`"school_vacation".start_date = ?`, calendar.Date(start)).
		Where(`"school_vacation".end_date = ?`, calendar.Date(end)).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("find school vacation: %w", err)
	}
	return exists, nil
}

func (s *SchoolVacationStore) AcknowledgeSchoolVacation(ctx context.Context, region, start, end string, accountID *int64) error {
	db, schoolID, err := tenantTransaction(ctx)
	if err != nil {
		return err
	}
	row := &schoolVacationAcknowledgementRow{
		TenantID: schoolID, RegionCode: region, StartDate: calendar.Date(start), EndDate: calendar.Date(end),
		AcknowledgedByAccountID: accountID,
	}
	if _, err := db.NewInsert().Model(row).ModelTableExpr(`calendar.school_vacation_acknowledgements`).
		Column("tenant_id", "region_code", "start_date", "end_date", "acknowledged_by_account_id").
		On("CONFLICT (tenant_id, region_code, start_date, end_date) DO NOTHING").
		Exec(ctx); err != nil {
		return fmt.Errorf("acknowledge school vacation: %w", err)
	}
	return nil
}

func (s *SchoolVacationStore) ReplaceSchoolVacations(ctx context.Context, region, from, to string, records []domain.SchoolVacationRecord) (int, error) {
	db, err := tenant.CurrentTransaction[bun.IDB](ctx)
	if err != nil {
		return 0, fmt.Errorf("school vacations: %w", err)
	}
	if _, err := db.NewDelete().Model((*schoolVacationRow)(nil)).ModelTableExpr(schoolVacationTable).
		Where(`"school_vacation".region_code = ?`, region).
		Where(`"school_vacation".end_date >= ?`, calendar.Date(from)).
		Where(`"school_vacation".start_date <= ?`, calendar.Date(to)).
		Exec(ctx); err != nil {
		return 0, fmt.Errorf("remove stored school vacations: %w", err)
	}
	if len(records) == 0 {
		return 0, nil
	}
	rows := make([]schoolVacationRow, 0, len(records))
	for _, record := range records {
		rows = append(rows, schoolVacationRow{
			RegionCode: region, Name: record.Name, StartDate: calendar.Date(record.StartDate), EndDate: calendar.Date(record.EndDate),
		})
	}
	result, err := db.NewInsert().Model(&rows).ModelTableExpr(`calendar.school_vacations`).
		Column("region_code", "name", "start_date", "end_date").
		On("CONFLICT (region_code, start_date, end_date) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("store school vacations: %w", err)
	}
	stored, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count stored school vacations: %w", err)
	}
	return int(stored), nil
}

// tenantTransaction resolves the tenant transaction and its school; a read
// or write of the confirmations without a school would lose its predicate.
func tenantTransaction(ctx context.Context) (bun.IDB, int64, error) {
	db, err := tenant.CurrentTransaction[bun.IDB](ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("school vacations: %w", err)
	}
	schoolID := tenant.FromContext(ctx)
	if schoolID <= 0 {
		return nil, 0, errors.New("school vacations: tenant is required")
	}
	return db, schoolID, nil
}
