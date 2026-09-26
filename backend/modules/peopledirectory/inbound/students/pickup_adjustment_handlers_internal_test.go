package students

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
)

func TestRenderPickupAdjustmentError_CareOfferingsDisabledIsConflict(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	renderPickupAdjustmentError(recorder, request, careplan.ErrCareOfferingsDisabled)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"pickup.offerings_disabled"`)
}

func TestRenderPickupAdjustmentError_CompleteWithdrawalConfirmationIsConflict(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	renderPickupAdjustmentError(recorder, request, careplan.ErrCompleteWithdrawalConfirmationRequired)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"enrollment.complete_withdrawal_confirmation_required"`)
}
