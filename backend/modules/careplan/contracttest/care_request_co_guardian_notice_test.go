package contracttest_test

import (
	"context"
	"log/slog"
	"testing"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	communicationtest "github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// messagingOnSettings lets the emitter really write pills, so the co-guardian
// audience split is observable in the thread instead of only in a mock.
type messagingOnSettings struct{}

func (messagingOnSettings) ResolveBoolForTenant(_ context.Context, _ int64, _ string) (bool, error) {
	return true, nil
}

// staticShareVisibility is the sharing port with a fixed answer, so a care
// test can say "this request was shared with Klaus" without importing the
// parents domain.
type staticShareVisibility struct{ recipients []int64 }

func (s staticShareVisibility) SharedRecipientAccountIDs(
	_ context.Context, _ int64, _ string, _ int64,
) ([]int64, error) {
	return s.recipients, nil
}

// TestCareDecisionTellsTheOtherGuardian is story 47 for the care domain: a
// decision on one parent's request changes the whole family's care, so the
// other guardian hears about it — neutrally, unless the submitter explicitly
// shared the request with them.
func TestCareDecisionTellsTheOtherGuardian(t *testing.T) {
	t.Parallel()

	t.Run("not shared: neutral line without reason or author", func(t *testing.T) {
		t.Parallel()
		f := newCareFixture(t)
		other := testpkg.CreateTestCoGuardianForStudent(t, f.db, f.chain.StudentID, "Klaus", "Zweitelternteil")
		f.svc = careServiceWithCoGuardianEmitter(t, f, nil)

		req := f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))
		_, err := f.svc.Decide(f.staffCtx(f.staffAccount), carerequests.DecideInput{
			RequestID: req.ID, Approve: true, Reason: "Passt so", ReviewedBy: f.staffAccount,
		})
		require.NoError(t, err)

		pills, err := api.ParentStoreGuardianMessagesFixture(func(read func(context.Context) error) error {
			return tenant.WithTenantTx(testpkg.WithTenantRuntime(t, context.Background(), f.db), f.db, f.chain.TenantID, func(ctx context.Context, _ bun.Tx) error { return read(ctx) })
		}, f.reposMessages.ParentMessageThread, communicationtest.NewParentMessageRepository(f.db), f.chain.StudentID, other.AccountID, 100)
		require.NoError(t, err)
		require.Len(t, pills, 1, "the other guardian hears about the decision exactly once")
		assert.Contains(t, pills[0].Body, "Betreuungsstand geändert")
		assert.Empty(t, pills[0].DecisionReason, "the staff reason belongs to the submitting guardian")
		assert.NotContains(t, pills[0].Body, "Passt so")
	})

	t.Run("shared: the recipient gets the full pill", func(t *testing.T) {
		t.Parallel()
		f := newCareFixture(t)
		other := testpkg.CreateTestCoGuardianForStudent(t, f.db, f.chain.StudentID, "Klaus", "Zweitelternteil")
		f.svc = careServiceWithCoGuardianEmitter(t, f, []int64{other.AccountID})

		req := f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))
		_, err := f.svc.Decide(f.staffCtx(f.staffAccount), carerequests.DecideInput{
			RequestID: req.ID, Approve: true, Reason: "Passt so", ReviewedBy: f.staffAccount,
		})
		require.NoError(t, err)

		pills, err := api.ParentStoreGuardianMessagesFixture(func(read func(context.Context) error) error {
			return tenant.WithTenantTx(testpkg.WithTenantRuntime(t, context.Background(), f.db), f.db, f.chain.TenantID, func(ctx context.Context, _ bun.Tx) error { return read(ctx) })
		}, f.reposMessages.ParentMessageThread, communicationtest.NewParentMessageRepository(f.db), f.chain.StudentID, other.AccountID, 100)
		require.NoError(t, err)
		require.Len(t, pills, 1)
		assert.Equal(t, "Passt so", pills[0].DecisionReason,
			"a guardian the request was shared with sees the same decision the submitter does")
	})
}

// careServiceWithCoGuardianEmitter rebuilds the fixture's service with a real
// (messaging-enabled) emitter and the given explicit share recipients.
func careServiceWithCoGuardianEmitter(t *testing.T, f *careFixture, sharedWith []int64) carerequests.Service {
	t.Helper()
	emitter := f.emitter(t, messagingOnSettings{}, testpkg.NewRecordingBroadcaster())
	return owners.NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
		f.reposCare.CarePlan(), f.persons,
		f.schedules.Arrivals, f.schedules.Pickups, newPickupChangePresence(t, f.db), f.autoExcusal,
		f.reposCare.CarePlan(),
		f.caller, emitter, nil,
		testpkg.RequestReviewPolicy[*peopleValues.StudentState]{UserContext: f.caller}, testpkg.PermissionsFromContext,
		nil, slog.Default(), f.services.StudentAudit,
		owners.WithRequestShareVisibility(staticShareVisibility{recipients: sharedWith}),
	)
}
