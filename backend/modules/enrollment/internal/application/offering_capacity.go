package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// waitlistEnabledSettingKey names the tenant setting the capacity gate
// resolves in its error text.
const waitlistEnabledSettingKey = "enrollment.waitlist_enabled"

// CapacityOfferings locks the care offerings a capacity check counts, by
// ascending id.
type CapacityOfferings interface {
	ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error)
}

// CapacityPeaks reads an offering's peak occupancy in a date window,
// excluding the claims of the replaced request children.
type CapacityPeaks interface {
	OfferingCapacityPeak(ctx context.Context, offeringID int64, replacedRequestChildIDs []int64, from, until enrollment.Date) (int, error)
}

// OfferingCapacity is the shared capacity gate behind every path that
// (re-)creates active offering claims: a submission and its edit/replace
// variants, and the admin restore of a withdrawn request. It locks the
// selected offerings by ascending id — the same order the offering-change
// approval takes — counts active claims in the phase's remaining capacity
// window, and flags over-capacity children per the phase's overflow mode
// (waitlist/reject/allow). A claim carrying its own validity interval is
// checked only against the peak occupancy inside that interval, so a dated
// switch never competes with capacity pressure it doesn't overlap; claims
// queued earlier in the same run count wherever their intervals overlap the
// checked claim, not only on identical windows.
type OfferingCapacity struct {
	offerings       CapacityOfferings
	peaks           CapacityPeaks
	waitlistEnabled func(context.Context) (bool, error)
}

var _ enrollment.OfferingCapacity = (*OfferingCapacity)(nil)

// NewOfferingCapacity composes the gate. A nil waitlistEnabled fails a
// waitlist-mode phase with a configuration error; nil peaks disable the gate.
func NewOfferingCapacity(offerings CapacityOfferings, peaks CapacityPeaks, waitlistEnabled func(context.Context) (bool, error)) *OfferingCapacity {
	return &OfferingCapacity{offerings: offerings, peaks: peaks, waitlistEnabled: waitlistEnabled}
}

// ApplyCapacityOverflow runs the gate and maps each over-capacity child's
// position to the status it must take instead of submitted.
func (c *OfferingCapacity) ApplyCapacityOverflow(ctx context.Context, check enrollment.CapacityCheck) (map[int]string, error) {
	overrides := make(map[int]string)
	if c.peaks == nil || len(check.Claims) == 0 {
		return overrides, nil
	}
	phase := check.Phase
	// Historical manual approvals and late invites can legitimately target a
	// completed care period. They create no present or future capacity claim,
	// so querying from today through the already-ended phase would be empty.
	if calendar.Date(phase.ServiceEndDate).Before(calendar.TodayDate()) {
		return overrides, nil
	}
	// Serialize this count with offering-change approvals and other
	// submissions. Both paths lock care offerings by ascending id before they
	// inspect capacity and write the booking links.
	if c.offerings == nil {
		return nil, errors.New("care offering repository is not configured")
	}
	openByID, err := c.lockClaimedOfferings(ctx, check.Claims)
	if err != nil {
		return nil, err
	}
	mode, err := c.overflowMode(ctx, phase)
	if err != nil {
		return nil, err
	}
	run := newCapacityRun(ctx, c.peaks, phase, openByID, check)
	return run.apply(mode, overrides)
}

// lockClaimedOfferings locks every claimed offering by ascending id and
// fails closed when one is missing or inactive. The catalog the callers
// validated against was read before entering the write transaction; only
// the locked rows count here, so a concurrent capacity reduction or
// deactivation cannot be missed between selection validation and booking
// creation.
func (c *OfferingCapacity) lockClaimedOfferings(ctx context.Context, claims [][]enrollment.OfferingClaim) (map[int64]*enrollmentModels.CareOffering, error) {
	selectedIDs := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, childClaims := range claims {
		for _, claim := range childClaims {
			if claim.OfferingID > 0 && !seen[claim.OfferingID] {
				seen[claim.OfferingID] = true
				selectedIDs = append(selectedIDs, claim.OfferingID)
			}
		}
	}
	sort.Slice(selectedIDs, func(i, j int) bool { return selectedIDs[i] < selectedIDs[j] })
	lockedOfferings, err := c.offerings.ListByIDsForUpdate(ctx, selectedIDs)
	if err != nil {
		return nil, fmt.Errorf("lock care offering capacity: %w", err)
	}
	openByID := careOfferingsByID(lockedOfferings)
	for _, offeringID := range selectedIDs {
		offering := openByID[offeringID]
		if offering == nil || !offering.IsActive {
			return nil, selection.ErrCareOfferingClosed
		}
	}
	return openByID, nil
}

// overflowMode resolves the phase's overflow mode. A tenant-wide waitlist
// disable wins: overflow remains deterministic and safe, accepting above
// capacity instead of manufacturing a forbidden status.
func (c *OfferingCapacity) overflowMode(ctx context.Context, phase *enrollment.Phase) (string, error) {
	mode := phase.CareOverflowMode
	if mode == "" {
		mode = enrollment.PhaseCareOverflowWaitlist
	}
	if mode != enrollment.PhaseCareOverflowWaitlist {
		return mode, nil
	}
	if c.waitlistEnabled == nil {
		return "", errors.New("enrollment settings resolver is not configured")
	}
	waitlistEnabled, err := c.waitlistEnabled(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", waitlistEnabledSettingKey, err)
	}
	if !waitlistEnabled {
		return enrollment.PhaseCareOverflowAllow, nil
	}
	return mode, nil
}

// careOfferingsByID indexes offerings by id, skipping nil rows.
func careOfferingsByID(offerings []*enrollmentModels.CareOffering) map[int64]*enrollmentModels.CareOffering {
	byID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			byID[offering.ID] = offering
		}
	}
	return byID
}

// dateInterval is a half-open [from, until) window of calendar days.
type dateInterval struct {
	from, until calendar.Date
}

// peakKey caches the DB occupancy peak per offering and window — full-window
// submissions re-check the same window once per child.
type peakKey struct {
	offeringID  int64
	from, until calendar.Date
}

// capacityRun is one pass of the gate over the claims in submission order.
type capacityRun struct {
	ctx       context.Context
	peaks     CapacityPeaks
	openByID  map[int64]*enrollmentModels.CareOffering
	check     enrollment.CapacityCheck
	window    dateInterval
	peakCache map[peakKey]int
	// queuedByOffering holds the clamped intervals of claims from earlier
	// children in this run, so a later claim queues against exactly the
	// queued claims its own window overlaps — identical, partially
	// overlapping, and containing windows alike; disjoint dated intervals on
	// the same offering still count independently.
	queuedByOffering   map[int64][]dateInterval
	preservedRemaining map[int64]int
}

func newCapacityRun(ctx context.Context, peaks CapacityPeaks, phase *enrollment.Phase, openByID map[int64]*enrollmentModels.CareOffering, check enrollment.CapacityCheck) *capacityRun {
	// The phase's remaining capacity window; every claim interval is
	// clamped into it before counting.
	capacityFrom := calendar.TodayDate()
	if calendar.Date(phase.ServiceStartDate).After(capacityFrom) {
		capacityFrom = calendar.Date(phase.ServiceStartDate)
	}
	preservedRemaining := make(map[int64]int, len(check.PreservedClaims))
	for offeringID, n := range check.PreservedClaims {
		preservedRemaining[offeringID] = n
	}
	return &capacityRun{
		ctx: ctx, peaks: peaks, openByID: openByID, check: check,
		window:             dateInterval{from: capacityFrom, until: calendar.Date(phase.ServiceEndDate).AddDays(1)},
		peakCache:          make(map[peakKey]int),
		queuedByOffering:   make(map[int64][]dateInterval),
		preservedRemaining: preservedRemaining,
	}
}

func (r *capacityRun) apply(mode string, overrides map[int]string) (map[int]string, error) {
	for childIdx, childClaims := range r.check.Claims {
		childOver, err := r.childOverCapacity(mode, childClaims)
		if err != nil {
			return nil, err
		}
		if childOver && mode == enrollment.PhaseCareOverflowWaitlist {
			overrides[childIdx] = enrollment.ChildStatusWaitlisted
		}
	}
	return overrides, nil
}

// childOverCapacity checks and queues every claim of one child. Waitlisted
// children keep occupying: the DB peak counts waitlisted claims too, so the
// in-run queue must as well.
func (r *capacityRun) childOverCapacity(mode string, childClaims []enrollment.OfferingClaim) (bool, error) {
	childOver := false
	for _, claim := range childClaims {
		window, ok := r.clampClaim(claim)
		if !ok {
			continue
		}
		offering, found := r.openByID[claim.OfferingID]
		if !found {
			// Should be impossible (the offering selection was validated first).
			return false, fmt.Errorf("submit: offering %d not in open catalog", claim.OfferingID)
		}
		over, err := r.claimOver(offering, claim.OfferingID, window)
		if err != nil {
			return false, err
		}
		if over {
			childOver = true
			if mode == enrollment.PhaseCareOverflowReject {
				return false, fmt.Errorf("%w: offering %d", enrollment.ErrCareOfferingFull, claim.OfferingID)
			}
		}
		r.queuedByOffering[claim.OfferingID] = append(r.queuedByOffering[claim.OfferingID], window)
	}
	return childOver, nil
}

// claimOver decides one claim: unlimited offerings and slots the replaced
// request already held never overflow.
func (r *capacityRun) claimOver(offering *enrollmentModels.CareOffering, offeringID int64, window dateInterval) (bool, error) {
	if offering.Capacity == nil {
		return false, nil
	}
	if r.preservedRemaining[offeringID] > 0 {
		r.preservedRemaining[offeringID]--
		return false, nil
	}
	return r.claimOverCapacity(offeringID, *offering.Capacity, window)
}

// clampClaim reduces a claim's interval to the remaining capacity window; ok
// is false when nothing of the interval lies inside it — such a claim holds
// no future slot.
func (r *capacityRun) clampClaim(claim enrollment.OfferingClaim) (dateInterval, bool) {
	window := r.window
	if claim.ValidFrom != nil && claim.ValidFrom.After(window.from) {
		window.from = *claim.ValidFrom
	}
	if claim.ValidUntil != nil && claim.ValidUntil.Before(window.until) {
		window.until = *claim.ValidUntil
	}
	return window, window.from.Before(window.until)
}

func (r *capacityRun) countPeak(offeringID int64, from, until calendar.Date) (int, error) {
	key := peakKey{offeringID: offeringID, from: from, until: until}
	if cached, ok := r.peakCache[key]; ok {
		return cached, nil
	}
	count, err := r.peaks.OfferingCapacityPeak(r.ctx, offeringID, r.check.ReplacedRequestChildIDs, enrollment.Date(from), enrollment.Date(until))
	if err != nil {
		return 0, fmt.Errorf("submit: count offering %d: %w", offeringID, err)
	}
	r.peakCache[key] = count
	return count, nil
}

// claimOverCapacity reports whether one more claim with this window exceeds
// the offering's capacity on any day of the window. The window is cut at
// every boundary a queued claim contributes; inside each resulting segment
// the queued coverage is constant, so segment DB peak + queued coverage + 1
// is the exact combined occupancy peak.
func (r *capacityRun) claimOverCapacity(offeringID int64, capacity int, window dateInterval) (bool, error) {
	queued := r.queuedByOffering[offeringID]
	boundaries := segmentBoundaries(window, queued)
	for i := 0; i+1 < len(boundaries); i++ {
		segFrom, segUntil := boundaries[i], boundaries[i+1]
		if !segFrom.Before(segUntil) {
			continue // duplicate boundary
		}
		peak, err := r.countPeak(offeringID, segFrom, segUntil)
		if err != nil {
			return false, err
		}
		current := max(peak-r.check.PreservedClaims[offeringID], 0)
		if current+queuedCover(queued, segFrom, segUntil)+1 > capacity {
			return true, nil
		}
	}
	return false, nil
}

// segmentBoundaries returns the window's bounds plus every queued interval
// bound inside it, sorted.
func segmentBoundaries(window dateInterval, queued []dateInterval) []calendar.Date {
	boundaries := []calendar.Date{window.from, window.until}
	for _, qi := range queued {
		if qi.from.After(window.from) && qi.from.Before(window.until) {
			boundaries = append(boundaries, qi.from)
		}
		if qi.until.After(window.from) && qi.until.Before(window.until) {
			boundaries = append(boundaries, qi.until)
		}
	}
	sort.Slice(boundaries, func(i, j int) bool { return boundaries[i].Before(boundaries[j]) })
	return boundaries
}

// queuedCover counts the queued intervals covering a segment. Segments are
// elementary w.r.t. queued boundaries, so a queued interval overlaps the
// segment iff it contains it.
func queuedCover(queued []dateInterval, segFrom, segUntil calendar.Date) int {
	cover := 0
	for _, qi := range queued {
		if !qi.from.After(segFrom) && !qi.until.Before(segUntil) {
			cover++
		}
	}
	return cover
}
