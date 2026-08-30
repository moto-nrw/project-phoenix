package schedule_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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

func careCoGuardianPills(
	t *testing.T, f *careFixture, guardianAccountID int64,
) []*usersModels.ParentMessage {
	t.Helper()
	var msgs []*usersModels.ParentMessage
	require.NoError(t, tenant.WithTenantTx(
		testpkg.WithTenantRuntime(t, context.Background(), f.db), f.db, f.chain.TenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			thread, err := f.repos.ParentMessageThread.FindByStudentGuardian(txCtx, f.chain.StudentID, guardianAccountID)
			if err != nil || thread == nil {
				return err
			}
			msgs, err = f.repos.ParentMessage.ListByThread(txCtx, thread.ID, 100)
			return err
		}))
	return msgs
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
		attachCareCoGuardianEmitter(t, f, nil)

		req := f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))
		_, err := f.svc.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{
			RequestID: req.ID, Approve: true, Reason: "Passt so", ReviewedBy: f.staffAccount,
		})
		require.NoError(t, err)

		pills := careCoGuardianPills(t, f, other.AccountID)
		require.Len(t, pills, 1, "the other guardian hears about the decision exactly once")
		assert.Contains(t, pills[0].Body, "Betreuungsstand geändert")
		assert.Empty(t, pills[0].DecisionReason, "the staff reason belongs to the submitting guardian")
		assert.NotContains(t, pills[0].Body, "Passt so")
	})

	t.Run("shared: the recipient gets the full pill", func(t *testing.T) {
		t.Parallel()
		f := newCareFixture(t)
		other := testpkg.CreateTestCoGuardianForStudent(t, f.db, f.chain.StudentID, "Klaus", "Zweitelternteil")
		attachCareCoGuardianEmitter(t, f, []int64{other.AccountID})

		req := f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))
		_, err := f.svc.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{
			RequestID: req.ID, Approve: true, Reason: "Passt so", ReviewedBy: f.staffAccount,
		})
		require.NoError(t, err)

		pills := careCoGuardianPills(t, f, other.AccountID)
		require.Len(t, pills, 1)
		assert.Equal(t, "Passt so", pills[0].DecisionReason,
			"a guardian the request was shared with sees the same decision the submitter does")
	})
}

// attachCareCoGuardianEmitter rebuilds the fixture's service with a real
// (messaging-enabled) emitter and the given explicit share recipients.
func attachCareCoGuardianEmitter(t *testing.T, f *careFixture, sharedWith []int64) {
	t.Helper()
	emitter := f.emitter(t, f.repos.ParentMessage, messagingOnSettings{}, testpkg.NewRecordingBroadcaster())
	svc := schedule.NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
		f.repos.CareScheduleChangeRequest, f.repos.Student, f.repos.Person,
		f.sf.ArrivalSchedule, f.sf.PickupSchedule,
		f.repos.StudentPickupException, f.repos.Attendance, f.autoExcusal,
		f.sf.UserContext, emitter, nil,
		testpkg.RequestReviewPolicy{UserContext: f.sf.UserContext},
		nil, slog.Default(), f.sf.StudentAudit,
	)
	sink, ok := svc.(interface {
		SetRequestShareVisibility(parentmessaging.ShareVisibilityResolver)
	})
	require.True(t, ok, "the care service must accept the sharing resolver the factory injects")
	sink.SetRequestShareVisibility(staticShareVisibility{recipients: sharedWith})
	f.svc = svc
}
