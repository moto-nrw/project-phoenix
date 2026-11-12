package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) FindSchoolEmailIdentities(ctx context.Context, emails []string, tenantID int64) (map[string]int64, map[string][]int64, domain.OperationStats, error) {
	accounts := make(map[string]int64)
	invited := make(map[string][]int64)
	if len(emails) == 0 {
		return accounts, invited, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, nil, domain.OperationStats{}, err
	}
	var rows []struct {
		Email     string
		AccountID int64
		PersonID  int64
	}
	started := time.Now()
	err = db.NewRaw(`SELECT LOWER(a.email) AS email, a.id AS account_id, 0::bigint AS person_id
		FROM auth.accounts a JOIN auth.account_tenants m ON m.account_id = a.id
		WHERE LOWER(a.email) IN (?) AND a.personal_data_erased_at IS NULL
		AND m.tenant_id = ? AND m.status = 'active'
		UNION ALL
		SELECT LOWER(email), 0::bigint, person_id FROM auth.invitation_tokens
		WHERE LOWER(email) IN (?) AND tenant_id = ? AND used_at IS NULL AND person_id IS NOT NULL`,
		bun.List(emails), tenantID, bun.List(emails), tenantID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, nil, stats, fmt.Errorf("identity access postgres: find school email identities: %w", err)
	}
	for _, row := range rows {
		if row.AccountID > 0 {
			accounts[row.Email] = row.AccountID
		} else {
			invited[row.Email] = append(invited[row.Email], row.PersonID)
		}
	}
	return accounts, invited, stats, nil
}
