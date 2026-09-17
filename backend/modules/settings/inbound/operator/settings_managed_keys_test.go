package operator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
)

func TestGuardOperatorDirectManagedSettingWrite_BlocksAGBDocumentURL(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	blocked := guardOperatorDirectManagedSettingWrite(w, r, configModel.KeyEnrollmentLegalAGBDocumentURL)

	assert.True(t, blocked)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGuardOperatorDirectManagedSettingWrite_AllowsRegularSettings(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	blocked := guardOperatorDirectManagedSettingWrite(w, r, configModel.KeyEnrollmentLegalAGBText)

	assert.False(t, blocked)
	assert.Equal(t, http.StatusOK, w.Code)
}
