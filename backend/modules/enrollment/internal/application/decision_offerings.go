package application

import (
	"context"
	"fmt"
	"log/slog"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ListChildOfferings returns the offerings each child in this request
// picked. Per-child rows are keyed by request_child_id; offerings
// missing a parent_choice day picker land with SelectedDays == nil.
// Used by the admin detail endpoint to render the Betreuungsangebote
// next to each child for the decision UI.
//
// Rows that have already expired are dropped, but rows that take
// effect later are kept and flagged StartsLater (#2185). The parent
// portal has always shown those; listing only the currently effective
// selection left staff unable to see an approved change before its
// effective date and unable to answer the family's questions about it.
func (d *Decisions) ListChildOfferings(ctx context.Context, requestID int64) (map[int64]enrollment.ChildOfferingSet, error) {
	if requestID <= 0 {
		return nil, fmt.Errorf("decision: request_id required")
	}
	request, err := decodedRequestByID(ctx, d.deps.Requests, requestID, false)
	if err != nil || request == nil {
		return nil, fmt.Errorf("decision: load request for offerings: %w", err)
	}
	phase, err := d.deps.Phases.Phase(ctx, request.PhaseID)
	if err != nil || phase == nil {
		return nil, fmt.Errorf("decision: load phase for offerings: %w", err)
	}
	children, err := decodedChildrenOfRequest(ctx, d.deps.Children, requestID, false)
	if err != nil {
		return nil, fmt.Errorf("decision: list children for offerings: %w", err)
	}
	childIDs := make([]int64, 0, len(children))
	for _, child := range children {
		childIDs = append(childIDs, child.ID)
	}
	today := d.todayDate()
	// The date the WRITE path treats as "now".
	selectionDate := offeringSelectionDateOn(phase, today)
	links, err := enrollment.OfferingHistoryRecordsForChildren(ctx, d.deps.Children, childIDs)
	if err != nil {
		return nil, fmt.Errorf("decision: list child offering history: %w", err)
	}
	currentLinks, err := enrollment.OfferingSelectionRecordsForChildrenAt(ctx, d.deps.Children, childIDs, selectionDate)
	if err != nil {
		return nil, fmt.Errorf("decision: list current child offerings: %w", err)
	}
	sets := childOfferingSets{
		onDate:        careplan.BookingViewDate(today, calendar.Date(phase.ServiceEndDate)),
		selectionDate: selectionDate,
		currentIDs:    offeringLinkIDSet(currentLinks),
		offeringByID:  d.careOfferingsForLinks(ctx, links),
	}
	linksByChild := offeringLinksByChild(links)
	out := make(map[int64]enrollment.ChildOfferingSet, len(children))
	for _, child := range children {
		out[child.ID] = sets.build(linksByChild[child.ID])
	}
	return out, nil
}

// childOfferingSets splits a child's offering history into its current and
// upcoming bookings.
type childOfferingSets struct {
	onDate        calendar.Date
	selectionDate calendar.Date
	currentIDs    map[int64]bool
	offeringByID  map[int64]*enrollmentModels.CareOffering
}

func (s childOfferingSets) build(links []*enrollment.RequestChildOfferingRecord) enrollment.ChildOfferingSet {
	set := enrollment.ChildOfferingSet{}
	for _, link := range links {
		if link == nil || (link.ValidUntil != nil && !link.ValidUntil.After(s.onDate)) {
			continue
		}
		row := childOfferingRow(link, s.offeringByID[link.CareOfferingID], s.onDate)
		switch {
		case s.currentIDs[link.ID]:
			set.Current = append(set.Current, row)
		case link.ValidFrom != nil && link.ValidFrom.After(s.selectionDate):
			set.Upcoming = append(set.Upcoming, row)
		default:
			// Neither on file nor ahead: a superseded interval that only
			// describes the past. Showing it would read as a booking.
		}
	}
	return set
}

// offeringSelectionDateOn is the day within the phase the offering write path
// treats as now: the phase start before it begins, its end after it ended.
func offeringSelectionDateOn(phase *enrollment.Phase, today calendar.Date) calendar.Date {
	if phase == nil {
		return today
	}
	if today.Before(calendar.Date(phase.ServiceStartDate)) {
		return calendar.Date(phase.ServiceStartDate)
	}
	if today.After(calendar.Date(phase.ServiceEndDate)) {
		return calendar.Date(phase.ServiceEndDate)
	}
	return today
}

func offeringLinksByChild(links []*enrollment.RequestChildOfferingRecord) map[int64][]*enrollment.RequestChildOfferingRecord {
	result := make(map[int64][]*enrollment.RequestChildOfferingRecord)
	for _, link := range links {
		if link != nil {
			result[link.RequestChildID] = append(result[link.RequestChildID], link)
		}
	}
	return result
}

func offeringLinkIDSet(links []*enrollment.RequestChildOfferingRecord) map[int64]bool {
	result := make(map[int64]bool, len(links))
	for _, link := range links {
		if link != nil {
			result[link.ID] = true
		}
	}
	return result
}

// careOfferingsForLinks resolves the catalog entries behind a child's
// offering links in one query. Returns an empty map when no catalog is
// wired, leaving rows with their link-side fields only.
//
// A failed catalog lookup degrades instead of failing the call, and
// deliberately so: the caller's error is best-effort at the handler,
// which would render the child with NO Betreuungsangebote at all. An
// admin then opens the correction editor on an empty selection, and an
// untouched save replaces the family's real bookings with nothing. A
// row with a missing name is recoverable; a silently emptied booking
// list is not.
func (d *Decisions) careOfferingsForLinks(ctx context.Context, links []*enrollment.RequestChildOfferingRecord) map[int64]*enrollmentModels.CareOffering {
	byID := make(map[int64]*enrollmentModels.CareOffering, len(links))
	if d.deps.Offerings == nil || len(links) == 0 {
		return byID
	}
	ids := uniqueCareOfferingIDs(links)
	offerings, err := d.deps.Offerings.ListByIDs(ctx, ids)
	if err != nil {
		d.logger().Warn("decision: care offering catalog lookup failed, rendering bookings without catalog data",
			slog.String("error", err.Error()),
			slog.Int("offering_count", len(ids)),
		)
		return byID
	}
	for _, offering := range offerings {
		if offering != nil {
			byID[offering.ID] = offering
		}
	}
	return byID
}

// childOfferingRow merges the link (what the child booked, from when)
// with the catalog entry (what the offering is). A missing catalog
// entry — deleted offering — still yields a row so the admin sees the
// booking exists, unlike the parent view which skips it.
func childOfferingRow(
	link *enrollment.RequestChildOfferingRecord,
	offering *enrollmentModels.CareOffering,
	onDate calendar.Date,
) enrollment.ChildOfferingRow {
	row := enrollment.ChildOfferingRow{
		OfferingID:            link.CareOfferingID,
		SelectedDays:          link.SelectedDays,
		ManualSelectedDays:    link.ManualSelectedDays,
		AutomaticSelectedDays: link.AutomaticSelectedDays,
		ValidFrom:             link.ValidFrom,
		ValidUntil:            link.ValidUntil,
		StartsLater:           link.ValidFrom != nil && link.ValidFrom.After(onDate),
	}
	if offering != nil {
		row.OfferingName = offering.Name
		row.DaysOfWeekMode = offering.DaysOfWeekMode
		row.AvailableDays = offering.AvailableDays
		row.IncludesLunch = offering.IncludesLunch
		row.IncludesHolidayCare = offering.IncludesHolidayCare
		row.PriceCents = offering.PriceCents
	}
	return row
}
