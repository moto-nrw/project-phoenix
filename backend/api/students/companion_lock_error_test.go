package students

// The companion lock protocol refuses a row it may not wait for (see
// lockStudentCompanionGraph): a linked child being edited elsewhere aborts the
// request. Nothing is written and a retry succeeds, so the status has to say
// "try again", not "the server broke". A 500 here would also be logged as a
// server fault and page whoever watches the error rate — for a perfectly
// legitimate concurrent edit of two children in one Laufgemeinschaft.

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/carelifecycle"
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

func TestUpdateStudentTxErrorRenderer_CompanionLockBusy(t *testing.T) {
	t.Parallel()

	t.Run("busy lock is a retriable conflict", func(t *testing.T) {
		resp := rendererStatus(t, updateStudentTxErrorRenderer(carelifecycle.ErrCompanionLockBusy))

		assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
		// The German sentence travels to the UI unchanged — it is the only
		// instruction the user gets ("in einem Moment erneut speichern").
		assert.Equal(t, studentdeletion.ErrCompanionLockBusy.Error(), resp.ErrorText)
		assert.Equal(t, carelifecycle.ErrCompanionLockBusy.Error(), resp.ErrorText, "the workflow keeps the user-facing text of the update path")
	})

	t.Run("still a conflict when wrapped by a caller", func(t *testing.T) {
		// The repository raises the sentinel several layers below the handler,
		// and intermediate layers are free to add context — errors.Is has to
		// carry the classification, not a top-level equality check.
		resp := rendererStatus(t, updateStudentTxErrorRenderer(
			errors.Join(errors.New("update student"), userModels.ErrCompanionLockBusy),
		))

		assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	})

	t.Run("the Care Plan and models sentinels are the same instance", func(t *testing.T) {
		// carelifecycle re-exports the models sentinel. If that ever becomes a
		// separate errors.New, the repository's error would silently fall
		// through to the 500 branch.
		assert.Equal(t, userModels.ErrCompanionLockBusy, carelifecycle.ErrCompanionLockBusy)
	})

	t.Run("an unrelated error is still a server error", func(t *testing.T) {
		resp := rendererStatus(t, updateStudentTxErrorRenderer(errors.New("boom")))

		assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	})
}

func TestStudentDeletionErrorRenderer_CompanionLockBusy(t *testing.T) {
	t.Parallel()

	// Deleting a child drops its links, which takes the same far-end locks as an
	// update — so the delete path needs the identical classification.
	t.Run("busy lock is a retriable conflict", func(t *testing.T) {
		resp := rendererStatus(t, studentDeletionErrorRenderer(studentdeletion.ErrCompanionLockBusy))

		assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
		assert.Equal(t, studentdeletion.ErrCompanionLockBusy.Error(), resp.ErrorText)
		assert.Equal(t, carelifecycle.ErrCompanionLockBusy.Error(), resp.ErrorText, "the workflow keeps the user-facing text of the update path")
	})

	t.Run("an unrelated error is still a server error", func(t *testing.T) {
		resp := rendererStatus(t, studentDeletionErrorRenderer(errors.New("boom")))

		assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	})
}

// TestUpdateStudentTxErrorRenderer_CompanionsChanged pins the third 409 this
// endpoint can answer with: the submitted "läuft mit" list was built on a
// snapshot someone else has since replaced.
//
// It needs its own code because the retry differs from both siblings: the lock
// collision is retriable with the SAME payload, the plan conflict is answered
// with a confirmation — but re-sending this list is exactly the write that was
// refused, so the client has to reload first.
func TestUpdateStudentTxErrorRenderer_CompanionsChanged(t *testing.T) {
	t.Parallel()

	t.Run("a stale list is a coded, retriable conflict", func(t *testing.T) {
		resp := rendererStatus(t, updateStudentTxErrorRenderer(carelifecycle.ErrCompanionsChanged))

		assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
		assert.Equal(t, CodeCompanionsChanged, resp.Code)
		// The German sentence reaches the UI unchanged — it carries the one
		// instruction that gets the user out ("neu laden").
		assert.Equal(t, carelifecycle.ErrCompanionsChanged.Error(), resp.ErrorText)
	})

	t.Run("still classified when wrapped by a caller", func(t *testing.T) {
		resp := rendererStatus(t, updateStudentTxErrorRenderer(
			errors.Join(errors.New("update student"), carelifecycle.ErrCompanionsChanged),
		))

		assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
		assert.Equal(t, CodeCompanionsChanged, resp.Code)
	})

	t.Run("its code is distinct from the lock collision", func(t *testing.T) {
		// The client keys "reload first" off this code and "just try again" off
		// the other; collapsing them would show the wrong instruction.
		assert.NotEqual(t, CodeCompanionLockBusy, CodeCompanionsChanged)
	})
}
