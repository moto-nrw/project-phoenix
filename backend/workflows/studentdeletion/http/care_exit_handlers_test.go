package studentdeletionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCareWithdrawalHandlers_StaleDeletionRollsBackCompletion(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRouteWithCareLifecycle(t, false)
	repos := newStudentTestPeopleRows(testpkg.NewDatabase(t, tc.db))
	student := testpkg.CreateTestStudent(t, tc.db, "Api", "StaleDeletion", "3a")
	_, actor := testpkg.CreateTestTeacherWithAccount(t, tc.db, "CareWithdrawal", "StaleDelete")
	completion := testpkg.CreateTestCareWithdrawalCompletion(t, tc.db, student.ID, actor.ID, calendar.TodayDate())
	claims := testutil.AdminTestClaims(int(actor.ID))

	previewRequest := testutil.NewAuthenticatedRequest(
		t, http.MethodGet, fmt.Sprintf("/care-withdrawals/%d/deletion-impact", completion.ID), nil,
	)
	previewResponse := authExec(t, tc, previewRequest, claims, []string{"users:delete"})
	require.Equal(t, http.StatusOK, previewResponse.Code, previewResponse.Body.String())
	var preview struct {
		Data struct {
			ConfirmationName string `json:"confirmation_name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(previewResponse.Body.Bytes(), &preview))

	deleteRequest := testutil.NewAuthenticatedRequest(
		t, http.MethodDelete, fmt.Sprintf("/care-withdrawals/%d", completion.ID), map[string]any{
			"expected_fingerprint": "stale",
			"confirmation_name":    preview.Data.ConfirmationName,
			"reason":               studentdeletion.ReasonPrivacyRequest,
			"acknowledged":         true,
		},
	)
	deleteResponse := authExec(t, tc, deleteRequest, claims, []string{"users:delete"})
	require.Equal(t, http.StatusConflict, deleteResponse.Code, deleteResponse.Body.String())
	assert.Contains(t, deleteResponse.Body.String(), `"code":"students.deletion_preview_changed"`)

	stored, err := repos.CareWithdrawal.FindByID(testpkg.Ctx(t), completion.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, careplan.WithdrawalStatePending, stored.State)
	assert.Equal(t, student.ID, *stored.StudentID)
}
