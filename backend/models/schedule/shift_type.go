package schedule

import (
	"errors"
	"regexp"
	"strings"
)

const (
	maxShiftTypeNameLength        = 100
	maxShiftTypeDescriptionLength = 500
	DefaultShiftTypeColor         = "#6B7280"
)

// hexColorPattern matches #RRGGBB or #RGB hex colors (mirrors activities.Category).
var shiftTypeHexColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

// ShiftType is a tenant-configurable label for a planned staff shift
// (Schichtart): the *kind* of duty, not just its wall-clock window. Schools
// define their own set (e.g. Betreuung/Zeit am Kind, Vorbereitung, Vertretungs-
// unterricht, Pause, Sonstiges); nothing is hard-wired. A StaffShift may point
// at one shift type via a nullable FK — the color makes different duties
// immediately distinguishable in the Dienstplan week grid (#1836).
type ShiftType struct {
	Model `bun:"schema:schedule,table:shift_types"`
	TenantModel
	Name        string `bun:"name,notnull" json:"name"`
	Color       string `bun:"color,notnull" json:"color"`
	Description string `bun:"description" json:"description,omitempty"`
	// No bun `default:true` tag on purpose: bun writes DEFAULT (not the value)
	// for any zero-value field that carries a default, so `false` would silently
	// become the column default TRUE on INSERT and an admin could never create an
	// inactive shift type. The DB column keeps its own DEFAULT TRUE for raw
	// inserts; through this model bun always sends the explicit bool.
	IsActive bool `bun:"is_active,notnull" json:"is_active"`
}

// expandShorthandHex turns a validated 3-digit "#RGB" color into its 6-digit
// "#RRGGBB" form so stored colors are always the shape the frontend's native
// <input type="color"> can display and round-trip. The input must already have
// matched shiftTypeHexColorPattern.
func expandShorthandHex(color string) string {
	if len(color) != 4 { // "#" + 3 hex digits
		return color
	}
	r, g, b := color[1], color[2], color[3]
	return string([]byte{'#', r, r, g, g, b, b})
}

// Validate normalizes and checks the shift type. It trims the name, defaults an
// empty color to gray, prepends a missing "#", enforces a hex color, and
// expands a 3-digit shorthand to the 6-digit form.
func (t *ShiftType) Validate() error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return errors.New("shift type name is required")
	}
	if len([]rune(t.Name)) > maxShiftTypeNameLength {
		return errors.New("shift type name must not exceed 100 characters")
	}
	if len([]rune(t.Description)) > maxShiftTypeDescriptionLength {
		return errors.New("shift type description must not exceed 500 characters")
	}

	color := strings.TrimSpace(t.Color)
	if color == "" {
		color = DefaultShiftTypeColor
	}
	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}
	if !shiftTypeHexColorPattern.MatchString(color) {
		return errors.New("invalid color format, must be a valid hex color")
	}
	t.Color = strings.ToUpper(expandShorthandHex(color))

	return nil
}
