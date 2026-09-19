package dataimport

import "strings"

// These are import-file values, not permission grants. The receiving owner
// derives and enforces the permissions for each student relationship.
// guardianRoleAliases maps the German labels of the "ErzN.Rolle" column (and
// the raw preset names) to the stored guardian_role presets.
var guardianRoleAliases = map[string]string{
	"hauptsorgeberechtigt": "primary_guardian", "hauptsorgeberechtigte": "primary_guardian", "hauptsorgeberechtigter": "primary_guardian",
	"sorgeberechtigt": "legal_guardian", "sorgeberechtigte": "legal_guardian", "sorgeberechtigter": "legal_guardian",
	"mitsorgeberechtigt": "co_guardian", "mitsorgeberechtigte": "co_guardian", "mitsorgeberechtigter": "co_guardian",
	"notfallkontakt": "emergency_contact",
	"nur abholung":   "pickup_only", "abholperson": "pickup_only", "abholung": "pickup_only",
	"sozialarbeit": "social_worker", "sozialarbeiter": "social_worker", "sozialarbeiterin": "social_worker",
	"benutzerdefiniert": "custom",
}

// MapGuardianRole resolves a "ErzN.Rolle" cell to a stored preset. Returns
// ("", false) for an unknown label so the caller can report it; an empty cell
// maps to ("", true) meaning "derive the default".
func MapGuardianRole(raw string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "", true
	}
	if mapped, ok := guardianRoleAliases[normalized]; ok {
		return mapped, true
	}
	switch normalized {
	case "primary_guardian", "legal_guardian", "co_guardian",
		"emergency_contact", "pickup_only", "social_worker", "custom":
		return normalized, true
	}
	return "", false
}
