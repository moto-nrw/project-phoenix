package authorize

import "github.com/moto-nrw/project-phoenix/internal/seedtoken"

// SeedInvitationTokenHeader is the request header the local seeder sets to
// receive raw invitation tokens.
const SeedInvitationTokenHeader = seedtoken.Header

// ShouldExposeSeedInvitationToken gates the seed-only raw invitation token in
// an invite response: the header must ask for it, the app environment must
// be a local one and the request must come from a local host.
func ShouldExposeSeedInvitationToken(headerValue, requestHost, appEnv string) bool {
	return seedtoken.ShouldExposeInvitationToken(headerValue, requestHost, appEnv)
}
