package announcement

import (
	"net/http"
	"net/http/httptest"
	"testing"

	announcementService "github.com/moto-nrw/project-phoenix/services/announcement"
	"github.com/stretchr/testify/assert"
)

func TestRenderAnnouncementErrorMapsSystemAnnouncementImmutableToConflict(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/parent-announcements/1", nil)

	renderAnnouncementError(recorder, request, announcementService.ErrSystemAnnouncementImmutable)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"system_announcement_immutable"`)
}
