package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// A mapping of an account to a school refuses a caterer account in the same
// statement (ADR 0077). Each write of auth.account_tenants is
//
//	refuseCatererAccount + <INSERT ... FROM caterer WHERE NOT caterer.refused ... RETURNING 1> + reportCatererRefusal
//
// with the account ID bound first, and runs through bindAccountToSchool. The
// refusal costs no statement of its own, and no flow can bind a caterer
// account by forgetting a check. The parts are constants so the statement
// stays statically resolvable.
const (
	refuseCatererAccount = `WITH caterer AS (
			SELECT EXISTS (SELECT 1 FROM auth.caterer_accounts WHERE account_id = ?) AS refused
		), mapped AS (`
	reportCatererRefusal = `)
		SELECT caterer.refused, (SELECT COUNT(*) FROM mapped) AS mapped FROM caterer`
)

// bindAccountToSchool runs one mapping write built from the constants above
// and reports a caterer account as domain.ErrCatererAccountJoinsNoSchool.
func (s *Store) bindAccountToSchool(ctx context.Context, operation string, statement func(bun.IDB) *bun.RawQuery) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	var outcome []struct {
		Refused bool  `bun:"refused"`
		Mapped  int64 `bun:"mapped"`
	}
	started := time.Now()
	err = statement(db).Scan(ctx, &outcome)
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
