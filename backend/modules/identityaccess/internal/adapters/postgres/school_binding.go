package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// bindAccountToSchool runs one write of auth.account_tenants and refuses a
// caterer account in the same statement (ADR 0077). Every mapping of an
// account to a school goes through here, so no flow can bind a caterer
// account by forgetting a check, and the refusal costs no statement of its
// own. The insert reads its values from the CTE "caterer"; it must select
// FROM caterer WHERE NOT caterer.refused and RETURN 1 per written row. Its
// placeholders follow the account ID the refusal binds first.
func (s *Store) bindAccountToSchool(ctx context.Context, operation string, accountID int64, insert string, args ...any) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	var outcome []struct {
		Refused bool  `bun:"refused"`
		Mapped  int64 `bun:"mapped"`
	}
	started := time.Now()
	err = db.NewRaw(`WITH caterer AS (
			SELECT EXISTS (SELECT 1 FROM auth.caterer_accounts WHERE account_id = ?) AS refused
		), mapped AS (`+insert+`)
		SELECT caterer.refused, (SELECT COUNT(*) FROM mapped) AS mapped FROM caterer`,
		append([]any{accountID}, args...)...).Scan(ctx, &outcome)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: %s: %w", operation, err)
	}
	if len(outcome) != 1 {
		return stats, fmt.Errorf("identity access postgres: %s: no outcome returned", operation)
	}
	if outcome[0].Refused {
		return stats, domain.ErrCatererAccountJoinsNoSchool
	}
	stats.Rows = outcome[0].Mapped
	return stats, nil
}
