package enrollment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/users"
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
	if s.OfferingAdjustmentRepo == nil {
		return nil, fmt.Errorf("decision: offering adjustment repo not configured")
	}
	if requestID <= 0 || requestChildID <= 0 {
		return nil, fmt.Errorf("%w: request_id and child_id are required", ErrOfferingAdjustmentInvalid)
	}
	child, err := s.RequestChildRepo.FindByID(ctx, requestChildID)
	if err != nil || child == nil || child.RequestID != requestID {
		return nil, ErrDecisionChildNotFound
	}
	return s.OfferingAdjustmentRepo.ListByRequestChildID(ctx, requestChildID)
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
	if s.RequestRepo == nil || s.RequestChildRepo == nil || s.RequestChildOfferingRepo == nil ||
		s.CareOfferingRepo == nil || s.PhaseRepo == nil || s.OfferingAdjustmentRepo == nil {
		return nil, fmt.Errorf("decision: offering adjustment dependencies are not configured")
	}

	req, err := s.RequestRepo.FindByID(ctx, input.RequestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	child, err := s.RequestChildRepo.FindByID(ctx, input.ChildID)
	if err != nil || child == nil || child.RequestID != req.ID {
		return nil, ErrDecisionChildNotFound
	}
	if child.Status != enrollmentModels.ChildStatusApproved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
		return nil, fmt.Errorf("%w: only approved children with a linked student can be adjusted", ErrOfferingAdjustmentInvalid)
	}
	phase, err := s.PhaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil || phase == nil {
		return nil, fmt.Errorf("decision: load adjustment phase: %w", err)
	}

	offerings, err := s.CareOfferingRepo.ListByPhase(ctx, req.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("decision: list phase offerings for adjustment: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		offeringByID[offering.ID] = offering
	}
	activeOfferings, err := s.CareOfferingRepo.ListActiveByPhase(ctx, req.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("decision: list active phase offerings for adjustment: %w", err)
	}
	activeOfferingByID := make(map[int64]*enrollmentModels.CareOffering, len(activeOfferings))
	for _, offering := range activeOfferings {
		activeOfferingByID[offering.ID] = offering
		offeringByID[offering.ID] = offering
	}

	beforeLinks, err := s.RequestChildOfferingRepo.ListByRequestChildID(ctx, child.ID)
	if err != nil {
		return nil, fmt.Errorf("decision: list current child offerings: %w", err)
	}
	beforeOfferingIDs := offeringIDsFromLinks(beforeLinks)
	beforeOfferingByID := map[int64]*enrollmentModels.CareOffering{}
	if len(beforeOfferingIDs) > 0 {
		beforeOfferings, listErr := s.CareOfferingRepo.ListByIDs(ctx, beforeOfferingIDs)
		if listErr != nil {
			return nil, fmt.Errorf("decision: list existing child offerings for adjustment: %w", listErr)
		}
		for _, offering := range beforeOfferings {
			beforeOfferingByID[offering.ID] = offering
			offeringByID[offering.ID] = offering
		}
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
		if activeOfferingByID[row.OfferingID] == nil && beforeOfferingByID[row.OfferingID] == nil {
			return nil, fmt.Errorf("%w: care offering %d is not available for this child adjustment", ErrCareOfferingMissing, row.OfferingID)
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
	allowedOfferingByID := make(map[int64]*enrollmentModels.CareOffering, len(activeOfferingByID)+len(beforeOfferingByID))
	for id, offering := range activeOfferingByID {
		allowedOfferingByID[id] = offering
	}
	for id, offering := range beforeOfferingByID {
		allowedOfferingByID[id] = offering
	}
	children := []SubmitChild{submitChild}
	materialized, err := materializeAndValidateChildrenOfferingSelections(children, allowedOfferingByID, phase.CareOfferingSelectionMode)
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

	if err := s.RequestChildOfferingRepo.ReplaceForRequestChild(ctx, child.ID, replacement); err != nil {
		return nil, fmt.Errorf("decision: replace child offerings: %w", err)
	}
	if err := s.rematerializeAdjustedEnrollments(ctx, child.ID, *child.CreatedStudentID, beforeLinks, phase); err != nil {
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
	if err := s.OfferingAdjustmentRepo.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("decision: create offering adjustment audit: %w", err)
	}
	return s.RequestChildRepo.FindByID(ctx, child.ID)
}

func (s *decisionService) SyncApprovedChildData(ctx context.Context, input SyncApprovedChildDataInput) (*enrollmentModels.RequestChild, error) {
	if input.RequestID <= 0 || input.ChildID <= 0 {
		return nil, fmt.Errorf("%w: request_id and child_id are required", ErrOfferingAdjustmentInvalid)
	}
	if s.RequestRepo == nil || s.RequestChildRepo == nil || s.StudentRepo == nil || s.PersonRepo == nil {
		return nil, fmt.Errorf("decision: approved child sync dependencies are not configured")
	}

	req, err := s.RequestRepo.FindByID(ctx, input.RequestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	child, err := s.RequestChildRepo.FindByID(ctx, input.ChildID)
	if err != nil || child == nil || child.RequestID != req.ID {
		return nil, ErrDecisionChildNotFound
	}
	if child.Status != enrollmentModels.ChildStatusApproved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
		return nil, fmt.Errorf("%w: only approved children with a linked student can be synced", ErrOfferingAdjustmentInvalid)
	}

	student, err := s.StudentRepo.FindByIDForUpdate(ctx, *child.CreatedStudentID)
	if err != nil || student == nil {
		return nil, ErrDecisionStudentNotFound
	}
	person, err := s.PersonRepo.FindByID(ctx, student.PersonID)
	if err != nil || person == nil {
		return nil, fmt.Errorf("decision: load approved child person: %w", err)
	}

	person.FirstName = child.FirstName
	person.LastName = child.LastName
	dob := child.DateOfBirth
	person.Birthday = &dob
	if err := s.PersonRepo.Update(ctx, person); err != nil {
		return nil, fmt.Errorf("decision: sync approved child person: %w", err)
	}

	// Re-derive the school_class exactly like the rollover approval path:
	// the concrete class carried by the edit wins; otherwise a bare grade
	// placeholder tracks a grade change ("1" -> "2"), a concrete class whose
	// grade still matches is kept ("3b" at grade 3), and a concrete class
	// stranded on a now-stale grade ("2a" while the edit bumps to grade 3)
	// falls back to the new bare grade rather than leaving the student in a
	// mismatched class. Issue #1833.
	student.SchoolClass = s.resolveRolloverSchoolClass(child, student.SchoolClass)
	guardianEmail := strings.TrimSpace(strings.ToLower(req.GuardianEmail))
	if guardianEmail != "" {
		student.GuardianEmail = &guardianEmail
	}
	student.GuardianPhone = req.GuardianPhone
	if err := s.StudentRepo.Update(ctx, student); err != nil {
		return nil, fmt.Errorf("decision: sync approved child student: %w", err)
	}

	var guardian *users.GuardianProfile
	if input.ReplaceTargetedData && s.GuardianProfileRepo != nil {
		guardian, err = s.reconcilePrimaryGuardianLink(ctx, req, student.ID)
		if err != nil {
			return nil, err
		}
	}

	keepGuardianProfileIDs := map[int64]bool{}
	if input.ReplaceTargetedData {
		var relinkErr error
		keepGuardianProfileIDs, relinkErr = s.reconcileApprovedChildGuardians(ctx, req, student.ID, input.PreviousRequestGuardians)
		if relinkErr != nil {
			return nil, relinkErr
		}
	}

	if s.GuardianProfileRepo != nil {
		if guardian == nil {
			resolved, _, gerr := s.resolveGuardianProfile(ctx, req)
			if gerr == nil {
				guardian = resolved
			}
		}
		if guardian != nil {
			if terr := s.applyTargetedFields(ctx, req, child, student, guardian, input.ActorAccountID, targetedFieldSyncOptions{
				Replace:                input.ReplaceTargetedData,
				PreviousSnapshot:       input.PreviousSnapshot,
				KeepGuardianProfileIDs: keepGuardianProfileIDs,
			}); terr != nil {
				s.Logger.Warn("decision: approved child targeted-field sync had errors",
					slog.Int64("request_id", req.ID),
					slog.Int64("child_id", child.ID),
					slog.String("error", terr.Error()),
				)
				if input.ReplaceTargetedData {
					return nil, fmt.Errorf("decision: approved child targeted-field replacement sync: %w", terr)
				}
			}
		}
	}
	if input.ReplaceTargetedData {
		if _, relinkErr := s.reconcileApprovedChildGuardians(ctx, req, student.ID, input.PreviousRequestGuardians); relinkErr != nil {
			return nil, relinkErr
		}
	}

	return s.RequestChildRepo.FindByID(ctx, child.ID)
}

func (s *decisionService) actorSnapshot(ctx context.Context, accountID int64) (*string, *string) {
	var name *string
	if s.PersonRepo != nil {
		if person, err := s.PersonRepo.FindByAccountID(ctx, accountID); err == nil && person != nil {
			fullName := strings.TrimSpace(person.GetFullName())
			if fullName != "" {
				name = &fullName
			}
		}
	}
	var email *string
	if s.AccountRepo != nil {
		if account, err := s.AccountRepo.FindByID(ctx, accountID); err == nil && account != nil && strings.TrimSpace(account.Email) != "" {
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

func offeringIDsFromLinks(links []*enrollmentModels.RequestChildOffering) []int64 {
	ids := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))
	for _, link := range links {
		if link == nil || link.CareOfferingID <= 0 || seen[link.CareOfferingID] {
			continue
		}
		seen[link.CareOfferingID] = true
		ids = append(ids, link.CareOfferingID)
	}
	return ids
}

func (s *decisionService) rematerializeAdjustedEnrollments(
	ctx context.Context,
	requestChildID, studentID int64,
	beforeLinks []*enrollmentModels.RequestChildOffering,
	phase *enrollmentModels.Phase,
) error {
	if s.StudentEnrollmentRepo == nil {
		return nil
	}
	if len(beforeLinks) > 0 {
		if err := s.backfillLegacyAdjustedEnrollments(ctx, requestChildID, studentID, beforeLinks); err != nil {
			return err
		}
	}
	if _, err := s.StudentEnrollmentRepo.DeleteByEnrollmentRequestChild(ctx, studentID, requestChildID); err != nil {
		return fmt.Errorf("decision: delete sourced adjusted enrollments: %w", err)
	}
	return s.materializeEnrollments(ctx, requestChildID, studentID, phase)
}

func (s *decisionService) backfillLegacyAdjustedEnrollments(
	ctx context.Context,
	requestChildID, studentID int64,
	beforeLinks []*enrollmentModels.RequestChildOffering,
) error {
	if s.CareOfferingRepo == nil {
		return nil
	}
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, offeringIDsFromLinks(beforeLinks))
	if err != nil {
		return fmt.Errorf("decision: list existing child offerings for legacy enrollment cleanup: %w", err)
	}
	groupIDs := make([]int64, 0, len(offerings))
	seen := make(map[int64]bool, len(offerings))
	for _, offering := range offerings {
		if offering == nil || offering.ActivityGroupID == nil || *offering.ActivityGroupID <= 0 || seen[*offering.ActivityGroupID] {
			continue
		}
		seen[*offering.ActivityGroupID] = true
		groupIDs = append(groupIDs, *offering.ActivityGroupID)
	}
	if len(groupIDs) == 0 {
		return nil
	}
	if _, err := s.StudentEnrollmentRepo.BackfillEnrollmentRequestChildSource(ctx, studentID, requestChildID, groupIDs); err != nil {
		return fmt.Errorf("decision: backfill legacy adjusted enrollments: %w", err)
	}
	return nil
}
