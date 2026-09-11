package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

// SFTP target for the manual transfer of Zeitwirtschafts-/DATEV exports
// (#3050). Exactly ONE target per school.
//
// It lives under Einstellungen → System in the "Schnittstellen" category: a
// connection to an outside system is infrastructure, not payroll bookkeeping,
// and the next such connection belongs beside it rather than in whatever
// screen happens to use it first. The switch is the gate — with it off, the
// export dialog offers no transfer at all.
//
// Host, username, password, directory and fingerprint default to the EMPTY
// STRING: a school without a target has no target, and an invented preset
// would point a payroll file at a stranger. Only the port carries a default,
// because 22 is the protocol's own standard rather than a per-school value.
//
// The fingerprint is not optional. Without it the client cannot tell the
// configured counterpart from anything else answering on that address, so an
// empty fingerprint keeps the transfer switched off just like a missing host.
func init() {
	// A hostname or literal IP. The real reachability rules (public address
	// only, no loopback/private ranges) are enforced at connect time in the
	// SFTP adapter — a pattern cannot decide them.
	hostPattern := `^[A-Za-z0-9]([A-Za-z0-9.\-]{0,253}[A-Za-z0-9])?$`
	usernamePattern := `^[A-Za-z0-9._@\-]{1,64}$`
	// Absolute POSIX path. The charset excludes backslashes and spaces; the
	// ".." rejection lives in the resolver, since RE2 has no lookahead.
	directoryPattern := `^/[A-Za-z0-9._/\-]{0,255}$`
	// OpenSSH's SHA256 fingerprint: base64 without padding, 43 characters.
	// This is exactly what `ssh-keyscan -t <type> host | ssh-keygen -lf -`
	// prints and what ssh.FingerprintSHA256 produces.
	fingerprintPattern := `^SHA256:[A-Za-z0-9+/]{43}$`

	minPort := 1.0
	maxPort := 65535.0

	config.Register(config.Definition{
		Key:             config.KeySFTPEnabled,
		Label:           "SFTP-Übertragung",
		Description:     "Schaltet die Übertragung der Zeitkonten-Exporte ein. Im Export-Dialog erscheint dann neben dem Herunterladen die Übertragung. Solange eine der Angaben unten fehlt, wird nichts übertragen.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "schnittstellen",
		SortOrder:       10,
		AccessPolicy:    config.AccessAdminOnly,
	})

	config.Register(config.Definition{
		Key:             config.KeySFTPHost,
		Label:           "Adresse der Gegenstelle",
		Description:     "Adresse der Stelle, die die Datei entgegennimmt, zum Beispiel dateien.beispiel.de. Diese Angabe erhalten Sie von dort. Adressen im eigenen Netz sind nicht möglich.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "schnittstellen",
		SortOrder:       20,
		Validation:      &config.ValidationRules{Pattern: &hostPattern, AllowEmpty: true},
		AccessPolicy:    config.AccessAdminOnly,
		DependsOn:       config.DependsOnEq(config.KeySFTPEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeySFTPPort,
		Label:           "Port",
		Description:     "Üblich ist 22. Ändern Sie die Zahl nur, wenn die Gegenstelle eine andere nennt.",
		Type:            config.FieldNumber,
		Default:         22,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "schnittstellen",
		SortOrder:       30,
		Validation:      &config.ValidationRules{Min: &minPort, Max: &maxPort},
		AccessPolicy:    config.AccessAdminOnly,
		DependsOn:       config.DependsOnEq(config.KeySFTPEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeySFTPUsername,
		Label:           "Benutzername",
		Description:     "Benutzername für die Anmeldung bei der Gegenstelle. Auch diese Angabe erhalten Sie von dort.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "schnittstellen",
		SortOrder:       40,
		Validation:      &config.ValidationRules{Pattern: &usernamePattern, AllowEmpty: true},
		AccessPolicy:    config.AccessAdminOnly,
		DependsOn:       config.DependsOnEq(config.KeySFTPEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeySFTPPassword,
		Label:           "Passwort",
		Description:     "Passwort für die Anmeldung bei der Gegenstelle. Nach dem Speichern wird es nicht mehr angezeigt.",
		Type:            config.FieldPassword,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "schnittstellen",
		SortOrder:       50,
		Validation:      &config.ValidationRules{AllowEmpty: true},
		AccessPolicy:    config.AccessAdminOnly,
		DependsOn:       config.DependsOnEq(config.KeySFTPEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeySFTPRemoteDirectory,
		Label:           "Zielordner",
		Description:     "Ordner bei der Gegenstelle, in dem die Datei abgelegt wird, zum Beispiel /upload/lohn. Der Ordner muss dort schon vorhanden sein.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "schnittstellen",
		SortOrder:       60,
		Validation:      &config.ValidationRules{Pattern: &directoryPattern, AllowEmpty: true},
		AccessPolicy:    config.AccessAdminOnly,
		DependsOn:       config.DependsOnEq(config.KeySFTPEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeySFTPHostKeyFingerprint,
		Label:           "Fingerabdruck der Gegenstelle",
		Description:     "Pflichtangabe. Der Fingerabdruck beginnt mit SHA256: und wird Ihnen von der Gegenstelle genannt. Er stellt sicher, dass die Datei wirklich dort ankommt. Passt er nicht, wird nichts übertragen. Wichtig: Eine Gegenstelle hat oft mehrere Schlüssel. Fragen Sie nach dem Fingerabdruck für den RSA-Schlüssel.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "schnittstellen",
		SortOrder:       70,
		Validation:      &config.ValidationRules{Pattern: &fingerprintPattern, AllowEmpty: true},
		AccessPolicy:    config.AccessAdminOnly,
		DependsOn:       config.DependsOnEq(config.KeySFTPEnabled, true),
	})
}
