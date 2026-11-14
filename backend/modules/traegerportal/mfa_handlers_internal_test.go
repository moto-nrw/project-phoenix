package traegerportal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

// exchangeFixture is a Träger code step whose second factor and session
// mint are fakes: it records what the exchange asked to remember.
type exchangeFixture struct {
	prepareErr     error
	prepared       []int64
	issued         int
	mintedTraeger  int64
	organizationID int64
}

func (f *exchangeFixture) resource() *Resource {
	const accountID = 4711
	return &Resource{
		auth: AuthRuntime{
			Login: func(context.Context, string, string, string, string, int64, string) (*identityaccess.TraegerLoginResult, error) {
				return nil, errors.New("not used")
			},
			IssueTokens: func(_ context.Context, _ VerifiedChallenge, organizationID int64, _, _ string) (*identityaccess.TraegerLoginResult, error) {
				f.mintedTraeger = organizationID
				return &identityaccess.TraegerLoginResult{AccessToken: "access", RefreshToken: "refresh", OrganizationID: organizationID}, nil
			},
		},
		mfa: MFARuntime{
			VerifyChallenge: func(context.Context, string, string) (VerifiedChallenge, error) {
				return VerifiedChallenge{AccountID: accountID}, nil
			},
			VerifyChallengeForOwner: func(context.Context, string, string, int64) (VerifiedChallenge, error) {
				return VerifiedChallenge{AccountID: accountID}, nil
			},
			ResendChallenge: func(context.Context, string, net.IP) (string, error) { return "", nil },
			StartChallenge:  func(context.Context, int64, net.IP, int64) (string, error) { return "", nil },
			Enroll:          func(context.Context, VerifiedChallenge) error { return nil },
			PrepareTrustedDevice: func(_ context.Context, _, organizationID int64, _ string, _ net.IP) (identityaccess.TrustedDeviceIssue, error) {
				f.prepared = append(f.prepared, organizationID)
				if f.prepareErr != nil {
					return nil, f.prepareErr
				}
				return func(context.Context) (string, time.Time, error) {
					f.issued++
					return "signed-device", time.Now().Add(time.Hour), nil
				}, nil
			},
			ListTrustedDevices: func(context.Context, int64, int64) ([]identityaccess.AccountTrustedDevice, error) {
				return nil, nil
			},
			RevokeTrustedDevice: func(context.Context, int64, int64, int64) error { return nil },
		},
	}
}

func (f *exchangeFixture) verify(t *testing.T, remember bool) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"challenge_token": "challenge", "code": "123456", "organization_id": f.organizationID, "remember_device": remember,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.resource().mfaVerify(rec, req)
	return rec
}

// trustedDeviceCookie is the remember-device cookie every portal sets.
const trustedDeviceCookie = "mfa_trust_device"

func deviceCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == trustedDeviceCookie {
			return cookie
		}
	}
	return nil
}

func TestTraegerCodeStepRemembersOnlyTheChosenDeviceAtItsTraeger(t *testing.T) {
	t.Parallel()
	fixture := &exchangeFixture{organizationID: 812}

	rec := fixture.verify(t, false)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, fixture.prepared, "without the choice nothing is prepared")
	assert.Nil(t, deviceCookie(rec))

	rec = fixture.verify(t, true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []int64{812}, fixture.prepared, "the device is remembered at the Träger of the sign-in")
	assert.Equal(t, int64(812), fixture.mintedTraeger)
	assert.Equal(t, 1, fixture.issued)
	cookie := deviceCookie(rec)
	require.NotNil(t, cookie)
	assert.Equal(t, "signed-device", cookie.Value)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
}

// A failure reading the trusted-device settings costs the remembered
// device, never the sign-in (#4641).
func TestTraegerCodeStepSignsInWhenTheDeviceSettingsFail(t *testing.T) {
	t.Parallel()
	fixture := &exchangeFixture{organizationID: 812, prepareErr: identityaccess.ErrMFAStatusUnavailable}

	rec := fixture.verify(t, true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"access_token":"access"`)
	assert.Zero(t, fixture.issued)
	assert.Nil(t, deviceCookie(rec))
}

func TestTraegerCodeStepWithoutTheTraegerNamedRemembersNothing(t *testing.T) {
	t.Parallel()
	fixture := &exchangeFixture{}

	rec := fixture.verify(t, true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, fixture.prepared, "a device needs its Träger")
	assert.Nil(t, deviceCookie(rec))
}
