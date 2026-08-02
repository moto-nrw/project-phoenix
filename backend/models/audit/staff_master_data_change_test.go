package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Stammdaten trail is append-only and must never store a no-op row: an
// entry whose old and new value are identical would claim a change that never
// happened (#1423).
func TestStaffMasterDataChange_Validate(t *testing.T) {
	valid := func() *StaffMasterDataChange {
		return &StaffMasterDataChange{
			StaffID:   9001,
			ChangedBy: 9002,
			Section:   StammdatenSectionPerson,
			FieldName: "last_name",
			OldValue:  "Mustermann",
			NewValue:  "Musterfrau",
		}
	}

	require.NoError(t, valid().Validate())

	for _, section := range []string{
		StammdatenSectionPerson,
		StammdatenSectionKontakt,
		StammdatenSectionArbeitsvertrag,
		StammdatenSectionQualifikation,
		StammdatenSectionBankSteuer,
	} {
		change := valid()
		change.Section = section
		require.NoError(t, change.Validate(), section)
	}

	tests := []struct {
		name    string
		mutate  func(*StaffMasterDataChange)
		wantErr string
	}{
		{"missing staff", func(c *StaffMasterDataChange) { c.StaffID = 0 }, "staff_id is required"},
		{"missing actor", func(c *StaffMasterDataChange) { c.ChangedBy = 0 }, "changed_by is required"},
		{"unknown section", func(c *StaffMasterDataChange) { c.Section = "dokumente" }, "unknown stammdaten section"},
		{"missing field", func(c *StaffMasterDataChange) { c.FieldName = "" }, "field_name is required"},
		{"no-op change", func(c *StaffMasterDataChange) { c.NewValue = c.OldValue }, "old and new value are identical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			change := valid()
			tt.mutate(change)
			err := change.Validate()
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}
