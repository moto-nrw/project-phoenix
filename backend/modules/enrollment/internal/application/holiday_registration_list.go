package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The list reads of the review (#4663): the list of a period, its recipients
// for the Elternabrechnung (#5625) and the recorded read of both.

// ListHolidayRegistrations lists the registrations of the filter with the
// status counts of the whole period, and records the read. The list names
// no postal address (#5625).
func (h *HolidayRegistrationReview) ListHolidayRegistrations(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, filter enrollment.HolidayRegistrationFilter) (enrollment.HolidayRegistrationList, error) {
	all, matching, err := h.listPeriod(ctx, reviewer, periodID, filter)
	if err != nil {
		return enrollment.HolidayRegistrationList{}, err
	}
	list := enrollment.HolidayRegistrationList{
		Registrations: make([]enrollment.HolidayRegistrationSummary, 0, len(matching)), Total: len(all), StatusCounts: map[string]int{},
	}
	for _, registration := range all {
		list.StatusCounts[registration.Status]++
	}
	for _, registration := range matching {
		list.Registrations = append(list.Registrations, registration.HolidayRegistrationSummary)
	}
	return list, nil
}

// ListHolidayRegistrationRecipients lists the registrations of the filter
// with the postal address of the parent who registered, for the invoices of
// the Elternabrechnung (#5625), and records the read like the list.
func (h *HolidayRegistrationReview) ListHolidayRegistrationRecipients(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, filter enrollment.HolidayRegistrationFilter) ([]enrollment.HolidayRegistrationRecipient, error) {
	_, matching, err := h.listPeriod(ctx, reviewer, periodID, filter)
	if err != nil {
		return nil, err
	}
	recipients := make([]enrollment.HolidayRegistrationRecipient, len(matching))
	for i, registration := range matching {
		recipients[i] = enrollment.HolidayRegistrationRecipient{
			HolidayRegistrationSummary: registration.HolidayRegistrationSummary, GuardianAddress: registration.GuardianAddress,
			HasBenefitsProof: registration.BenefitsProofAttached,
		}
	}
	return recipients, nil
}

// listPeriod reads every registration of the period and those of the
// filter, and records the read in the trail.
func (h *HolidayRegistrationReview) listPeriod(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, filter enrollment.HolidayRegistrationFilter) ([]enrollment.HolidayRegistrationDetail, []enrollment.HolidayRegistrationDetail, error) {
	period, all, err := h.readPeriod(ctx, reviewer, periodID, filter)
	if err != nil {
		return nil, nil, err
	}
	matching := []enrollment.HolidayRegistrationDetail{}
	for _, registration := range all {
		if holidayRegistrationMatches(registration.HolidayRegistrationSummary, filter) {
			matching = append(matching, registration)
		}
	}
	if err := h.audit(ctx, reviewer, period.ID, 0, enrollment.HolidayAuditList, map[string]any{
		"filter": holidayFilterDetails(filter), "registrations": len(matching),
	}); err != nil {
		return nil, nil, err
	}
	return all, matching, nil
}
