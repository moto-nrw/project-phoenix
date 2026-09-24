package application

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// offeringDecisionRecencyDays bounds how long a decided request keeps being
// reported: long enough that a guardian sees the outcome on the next visit,
// short enough that the card does not turn into a history list.
const offeringDecisionRecencyDays = 14

// GetForStudent returns the child's open request (nil when none) plus its
// live diff against the current booking, and the last decision.
func (s *OfferingChanges) GetForStudent(ctx context.Context, studentID int64) (*careplan.OfferingChangeView, error) {
	pending, err := s.deps.Rows.PendingForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: get pending: %w", err)
	}
	decision, err := s.lastDecisionForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if pending == nil && decision == nil {
		return nil, nil
	}
	view := &careplan.OfferingChangeView{Request: pending, LastDecision: decision}
	if pending == nil {
		return view, nil
	}
	diff, diffErr := s.diffForRequest(ctx, *pending)
	if diffErr != nil {
		// A diff that cannot be built must not hide the request itself: the
		// guardian still needs to see that one is open.
		s.logger().Warn("offering change: build diff failed",
			slog.Int64("request_id", pending.ID),
			slog.String("error", diffErr.Error()),
		)
		return view, nil
	}
	view.Diff = diff
	return view, nil
}

// lastDecisionForStudent returns the newest approved or rejected request
// decided within the recency window. Withdrawals are skipped: the guardian
// withdrew it and does not need to be told.
func (s *OfferingChanges) lastDecisionForStudent(ctx context.Context, studentID int64) (*careplan.OfferingChangeDecision, error) {
	rows, err := s.deps.Rows.ListByStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list requests: %w", err)
	}
	cutoff := time.Now().AddDate(0, 0, -offeringDecisionRecencyDays)
	for _, row := range rows {
		if row.ReviewedAt == nil || (row.Status != careplan.OfferingChangeApproved && row.Status != careplan.OfferingChangeRejected) {
			continue
		}
		recent, err := s.decisionStillReported(ctx, row, cutoff)
		if err != nil {
			return nil, err
		}
		if !recent {
			// Rows are newest first, so everything below is older too.
			return nil, nil
		}
		return s.decisionView(ctx, row)
	}
	return nil, nil
}

// decisionStillReported keeps a decision visible inside the recency window,
// while an approval has not taken effect yet, and while an approved complete
// withdrawal is still the child's state.
func (s *OfferingChanges) decisionStillReported(ctx context.Context, row careplan.OfferingChangeRequest, cutoff time.Time) (bool, error) {
	approved := row.Status == careplan.OfferingChangeApproved
	futureApproval := approved && s.todayDate().Before(offeringChangeEffectiveFrom(row))
	keepWithdrawalStatus := false
	if approved && row.ApprovedCompleteWithdrawal {
		var err error
		keepWithdrawalStatus, err = s.keepCompleteWithdrawalStatus(ctx, row.StudentID)
		if err != nil {
			return false, err
		}
	}
	return !row.ReviewedAt.Before(cutoff) || futureApproval || keepWithdrawalStatus, nil
}

func (s *OfferingChanges) decisionView(ctx context.Context, row careplan.OfferingChangeRequest) (*careplan.OfferingChangeDecision, error) {
	snapshot, err := decisionSnapshot(row)
	if err != nil {
		return nil, fmt.Errorf("offering change: list requests: %w", err)
	}
	decision := &careplan.OfferingChangeDecision{
		ID: row.ID, SubmittedBy: row.SubmittedBy, Status: row.Status,
		CompleteWithdrawal: row.ApprovedCompleteWithdrawal, DecidedAt: *row.ReviewedAt,
		EffectiveFrom: offeringChangeEffectiveFrom(row),
	}
	if row.DecisionReason != nil {
		decision.Reason = *row.DecisionReason
	}
	if snapshot != nil {
		decision.AppliedDiff = diffEntriesFromSnapshot(snapshot.Diff)
		decision.OverriddenOfferings = overridesFromSnapshot(snapshot.OverriddenOfferings)
	}
	requested, reqErr := s.requestedItems(ctx, row)
	if reqErr != nil {
		// The outcome matters more than the recap: report the decision even
		// when the payload cannot be resolved to names.
		s.logger().Warn("offering change: resolve requested offerings failed",
			slog.Int64("request_id", row.ID),
			slog.String("error", reqErr.Error()),
		)
		return decision, nil
	}
	decision.Requested = requested
	return decision, nil
}

func (s *OfferingChanges) keepCompleteWithdrawalStatus(ctx context.Context, studentID int64) (bool, error) {
	student, err := s.deps.Students.FindStudent(ctx, studentID)
	if err != nil {
		return false, fmt.Errorf("offering change: load student withdrawal state: %w", err)
	}
	if careEnded(student, s.todayDate()) {
		return true, nil
	}
	pending, err := s.deps.Students.HasPendingWithdrawalCompletion(ctx, studentID)
	if err != nil {
		return false, fmt.Errorf("offering change: load pending withdrawal completion: %w", err)
	}
	return pending, nil
}

// requestedItems reads the stored payload back into named offerings, in
// catalog order. An offering deleted from the catalog since is listed by id
// rather than dropped: the family asked for it.
func (s *OfferingChanges) requestedItems(ctx context.Context, row careplan.OfferingChangeRequest) ([]careplan.OfferingChangeRequestedItem, error) {
	selections, err := requestSelections(row)
	if err != nil {
		return nil, err
	}
	if len(selections) == 0 {
		return []careplan.OfferingChangeRequestedItem{}, nil
	}
	ids := make([]int64, 0, len(selections))
	for _, selected := range selections {
		ids = append(ids, selected.OfferingID)
	}
	offerings, err := s.offeringsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list requested offerings: %w", err)
	}
	nameByID := make(map[int64]string, len(offerings))
	sortByID := make(map[int64]int, len(offerings))
	for _, offering := range offerings {
		nameByID[offering.ID] = offering.Name
		sortByID[offering.ID] = offering.SortOrder
	}
	items := make([]careplan.OfferingChangeRequestedItem, 0, len(selections))
	for _, selected := range selections {
		name := nameByID[selected.OfferingID]
		if name == "" {
			name = fmt.Sprintf("Angebot %d", selected.OfferingID)
		}
		items = append(items, careplan.OfferingChangeRequestedItem{
			OfferingID: selected.OfferingID, Name: name, Days: append([]string(nil), selected.SelectedDays...),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, right := sortByID[items[i].OfferingID], sortByID[items[j].OfferingID]
		if left == right {
			return items[i].Name < items[j].Name
		}
		return left < right
	})
	return items, nil
}
