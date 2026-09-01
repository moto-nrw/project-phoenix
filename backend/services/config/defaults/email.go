package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

// Tenant mail identity (#1936). Answers to tenant-bound e-mail must reach the
// OGS, not moto. Only the return path is configurable: the visible sender
// stays the central authenticated address, because putting a school's own
// domain in From without a verified domain would be spoofing and would damage
// the deliverability we are still repairing (#1215).
func init() {
	// Deliberately permissive: a school may legitimately point replies at a
	// shared mailbox on any domain. AllowEmpty keeps "not configured" a valid
	// state — the resolver then falls back to the school contact address.
	replyToPattern := `^[^@\s]+@[^@\s]+\.[^@\s]+$`

	config.Register(config.Definition{
		Key:   config.KeyEmailReplyToAddress,
		Label: "Antwortadresse für Eltern-E-Mails",
		Description: "An diese Adresse gehen Antworten, wenn Eltern auf eine E-Mail der OGS antworten " +
			"(Einladung, Anmeldung, Elternmitteilung). Ohne Eintrag wird die Kontaktadresse der Schule verwendet. " +
			"Der sichtbare Absender bleibt die zentrale moto-Adresse.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "e-mail",
		SortOrder:       1,
		Validation: &config.ValidationRules{
			AllowEmpty: true,
			Pattern:    &replyToPattern,
		},
	})
}
