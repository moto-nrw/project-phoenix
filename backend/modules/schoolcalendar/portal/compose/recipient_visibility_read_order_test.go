package compose_test

import (
	"context"
	"strconv"
	"testing"

	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type recipientLinkReader struct {
	read  func(context.Context, int64) ([]*calendarCompose.StudentGuardian, error)
	after func()
}

func (p recipientLinkReader) FindByGuardianProfileID(ctx context.Context, id int64) ([]*calendarCompose.StudentGuardian, error) {
	rows, err := p.read(ctx, id)
	if err == nil {
		p.after()
	}
	return rows, err
}

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

// The port wrapper supplies a scheduling point, not substituted data. Both
// candidate contacts and baseline individual links come from the real stores.
func TestRecipientOptionsObserveLaterPortalRevocation(t *testing.T) {
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
	cfg.StudentGuardianRepo = recipientLinkReader{read: cfg.StudentGuardianRepo.FindByGuardianProfileID, after: after}
	cfg.GuardianProfileRepo = recipientContactReader{GuardianDirectory: cfg.GuardianProfileRepo, after: after}
	svc := owners.NewCalendarPortal(cfg)
	out, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), db, svc.RecipientOptions, "schneider", 50)
	require.NoError(t, err)
	require.True(t, fired)
	require.Equal(t, []string{strconv.FormatInt(parents[0].ID, 10)}, parentOptionIDs(out.Parents))
}
