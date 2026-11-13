package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The trail of the hint on a known child (#6683, ADR 0214). The Träger
// projection compares the registered children with the children of the
// Träger's schools in its administrative transaction; this records the
// comparison in the Träger transaction before the route answers, so no hint
// reaches the Träger-Büro without an entry.

// HolidayKnownChildStore writes the entry of the comparison.
type HolidayKnownChildStore interface {
	AppendHolidayAudit(ctx context.Context, organizationID, periodID, registrationID int64, action string, details any, actorAccountID int64) error
}

// HolidayKnownChildTrail serves enrollment.HolidayKnownChildTrail.
type HolidayKnownChildTrail struct {
	store   HolidayKnownChildStore
	periods HolidayPeriodStore
}

var _ enrollment.HolidayKnownChildTrail = (*HolidayKnownChildTrail)(nil)

// NewHolidayKnownChildTrail builds the trail of the comparison.
func NewHolidayKnownChildTrail(store HolidayKnownChildStore, periods HolidayPeriodStore) *HolidayKnownChildTrail {
	if store == nil || periods == nil {
		panic("enrollment: the known child trail needs the review store and the period store")
	}
	return &HolidayKnownChildTrail{store: store, periods: periods}
}

// RecordKnownHolidayChildren checks the period against the reviewer's Träger
// and writes one entry for the period: the registration asked for, if one
// was, and the IDs of the children found. It answers the children of known
// that belong to the registration asked for, in their order.
func (t *HolidayKnownChildTrail) RecordKnownHolidayChildren(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, known []enrollment.HolidayKnownChild) ([]enrollment.HolidayKnownChild, error) {
	if err := holidayReviewKey(reviewer, periodID, 1); err != nil {
		return nil, err
	}
	if registrationID < 0 {
		return nil, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	_, found, err := t.periods.FindHolidayPeriod(ctx, reviewer.OrganizationID, periodID, false)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday period: %w", err)
	}
	if !found {
		return nil, enrollment.ErrHolidayPeriodNotFound
	}
	answer := []enrollment.HolidayKnownChild{}
	children := []int64{}
	for _, child := range known {
		if registrationID > 0 && child.RegistrationID != registrationID {
			continue
		}
		answer = append(answer, child)
		children = append(children, child.ChildID)
	}
	details := map[string]any{"children": children}
	if registrationID > 0 {
		details["registration_id"] = registrationID
	}
	if err := t.store.AppendHolidayAudit(ctx, reviewer.OrganizationID, periodID, 0, enrollment.HolidayAuditKnownChildren, details, reviewer.AccountID); err != nil {
		return nil, fmt.Errorf("enrollment: record holiday registration %s: %w", enrollment.HolidayAuditKnownChildren, err)
	}
	return answer, nil
}
