package device

import (
	"net/http"
	"os"

	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
)

func newLastSeenDebouncer() *LastSeenDebouncer { return NewLastSeenDebouncer() }

func DeviceAuthenticator(
	iotService iotSvc.Service,
	schools SchoolLookup,
	staffPINAuthenticator StaffPINAuthenticator,
	pinResolver PINResolver,
	fallbackPIN ...string,
) func(http.Handler) http.Handler {
	pin := os.Getenv("OGS_DEVICE_PIN")
	if len(fallbackPIN) > 0 {
		pin = fallbackPIN[0]
	}
	return DeviceAuthenticatorWithDebouncer(iotService, schools, staffPINAuthenticator, pinResolver, pin, NewLastSeenDebouncer())
}

func DeviceOnlyAuthenticator(iotService iotSvc.Service, schools SchoolLookup) func(http.Handler) http.Handler {
	return DeviceOnlyAuthenticatorWithDebouncer(iotService, schools, NewLastSeenDebouncer())
}
