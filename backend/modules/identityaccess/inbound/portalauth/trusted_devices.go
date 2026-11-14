package portalauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

// The own list and revoke of the remembered devices, one shape in every
// account portal (#6955). The portal reads the account and what its devices
// are bound to from the session and hands the handlers the bound call and
// its refusals; ownership is proven again in the owner, so a foreign or
// another portal's device answers 403 like an unknown one.

// Refusal answers a refused call of a signed-in account.
type Refusal func(w http.ResponseWriter, r *http.Request, err error)

// RefuseMFA answers by the code-step table every account portal shares.
func RefuseMFA(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, MFARules) }

// TrustedDeviceIDParam is the path parameter of the revoke route.
const TrustedDeviceIDParam = "deviceId"

// ListTrustedDevices answers the signed-in account's remembered devices as
// TrustedDeviceDTOs.
func ListTrustedDevices(w http.ResponseWriter, r *http.Request, list func(context.Context) ([]identityaccess.AccountTrustedDevice, error), refuse Refusal) {
	devices, err := list(r.Context())
	if err != nil {
		refuse(w, r, err)
		return
	}
	render.JSON(w, r, common.MapTrustedDevices(devices, func(d identityaccess.AccountTrustedDevice) common.TrustedDeviceRow {
		return common.TrustedDeviceRow{
			ID: d.ID, UserAgent: d.UserAgent, IPAddress: d.IPAddress,
			CreatedAt: d.CreatedAt, ExpiresAt: d.ExpiresAt, LastUsedAt: d.LastUsedAt,
		}
	}))
}

// RevokeTrustedDevice forgets the remembered device the path names and
// answers 204; a malformed id answers 400.
func RevokeTrustedDevice(w http.ResponseWriter, r *http.Request, revoke func(context.Context, int64) error, refuse Refusal) {
	deviceID, ok := common.ParsePositiveInt64IDWithError(w, r, TrustedDeviceIDParam, "invalid device id")
	if !ok {
		return
	}
	if err := revoke(r.Context(), deviceID); err != nil {
		refuse(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var (
	errPasswordRequired     = errors.New("current_password is required")
	errCurrentPasswordWrong = errors.New("current password is incorrect")
)

// SwitchOffFactor switches a voluntary second factor off once the owner
// checked the current password. A wrong password answers on its field, like
// the password change.
func SwitchOffFactor(w http.ResponseWriter, r *http.Request, disable func(ctx context.Context, currentPassword string, client Client) error, refuse Refusal) {
	var request struct {
		CurrentPassword string `json:"current_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.CurrentPassword == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errPasswordRequired))
		return
	}
	err := disable(r.Context(), request.CurrentPassword, ClientOf(r))
	switch {
	case err == nil:
		common.RespondNoContent(w, r)
	case errors.Is(err, identityaccess.ErrInvalidCredentials):
		common.RenderError(w, r, common.ErrorUnauthorizedOnField(errCurrentPasswordWrong, common.CodeIdentityCurrentPasswordWrong, "current_password"))
	default:
		refuse(w, r, err)
	}
}
