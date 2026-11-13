package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The phase eligibility gates of a submission and its edits (#1663): the
// audience, the eligible classes and grades, the enrolled-status probes, the
// pinned existing student and the per-student re-enrollment permission.

// validatePhaseEligibility enforces the audience gate and the per-child gates
// of a self-service submission; the caller's eligibilityGate decides whether
// it runs.
//
// linked_parents and existing_students need an authenticated,
// submit-eligible guardian account: an existing_students submission renews a
// live student, and name plus birthday is a recognition signal, not an
// authentication one.
func (s *Intake) validatePhaseEligibility(ctx context.Context, phase *enrollmentNative.Phase, req SubmitRequest) error {
	if audienceRequiresGuardianAccount(phase.Audience) &&
		(req.GuardianAccountID == nil || !req.GuardianSubmitEligible) {
		return enrollmentNative.ErrPhaseNotEligible
	}
	return s.validatePhaseChildEligibility(ctx, phase, req)
}

// audienceRequiresGuardianAccount reports whether a phase audience may only
// be submitted by an authenticated, submit-eligible guardian account. The
// same set is hidden by the parents-portal picker and refused by the public
// form gate.
func audienceRequiresGuardianAccount(audience string) bool {
	return audience == enrollmentNative.PhaseAudienceLinkedParents ||
		audience == enrollmentNative.PhaseAudienceExistingStudents
}

// validatePhaseChildEligibility enforces the per-child gates independently of
// the audience gate. Callers run validateAndNormalizeSchoolClasses first so
// this sees the stored class.
func (s *Intake) validatePhaseChildEligibility(ctx context.Context, phase *enrollmentNative.Phase, req SubmitRequest) error {
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
func (s *Intake) validateChildEnrolledStatus(ctx context.Context, phase *enrollmentNative.Phase, tenantID int64, child SubmitChild, childIndex int) error {
	if s.deps.Students == nil ||
		(phase.Audience != enrollmentNative.PhaseAudienceNewStudents &&
			phase.Audience != enrollmentNative.PhaseAudienceExistingStudents) {
		return nil
	}
	exists, err := s.deps.Students.ExistsEnrolledByNameAndBirthday(ctx, tenantID, child.FirstName, child.LastName, child.DateOfBirth)
	if err != nil {
		return fmt.Errorf("submit: check enrolled student for child %d: %w", childIndex, err)
	}
	if phase.Audience == enrollmentNative.PhaseAudienceNewStudents && exists {
		return fmt.Errorf("%w: child %d", enrollmentNative.ErrChildAlreadyEnrolled, childIndex)
	}
	if phase.Audience == enrollmentNative.PhaseAudienceExistingStudents && !exists {
		return fmt.Errorf("%w: child %d", enrollmentNative.ErrChildNotEnrolled, childIndex)
	}
	return nil
}

// validateChildClassEligibility enforces eligible_school_classes: when the
// list is non-empty, every child must declare one of the listed classes. It
// holds at every point of a request's life, so the change requests apply it
// too.
func validateChildClassEligibility(phase *enrollmentNative.Phase, children []SubmitChild) error {
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
			return fmt.Errorf("%w: child %d declares no school class", enrollmentNative.ErrChildClassNotEligible, i)
		}
		if _, ok := eligible[declared]; !ok {
			return fmt.Errorf("%w: child %d class %q", enrollmentNative.ErrChildClassNotEligible, i, declared)
		}
	}
	return nil
}

// validateChildGradeEligibility enforces eligible_grade_levels. A missing
// grade is a refusal, not a pass: treating it as eligible would silently
// disable the restriction.
func validateChildGradeEligibility(phase *enrollmentNative.Phase, children []SubmitChild) error {
	if len(phase.EligibleGradeLevels) == 0 {
		return nil
	}
	eligible := make(map[int]struct{}, len(phase.EligibleGradeLevels))
	for _, level := range phase.EligibleGradeLevels {
		eligible[level] = struct{}{}
	}
	for i := range children {
		if children[i].TargetGradeLevel == nil {
			return fmt.Errorf("%w: child %d declares no grade level", enrollmentNative.ErrChildGradeNotEligible, i)
		}
		declared := int(*children[i].TargetGradeLevel)
		if _, ok := eligible[declared]; !ok {
			return fmt.Errorf("%w: child %d grade %d", enrollmentNative.ErrChildGradeNotEligible, i, declared)
		}
	}
	return nil
}

// isTrustedEnrollmentSource reports whether a stored request came through a
// deliberate override path (admin manual enrollment or a late invite); an
// edit keeps the eligibility override its creation had.
func isTrustedEnrollmentSource(source string) bool {
	switch strings.TrimSpace(source) {
	case enrollmentNative.RequestSourceLateInvite, enrollmentNative.RequestSourceAdminManual:
		return true
	default:
		return false
	}
}

// hasRolloverGeneratedChild reports whether a child was carried forward by
// the rollover; such requests skip the self-service gates on renewal.
func hasRolloverGeneratedChild(children []*enrollmentNative.RequestChild) bool {
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
func (s *Intake) resolveMatchedStudentID(ctx context.Context, tenantID int64, phase *enrollmentNative.Phase, childIndex int, child SubmitChild) (*int64, error) {
	if s.deps.Students == nil || phase.Audience != enrollmentNative.PhaseAudienceExistingStudents {
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
		return nil, fmt.Errorf("%w: child %d", enrollmentNative.ErrChildEnrollmentAmbiguous, childIndex)
	}
	return nil, nil
}

// assertExistingStudentMatchResolved closes the gap between the enrolled
// gate and the pinned match: when the gate ran and proved an enrolled match,
// a missing pin means the student changed status underneath the write, and
// approval would create a duplicate.
func assertExistingStudentMatchResolved(phase *enrollmentNative.Phase, matchedStudentID *int64, eligibilityEnforced bool, childIndex int) error {
	if !eligibilityEnforced ||
		matchedStudentID != nil ||
		phase.Audience != enrollmentNative.PhaseAudienceExistingStudents {
		return nil
	}
	return fmt.Errorf("%w: child %d", enrollmentNative.ErrChildNotEnrolled, childIndex)
}

// reEnrollmentSubmitter is the identity a pinned re-enrollment is authorized
// against: the authenticated account, or the email an accountless late
// invite was issued to — never an edited contact email (#2164). AdminManaged
// marks the staff-authorized manual enrollmentNative.
type reEnrollmentSubmitter struct {
	GuardianAccountID *int64
	GuardianEmail     string
	AdminManaged      bool
}

func reEnrollmentSubmitterFor(submissionSource string, guardianAccountID *int64, guardianEmail string) reEnrollmentSubmitter {
	return reEnrollmentSubmitter{
		GuardianAccountID: guardianAccountID,
		GuardianEmail:     guardianEmailKey(guardianEmail),
		AdminManaged:      enrollmentNative.NormalizedSubmissionSource(submissionSource) == enrollmentNative.RequestSourceAdminManual,
	}
}

// reEnrollmentSubmitterForPersistedRequest restores the identity of a stored
// request. An accountless late-invite request reloads the address the school
// issued the invite to.
func reEnrollmentSubmitterForPersistedRequest(ctx context.Context, lateInvites DecisionLateInvites, req *enrollmentNative.Request) (reEnrollmentSubmitter, error) {
	if req == nil {
		return reEnrollmentSubmitter{}, errors.New("request is required")
	}
	email := req.GuardianEmail
	if req.GuardianAccountID == nil && enrollmentNative.NormalizedSubmissionSource(req.SubmissionSource) == enrollmentNative.RequestSourceLateInvite {
		if lateInvites == nil {
			return reEnrollmentSubmitter{}, errors.New("late invite repository is not configured")
		}
		invite, err := approvalLateInvite(ctx, lateInvites, req.ID)
		if err != nil {
			return reEnrollmentSubmitter{}, fmt.Errorf("load late invite for request %d: %w", req.ID, err)
		}
		email = invite.GuardianEmail
	}
	return reEnrollmentSubmitterFor(req.SubmissionSource, req.GuardianAccountID, email), nil
}

// holdReEnrollment holds the submitter's authority over the students behind
// the parent guard until the caller's transaction ends (#6638). Staff manual
// enrollment is the one bypass and holds nothing. It fails closed without
// the hold or without any identity; errors of the hold pass through. The
// change-request approval asks it for every pin and approved student before
// its phase locks.
func (s *Intake) holdReEnrollment(ctx context.Context, submitter reEnrollmentSubmitter, studentIDs, graph []int64, tenantID int64) (renewalHolds, error) {
	holds := make(renewalHolds, len(studentIDs))
	if len(studentIDs) == 0 {
		return holds, nil
	}
	if submitter.AdminManaged {
		for _, id := range studentIDs {
			holds[id] = true
		}
		return holds, nil
	}
	grants, err := submitterHoldsRenewal(ctx, s.deps.GuardianHold, submitter, studentIDs, graph, tenantID)
	if err != nil {
		return nil, fmt.Errorf("verify guardian re-enrollment permission: %w", err)
	}
	for _, id := range studentIDs {
		holds[id] = grants[id]
	}
	return holds, nil
}

// renewalHolds retains the candidate set and its locked grants until commit.
// An absent key can never acquire a child lock after the phase locks.
type renewalHolds map[int64]bool

func (s *Intake) holdGuardianReEnrollment(holds renewalHolds, matchedStudentID *int64, childIndex int) error {
	if matchedStudentID != nil && !holds[*matchedStudentID] {
		return fmt.Errorf("%w: child %d", enrollmentNative.ErrChildEnrollmentNotPermitted, childIndex)
	}
	return nil
}

// holdRenewalCandidates holds all candidates before the phase locks. Its
// results stay valid until commit and are reused by the final pin checks.
func (s *Intake) holdRenewalCandidates(ctx context.Context, tenantID int64, phase *enrollmentNative.Phase, children []SubmitChild, carried []*int64, submitter func() (reEnrollmentSubmitter, error)) (renewalHolds, error) {
	ids, err := s.renewalCandidates(ctx, tenantID, phase, children, carried, true)
	if err != nil || len(ids) == 0 {
		return renewalHolds{}, err
	}
	identity, err := submitter()
	if err != nil {
		return nil, err
	}
	if identity.GuardianAccountID == nil && !identity.AdminManaged && phase.Audience != enrollmentNative.PhaseAudienceExistingStudents {
		ids, _ = s.renewalCandidates(ctx, tenantID, phase, children, carried, false)
	}
	if len(ids) == 0 {
		return renewalHolds{}, nil
	}
	if identity.AdminManaged {
		return s.holdStaffRenewalCandidates(ctx, ids)
	}
	grants, err := submitterHoldsRenewal(ctx, s.deps.GuardianHold, identity, ids, nil, tenantID)
	if err != nil {
		return nil, fmt.Errorf("submit: hold re-enrollment candidates: %w", err)
	}
	holds := make(renewalHolds, len(ids))
	for _, id := range ids {
		holds[id] = grants[id]
	}
	return holds, nil
}

// Staff bypass guardian authorization but still hold children before phase
// locks: inserting a pin needs the student's foreign-key lock.
func (s *Intake) holdStaffRenewalCandidates(ctx context.Context, ids []int64) (renewalHolds, error) {
	if s.deps.Students == nil {
		return nil, errors.New("submit: student locks are not configured")
	}
	if err := s.deps.Students.LockStudents(ctx, ids); err != nil {
		return nil, fmt.Errorf("submit: hold staff re-enrollment candidates: %w", err)
	}
	holds := make(renewalHolds, len(ids))
	for _, id := range ids {
		holds[id] = true
	}
	return holds, nil
}

// renewalCandidates lists the candidates' student ids, ascending and unique.
func (s *Intake) renewalCandidates(ctx context.Context, tenantID int64, phase *enrollmentNative.Phase, children []SubmitChild, carried []*int64, match bool) ([]int64, error) {
	matching := match && s.deps.Students != nil && (phase.Audience == enrollmentNative.PhaseAudienceExistingStudents || audienceChecksEnrolledChildren(phase))
	var matches [][]int64
	if matching {
		var err error
		matches, err = s.matchRenewalCandidates(ctx, tenantID, children)
		if err != nil {
			return nil, err
		}
	}
	var ids []int64
	for i := range children {
		if i < len(carried) && carried[i] != nil {
			ids = append(ids, *carried[i])
		}
		if matching && len(matches[i]) == 1 {
			ids = append(ids, matches[i][0])
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}

// matchRenewalCandidates resolves every submitted identity in one directory
// read and requires an answer for each input, including missing matches.
func (s *Intake) matchRenewalCandidates(ctx context.Context, tenantID int64, children []SubmitChild) ([][]int64, error) {
	identities := make([]enrollmentNative.StudentIdentity, len(children))
	for i, child := range children {
		identities[i] = enrollmentNative.StudentIdentity{FirstName: child.FirstName, LastName: child.LastName, Birthday: child.DateOfBirth}
	}
	matches, err := s.deps.Students.FindEnrolledStudentIDsByIdentities(ctx, tenantID, identities)
	if err != nil {
		return nil, fmt.Errorf("submit: resolve re-enrollment candidates: %w", err)
	}
	if len(matches) != len(children) {
		return nil, errors.New("submit: incomplete re-enrollment candidate matches")
	}
	return matches, nil
}

// submitterHoldsRenewal is submitterMayRenewStudent behind the parent guard:
// the same identities, the same permission and the same staff bypass, with
// the facts locked until the caller's transaction ends.
func submitterHoldsRenewal(ctx context.Context, authorizer GuardianStudentHold, submitter reEnrollmentSubmitter, studentIDs, graph []int64, tenantID int64) (map[int64]bool, error) {
	if authorizer == nil {
		return nil, nil
	}
	switch {
	case submitter.GuardianAccountID != nil:
		return authorizer.HoldAccountStudentsPermission(ctx, *submitter.GuardianAccountID, studentIDs, graph, tenantID, guardianPermissionEnrollmentSubmit)
	case submitter.GuardianEmail != "":
		return authorizer.HoldGuardianEmailStudentsPermission(ctx, submitter.GuardianEmail, studentIDs, graph, tenantID, guardianPermissionEnrollmentSubmit)
	default:
		return nil, nil
	}
}

// submitterMayRenewStudent reports whether the request identity holds
// parent_portal.enrollment.submit on its own relationship to the student in
// the tenant now. Staff manual enrollment is the one bypass. It answers false
// without an authorizer or without any identity. It reads without locks, for
// the office's detail read only: every write that uses a parent's pin asks
// the same question behind the parent guard (submitterHoldsRenewal), since
// the school can remove the relationship in between (#6053, #6638).
func submitterMayRenewStudent(ctx context.Context, authorizer GuardianStudentAuthorizer, submitter reEnrollmentSubmitter, studentID, tenantID int64) (bool, error) {
	if submitter.AdminManaged {
		return true, nil
	}
	if authorizer == nil {
		return false, nil
	}
	switch {
	case submitter.GuardianAccountID != nil:
		return authorizer.AccountHasStudentPermission(ctx, *submitter.GuardianAccountID,
			studentID, tenantID, guardianPermissionEnrollmentSubmit)
	case submitter.GuardianEmail != "":
		return authorizer.GuardianEmailHasStudentPermission(ctx, submitter.GuardianEmail,
			studentID, tenantID, guardianPermissionEnrollmentSubmit)
	default:
		return false, nil
	}
}

// finalRenewalMatches rechecks identities in one read after the phase locks.
// New matches may be observed, but only a pre-held candidate may be pinned.
func (s *Intake) finalRenewalMatches(ctx context.Context, tenantID int64, phase *enrollmentNative.Phase, children []SubmitChild) ([][]int64, error) {
	if s.deps.Students == nil || (phase.Audience != enrollmentNative.PhaseAudienceExistingStudents && !audienceChecksEnrolledChildren(phase)) {
		return make([][]int64, len(children)), nil
	}
	return s.matchRenewalCandidates(ctx, tenantID, children)
}

func matchedRenewalStudent(phase *enrollmentNative.Phase, matches []int64, childIndex int) (*int64, error) {
	if len(matches) == 1 {
		id := matches[0]
		return &id, nil
	}
	if len(matches) > 1 && phase.Audience == enrollmentNative.PhaseAudienceExistingStudents {
		return nil, fmt.Errorf("%w: child %d", enrollmentNative.ErrChildEnrollmentAmbiguous, childIndex)
	}
	return nil, nil
}

// resolveEnrolledChildPin uses the held relationship grant for open and
// linked-parent phases. A newly matching child aborts before any later lock.
func (s *Intake) resolveEnrolledChildPin(ctx context.Context, phase *enrollmentNative.Phase, id *int64, submitter reEnrollmentSubmitter, holds renewalHolds, childIndex int) (*int64, error) {
	if id == nil || !audienceChecksEnrolledChildren(phase) || submitter.GuardianAccountID == nil {
		return nil, nil
	}
	if _, held := holds[*id]; !held {
		return nil, fmt.Errorf("%w: child %d", enrollmentNative.ErrChildEnrollmentNotPermitted, childIndex)
	}
	if !holds[*id] {
		return nil, nil
	}
	if err := s.deps.Requests.AcquireExistingStudentMatchLock(ctx, phase.ID); err != nil {
		return nil, fmt.Errorf("submit: acquire existing-student match lock: %w", err)
	}
	taken, err := s.deps.Requests.HasActiveRequestForMatchedStudent(ctx, phase.ID, *id, 0)
	if err != nil {
		return nil, fmt.Errorf("submit: enrolled child duplicate check: %w", err)
	}
	if taken {
		return nil, nil
	}
	return id, nil
}

func (s *Intake) submittedChildPin(ctx context.Context, phase *enrollmentNative.Phase, childIndex int, matches []int64, submitter reEnrollmentSubmitter, holds renewalHolds) (*int64, error) {
	pin, err := matchedRenewalStudent(phase, matches, childIndex)
	if err != nil || phase.Audience == enrollmentNative.PhaseAudienceExistingStudents {
		return pin, err
	}
	return s.resolveEnrolledChildPin(ctx, phase, pin, submitter, holds, childIndex)
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
		return fmt.Errorf("%w: child %d", enrollmentNative.ErrExistingStudentAlreadyRequested, childIndex)
	}
	return nil
}

// validateAndNormalizeSchoolClasses enforces the concrete-class rules (#1833)
// and sets each child's class to the stored value: nil when classes are not
// collected or the grade does not collect one, a listed class matching the
// child's grade otherwise, and a refusal when a required class is missing.
func (s *Intake) validateAndNormalizeSchoolClasses(ctx context.Context, phase *enrollmentNative.Phase, children []SubmitChild) error {
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
func normalizedSchoolClass(phase *enrollmentNative.Phase, allowed map[string]struct{}, collect bool, i int, child SubmitChild) (*string, error) {
	chosen := strings.TrimSpace(optionalString(child.TargetSchoolClass))
	grade := 0
	if child.TargetGradeLevel != nil {
		grade = int(*child.TargetGradeLevel)
	}
	if !collect || grade < 1 || (grade == 1 && !enrollmentNative.CollectsGrade1Class(phase)) {
		return nil, nil
	}
	if chosen == "" {
		// Only force a pick when the grade has a class to pick; otherwise
		// "Klasse offen" is the only valid outcome (#1833).
		if phase.RequireSchoolClass && gradeHasSelectableClass(allowed, grade) {
			return nil, fmt.Errorf("%w: child %d missing target_school_class", enrollmentNative.ErrInvalidSubmission, i)
		}
		return nil, nil
	}
	if _, ok := allowed[chosen]; !ok {
		return nil, fmt.Errorf("%w: child %d target_school_class %q not offered by this phase", enrollmentNative.ErrInvalidSubmission, i, chosen)
	}
	// The pick list mixes every grade's classes, so a grade-2 child must not
	// pick "3a". A class without a grade number is left to the list check.
	if prefix := gradePrefix(chosen); prefix != "" && prefix != strconv.Itoa(grade) {
		return nil, fmt.Errorf("%w: child %d target_school_class %q does not match target grade %d", enrollmentNative.ErrInvalidSubmission, i, chosen, grade)
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
