package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	usersAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	"github.com/stretchr/testify/assert"
)

func TestGuardianFailureKindMapsUnknownInvitationFailuresToInternal(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "internal", string(guardianFailureKind("")))
	assert.Equal(t, "forbidden", string(guardianFailureKind("forbidden")))
	assert.Equal(t, "invalid_request", string(guardianFailureKind("invalid_request")))
}

// An invitation whose mail could not be queued answers with its own code, so
// the school sees that nothing was sent and can try again (#4602).
func TestRenderPeopleDirectoryFailure_MailNotQueued(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("invite guardian to student: %w", identityaccess.ErrGuardianAccessMailNotQueued)
	rr := httptest.NewRecorder()

	renderPeopleDirectoryFailure(rr, httptest.NewRequest(http.MethodPost, "/guardians/students/1/invite", nil), usersAPI.FailureInternal, err)

	testutil.AssertErrorResponse(t, rr, http.StatusServiceUnavailable)
	assert.Contains(t, rr.Body.String(), `"code":"`+apiCommon.CodeIdentityGuardianInvitationEmailNotQueued+`"`)
}
