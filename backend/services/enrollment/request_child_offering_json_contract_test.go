package enrollment

import (
	"encoding/json"
	"testing"
	"time"

	legacy "github.com/moto-nrw/project-phoenix/models/enrollment"
	owner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/require"
)

func TestOwnerOfferingSelectionPreservesLegacyJSON(t *testing.T) {
	t.Parallel()
	for _, populated := range []bool{false, true} {
		name := "optional_fields_omitted"
		if populated {
			name = "dated_selection"
		}
		t.Run(name, func(t *testing.T) {
			row := &legacy.RequestChildOffering{RequestChildID: 9007199254740993, CareOfferingID: 9007199254740995}
			row.ID = 9007199254740997
			row.TenantID = 9007199254740999
			row.CreatedAt = time.Date(2027, 3, 28, 0, 0, 0, 0, time.UTC)
			row.UpdatedAt = row.CreatedAt
			if populated {
				from, until := owner.Date("2027-03-28"), owner.Date("2027-10-31")
				note := "selection history"
				row.ValidFrom, row.ValidUntil, row.Notes = &from, &until, &note
				row.SelectedDays = []string{"mon", "wed"}
				row.ManualSelectedDays = []string{"mon"}
				row.AutomaticSelectedDays = []string{"wed"}
			}
			before, err := json.Marshal(row)
			require.NoError(t, err)
			var migrated owner.RequestChildOffering
			require.NoError(t, json.Unmarshal(before, &migrated))
			require.Equal(t, row.ID, migrated.ID)
			require.Equal(t, row.RequestChildID, migrated.RequestChildID)
			after, err := json.Marshal(migrated)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after))
			converted := legacyOfferingSelections([]*owner.RequestChildOffering{&migrated})
			require.Equal(t, row, converted[0])
			if populated {
				require.Equal(t, owner.Date("2027-03-28"), *migrated.ValidFrom)
				require.Equal(t, owner.Date("2027-10-31"), *migrated.ValidUntil)
			}
		})
	}
}
