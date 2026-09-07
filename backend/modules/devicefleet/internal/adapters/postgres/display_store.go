package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/uptrace/bun"
)

const displayTableExpr = `display.displays AS "display"`

// displayRow is this owner's private mapping of display.displays.
type displayRow struct {
	bun.BaseModel `bun:"table:displays,alias:display"`

	ID        int64     `bun:"id,pk,autoincrement"`
	TenantID  int64     `bun:"tenant_id,notnull"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name      string    `bun:"name,notnull"`
	IsActive  bool      `bun:"is_active,notnull,default:true"`
	TokenHash string    `bun:"token_hash,notnull"`
}

// DisplayStore reads and writes display.displays for the Device Fleet owner.
type DisplayStore struct{ database Database }

// NewDisplayStore builds the display adapter over the supplied runtime.
func NewDisplayStore(database Database) *DisplayStore {
	if database == nil {
		panic("devicefleet postgres: database runtime is required")
	}
	return &DisplayStore{database: database}
}

// Create registers one display for the caller's tenant.
func (s *DisplayStore) Create(ctx context.Context, input domain.CreateDisplay) (domain.Display, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Display{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.Display{}, domain.OperationStats{}, errors.New("devicefleet postgres: tenant is required to create a display")
	}
	row := displayRow{TenantID: tenantID, Name: input.Name, IsActive: true, TokenHash: input.TokenHash}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`display.displays`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Display{}, stats, fmt.Errorf("devicefleet postgres: create display: %w", err)
	}
	stats.Rows = 1
	return toDisplayDomain(row), stats, nil
}

// Update writes exactly the supplied columns and reports how many rows
// matched, so a concurrent delete surfaces as zero instead of a false success.
func (s *DisplayStore) Update(ctx context.Context, input domain.UpdateDisplay) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	row := displayRow{ID: input.ID, UpdatedAt: time.Now()}
	columns := []string{"updated_at"}
	if input.Name != nil {
		row.Name = *input.Name
		columns = append(columns, "name")
	}
	if input.IsActive != nil {
		row.IsActive = *input.IsActive
		columns = append(columns, "is_active")
	}
	if input.TokenHash != nil {
		row.TokenHash = *input.TokenHash
		columns = append(columns, "token_hash")
	}
	query := db.NewUpdate().Model(&row).
		ModelTableExpr(displayTableExpr).
		Column(columns...).
		Where(`"display".id = ?`, input.ID)
	if tenantID > 0 {
		query = query.Where(`"display".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: update display: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: count updated displays: %w", err)
	}
	stats.Rows = affected
	return affected, stats, nil
}

// Delete removes one display and reports how many rows matched.
func (s *DisplayStore) Delete(ctx context.Context, id int64) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewDelete().Model((*displayRow)(nil)).
		ModelTableExpr(displayTableExpr).
		Where(`"display".id = ?`, id)
	if tenantID > 0 {
		query = query.Where(`"display".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: delete display: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: count deleted displays: %w", err)
	}
	stats.Rows = affected
	return affected, stats, nil
}

// FindByID reads one display of the caller's tenant.
func (s *DisplayStore) FindByID(ctx context.Context, id int64) (domain.Display, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Display{}, false, domain.OperationStats{}, err
	}
	row := displayRow{}
	query := db.NewSelect().Model(&row).ModelTableExpr(displayTableExpr).Where(`"display".id = ?`, id)
	if tenantID > 0 {
		query = query.Where(`"display".tenant_id = ?`, tenantID)
	}
	return scanDisplay(ctx, query, &row, "find display")
}

// FindByTokenHash resolves a display from its token hash WITHOUT a tenant
// predicate. CONTRACT: only the admin scope may call it — the token is the
// only auth signal and the tenant is not known yet. The returned TenantID
// then scopes every downstream query.
func (s *DisplayStore) FindByTokenHash(ctx context.Context, tokenHash string) (domain.Display, bool, domain.OperationStats, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return domain.Display{}, false, domain.OperationStats{}, err
	}
	row := displayRow{}
	query := db.NewSelect().Model(&row).
		ModelTableExpr(displayTableExpr).
		Where(`"display".token_hash = ?`, tokenHash)
	return scanDisplay(ctx, query, &row, "find display by token hash")
}

func scanDisplay(
	ctx context.Context,
	query *bun.SelectQuery,
	row *displayRow,
	operation string,
) (domain.Display, bool, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Display{}, false, stats, nil
	}
	if err != nil {
		return domain.Display{}, false, stats, fmt.Errorf("devicefleet postgres: %s: %w", operation, err)
	}
	stats.Rows = 1
	return toDisplayDomain(*row), true, stats, nil
}

// List reads every display of the caller's tenant.
func (s *DisplayStore) List(ctx context.Context) ([]domain.Display, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []displayRow
	query := db.NewSelect().Model(&rows).ModelTableExpr(displayTableExpr)
	if tenantID > 0 {
		query = query.Where(`"display".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("devicefleet postgres: list displays: %w", err)
	}
	stats.Rows = int64(len(rows))
	displays := make([]domain.Display, 0, len(rows))
	for _, row := range rows {
		displays = append(displays, toDisplayDomain(row))
	}
	return displays, stats, nil
}

func toDisplayDomain(row displayRow) domain.Display {
	return domain.Display{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, IsActive: row.IsActive, TokenHash: row.TokenHash,
	}
}
