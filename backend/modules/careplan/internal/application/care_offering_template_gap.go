package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// DecideOfferingTemplateGap records the school's decision that the booked
// children of the listed classes need no Regeltermin of the offering
// (#5140). The offering must exist in the tenant; deactivated offerings keep
// their decision for a later reactivation. Repeating the stored decision
// writes nothing.
func (c *CareOfferingCatalog) DecideOfferingTemplateGap(ctx context.Context, offeringID int64, schoolClasses []string) error {
	offering, err := c.deps.Records.FindCareOffering(ctx, offeringID)
	if errors.Is(err, careplan.ErrCareOfferingNotFound) {
		return fmt.Errorf("care offering %d not found: %w", offeringID, err)
	}
	if err != nil {
		return fmt.Errorf("failed to find care offering: %w", err)
	}
	if slices.Equal(offering.TemplateGapAckClasses, schoolClasses) {
		return nil
	}
	if err := c.deps.Records.SetCareOfferingTemplateGapAck(ctx, offeringID, schoolClasses); err != nil {
		return fmt.Errorf("failed to decide care offering without a Regeltermin: %w", err)
	}
	return nil
}
