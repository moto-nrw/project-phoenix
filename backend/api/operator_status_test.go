package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The operator dashboard deactivates and reactivates the accounts of team
// members (#4047).

func TestDeactivateOperatorAnswersTheDeactivatedAccount(t *testing.T) {
	t.Parallel()
	deactivatedAt := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	var captured identityaccess.OperatorStatusChange
	resource := identityoperator.NewInvitationsResource(&invitationMockService{
		changeOperatorStatusFn: func(_ context.Context, change identityaccess.OperatorStatusChange) (identityaccess.Operator, error) {
			captured = change
			return identityaccess.Operator{ID: change.OperatorID, Email: "geht@example.com", DisplayName: "Geht", DeactivatedAt: &deactivatedAt}, nil
		},
	})
	rr := httptest.NewRecorder()

	resource.DeactivateOperator(rr, invitationReqWithClaimsAndID(http.MethodPost, "/operators/7/deactivate", nil, 42, "7"))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, int64(7), captured.OperatorID)
	assert.Equal(t, int64(42), captured.ActorID, "the signed-in operator is the actor")
	assert.False(t, captured.Active)
	var body struct {
		Data identityoperator.OperatorListItem `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.False(t, body.Data.Active)
	require.NotNil(t, body.Data.DeactivatedAt)
	assert.True(t, deactivatedAt.Equal(*body.Data.DeactivatedAt))
	require.NotNil(t, body.Data.DeletionDueAt)
	assert.True(t, time.Date(2028, time.January, 1, 0, 0, 0, 0, calendar.Berlin).Equal(*body.Data.DeletionDueAt),
		"deactivated in 2026, the account goes on 1 January 2028")
}

func TestReactivateOperatorAnswersTheActiveAccount(t *testing.T) {
	t.Parallel()
	resource := identityoperator.NewInvitationsResource(&invitationMockService{
		changeOperatorStatusFn: func(_ context.Context, change identityaccess.OperatorStatusChange) (identityaccess.Operator, error) {
			return identityaccess.Operator{ID: change.OperatorID, Active: change.Active}, nil
		},
	})
	rr := httptest.NewRecorder()

	resource.ReactivateOperator(rr, invitationReqWithClaimsAndID(http.MethodPost, "/operators/7/reactivate", nil, 42, "7"))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"active":true`)
	assert.NotContains(t, rr.Body.String(), "deactivated_at")
	assert.NotContains(t, rr.Body.String(), "deletion_due_at")
}

func TestOperatorStatusChangeRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		id     string
		err    error
		status int
		code   string
	}{
		{name: "invalid id", id: "abc", status: http.StatusBadRequest, code: "general.input"},
		{name: "non-positive id", id: "0", status: http.StatusBadRequest, code: "general.input"},
		{name: "unknown operator", id: "7", err: identityaccess.ErrOperatorNotFound, status: http.StatusNotFound, code: "identity.account_not_found"},
		{name: "own account", id: "42", err: identityaccess.ErrOperatorSelfDeactivation, status: http.StatusConflict, code: "identity.operator_self_deactivation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resource := identityoperator.NewInvitationsResource(&invitationMockService{
				changeOperatorStatusFn: func(context.Context, identityaccess.OperatorStatusChange) (identityaccess.Operator, error) {
					return identityaccess.Operator{}, tc.err
				},
			})
			rr := httptest.NewRecorder()

			resource.DeactivateOperator(rr, invitationReqWithClaimsAndID(http.MethodPost, "/operators/"+tc.id+"/deactivate", nil, 42, tc.id))

			assert.Equal(t, tc.status, rr.Code, rr.Body.String())
			assert.Contains(t, rr.Body.String(), `"code":"`+tc.code+`"`)
		})
	}
}

func TestListInvitationsMarksTheSignedInOperator(t *testing.T) {
	t.Parallel()
	resource := identityoperator.NewInvitationsResource(&invitationMockService{
		listOperatorsFn: func(context.Context) ([]*identityaccess.Operator, error) {
			return []*identityaccess.Operator{{ID: 42, Active: true}, {ID: 7}}, nil
		},
	})
	rr := httptest.NewRecorder()

	resource.ListInvitations(rr, invitationReqWithClaims(http.MethodGet, "/invitations", nil, 42))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var body struct {
		Data identityoperator.InvitationsListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Len(t, body.Data.Operators, 2)
	assert.True(t, body.Data.Operators[0].IsSelf)
	assert.False(t, body.Data.Operators[1].IsSelf)
}
