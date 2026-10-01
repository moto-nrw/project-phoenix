package parentpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	countPreferenceTable     = "users.parent_message_count_preferences"
	countPreferenceTableExpr = `users.parent_message_count_preferences AS "count_preference"`
	countPreferenceAlias     = "count_preference"
)

// CountPreferenceStore persists which parent conversations a staff member's
// own counter counts (#3673). One row per (school, account); no row means the
// account never chose, and the caller treats that as its default.
type CountPreferenceStore struct{ store }

// NewCountPreferenceStore builds the preference store over the ambient
// transaction runtime.
func NewCountPreferenceStore(database Database) *CountPreferenceStore {
	return &CountPreferenceStore{store: newStore(database)}
}

type countPreferenceRow struct {
	bun.BaseModel `bun:"table:users.parent_message_count_preferences,alias:count_preference"`
	TenantID      int64  `bun:"tenant_id,pk"`
	AccountID     int64  `bun:"account_id,pk"`
	CountScope    string `bun:"count_scope,notnull"`
}

// CountScope returns the stored choice of the account in the current school,
// or "" when the account never chose.
func (s *CountPreferenceStore) CountScope(ctx context.Context, accountID int64) (string, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return "", err
	}
	var row countPreferenceRow
	query := db.NewSelect().
		Model(&row).
		ModelTableExpr(countPreferenceTableExpr).
		Column("count_scope").
		Where(`"count_preference".account_id = ?`, accountID)
	query = withTenant(query, countPreferenceAlias, tenantID)
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("find parent message count preference: %w", err)
	}
	return row.CountScope, nil
}

// SetCountScope stores the account's choice in the current school, replacing
// an earlier one. The caller validates the value; the table's CHECK is the
// last line.
func (s *CountPreferenceStore) SetCountScope(ctx context.Context, accountID int64, scope string) error {
	if accountID <= 0 {
		return errors.New("set parent message count preference: account ID is required")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if tenantID <= 0 {
		return errors.New("set parent message count preference: school is required")
	}
	row := &countPreferenceRow{TenantID: tenantID, AccountID: accountID, CountScope: scope}
	if _, err := db.NewInsert().
		Model(row).
		ModelTableExpr(countPreferenceTable).
		On("CONFLICT (tenant_id, account_id) DO UPDATE").
		Set("count_scope = EXCLUDED.count_scope").
		Exec(ctx); err != nil {
		return fmt.Errorf("upsert parent message count preference: %w", err)
	}
	return nil
}
