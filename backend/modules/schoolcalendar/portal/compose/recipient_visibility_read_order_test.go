package compose_test

import (
	"context"
	"strconv"
	"testing"

	calendarCompose "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type recipientContactReader struct {
	calendarCompose.GuardianDirectory
	after func()
}

func (p recipientContactReader) ListPortalContacts(ctx context.Context, guardians, students []int64) ([]calendarCompose.GuardianContact, error) {
	rows, err := p.GuardianDirectory.ListPortalContacts(ctx, guardians, students)
	if err == nil {
		p.after()
	}
	return rows, err
}

// The picker reads the portal visibility of all candidates once (#5784,
// decision E2: one early shared read replaces the former per-parent READ
// COMMITTED reads). A revocation committed after that read is therefore not
// seen by this listing; creating an appointment resolves its targets afresh.
// The port wrapper supplies a scheduling point, not substituted data.
func TestRecipientOptionsShareOnePortalVisibilityRead(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for range 2 {
		testpkg.CreateTestParentGuardianChain(t, db)
	}
	cfg := calendarTestConfig(t, db)
	parents, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), db, cfg.GuardianProfileRepo.SearchByText, "schneider", 50)
	require.NoError(t, err)
	require.Len(t, parents, 2)
	laterID := parents[1].ID
	fired := false
	after := func() {
		if fired {
			return
		}
		fired = true
		err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			_, err := tx.NewRaw(`UPDATE auth.guardian_student_access AS access
				SET permissions = jsonb_set(access.permissions, '{parent_portal.access}', 'false'::jsonb)
				FROM users.student_guardian_relationships AS link
				WHERE access.tenant_id = ? AND access.relationship_id = link.id
				AND link.tenant_id = access.tenant_id AND link.guardian_profile_id = ?`, testpkg.Tenant(t), laterID).Exec(ctx)
			return err
		})
		require.NoError(t, err)
	}
	cfg.GuardianProfileRepo = recipientContactReader{GuardianDirectory: cfg.GuardianProfileRepo, after: after}
	svc := owners.NewCalendarPortal(cfg)
	out, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), db, svc.RecipientOptions, "schneider", 50)
	require.NoError(t, err)
	require.True(t, fired)
	require.Equal(t, []string{strconv.FormatInt(parents[0].ID, 10), strconv.FormatInt(laterID, 10)}, parentOptionIDs(out.Parents))
	fresh, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), db, svc.RecipientOptions, "schneider", 50)
	require.NoError(t, err)
	require.Equal(t, []string{strconv.FormatInt(parents[0].ID, 10)}, parentOptionIDs(fresh.Parents), "the next listing sees the revocation")
}
