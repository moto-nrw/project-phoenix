package portalauth

import (
	"context"
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
