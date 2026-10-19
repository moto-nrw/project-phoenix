package dataimport

import "strings"

// MapRelationshipType converts German relationship types to valid English types
func MapRelationshipType(germanType string) string {
	normalized := strings.ToLower(strings.TrimSpace(germanType))

	// Map German terms to English types
	mapping := map[string]string{
		// Parent types
		"mutter":     "parent",
		"vater":      "parent",
		"mama":       "parent",
		"papa":       "parent",
		"elternteil": "parent",
		"parent":     "parent",

		// Guardian types
		"vormund":                "guardian",
		"erziehungsberechtigter": "guardian",
		"erziehungsberechtigte":  "guardian",
		"guardian":               "guardian",

		// Relative types
		"großmutter":  "relative",
		"großvater":   "relative",
		"oma":         "relative",
		"opa":         "relative",
		"tante":       "relative",
		"onkel":       "relative",
		"geschwister": "relative",
		"bruder":      "relative",
		"schwester":   "relative",
		"relative":    "relative",

		// Other types
		"sonstige": "other",
		"andere":   "other",
		"other":    "other",
	}

	if mapped, ok := mapping[normalized]; ok {
		return mapped
	}

	// Default to "other" for unknown types
	return "other"
}
