package compose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// countingProvider accepts every mail and counts it by kind.
type countingProvider struct {
	acceptingProvider
	sent map[string]int
}

func (p countingProvider) SendEmail(_ context.Context, intent deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	p.sent[intent.Template]++
	return deliveryContract.ProviderResult{}, nil
}

// Through the real worker and store: while one child of the family is in care,
// the school's mailings reach the guardian. Once the last child left, they stay
// back and say why, and the mail the family needs on its own still goes out
// (#5960).
func TestNoSchoolMailingReachesAFamilyWhoseLastChildLeft(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.WithTenantRuntime(t, context.Background(), db)
	school := testpkg.Tenant(t)
	guardian := testpkg.CreateTestGuardianProfileNamed(t, db, "Erika", "Muster", "erika")
	first := testpkg.CreateTestStudent(t, db, "Erstes", "Kind", "2a")
	second := testpkg.CreateTestStudent(t, db, "Zweites", "Kind", "4b")
	testpkg.CreateTestStudentGuardianLink(t, db, first.ID, guardian.ID, "primary_guardian")
	testpkg.CreateTestStudentGuardianLink(t, db, second.ID, guardian.ID, "primary_guardian")
	leave := func(studentID int64, status string) {
		_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET status = ?
			WHERE tenant_id = ? AND student_profile_id = ?`, status, school, studentID)
		require.NoError(t, err)
	}
	queue := func(kind string) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox (tenant_id, kind, recipient, payload, status, next_retry_at)
			VALUES (?, ?, jsonb_build_object('address', ?::text), '{}'::jsonb, 'pending', NOW() - INTERVAL '1 minute')
			RETURNING id`, school, kind, *guardian.Email).Scan(ctx, &id))
		return id
	}
	runtime, err := NewModule(ModuleDependencies{DB: db, People: noGuardians{}, Observe: func(deliveryContract.Observation) {}})
	require.NoError(t, err)
	provider := countingProvider{sent: map[string]int{}}
	worker, err := NewWorker(runtime, provider)
	require.NoError(t, err)
	mailings := []string{"parent_announcement", "parent_message", "appointment_published", "guardian_invitation"}

	leave(first.ID, "inactive")
	for _, kind := range mailings {
		queue(kind)
	}
	_, err = worker.RunOnce(ctx, 50, 5)
	require.NoError(t, err)
	for _, kind := range mailings {
		assert.Equal(t, 1, provider.sent[kind], "%s reaches a family with one child still in care", kind)
	}

	leave(second.ID, "alumnus")
	held := make(map[string]int64, len(mailings))
	for _, kind := range mailings {
		held[kind] = queue(kind)
	}
	queue("care_contract_notice")
	_, err = worker.RunOnce(ctx, 50, 5)
	require.NoError(t, err)
	assert.Equal(t, 1, provider.sent["care_contract_notice"], "the family still gets the mail it needs on its own")
	for kind, id := range held {
		assert.Equal(t, 1, provider.sent[kind], "%s no longer reaches the family", kind)
		var status, reason string
		require.NoError(t, db.NewRaw(`SELECT status, last_error FROM platform.email_outbox WHERE id = ?`, id).Scan(ctx, &status, &reason))
		assert.Equal(t, "cancelled", status, kind)
		assert.Equal(t, deliveryContract.MailHeldByDeparture, reason, kind)
	}
}
