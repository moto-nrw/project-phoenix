package operator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/stretchr/testify/assert"
)

func TestGuardOperatorDirectManagedSettingWrite_BlocksAGBDocumentURL(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	blocked := guardOperatorDirectManagedSettingWrite(w, r, settings.KeyEnrollmentLegalAGBDocumentURL)

	assert.True(t, blocked)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGuardOperatorDirectManagedSettingWrite_AllowsRegularSettings(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	blocked := guardOperatorDirectManagedSettingWrite(w, r, "enrollment.legal_agb_text")

	assert.False(t, blocked)
	assert.Equal(t, http.StatusOK, w.Code)
}
