package students

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

func TestWithdrawalDeletionErrorRendererUsesStableCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"stale preview", studentdeletion.ErrPreviewChanged, http.StatusConflict, errCodeStudentDeletionPreviewChanged},
		{"confirmation mismatch", studentdeletion.ErrConfirmationMismatch, http.StatusBadRequest, errCodeStudentDeletionConfirmationMismatch},
		{"acknowledgement missing", studentdeletion.ErrNotAcknowledged, http.StatusBadRequest, errCodeStudentDeletionAcknowledgement},
		{"invalid reason", studentdeletion.ErrInvalidReason, http.StatusBadRequest, errCodeStudentDeletionInvalidReason},
		{"alumnus", studentdeletion.ErrAlumnus, http.StatusConflict, errCodeStudentDeletionAlumnus},
		{"retention not ended", studentdeletion.ErrRetentionNotEnded, http.StatusBadRequest, errCodeStudentDeletionRetentionNotEnded},
		{"companion blocker", studentdeletion.ErrCompanionWouldLoseDeparture, http.StatusConflict, errCodeStudentDeletionCompanionBlocked},
		{"companion lock", studentdeletion.ErrCompanionLockBusy, http.StatusConflict, errCodeStudentDeletionCompanionLockBusy},
		{"completion missing", userService.ErrCareWithdrawalNotFound, http.StatusNotFound, errCodeCareWithdrawalNotFound},
		{"completion missing under lock", studentdeletion.ErrWithdrawalNotFound, http.StatusNotFound, errCodeCareWithdrawalNotFound},
		{"completion resolved", userModels.ErrCareWithdrawalAlreadyResolved, http.StatusConflict, errCodeCareWithdrawalAlreadyResolved},
		{"completion resolved under lock", studentdeletion.ErrWithdrawalAlreadyResolved, http.StatusConflict, errCodeCareWithdrawalAlreadyResolved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodDelete, "/", nil)
			renderError(recorder, request, withdrawalDeletionErrorRenderer(tt.err))

			assert.Equal(t, tt.status, recorder.Code)
			var response struct {
				Code string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, tt.code, response.Code)
		})
	}
}
