package compose

import (
	"context"
	"log/slog"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/uptrace/bun"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// GuardianProfileLoader resolves the autofill payload the public + parent
// enrollment forms need: guardian profile, primary phone, linked active
// students. Handlers used to issue raw multi-schema queries directly;
// this service keeps the data access inside the repository and the
// tenant transaction handling in one place.
type GuardianProfileLoader struct {
	repo   peoplePorts.GuardianProfileRepository
	db     *bun.DB
	logger *slog.Logger
}

// NewGuardianProfileLoader wires a loader against the repo + db.
func NewGuardianProfileLoader(repo peoplePorts.GuardianProfileRepository, db *bun.DB, logger *slog.Logger) *GuardianProfileLoader {
	if logger == nil {
		logger = slog.Default()
	}
	return &GuardianProfileLoader{repo: repo, db: db, logger: logger}
}

// LoadForTenant looks up the guardian profile linked to accountID
// under the given tenant. Wraps the read in WithTenantTx so RLS
// scopes the join. Returns (nil, nil) when the account has no
// profile in this tenant — callers fall through to their default
// (claims-derived) response.
func (s *GuardianProfileLoader) LoadForTenant(ctx context.Context, accountID, tenantID int64) (*peopleValues.GuardianProfileWithChildren, error) {
	if accountID <= 0 || tenantID <= 0 {
		return nil, nil
	}

	var result *peopleValues.GuardianProfileWithChildren
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		loaded, loadErr := s.repo.LoadProfileWithChildren(txCtx, accountID)
		if loadErr != nil {
			return loadErr
		}
		result = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
