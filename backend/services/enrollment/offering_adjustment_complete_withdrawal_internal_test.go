package enrollment

import (
	"testing"

	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// TestOrdinarySaveStillEnforcesRequiredCare pins that the complete
// withdrawal Care Plan's offering adjustments allow (#3561) never reaches an
// ordinary enrollment save: required care and the minimum selection hold.
func TestOrdinarySaveStillEnforcesRequiredCare(t *testing.T) {
	t.Parallel()
	care := &enrollmentModels.CareOffering{
		ID: 1, Name: "Ganztag", CountsAsCare: true,
		IsRequired: true, DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays: []string{"mon", "tue"},
	}
	lunch := &enrollmentModels.CareOffering{
		ID: 2, Name: "Mittagessen", CountsAsCare: false,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	catalog := map[int64]*enrollmentModels.CareOffering{care.ID: care, lunch.ID: lunch}
	children := []SubmitChild{{OfferingIDs: []int64{lunch.ID}}}

	_, err := materializeAndValidateChildrenOfferingSelectionsGrandfathering(
		children, catalog, enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, GrandfatheredOfferings{},
	)
	require.Error(t, err, "ordinary saves must still enforce required care and minimum selection")
}
