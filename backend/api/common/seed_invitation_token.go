package common

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
)

// ShouldExposeSeedInvitationToken reports whether the request may receive
// the raw invitation token: only the local seeder, only in a local
// environment. Handlers call it to support demo seeding and nothing else.
func ShouldExposeSeedInvitationToken(r *http.Request, appEnv string) bool {
	return authorize.ShouldExposeSeedInvitationToken(r.Header.Get(authorize.SeedInvitationTokenHeader), r.Host, appEnv)
}
