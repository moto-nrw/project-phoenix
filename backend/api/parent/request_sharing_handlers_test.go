package parent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

func TestParseRecipientGuardianProfileIDsRejectsForgedValuesAndDuplicates(t *testing.T) {
	t.Parallel()

	for _, raw := range [][]string{{"0"}, {"-1"}, {"abc"}, {"7", "7"}} {
		_, err := parseRecipientGuardianProfileIDs(raw)
		assert.ErrorIs(t, err, parentService.ErrRequestSharingInvalid)
	}
	ids, err := parseRecipientGuardianProfileIDs([]string{"7", " 9 "})
	require.NoError(t, err)
	assert.Equal(t, []int64{7, 9}, ids)
}

func TestRenderRequestSharingErrorUsesStableFamilyProtectionCode(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/parent/me/children/1/request-sharing/excused/2", nil)
	renderRequestSharingError(recorder, request, parentService.ErrRequestSharingForbidden)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"family_protection"`)
}

func TestRequestSharingResponseNeverExposesAccountIDs(t *testing.T) {
	t.Parallel()

	response := toRequestSharingResponse(&parentService.RequestSharingState{
		Recipients: []parentService.RequestSharingRecipient{{
			GuardianProfileID: 7, FirstName: "Mara", LastName: "Muster", Selected: true,
		}},
	})
	require.Len(t, response.Recipients, 1)
	assert.Equal(t, "7", response.Recipients[0].GuardianProfileID)
	assert.True(t, response.Recipients[0].Selected)
}
