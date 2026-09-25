package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The phase eligibility gates of a submission and its edits (#1663): the
// audience, the eligible classes and grades, the enrolled-status probes, the
// pinned existing student and the per-student re-enrollment permission.

// validatePhaseEligibility enforces the phase's eligibility for the
// self-service paths. The trusted paths — admin manual enrollment and late
// invites, both recognizable by AllowClosedPhase — bypass the audience gate;
// the per-child re-enrollment permission still applies to them.
//
// linked_parents and existing_students need an authenticated,
// submit-eligible guardian account: an existing_students submission renews a
// live student, and name plus birthday is a recognition signal, not an
// authentication one.
func (s *Intake) validatePhaseEligibility(ctx context.Context, phase *enrollment.Phase, req SubmitRequest) error {
	if req.AllowClosedPhase {
		return nil
	}
	if audienceRequiresGuardianAccount(phase.Audience) &&
		(req.GuardianAccountID == nil || !req.GuardianSubmitEligible) {
		return enrollment.ErrPhaseNotEligible
	}
	return s.validatePhaseChildEligibility(ctx, phase, req)
}

// audienceRequiresGuardianAccount reports whether a phase audience may only
// be submitted by an authenticated, submit-eligible guardian account. The
// same set is hidden by the parents-portal picker and refused by the public
// form gate.
func audienceRequiresGuardianAccount(audience string) bool {
	return audience == enrollment.PhaseAudienceLinkedParents ||
		audience == enrollment.PhaseAudienceExistingStudents
}

// validatePhaseChildEligibility enforces the per-child gates independently of
// the audience gate. Callers run validateAndNormalizeSchoolClasses first so
// this sees the stored class.
func (s *Intake) validatePhaseChildEligibility(ctx context.Context, phase *enrollment.Phase, req SubmitRequest) error {
	if err := validateChildGradeEligibility(phase, req.Children); err != nil {
		return err
	}
	if err := validateChildClassEligibility(phase, req.Children); err != nil {
		return err
	}
	for i := range req.Children {
		if err := s.validateChildEnrolledStatus(ctx, phase, req.TenantID, req.Children[i], i); err != nil {
			return err
		}
	}
	return nil
}

// validateChildEnrolledStatus applies the audience's enrolled-status gate to
// one child: new_students refuses an already enrolled child, existing_students
// one without an enrolled record. Both share one name-and-birthday probe.
func (s *Intake) validateChildEnrolledStatus(ctx context.Context, phase *enrollment.Phase, tenantID int64, child SubmitChild, childIndex int) error {
	if s.deps.Students == nil ||
		(phase.Audience != enrollment.PhaseAudienceNewStudents &&
			phase.Audience != enrollment.PhaseAudienceExistingStudents) {
		return nil
	}
	exists, err := s.deps.Students.ExistsEnrolledByNameAndBirthday(ctx, tenantID, child.FirstName, child.LastName, child.DateOfBirth)
	if err != nil {
		return fmt.Errorf("submit: check enrolled student for child %d: %w", childIndex, err)
	}
	if phase.Audience == enrollment.PhaseAudienceNewStudents && exists {
		return fmt.Errorf("%w: child %d", enrollment.ErrChildAlreadyEnrolled, childIndex)
	}
	if phase.Audience == enrollment.PhaseAudienceExistingStudents && !exists {
		return fmt.Errorf("%w: child %d", enrollment.ErrChildNotEnrolled, childIndex)
	}
	return nil
}

// validateChildClassEligibility enforces eligible_school_classes: when the
// list is non-empty, every child must declare one of the listed classes. It
// holds at every point of a request's life, so the change requests apply it
// too.
func validateChildClassEligibility(phase *enrollment.Phase, children []SubmitChild) error {
	eligible := make(map[string]struct{}, len(phase.EligibleSchoolClasses))
	for _, c := range phase.EligibleSchoolClasses {
		if t := strings.TrimSpace(c); t != "" {
			eligible[t] = struct{}{}
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	for i := range children {
		declared := strings.TrimSpace(optionalString(children[i].TargetSchoolClass))
		if declared == "" {
			return fmt.Errorf("%w: child %d declares no school class", enrollment.ErrChildClassNotEligible, i)
		}
		if _, ok := eligible[declared]; !ok {
			return fmt.Errorf("%w: child %d class %q", enrollment.ErrChildClassNotEligible, i, declared)
		}
	}
	return nil
}

// validateChildGradeEligibility enforces eligible_grade_levels. A missing
// grade is a refusal, not a pass: treating it as eligible would silently
// disable the restriction.
func validateChildGradeEligibility(phase *enrollment.Phase, children []SubmitChild) error {
	if len(phase.EligibleGradeLevels) == 0 {
		return nil
	}
	eligible := make(map[int]struct{}, len(phase.EligibleGradeLevels))
	for _, level := range phase.EligibleGradeLevels {
		eligible[level] = struct{}{}
	}
	for i := range children {
		if children[i].TargetGradeLevel == nil {
			return fmt.Errorf("%w: child %d declares no grade level", enrollment.ErrChildGradeNotEligible, i)
		}
		declared := int(*children[i].TargetGradeLevel)
		if _, ok := eligible[declared]; !ok {
			return fmt.Errorf("%w: child %d grade %d", enrollment.ErrChildGradeNotEligible, i, declared)
		}
	}
	return nil
}

// isTrustedEnrollmentSource reports whether a stored request came through a
// deliberate override path (admin manual enrollment or a late invite); an
// edit keeps the eligibility override its creation had.
func isTrustedEnrollmentSource(source string) bool {
	switch strings.TrimSpace(source) {
	case enrollmentModels.RequestSourceLateInvite, enrollmentModels.RequestSourceAdminManual:
		return true
	default:
		return false
	}
}

// hasRolloverGeneratedChild reports whether a child was carried forward by
// the rollover; such requests skip the self-service gates on renewal.
func hasRolloverGeneratedChild(children []*RequestChild) bool {
	for _, child := range children {
		if child.RolloverSourceChildID != nil {
			return true
		}
	}
	return false
}

// resolveMatchedStudentID returns the enrolled student an existing_students
// child matched. The directory answers nil for no match and for more than
// one; an ambiguous match is refused rather than left to the fresh-create
// branch, which would add yet another duplicate.
func (s *Intake) resolveMatchedStudentID(ctx context.Context, tenantID int64, phase *enrollment.Phase, childIndex int, child SubmitChild) (*int64, error) {
	if s.deps.Students == nil || phase.Audience != enrollment.PhaseAudienceExistingStudents {
		return nil, nil
	}
	id, err := s.deps.Students.FindEnrolledStudentIDByNameAndBirthday(ctx, tenantID, child.FirstName, child.LastName, child.DateOfBirth)
	if err != nil {
		return nil, fmt.Errorf("submit: resolve matched student: %w", err)
	}
	if id != nil {
		return id, nil
	}
	exists, err := s.deps.Students.ExistsEnrolledByNameAndBirthday(ctx, tenantID, child.FirstName, child.LastName, child.DateOfBirth)
	if err != nil {
		return nil, fmt.Errorf("submit: resolve matched student ambiguity: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("%w: child %d", enrollment.ErrChildEnrollmentAmbiguous, childIndex)
	}
	return nil, nil
}

// assertExistingStudentMatchResolved closes the gap between the enrolled
// gate and the pinned match: when the gate ran and proved an enrolled match,
// a missing pin means the student changed status underneath the write, and
// approval would create a duplicate.
func assertExistingStudentMatchResolved(phase *enrollment.Phase, matchedStudentID *int64, eligibilityEnforced bool, childIndex int) error {
	if !eligibilityEnforced ||
		matchedStudentID != nil ||
		phase.Audience != enrollment.PhaseAudienceExistingStudents {
		return nil
	}
	return fmt.Errorf("%w: child %d", enrollment.ErrChildNotEnrolled, childIndex)
}

// reEnrollmentSubmitter is the identity a pinned re-enrollment is authorized
// against: the authenticated account, or the email an accountless late
// invite was issued to — never an edited contact email (#2164). AdminManaged
// marks the staff-authorized manual enrollment.
type reEnrollmentSubmitter struct {
	GuardianAccountID *int64
	GuardianEmail     string
	AdminManaged      bool
}

func reEnrollmentSubmitterFor(submissionSource string, guardianAccountID *int64, guardianEmail string) reEnrollmentSubmitter {
	return reEnrollmentSubmitter{
		GuardianAccountID: guardianAccountID,
		GuardianEmail:     lowerTrim(guardianEmail),
		AdminManaged:      enrollment.NormalizedSubmissionSource(submissionSource) == enrollmentModels.RequestSourceAdminManual,
	}
}

// reEnrollmentSubmitterForPersistedRequest restores the identity of a stored
// request. An accountless late-invite request reloads the address the school
// issued the invite to.
func reEnrollmentSubmitterForPersistedRequest(ctx context.Context, lateInvites DecisionLateInvites, req *enrollmentModels.Request) (reEnrollmentSubmitter, error) {
	if req == nil {
		return reEnrollmentSubmitter{}, errors.New("request is required")
	}
	email := req.GuardianEmail
	if req.GuardianAccountID == nil && enrollment.NormalizedSubmissionSource(req.SubmissionSource) == enrollmentModels.RequestSourceLateInvite {
		if lateInvites == nil {
			return reEnrollmentSubmitter{}, errors.New("late invite repository is not configured")
		}
		invite, err := lateInvites.LateInviteByUsedRequestID(ctx, req.ID)
		if err != nil {
			return reEnrollmentSubmitter{}, fmt.Errorf("load late invite for request %d: %w", req.ID, err)
		}
		email = invite.GuardianEmail
	}
	return reEnrollmentSubmitterFor(req.SubmissionSource, req.GuardianAccountID, email), nil
}

// assertGuardianMayReEnrollStudent enforces that a request renews student S
// only if its account, or the email its late invite was issued to, already
// holds parent_portal.enrollment.submit on its own relationship to S. Staff
// manual enrollment is the one bypass. It fails closed without an authorizer
// or without any identity.
func (s *Intake) assertGuardianMayReEnrollStudent(ctx context.Context, submitter reEnrollmentSubmitter, matchedStudentID *int64, tenantID int64, childIndex int) error {
	if matchedStudentID == nil || submitter.AdminManaged {
		return nil
	}
	if s.deps.GuardianAuthorizer == nil {
		return fmt.Errorf("%w: child %d", enrollment.ErrChildEnrollmentNotPermitted, childIndex)
	}
	var (
		granted bool
		err     error
	)
	switch {
	case submitter.GuardianAccountID != nil:
		granted, err = s.deps.GuardianAuthorizer.AccountHasStudentPermission(ctx, *submitter.GuardianAccountID,
			*matchedStudentID, tenantID, guardianPermissionEnrollmentSubmit)
	case submitter.GuardianEmail != "":
		granted, err = s.deps.GuardianAuthorizer.GuardianEmailHasStudentPermission(ctx, submitter.GuardianEmail,
			*matchedStudentID, tenantID, guardianPermissionEnrollmentSubmit)
	default:
		return fmt.Errorf("%w: child %d", enrollment.ErrChildEnrollmentNotPermitted, childIndex)
	}
	if err != nil {
		return fmt.Errorf("submit: verify guardian re-enrollment permission for child %d: %w", childIndex, err)
	}
	if !granted {
		return fmt.Errorf("%w: child %d", enrollment.ErrChildEnrollmentNotPermitted, childIndex)
	}
	return nil
}

// guardMatchedStudentUnique refuses a child pinned to a student another
// active request of the phase already targets. The phase-wide advisory lock
// makes the check race-free across distinct guardian emails. It applies
// regardless of the duplicate policy, because it protects a live student.
// excludeRequestChildID exempts an already stored row re-checking its own pin.
func (s *Intake) guardMatchedStudentUnique(ctx context.Context, phaseID int64, matchedStudentID *int64, excludeRequestChildID int64, childIndex int) error {
	if matchedStudentID == nil {
		return nil
	}
	if err := s.deps.Requests.AcquireExistingStudentMatchLock(ctx, phaseID); err != nil {
		return fmt.Errorf("submit: acquire existing-student match lock: %w", err)
	}
	has, err := s.deps.Requests.HasActiveRequestForMatchedStudent(ctx, phaseID, *matchedStudentID, excludeRequestChildID)
	if err != nil {
		return fmt.Errorf("submit: matched-student duplicate check for child %d: %w", childIndex, err)
	}
	if has {
		return fmt.Errorf("%w: child %d", enrollment.ErrExistingStudentAlreadyRequested, childIndex)
	}
	return nil
}

// validateAndNormalizeSchoolClasses enforces the concrete-class rules (#1833)
// and sets each child's class to the stored value: nil when classes are not
// collected or the grade does not collect one, a listed class matching the
// child's grade otherwise, and a refusal when a required class is missing.
func (s *Intake) validateAndNormalizeSchoolClasses(ctx context.Context, phase *enrollment.Phase, children []SubmitChild) error {
	collect, err := s.collectsSchoolClass(ctx)
	if err != nil {
		return fmt.Errorf("resolve collect_school_class: %w", err)
	}
	allowed := make(map[string]struct{}, len(phase.AvailableSchoolClasses))
	for _, c := range phase.AvailableSchoolClasses {
		if t := strings.TrimSpace(c); t != "" {
			allowed[t] = struct{}{}
		}
	}
	for i := range children {
		class, err := normalizedSchoolClass(phase, allowed, collect, i, children[i])
		if err != nil {
			return err
		}
		children[i].TargetSchoolClass = class
	}
	return nil
}

// normalizedSchoolClass decides one child's stored class. Grade 1 stays
// grade-level-only unless the phase collects a grade-1 class; grade-less
// rows never collect one.
func normalizedSchoolClass(phase *enrollment.Phase, allowed map[string]struct{}, collect bool, i int, child SubmitChild) (*string, error) {
	chosen := strings.TrimSpace(optionalString(child.TargetSchoolClass))
	grade := 0
	if child.TargetGradeLevel != nil {
		grade = int(*child.TargetGradeLevel)
	}
	if !collect || grade < 1 || (grade == 1 && !enrollment.CollectsGrade1Class(phase)) {
		return nil, nil
	}
	if chosen == "" {
		// Only force a pick when the grade has a class to pick; otherwise
		// "Klasse offen" is the only valid outcome (#1833).
		if phase.RequireSchoolClass && gradeHasSelectableClass(allowed, grade) {
			return nil, fmt.Errorf("%w: child %d missing target_school_class", enrollment.ErrInvalidSubmission, i)
		}
		return nil, nil
	}
	if _, ok := allowed[chosen]; !ok {
		return nil, fmt.Errorf("%w: child %d target_school_class %q not offered by this phase", enrollment.ErrInvalidSubmission, i, chosen)
	}
	// The pick list mixes every grade's classes, so a grade-2 child must not
	// pick "3a". A class without a grade number is left to the list check.
	if prefix := gradePrefix(chosen); prefix != "" && prefix != strconv.Itoa(grade) {
		return nil, fmt.Errorf("%w: child %d target_school_class %q does not match target grade %d", enrollment.ErrInvalidSubmission, i, chosen, grade)
	}
	return &chosen, nil
}

// gradeHasSelectableClass reports whether a child in the grade can pick at
// least one offered class: a matching grade number or a class without one.
func gradeHasSelectableClass(allowed map[string]struct{}, grade int) bool {
	want := strconv.Itoa(grade)
	for class := range allowed {
		if prefix := gradePrefix(class); prefix == "" || prefix == want {
			return true
		}
	}
	return false
}
