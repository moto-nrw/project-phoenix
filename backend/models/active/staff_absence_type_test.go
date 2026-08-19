package active

import (
	"strings"
	"testing"
)

func TestStaffAbsenceTypeValidateTrimsName(t *testing.T) {
	at := &StaffAbsenceType{Name: "  Regenerationstag  "}
	if err := at.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if at.Name != "Regenerationstag" {
		t.Errorf("expected trimmed name, got %q", at.Name)
	}
}

func TestStaffAbsenceTypeValidateDefaultsBaseTypeToOther(t *testing.T) {
	at := &StaffAbsenceType{Name: "Ferienzeit"}
	if err := at.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if at.BaseType != AbsenceTypeOther {
		t.Errorf("expected base type %q, got %q", AbsenceTypeOther, at.BaseType)
	}
}

func TestStaffAbsenceTypeValidateRejectsEmptyName(t *testing.T) {
	for _, name := range []string{"", "   ", "\t\n"} {
		at := &StaffAbsenceType{Name: name}
		if err := at.Validate(); err == nil {
			t.Errorf("expected error for name %q", name)
		}
	}
}

func TestStaffAbsenceTypeValidateRejectsOverlongName(t *testing.T) {
	at := &StaffAbsenceType{Name: strings.Repeat("ä", 101)}
	if err := at.Validate(); err == nil {
		t.Error("expected error for a 101-character name")
	}
	// Counted in runes, not bytes: 100 Umlaute are 200 bytes but a legal name.
	ok := &StaffAbsenceType{Name: strings.Repeat("ä", 100)}
	if err := ok.Validate(); err != nil {
		t.Errorf("unexpected error for a 100-rune name: %v", err)
	}
}

func TestStaffAbsenceTypeValidateRejectsUnknownBaseType(t *testing.T) {
	at := &StaffAbsenceType{Name: "Regenerationstag", BaseType: "regeneration"}
	if err := at.Validate(); err == nil {
		t.Error("expected error for an unknown base type")
	}
}
