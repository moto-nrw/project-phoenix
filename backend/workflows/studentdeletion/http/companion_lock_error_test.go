package studentdeletionhttp

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rendererStatus reads the HTTP status a renderer will set. The renderers under
// test always produce *common.ErrResponse; anything else is a bug worth failing
// on rather than skipping over.
func rendererStatus(t *testing.T, renderer interface{}) *common.ErrResponse {
	t.Helper()
	resp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "renderer is not a *common.ErrResponse: %T", renderer)
	return resp
}

func TestStudentDeletionErrorRenderer_CompanionLockBusy(t *testing.T) {
	t.Parallel()

	// Deleting a child drops its links, which takes the same far-end locks as an
	// update — so the delete path needs the identical classification.
	t.Run("busy lock is a retriable conflict", func(t *testing.T) {
		resp := rendererStatus(t, (&Resource{ResourceConfig: ResourceConfig{ConstraintViolation: transactionFailures.IsConstraintViolation}}).studentDeletionErrorRenderer(studentdeletion.ErrCompanionLockBusy))

		assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
		assert.Equal(t, studentdeletion.ErrCompanionLockBusy.Error(), resp.Detail)
		assert.Equal(t, careplan.ErrCompanionLockBusy.Error(), resp.Detail, "the workflow keeps the user-facing text of the update path")
	})

	t.Run("an unrelated error is still a server error", func(t *testing.T) {
		resp := rendererStatus(t, (&Resource{ResourceConfig: ResourceConfig{ConstraintViolation: transactionFailures.IsConstraintViolation}}).studentDeletionErrorRenderer(errors.New("boom")))

		assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	})
}
