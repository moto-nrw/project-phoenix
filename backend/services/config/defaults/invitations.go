package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	// --- Guardian invitation token expiry ---
	//
	// Operator-only because token TTL is auth plumbing. Schools shouldn't
	// be able to widen the window (longer-lived tokens are a security
	// concern) without operator review. Mirrors the staff invitation
	// expiry pattern from the env var INVITATION_TOKEN_EXPIRY_HOURS but
	// adds per-tenant override capability through the registry.

	minExpiry := float64(1)
	maxExpiry := float64(168) // one week
	config.Register(config.Definition{
		Key:             config.KeyGuardianInvitationTokenExpiryHours,
		Label:           "Gültigkeitsdauer Eltern-Einladung (Stunden)",
		Description:     "Wie lange ein Einladungslink für Erziehungsberechtigte gültig bleibt, bevor eine neue Einladung versendet werden muss.",
		Type:            config.FieldNumber,
		Default:         48,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "einladungen",
		SortOrder:       20,
		Validation:      &config.ValidationRules{Min: &minExpiry, Max: &maxExpiry},
		AccessPolicy:    config.AccessOperatorOnly,
	})
}
