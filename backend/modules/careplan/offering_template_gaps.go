package careplan

import "context"

// Offering without a matching Regeltermin (#5140): the Betreuungsplan names
// an offering whose booked children no Regeltermin takes over. The school
// either creates or connects one, or decides that these children need no
// Regeltermin of their own. Care Plan stores that decision on the offering,
// per planning period as the school classes it covers, because the offering
// owns its bookings; the Timetable owner compares it with the classes still
// uncovered in the same period.

// MaxTemplateGapAckClasses bounds one decision; a school has far fewer
// classes.
const MaxTemplateGapAckClasses = 200

// CareOfferingTemplateGapCommand writes the decision on the offering row.
type CareOfferingTemplateGapCommand interface {
	// SetCareOfferingTemplateGapAck stores the classes the decision covers
	// in the planning period; an empty list withdraws it for that period.
	// ErrCareOfferingNotFound for an unknown offering,
	// ErrInvalidCareOffering for an invalid period or list.
	SetCareOfferingTemplateGapAck(ctx context.Context, offeringID, calendarPeriodID int64, schoolClasses []string) error
}

// CareOfferingTemplateGapDecision is the catalog's command for the
// Betreuungsplan.
type CareOfferingTemplateGapDecision interface {
	// DecideOfferingTemplateGap records that the booked children of the
	// listed classes need no Regeltermin of the offering in the planning
	// period; an empty list withdraws the decision so the hint shows again.
	DecideOfferingTemplateGap(ctx context.Context, offeringID, calendarPeriodID int64, schoolClasses []string) error
}
