package students

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		{"stale preview", studentdeletion.ErrPreviewChanged, http.StatusConflict, common.CodeStudentsDeletionPreviewChanged},
		{"confirmation mismatch", studentdeletion.ErrConfirmationMismatch, http.StatusBadRequest, common.CodeStudentsDeletionConfirmationMismatch},
		{"acknowledgement missing", studentdeletion.ErrNotAcknowledged, http.StatusBadRequest, common.CodeStudentsDeletionAcknowledgementRequired},
		{"invalid reason", studentdeletion.ErrInvalidReason, http.StatusBadRequest, common.CodeStudentsDeletionInvalidReason},
		{"alumnus", studentdeletion.ErrAlumnus, http.StatusConflict, common.CodeStudentsDeletionAlumnus},
		{"retention not ended", studentdeletion.ErrRetentionNotEnded, http.StatusBadRequest, common.CodeStudentsDeletionRetentionNotEnded},
		{"companion blocker", studentdeletion.ErrCompanionWouldLoseDeparture, http.StatusConflict, common.CodeStudentsDeletionCompanionBlocked},
		{"companion lock", studentdeletion.ErrCompanionLockBusy, http.StatusConflict, common.CodeStudentsDeletionCompanionLockBusy},
		{"completion missing", careplan.ErrCareWithdrawalNotFound, http.StatusNotFound, common.CodeStudentsCareWithdrawalNotFound},
		{"completion missing under lock", studentdeletion.ErrWithdrawalNotFound, http.StatusNotFound, common.CodeStudentsCareWithdrawalNotFound},
		{"completion resolved", careplan.ErrCareWithdrawalAlreadyResolved, http.StatusConflict, common.CodeStudentsCareWithdrawalAlreadyResolved},
		{"completion resolved under lock", studentdeletion.ErrWithdrawalAlreadyResolved, http.StatusConflict, common.CodeStudentsCareWithdrawalAlreadyResolved},
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
