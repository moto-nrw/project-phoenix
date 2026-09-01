package users_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// staticShareVisibility is the sharing port with a fixed answer, so this test
// can say "the request was shared with Klaus" without importing the parents
// domain (which is exactly why the port exists).
type staticShareVisibility struct{ recipients []int64 }

func (s staticShareVisibility) SharedRecipientAccountIDs(
	_ context.Context, _ int64, _ string, _ int64,
) ([]int64, error) {
	return s.recipients, nil
}

// TestMasterDataDecisionTellsTheOtherGuardian is story 47 for the Stammdaten
// domain: the other guardian learns that the child's data changed, without the
// reason or the author — unless the submitter shared the request with them.
func TestMasterDataDecisionTellsTheOtherGuardian(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		shareWith   func(other testpkg.ParentChain) []int64
		wantBody    string
		wantReason  string
		wantRequest string
	}{
		{
			name:      "not shared: neutral line",
			shareWith: func(testpkg.ParentChain) []int64 { return nil },
			// No reason, no request status: a co-guardian who was not shared
			// with filed nothing and reads nothing of the submitter's.
			wantBody:   "Betreuungsstand geändert: Stammdaten",
			wantReason: "",
		},
		{
			name:        "shared: the recipient gets the full pill",
			shareWith:   func(other testpkg.ParentChain) []int64 { return []int64{other.AccountID} },
			wantBody:    "Anfrage bestätigt, Stammdaten übernommen",
			wantReason:  "Passt so",
			wantRequest: userModels.ParentMessageRequestStatusDone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := testpkg.SetupTestDB(t)
			repos := repositories.NewFactory(db)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			other := testpkg.CreateTestCoGuardianForStudent(t, db, chain.StudentID, "Klaus", "Zweitelternteil")

			broadcaster := testpkg.NewRecordingBroadcaster()
			emitter := parentmessaging.NewEmitter(db, repos.ParentMessageThread, repos.ParentMessage,
				reviewNotesSettings{enabled: true}, broadcaster, slog.Default())
			testpkg.SetTenantRuntime(t, emitter, db)
			svc := userService.NewMasterDataReviewServiceWithAuditAndPolicy(
				repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, emitter, nil,
				testpkg.RequestReviewPolicy{}, nil, slog.Default(), broadcaster)
			sink, ok := svc.(interface {
				SetRequestShareVisibility(parentmessaging.ShareVisibilityResolver)
			})
			require.True(t, ok, "the review service must accept the sharing resolver the factory injects")
			sink.SetRequestShareVisibility(staticShareVisibility{recipients: tc.shareWith(other)})

			row := insertPendingChange(t, db, repos, chain,
				userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)
			require.NoError(t, testpkg.WithTenantTx(t, authorizedCtx(context.Background()), db, chain.TenantID,
				func(txCtx context.Context, _ bun.Tx) error {
					_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{
						RequestID: row.ID, Approve: true, Reason: "Passt so", ReviewedBy: chain.AccountID,
					})
					return e
				}))

			pill := coGuardianPill(t, db, repos, chain.TenantID, chain.StudentID, other.AccountID)
			require.NotNil(t, pill, "the other guardian must hear that the child's data changed")
			assert.Equal(t, tc.wantBody, pill.Body)
			assert.Equal(t, tc.wantReason, pill.DecisionReason)
			assert.Equal(t, tc.wantRequest, pill.RequestStatus)
		})
	}
}

func coGuardianPill(
	t *testing.T, db *bun.DB, repos *repositories.Factory, tenantID, studentID, guardianAccountID int64,
) *userModels.ParentMessage {
	t.Helper()
	var pill *userModels.ParentMessage
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			thread, err := repos.ParentMessageThread.FindByStudentGuardian(txCtx, studentID, guardianAccountID)
			if err != nil || thread == nil {
				return err
			}
			msgs, err := repos.ParentMessage.ListByThread(txCtx, thread.ID, 50)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				if m.EventType == userModels.ParentMessageEventRequestStatus {
					pill = m
				}
			}
			return nil
		}))
	return pill
}
