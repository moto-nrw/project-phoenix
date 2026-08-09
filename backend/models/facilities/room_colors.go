package facilities

import "strings"

// RoomColorUniqueConstraintName is the index name created by migration
// 1.15.45. Service layer matches it via pgdriver.Error.Field('n') so a
// generic ErrDuplicateRoom doesn't shadow the more specific
// ErrColorAlreadyInUse for the colour-conflict path. Migration uses the same
// name so both ends stay in sync from a single source.
const RoomColorUniqueConstraintName = "uniq_facilities_rooms_tenant_color"

// RoomWCAliasUniqueConstraintName is the partial unique index created by
// migration 1.15.48 to enforce "at most one canonical toilet alias per
// tenant" — predicate is `name IN ('WC','Toilette')`, mirroring
// constants.IsWCRoomName. The application-level guard in
// CreateRoom/UpdateRoom closes the common path; this index closes the
// TOCTOU race where two concurrent admin requests (one for "WC", one for
// "Toilette") both pass the in-app check and both insert. Migration uses
// the same name so both ends stay in sync from a single source.
const RoomWCAliasUniqueConstraintName = "uniq_facilities_rooms_tenant_wc_alias"

// reservedRoomColors is the set of hex codes that rooms cannot adopt.
//
// Most entries mirror the frontend status palette
// (frontend/src/lib/location-helper.ts LOCATION_COLORS) — letting a room
// carry one of those would make its badge visually indistinguishable from a
// status like "Unterwegs" or "Schulhof".
//
// One entry — #4F46E5 — is reserved for a different reason: it was the
// forced default produced by the rooms.config.tsx transformBeforeSubmit bug,
// stamped onto every saved room before Issue #1324. Migration 1.15.45 NULLs
// every row carrying this hex (audit.room_color_migration_backup keeps the
// originals). Allowing an admin to re-pick the same hex afterwards would
// recreate the same situation the migration just cleaned up — and would also
// collide with the backup table's restore semantics ("any row holding
// #4F46E5 must have come from the bug, never from a deliberate pick").
// Reserving it keeps that invariant explicit forever.
//
// Stored as uppercase #RRGGBB strings; Validate() upper-cases incoming values
// before lookup so input casing does not matter. Mirrors LOCATION_COLORS in
// frontend/src/lib/location-helper.ts. The drift test
// (room_colors_drift_test.go) catches divergence on the status palette half;
// #4F46E5 is intentionally backend-only and is excluded from that comparison
// (knownReserved excludes it via the legacyBugDefault constant).
var reservedRoomColors = map[string]struct{}{
	"#83CD2D":           {}, // GROUP_ROOM (eigener Gruppenraum, grün)
	"#5080D8":           {}, // OTHER_ROOM fallback (blau)
	"#6B7280":           {}, // HOME (Zuhause)
	"#F78C10":           {}, // SCHOOLYARD (Schulhof)
	"#D946EF":           {}, // TRANSIT (Unterwegs)
	"#78716C":           {}, // UNKNOWN (Unbekannt)
	"#DC2626":           {}, // SICK / DANGER (Krank / Gefahr)
	"#7C3AED":           {}, // EXCUSED (Entschuldigt)
	"#0891B2":           {}, // CLASS_TRIP (Klassenfahrt)
	"#365D83":           {}, // NOT_ARRIVAL (Kommt heute nicht)
	"#FF3130":           {}, // previous HOME status color
	"#EAB308":           {}, // WARNING (Wartet / unbesetzt) — was the SICK color before the palette move
	legacyBugDefaultHex: {}, // see migration 1.15.45 / room_colors.go header comment
}

// legacyBugDefaultHex is the colour the rooms.config.tsx forced-default bug
// stamped onto every save before Issue #1324. Pinned as a constant so the
// drift test can exclude it from the cross-codebase comparison without
// hardcoding the value in two places.
const legacyBugDefaultHex = "#4F46E5"

// IsReservedRoomColor reports whether color matches one of the status badge
// colors that rooms are not allowed to claim. The input is normalized to the
// canonical "#RRGGBB" upper-case form before lookup so "#5080d8", "5080D8"
// and "#5080D8" are all rejected the same way.
func IsReservedRoomColor(color string) bool {
	normalized := normalizeHexForReservedLookup(color)
	if normalized == "" {
		return false
	}
	_, found := reservedRoomColors[normalized]
	return found
}

// normalizeHexForReservedLookup expands #RGB to #RRGGBB and uppercases. Invalid
// inputs return "" — the caller is expected to have already validated format.
func normalizeHexForReservedLookup(color string) string {
	color = strings.ToUpper(strings.TrimSpace(color))
	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}
	switch len(color) {
	case 7: // #RRGGBB
		return color
	case 4: // #RGB → expand each nibble
		var b strings.Builder
		b.WriteByte('#')
		for i := 1; i < 4; i++ {
			b.WriteByte(color[i])
			b.WriteByte(color[i])
		}
		return b.String()
	default:
		return ""
	}
}
