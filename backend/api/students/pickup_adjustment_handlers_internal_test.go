package students

import (
	"net/http"
	"net/http/httptest"
	"testing"

	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/stretchr/testify/assert"
)

func TestRenderPickupAdjustmentError_CareOfferingsDisabledIsConflict(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	renderPickupAdjustmentError(recorder, request, enrollmentService.ErrCareOfferingsDisabled)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"pickup.offerings_disabled"`)
}

func TestRenderPickupAdjustmentError_CompleteWithdrawalConfirmationIsConflict(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	renderPickupAdjustmentError(recorder, request, enrollmentService.ErrCompleteWithdrawalConfirmationRequired)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"enrollment.complete_withdrawal_confirmation_required"`)
}
