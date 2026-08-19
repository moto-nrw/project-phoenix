package active

import (
	"errors"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

const maxAbsenceTypeNameLength = 100

// StaffAbsenceType is a tenant-defined display name for a staff absence
// (#2403). Tarif- and Arbeitsvertrag-specific wordings differ per school
// ("Regenerationstag", "Ferienzeit", "Sonderurlaub"), so the *label* is
// school data while the *arithmetic* stays a fixed, code-owned concern.
//
// The split: BaseType names the canonical absence type whose calculation the
// entry inherits — it is written into staff_absences.absence_type and drives
// Sollzeit, Stundenkonto, Urlaubskontingent, Monatskarte and every export
// exactly as before. Name is only ever rendered. In this first version every
// custom entry inherits AbsenceTypeOther, so a freely typed name can never
// silently move a day into the vacation quota or credit hours it should not.
//
// The five standard types (Urlaub, Krank, Fortbildung, Sonstige,
// Freizeitausgleich) are deliberately NOT rows: they are the constants in this
// package. Every school therefore has them by construction — nothing to seed,
// nothing that can be missing for one tenant, no duplicate rows, and no way to
// delete or rename one by accident. Only the school's own additions live here.
type StaffAbsenceType struct {
	base.Model `bun:"schema:active,table:staff_absence_types"`
	base.TenantModel
	Name string `bun:"name,notnull" json:"name"`
	// BaseType is the canonical absence type this entry is a named subtype of.
	// v1 always stores AbsenceTypeOther; the column exists so a later version
	// can give a custom art its own arithmetic without a second migration of
	// every existing row.
	BaseType string `bun:"base_type,notnull" json:"base_type"`
	// No bun `default:true` tag on purpose: bun writes DEFAULT (not the value)
	// for a zero-value field carrying a default, so `false` would silently
	// become the column default TRUE on INSERT and an entry could never be
	// deactivated. Mirrors schedule.ShiftType.
	IsActive bool `bun:"is_active,notnull" json:"is_active"`
}

// Validate normalizes and checks the absence type: it trims the name, rejects
// an empty or over-long one, and defaults/validates the base type.
func (t *StaffAbsenceType) Validate() error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return errors.New("absence type name is required")
	}
	if len([]rune(t.Name)) > maxAbsenceTypeNameLength {
		return errors.New("absence type name must not exceed 100 characters")
	}
	if t.BaseType == "" {
		t.BaseType = AbsenceTypeOther
	}
	if !slices.Contains(ValidAbsenceTypes, t.BaseType) {
		return errors.New("invalid absence base type")
	}
	return nil
}
