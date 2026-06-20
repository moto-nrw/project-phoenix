package enrollment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

type offeringAdjustmentSnapshot struct {
	OfferingID            string   `json:"offering_id"`
	OfferingName          string   `json:"offering_name"`
	DaysOfWeekMode        string   `json:"days_of_week_mode"`
	SelectedDays          []string `json:"selected_days,omitempty"`
	ManualSelectedDays    []string `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string `json:"automatic_selected_days,omitempty"`
	AvailableDays         []string `json:"available_days,omitempty"`
}

func (s *decisionService) ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*auditModels.EnrollmentOfferingAdjustment, error) {
	if s.offeringAdjustmentRepo == nil {
		return nil, fmt.Errorf("decision: offering adjustment repo not configured")
	}
	if requestID <= 0 || requestChildID <= 0 {
		return nil, fmt.Errorf("%w: request_id and child_id are required", ErrOfferingAdjustmentInvalid)
	}
	child, err := s.requestChildRepo.FindByID(ctx, requestChildID)
	if err != nil || child == nil || child.RequestID != requestID {
		return nil, ErrDecisionChildNotFound
	}
	return s.offeringAdjustmentRepo.ListByRequestChildID(ctx, requestChildID)
}

func (s *decisionService) UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*enrollmentModels.RequestChild, error) {
	if input.RequestID <= 0 || input.ChildID <= 0 {
		return nil, fmt.Errorf("%w: request_id and child_id are required", ErrOfferingAdjustmentInvalid)
	}
	if input.ActorAccountID <= 0 {
		return nil, fmt.Errorf("%w: actor account id is required", ErrOfferingAdjustmentInvalid)
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: reason is required", ErrOfferingAdjustmentInvalid)
	}
	if s.requestRepo == nil || s.requestChildRepo == nil || s.requestChildOfferingRepo == nil ||
		s.careOfferingRepo == nil || s.phaseRepo == nil || s.offeringAdjustmentRepo == nil {
		return nil, fmt.Errorf("decision: offering adjustment dependencies are not configured")
	}

	req, err := s.requestRepo.FindByID(ctx, input.RequestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	child, err := s.requestChildRepo.FindByID(ctx, input.ChildID)
	if err != nil || child == nil || child.RequestID != req.ID {
		return nil, ErrDecisionChildNotFound
	}
	if child.Status != enrollmentModels.ChildStatusApproved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
		return nil, fmt.Errorf("%w: only approved children with a linked student can be adjusted", ErrOfferingAdjustmentInvalid)
	}
	phase, err := s.phaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil || phase == nil {
		return nil, fmt.Errorf("decision: load adjustment phase: %w", err)
	}

	offerings, err := s.careOfferingRepo.ListByPhase(ctx, req.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("decision: list phase offerings for adjustment: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		offeringByID[offering.ID] = offering
	}

	beforeLinks, err := s.requestChildOfferingRepo.ListByRequestChildID(ctx, child.ID)
	if err != nil {
		return nil, fmt.Errorf("decision: list current child offerings: %w", err)
	}
	beforeJSON, beforeSnapshotErr := adjustmentSnapshotJSON(beforeLinks, offeringByID)
	if beforeSnapshotErr != nil {
		return nil, beforeSnapshotErr
	}

	submitChild := SubmitChild{
		FirstName:        child.FirstName,
		LastName:         child.LastName,
		DateOfBirth:      child.DateOfBirth,
		TargetGradeLevel: child.TargetGradeLevel,
		CustomData:       child.CustomData,
		OfferingIDs:      make([]int64, 0, len(input.Offerings)),
		OfferingDays:     make([]SubmitOfferingDays, 0, len(input.Offerings)),
	}
	seen := make(map[int64]bool, len(input.Offerings))
	for _, row := range input.Offerings {
		if row.OfferingID <= 0 {
			return nil, fmt.Errorf("%w: offering_id is required", ErrOfferingAdjustmentInvalid)
		}
		if seen[row.OfferingID] {
			continue
		}
		seen[row.OfferingID] = true
		submitChild.OfferingIDs = append(submitChild.OfferingIDs, row.OfferingID)
		if len(row.SelectedDays) > 0 {
			submitChild.OfferingDays = append(submitChild.OfferingDays, SubmitOfferingDays{
				OfferingID:   row.OfferingID,
				SelectedDays: copyDays(row.SelectedDays),
			})
		}
	}
	sort.SliceStable(submitChild.OfferingIDs, func(i, j int) bool {
		left := offeringByID[submitChild.OfferingIDs[i]]
		right := offeringByID[submitChild.OfferingIDs[j]]
		if left == nil || right == nil || left.SortOrder == right.SortOrder {
			return submitChild.OfferingIDs[i] < submitChild.OfferingIDs[j]
		}
		return left.SortOrder < right.SortOrder
	})
	children := []SubmitChild{submitChild}
	materialized, err := materializeAndValidateChildrenOfferingSelections(children, offeringByID, phase.CareOfferingSelectionMode)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOfferingAdjustmentInvalid, err)
	}
	selections := materialized[0]

	replacement := make([]*enrollmentModels.RequestChildOffering, 0, len(selections))
	for _, selection := range selections {
		replacement = append(replacement, &enrollmentModels.RequestChildOffering{
			RequestChildID:        child.ID,
			CareOfferingID:        selection.OfferingID,
			SelectedDays:          selection.SelectedDays,
			ManualSelectedDays:    selection.ManualSelectedDays,
			AutomaticSelectedDays: selection.AutomaticSelectedDays,
		})
	}
	afterJSON, afterSnapshotErr := adjustmentSnapshotJSON(replacement, offeringByID)
	if afterSnapshotErr != nil {
		return nil, afterSnapshotErr
	}

	if err := s.requestChildOfferingRepo.ReplaceForRequestChild(ctx, child.ID, replacement); err != nil {
		return nil, fmt.Errorf("decision: replace child offerings: %w", err)
	}
	if err := s.rematerializeAdjustedEnrollments(ctx, child.ID, *child.CreatedStudentID, phase, beforeLinks, replacement, offeringByID); err != nil {
		return nil, err
	}
	actorName, actorEmail := s.actorSnapshot(ctx, input.ActorAccountID)
	entry := &auditModels.EnrollmentOfferingAdjustment{
		RequestID:          req.ID,
		RequestChildID:     child.ID,
		StudentID:          *child.CreatedStudentID,
		ActorAccountID:     input.ActorAccountID,
		ActorRole:          strings.TrimSpace(input.ActorRole),
		ActorNameSnapshot:  actorName,
		ActorEmailSnapshot: actorEmail,
		Reason:             reason,
		Before:             beforeJSON,
		After:              afterJSON,
	}
	if entry.ActorRole == "" {
		entry.ActorRole = "admin"
	}
	if err := s.offeringAdjustmentRepo.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("decision: create offering adjustment audit: %w", err)
	}
	return s.requestChildRepo.FindByID(ctx, child.ID)
}

func (s *decisionService) actorSnapshot(ctx context.Context, accountID int64) (*string, *string) {
	var name *string
	if s.personRepo != nil {
		if person, err := s.personRepo.FindByAccountID(ctx, accountID); err == nil && person != nil {
			fullName := strings.TrimSpace(person.GetFullName())
			if fullName != "" {
				name = &fullName
			}
		}
	}
	var email *string
	if s.accountRepo != nil {
		if account, err := s.accountRepo.FindByID(ctx, accountID); err == nil && account != nil && strings.TrimSpace(account.Email) != "" {
			value := account.Email
			email = &value
		}
	}
	return name, email
}

func adjustmentSnapshotJSON(links []*enrollmentModels.RequestChildOffering, offeringByID map[int64]*enrollmentModels.CareOffering) ([]byte, error) {
	rows := make([]offeringAdjustmentSnapshot, 0, len(links))
	for _, link := range links {
		if link == nil {
			continue
		}
		row := offeringAdjustmentSnapshot{
			OfferingID:            strconv.FormatInt(link.CareOfferingID, 10),
			SelectedDays:          copyDays(link.SelectedDays),
			ManualSelectedDays:    copyDays(link.ManualSelectedDays),
			AutomaticSelectedDays: copyDays(link.AutomaticSelectedDays),
		}
		if offering := offeringByID[link.CareOfferingID]; offering != nil {
			row.OfferingName = offering.Name
			row.DaysOfWeekMode = offering.DaysOfWeekMode
			row.AvailableDays = copyDays(offering.AvailableDays)
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return lessNumericString(rows[i].OfferingID, rows[j].OfferingID)
	})
	raw, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("decision: marshal offering adjustment snapshot: %w", err)
	}
	return raw, nil
}

func lessNumericString(left, right string) bool {
	leftID, leftErr := strconv.ParseInt(left, 10, 64)
	rightID, rightErr := strconv.ParseInt(right, 10, 64)
	if leftErr == nil && rightErr == nil {
		return leftID < rightID
	}
	return left < right
}

func (s *decisionService) rematerializeAdjustedEnrollments(
	ctx context.Context,
	requestChildID, studentID int64,
	phase *enrollmentModels.Phase,
	beforeLinks []*enrollmentModels.RequestChildOffering,
	afterLinks []*enrollmentModels.RequestChildOffering,
	offeringByID map[int64]*enrollmentModels.CareOffering,
) error {
	if s.studentEnrollmentRepo == nil {
		return nil
	}
	groupIDs := activityGroupIDsForOfferingLinks(beforeLinks, afterLinks, offeringByID)
	validUntil := phase.ServiceEndDate
	if _, err := s.studentEnrollmentRepo.DeleteByStudentGroupsAndWindow(ctx, studentID, groupIDs, phase.ServiceStartDate, &validUntil); err != nil {
		return fmt.Errorf("decision: delete previous adjusted enrollments: %w", err)
	}
	return s.materializeEnrollments(ctx, requestChildID, studentID, phase)
}

func activityGroupIDsForOfferingLinks(
	beforeLinks []*enrollmentModels.RequestChildOffering,
	afterLinks []*enrollmentModels.RequestChildOffering,
	offeringByID map[int64]*enrollmentModels.CareOffering,
) []int64 {
	seen := make(map[int64]bool)
	add := func(links []*enrollmentModels.RequestChildOffering) {
		for _, link := range links {
			if link == nil {
				continue
			}
			offering := offeringByID[link.CareOfferingID]
			if offering == nil || offering.ActivityGroupID == nil || *offering.ActivityGroupID <= 0 {
				continue
			}
			seen[*offering.ActivityGroupID] = true
		}
	}
	add(beforeLinks)
	add(afterLinks)
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
