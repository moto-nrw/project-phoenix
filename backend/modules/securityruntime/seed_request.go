package securityruntime

import "github.com/moto-nrw/project-phoenix/auth/authorize"

const SeedInvitationTokenHeader = authorize.SeedInvitationTokenHeader

// ShouldExposeSeedInvitationToken permits the local seeder's token response.
func ShouldExposeSeedInvitationToken(headerValue, host, appEnv string) bool {
	return authorize.ShouldExposeSeedInvitationToken(headerValue, host, appEnv)
}

// IsLocalSeedRequest checks both the seeder header and the local environment/host.
func IsLocalSeedRequest(headerValue, host, appEnv string) bool {
	return authorize.IsLocalSeedRequest(headerValue, host, appEnv)
}
