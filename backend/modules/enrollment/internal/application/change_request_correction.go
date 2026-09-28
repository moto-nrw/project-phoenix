package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// correctApprovedChildData fixes the enrollment record of an approved child
// and applies the enrollment-to-student projection. Offerings, consents,
// guardians and all other submission data stay untouched; an audit entry
// records the correction as an approved staff change request.
func (s *ChangeRequests) correctApprovedChildData(ctx context.Context, input enrollment.CorrectApprovedChildDataInput) (*changeRequestCase, error) {
	firstName, lastName, reason, err := validateAdminCorrectionInput(input)
	if err != nil {
		return nil, err
	}
	req, err := decodedRequestByID(ctx, s.deps.Requests, input.RequestID, true)
	if err != nil && !s.deps.Runtime.NotFound(err) {
		return nil, err
	}
	if err != nil || req == nil {
		return nil, enrollment.ErrRequestNotFound
	}
	children, err := decodedChildrenOfRequest(ctx, s.deps.Children, req.ID, true)
	if err != nil {
		return nil, fmt.Errorf("admin data correction: lock children: %w", err)
	}
	child := requestChildByID(children, input.ChildID)
	if child == nil {
		return nil, enrollment.ErrChangeRequestNotFound
	}
	if err := s.validateAdminCorrectionChild(child); err != nil {
		return nil, err
	}
	targetGradeLevel, targetSchoolClass, err := s.prepareAdminCorrectionTargets(ctx, req, child, input)
	if err != nil {
		return nil, err
	}
	baseSnapshot, err := s.currentSnapshot(ctx, req, children)
	if err != nil {
		return nil, err
	}
	if err := s.rejectOpenChangeRequestsForAdminCorrection(ctx, req, reason, input.ActorAccountID); err != nil {
		return nil, err
	}
	child.FirstName, child.LastName, child.DateOfBirth = firstName, lastName, input.DateOfBirth
	child.TargetGradeLevel, child.TargetSchoolClass = targetGradeLevel, targetSchoolClass
	if err := updateDecodedChild(ctx, s.deps.Children, child); err != nil {
		return nil, fmt.Errorf("admin data correction: update enrollment child: %w", err)
	}
	if _, err := s.deps.Decisions.SyncApprovedChildData(ctx, enrollment.ApprovedChildSync{
		RequestID: req.ID, ChildID: child.ID, ActorAccountID: input.ActorAccountID,
		PreviousSnapshot: []byte("null"),
	}); err != nil {
		return nil, fmt.Errorf("admin data correction: sync student: %w", err)
	}
	return s.recordAdminCorrection(ctx, req, children, child, baseSnapshot, reason, input.ActorAccountID)
}

// recordAdminCorrection stores the approved staff change request that
// documents a direct correction.
func (s *ChangeRequests) recordAdminCorrection(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild, child *RequestChild, baseSnapshot map[string]any, reason string, actorAccountID int64) (*changeRequestCase, error) {
	proposedSnapshot, err := s.currentSnapshot(ctx, req, children)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	row := &ChangeRequest{
		RequestID: req.ID, RequestChildID: &child.ID, Origin: enrollment.ChangeRequestOriginAdmin,
		Status: enrollment.ChangeRequestStatusApproved, AdminDecisionNote: &reason,
		BaseSnapshot: baseSnapshot, ProposedSnapshot: proposedSnapshot, Diff: snapshotDiff(baseSnapshot, proposedSnapshot),
		CreatedByAccountID: &actorAccountID, ReviewedByAccountID: &actorAccountID, ReviewedAt: &now,
	}
	if err := createChangeRequest(ctx, s.deps.Requests, row); err != nil {
		return nil, fmt.Errorf("admin data correction: create audit entry: %w", err)
	}
	return s.aggregateFromRow(ctx, row, true)
}

// rejectOpenChangeRequestsForAdminCorrection rejects the family's open
// change requests the correction supersedes and tells the family why.
func (s *ChangeRequests) rejectOpenChangeRequestsForAdminCorrection(ctx context.Context, req *enrollmentModels.Request, reason string, actorAccountID int64) error {
	rows, err := changeRequestsFromOwner(s.deps.Requests.OpenChangeRequestsForRequestForUpdate(ctx, req.ID))
	if err != nil {
		return fmt.Errorf("admin data correction: lock open change requests: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	note := "Diese Änderungsanfrage wurde durch eine direkte Korrektur der OGS ersetzt. Grund: " + reason
	now := time.Now()
	for _, row := range rows {
		if err := s.deps.Requests.MarkChangeRequestReviewed(ctx, row.ID, enrollment.ChangeRequestStatusRejected, &note, actorAccountID, now); err != nil {
			return fmt.Errorf("admin data correction: reject open change request %d: %w", row.ID, err)
		}
		actorID := actorAccountID
		if err := s.deps.Requests.InsertChangeRequestMessage(ctx, &enrollment.ChangeRequestMessage{
			ChangeRequestID: row.ID, AuthorType: enrollment.ChangeRequestMessageAuthorStaff, AuthorAccountID: &actorID, Body: note,
		}); err != nil {
			return fmt.Errorf("admin data correction: record rejection message for change request %d: %w", row.ID, err)
		}
		s.enqueueParentNotification(ctx, req.TenantID, req, row.ID, enrollment.MailKindChangeRequestRejected)
	}
	return nil
}

func validateAdminCorrectionInput(input enrollment.CorrectApprovedChildDataInput) (string, string, string, error) {
	firstName := strings.TrimSpace(input.FirstName)
	lastName := strings.TrimSpace(input.LastName)
	reason := strings.TrimSpace(input.Reason)
	if input.RequestID <= 0 || input.ChildID <= 0 || input.ActorAccountID <= 0 || firstName == "" || lastName == "" || input.DateOfBirth.IsZero() || reason == "" {
		return "", "", "", fmt.Errorf("%w: request, child, actor, child data, and reason are required", enrollment.ErrChangeRequestInvalidData)
	}
	return firstName, lastName, reason, nil
}

func requestChildByID(children []*RequestChild, childID int64) *RequestChild {
	for _, child := range children {
		if child != nil && child.ID == childID {
			return child
		}
	}
	return nil
}

func (s *ChangeRequests) validateAdminCorrectionChild(child *RequestChild) error {
	if child.Status != enrollmentModels.ChildStatusApproved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 || s.deps.Decisions == nil {
		return fmt.Errorf("%w: only approved children with a linked student can be corrected", enrollment.ErrChangeRequestNotAllowed)
	}
	return nil
}

// prepareAdminCorrectionTargets validates a changed grade or class against
// the collection settings; unchanged targets are kept as stored.
func (s *ChangeRequests) prepareAdminCorrectionTargets(ctx context.Context, req *enrollmentModels.Request, child *RequestChild, input enrollment.CorrectApprovedChildDataInput) (*int16, *string, error) {
	normalizedSchoolClass := normalizedOptionalString(input.TargetSchoolClass)
	gradeChanged := !sameGradeLevel(child.TargetGradeLevel, input.TargetGradeLevel)
	classChanged := !sameOptionalString(child.TargetSchoolClass, normalizedSchoolClass)
	if !gradeChanged && !classChanged {
		return child.TargetGradeLevel, child.TargetSchoolClass, nil
	}
	collectGradeLevel, collectSchoolClass, err := s.adminCorrectionCapabilities(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := validateAdminCorrectionTargetPermissions(gradeChanged, classChanged, collectGradeLevel, collectSchoolClass); err != nil {
		return nil, nil, err
	}
	targetGradeLevel := child.TargetGradeLevel
	if gradeChanged {
		if targetGradeLevel, err = s.validateAdminCorrectionGrade(ctx, input.TargetGradeLevel); err != nil {
			return nil, nil, err
		}
	}
	targetSchoolClass := child.TargetSchoolClass
	if classChanged {
		targetSchoolClass = normalizedSchoolClass
	}
	if collectGradeLevel && collectSchoolClass {
		return s.validateAdminCorrectionSchoolClass(ctx, req.PhaseID, SubmitChild{TargetGradeLevel: targetGradeLevel, TargetSchoolClass: targetSchoolClass})
	}
	if err := validatePreservedSchoolClassGrade(gradeChanged, targetGradeLevel, targetSchoolClass); err != nil {
		return nil, nil, err
	}
	return targetGradeLevel, targetSchoolClass, nil
}

func validateAdminCorrectionTargetPermissions(gradeChanged, classChanged, collectGradeLevel, collectSchoolClass bool) error {
	if gradeChanged && !collectGradeLevel {
		return fmt.Errorf("%w: target_grade_level cannot be changed while grade collection is disabled", enrollment.ErrChangeRequestInvalidData)
	}
	if classChanged && (!collectGradeLevel || !collectSchoolClass) {
		return fmt.Errorf("%w: target_school_class cannot be changed while school-class collection is disabled", enrollment.ErrChangeRequestInvalidData)
	}
	return nil
}

func validatePreservedSchoolClassGrade(gradeChanged bool, targetGradeLevel *int16, targetSchoolClass *string) error {
	if !gradeChanged {
		return nil
	}
	class := trimmedOptionalString(targetSchoolClass)
	grade := strconv.Itoa(int(*targetGradeLevel))
	if prefix := gradePrefix(class); prefix != "" && prefix != grade {
		return fmt.Errorf("%w: existing target_school_class %q does not match target grade %s", enrollment.ErrChangeRequestInvalidData, class, grade)
	}
	return nil
}

func (s *ChangeRequests) adminCorrectionCapabilities(ctx context.Context) (bool, bool, error) {
	if s.deps.Settings == nil {
		return false, false, errors.New("admin data correction: enrollment settings resolver is not configured")
	}
	collectGradeLevel, err := s.deps.Settings.CollectGradeLevel(ctx)
	if err != nil {
		return false, false, fmt.Errorf("admin data correction: resolve %s: %w", settingCollectGradeLevel, err)
	}
	collectSchoolClass, err := s.deps.Settings.CollectSchoolClass(ctx)
	if err != nil {
		return false, false, fmt.Errorf("admin data correction: resolve %s: %w", settingCollectSchoolClass, err)
	}
	return collectGradeLevel, collectSchoolClass, nil
}

func (s *ChangeRequests) validateAdminCorrectionGrade(ctx context.Context, grade *int16) (*int16, error) {
	if grade == nil {
		return nil, fmt.Errorf("%w: target_grade_level is required", enrollment.ErrChangeRequestInvalidData)
	}
	gradeMax, err := s.intake.resolveGradeMax(ctx)
	if err != nil {
		return nil, err
	}
	if *grade < 1 || int(*grade) > gradeMax {
		return nil, fmt.Errorf("%w: target_grade_level must be between 1 and %d", enrollment.ErrChangeRequestInvalidData, gradeMax)
	}
	return grade, nil
}

func (s *ChangeRequests) validateAdminCorrectionSchoolClass(ctx context.Context, phaseID int64, child SubmitChild) (*int16, *string, error) {
	phase, err := s.intake.intakePhase(ctx, phaseID)
	if err != nil {
		return nil, nil, fmt.Errorf("admin data correction: load phase: %w", err)
	}
	if phase == nil {
		return nil, nil, errors.New("admin data correction: phase not found")
	}
	children := []SubmitChild{child}
	if err := s.intake.validateAndNormalizeSchoolClasses(ctx, phase, children); err != nil {
		return nil, nil, err
	}
	return children[0].TargetGradeLevel, children[0].TargetSchoolClass, nil
}

func normalizedOptionalString(value *string) *string {
	trimmed := trimmedOptionalString(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
