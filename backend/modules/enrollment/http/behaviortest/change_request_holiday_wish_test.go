package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"testing"

	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangeRequestService_DropsUnaskedHolidayCareWish(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"creation", "approval"} {
		for _, stored := range []*bool{nil, new(true), new(false)} {
			name := "unknown"
			if stored != nil {
				name = fmt.Sprint(*stored)
			}
			t.Run(stage+"/"+name, func(t *testing.T) {
				t.Parallel()
				testpkg.OwnTenant(t)
				env := setupRequestTest(t)
				ctx := testpkg.Ctx(t)
				result, err := env.svc.Submit(ctx, validSubmission(t, env.phaseID))
				require.NoError(t, err)
				// An earlier form may have collected a wish. The current schema
				// does not ask it, so a proposal must preserve the stored answer.
				_, err = env.db.NewUpdate().TableExpr("enrollment.request_children").
					Set("holiday_care_wish = ?", stored).Where("id = ?", result.Children[0].ID).Exec(ctx)
				require.NoError(t, err)
				enableChangeRequestMode(t, env, result.Children[0].ID)
				proposed := proposedChangeSubmission(t, env, result)
				phone := "+49 221 555 4703"
				proposed.GuardianPhone = &phone
				injected := stored == nil || !*stored
				if stage == "creation" {
					proposed.Children[0].HolidayCareWish = &injected
				}
				svc := newChangeRequestServiceForTest(env)
				created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentAPI.CreateChangeRequestInput{
					Submission: proposed, ParentNote: "Bitte Telefonnummer ändern.",
				})
				require.NoError(t, err)
				children := created.ChangeRequest.ProposedSnapshot["children"].([]any)
				child := children[0].(map[string]any)
				if stage == "creation" {
					assert.Nil(t, child["holiday_care_wish"], "unasked input must not enter the proposal")
				} else {
					// A pending proposal from before the fix is revalidated on approval.
					child["holiday_care_wish"] = injected
					payload, err := json.Marshal(created.ChangeRequest.ProposedSnapshot)
					require.NoError(t, err)
					_, err = env.db.NewUpdate().TableExpr("enrollment.change_requests").
						Set("proposed_snapshot = ?::jsonb", string(payload)).Where("id = ?", created.ChangeRequest.ID).Exec(ctx)
					require.NoError(t, err)
				}
				_, err = testpkg.TenantQuery2(t, ctx, env.db, svc.Approve, created.ChangeRequest.ID, enrollmentAPI.ReviewChangeRequestInput{
					Note: "Freigegeben.", ActorAccountID: env.creatorID, ActorRole: "admin",
				})
				require.NoError(t, err)
				request, savedChildren, err := env.svc.GetByStatusToken(ctx, result.Request.StatusToken)
				require.NoError(t, err)
				assert.Equal(t, stored, savedChildren[0].HolidayCareWish)
				assert.Equal(t, &phone, request.GuardianPhone)
			})
		}
	}
}
