package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// guardianRoleName is the auth.roles.name value the guardian invitation
// flow uses on accept. Mirrored here so an approval that finds an
// existing account can attach the role for the new tenant directly.
const guardianRoleName = "guardian"

// DecisionService sentinel errors. Mapped to HTTP status codes by the
// admin handlers.
var (
	ErrDecisionRequestNotFound = errors.New("enrollment request not found")
	ErrDecisionChildNotFound   = errors.New("request child not found")
	ErrDecisionInvalidStatus   = errors.New("invalid decision status")
	ErrDecisionAlreadyTerminal = errors.New("child is already in a terminal status")
)

// DecisionStatus enumerates the per-child decisions an admin can apply.
// Mirrors the request_children.status CHECK constraint subset that
// admins are allowed to write (parent-initiated 'withdrawn' goes
// through a different path).
type DecisionStatus string

const (
	DecisionApproved    DecisionStatus = enrollmentModels.ChildStatusApproved
	DecisionWaitlisted  DecisionStatus = enrollmentModels.ChildStatusWaitlisted
	DecisionRejected    DecisionStatus = enrollmentModels.ChildStatusRejected
	DecisionUnderReview DecisionStatus = enrollmentModels.ChildStatusUnderReview
)

var validDecisionStatuses = map[DecisionStatus]bool{
	DecisionApproved:    true,
	DecisionWaitlisted:  true,
	DecisionRejected:    true,
	DecisionUnderReview: true,
}

// DecideInput carries the per-child decision the admin makes.
type DecideInput struct {
	RequestID  int64
	ChildID    int64
	Status     DecisionStatus
	Reason     string // optional; surfaced to parent only when phase.show_status_reason_to_parent
	ReviewedBy int64  // admin's auth account id
}

// DecideOutcome is what the admin handler gets back from Decide. It
// carries the refreshed RequestChild plus an optional follow-up
// instruction asking the handler to issue a guardian invitation
// post-commit (after the tenant tx the handler owns completes).
//
// We surface the invitation as a side-effect rather than firing it from
// inside the service so:
//   - the invitation flow's own DB writes happen only if the approval
//     tx committed cleanly
//   - the handler can apply best-effort error handling without rolling
//     back the approval
type DecideOutcome struct {
	Child         *enrollmentModels.RequestChild
	PendingInvite *PendingGuardianInvite
}

// PendingGuardianInvite is the post-commit hook for fresh approvals
// where the guardian doesn't yet have a portal account. The handler is
// expected to call services/auth.GuardianInvitationService.Create with
// these values once the tenant tx commits.
type PendingGuardianInvite struct {
	GuardianProfileID int64
	CreatedBy         int64 // admin auth account id (for audit)
}

// RequestSummary is the admin-list shape: one row per request with
// per-child counts so the admin can scan the queue without expanding
// every detail page.
type RequestSummary struct {
	Request  *enrollmentModels.Request
	Phase    *enrollmentModels.Phase
	Children []*enrollmentModels.RequestChild
}

// RequestFilters narrows the admin list. Zero-value fields are
// ignored.
type RequestFilters struct {
	PhaseID     int64
	ChildStatus string // matches when ANY child carries this status
}

// DecisionService backs the admin review UI. Slice 2 wires the full
// approval pipeline: status mutation + downstream record creation
// (users.persons / users.students / users.guardian_profiles /
// users.students_guardians / activities.student_enrollments) + outbox
// enqueue for parent decision emails. The guardian invitation is
// surfaced via DecideOutcome.PendingInvite so the handler can fire it
// post-commit.
type DecisionService interface {
	List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error)
	Get(ctx context.Context, requestID int64) (*RequestSummary, error)
	Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error)
}

// DecisionServiceConfig is the dep-injection bundle. The auth-side
// repos (Account/AccountTenant/AccountRole/Role) are the slice-2-fix
// addition: they let an approval that touches a known email attach
// the new tenant directly to the parent's existing portal account
// instead of mailing an invite that would overwrite their password
// on accept.
type DecisionServiceConfig struct {
	RequestRepo              enrollmentModels.RequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	PersonRepo               users.PersonRepository
	StudentRepo              users.StudentRepository
	StudentGuardianRepo      users.StudentGuardianRepository
	GuardianProfileRepo      users.GuardianProfileRepository
	StudentEnrollmentRepo    activities.StudentEnrollmentRepository
	AccountRepo              authModels.AccountRepository
	AccountTenantRepo        authModels.AccountTenantRepository
	AccountRoleRepo          authModels.AccountRoleRepository
	RoleRepo                 authModels.RoleRepository
	OutboxEnqueuer           OutboxEnqueuer
	FrontendURL              string
	Logger                   *slog.Logger
}

type decisionService struct {
	requestRepo              enrollmentModels.RequestRepository
	requestChildRepo         enrollmentModels.RequestChildRepository
	requestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	careOfferingRepo         enrollmentModels.CareOfferingRepository
	phaseRepo                enrollmentModels.PhaseRepository
	personRepo               users.PersonRepository
	studentRepo              users.StudentRepository
	studentGuardianRepo      users.StudentGuardianRepository
	guardianProfileRepo      users.GuardianProfileRepository
	studentEnrollmentRepo    activities.StudentEnrollmentRepository
	accountRepo              authModels.AccountRepository
	accountTenantRepo        authModels.AccountTenantRepository
	accountRoleRepo          authModels.AccountRoleRepository
	roleRepo                 authModels.RoleRepository
	outboxEnqueuer           OutboxEnqueuer
	frontendURL              string
	logger                   *slog.Logger
}

func NewDecisionService(cfg DecisionServiceConfig) DecisionService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &decisionService{
		requestRepo:              cfg.RequestRepo,
		requestChildRepo:         cfg.RequestChildRepo,
		requestChildOfferingRepo: cfg.RequestChildOfferingRepo,
		careOfferingRepo:         cfg.CareOfferingRepo,
		phaseRepo:                cfg.PhaseRepo,
		personRepo:               cfg.PersonRepo,
		studentRepo:              cfg.StudentRepo,
		studentGuardianRepo:      cfg.StudentGuardianRepo,
		guardianProfileRepo:      cfg.GuardianProfileRepo,
		studentEnrollmentRepo:    cfg.StudentEnrollmentRepo,
		accountRepo:              cfg.AccountRepo,
		accountTenantRepo:        cfg.AccountTenantRepo,
		accountRoleRepo:          cfg.AccountRoleRepo,
		roleRepo:                 cfg.RoleRepo,
		outboxEnqueuer:           cfg.OutboxEnqueuer,
		frontendURL:              cfg.FrontendURL,
		logger:                   logger,
	}
}

func (s *decisionService) List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error) {
	requests, err := s.requestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{
		PhaseID:     filters.PhaseID,
		ChildStatus: filters.ChildStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("decision: list requests: %w", err)
	}

	out := make([]*RequestSummary, 0, len(requests))
	for _, req := range requests {
		summary, err := s.assemble(ctx, req)
		if err != nil {
			return nil, err
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *decisionService) Get(ctx context.Context, requestID int64) (*RequestSummary, error) {
	req, err := s.requestRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	return s.assemble(ctx, req)
}

func (s *decisionService) assemble(ctx context.Context, req *enrollmentModels.Request) (*RequestSummary, error) {
	phase, err := s.phaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil {
		// Phase may have been deleted under us — surface as "phase
		// missing" but don't drop the row from the list.
		s.logger.Warn("decision: phase lookup failed",
			slog.Int64("request_id", req.ID),
			slog.Int64("phase_id", req.PhaseID),
			slog.String("error", err.Error()))
		phase = nil
	}
	children, err := s.requestChildRepo.ListByRequestID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("decision: list children for request %d: %w", req.ID, err)
	}
	return &RequestSummary{Request: req, Phase: phase, Children: children}, nil
}

// Decide updates a single child's status. When status==approved the
// service also creates the downstream records (Person + Student +
// GuardianProfile + StudentGuardian + StudentEnrollment[s]) inside the
// same tenant tx the handler provides — failure of any one rolls the
// whole approval back. Parent decision emails are enqueued via the
// outbox in the same tx; guardian invitation creation is returned as a
// PendingGuardianInvite for the handler to fire post-commit.
//
// Idempotency: applying the same status twice is a no-op. Re-applying
// any new status to an already-terminal child (approved/rejected/
// withdrawn) returns ErrDecisionAlreadyTerminal — admins must use
// dedicated revoke/promote flows for those (deferred).
func (s *decisionService) Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error) {
	if input.RequestID <= 0 {
		return nil, fmt.Errorf("%w: request_id required", ErrDecisionInvalidStatus)
	}
	if input.ChildID <= 0 {
		return nil, fmt.Errorf("%w: child_id required", ErrDecisionInvalidStatus)
	}
	if !validDecisionStatuses[input.Status] {
		return nil, fmt.Errorf("%w: %s", ErrDecisionInvalidStatus, input.Status)
	}

	request, err := s.requestRepo.FindByID(ctx, input.RequestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	phase, err := s.phaseRepo.FindByID(ctx, request.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("decision: load phase: %w", err)
	}

	children, err := s.requestChildRepo.ListByRequestID(ctx, input.RequestID)
	if err != nil {
		return nil, fmt.Errorf("decision: load children: %w", err)
	}

	var target *enrollmentModels.RequestChild
	for _, c := range children {
		if c.ID == input.ChildID {
			target = c
			break
		}
	}
	if target == nil {
		return nil, ErrDecisionChildNotFound
	}

	// No-op: same status. Don't bump reviewed_at when nothing changes.
	if target.Status == string(input.Status) {
		return &DecideOutcome{Child: target}, nil
	}

	// Block transitions out of a terminal status. Promotion flows
	// (waitlisted → approved, etc.) come in slice 2; for slice 1 the
	// admin can only move out of submitted / under_review / waitlisted.
	if target.IsTerminal() {
		return nil, ErrDecisionAlreadyTerminal
	}

	reason := strings.TrimSpace(input.Reason)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	outcome := &DecideOutcome{}

	// Approval is the heavy path: create downstream records first so
	// any failure rolls back BEFORE we flip the status. The status
	// update closes the loop after the records exist; if it fails the
	// records are still rolled back via the surrounding tenant tx.
	if input.Status == DecisionApproved {
		invite, err := s.applyApproval(ctx, request, target, phase, input.ReviewedBy)
		if err != nil {
			return nil, err
		}
		outcome.PendingInvite = invite
	}

	if err := s.requestChildRepo.UpdateStatus(ctx, target.ID, string(input.Status), reasonPtr, input.ReviewedBy); err != nil {
		return nil, fmt.Errorf("decision: update child status: %w", err)
	}

	s.logger.Info("enrollment decision applied",
		slog.Int64("request_id", input.RequestID),
		slog.Int64("child_id", input.ChildID),
		slog.String("status", string(input.Status)),
		slog.Int64("reviewed_by", input.ReviewedBy),
		slog.Bool("created_records", input.Status == DecisionApproved),
	)

	// Enqueue parent decision email. Best-effort: log on error but
	// don't roll back the approval. (Outbox writes share the outer tx,
	// so a hard failure WILL roll back — log+swallow keeps the
	// behaviour aligned with submit's "delivery is downstream of the
	// decision".)
	s.enqueueDecisionEmail(ctx, request, target, phase, input.Status, reasonPtr)

	// Refetch to surface DB-managed fields (reviewed_at, updated_at).
	refreshed, err := s.findChildByID(ctx, input.RequestID, input.ChildID)
	if err != nil {
		// Fall back to the in-memory copy with the new status applied.
		target.Status = string(input.Status)
		target.StatusReason = reasonPtr
		outcome.Child = target
		return outcome, nil
	}
	outcome.Child = refreshed
	return outcome, nil
}

// applyApproval creates the downstream records that an approval
// implies. Runs inside the outer tenant tx the handler provides — every
// repo call shares that tx via base.GetDB(ctx, db).
//
// Returns a PendingGuardianInvite when the guardian needs an invitation
// (no existing portal account) so the handler can fire it post-commit.
func (s *decisionService) applyApproval(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	phase *enrollmentModels.Phase,
	reviewedBy int64,
) (*PendingGuardianInvite, error) {
	if s.personRepo == nil || s.studentRepo == nil || s.guardianProfileRepo == nil ||
		s.studentGuardianRepo == nil {
		return nil, fmt.Errorf("decision: approval requires user repos (person/student/guardian)")
	}

	// 1. Resolve or create the guardian profile (per-tenant).
	guardian, profileWasNew, err := s.resolveGuardianProfile(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("decision: resolve guardian: %w", err)
	}

	// 1b. Cross-tenant account check. If the email already has a global
	// auth.accounts row (from another school's enrollment, an admin
	// invitation, etc.), attach the new tenant + this profile to it
	// directly. This bypasses the invitation flow entirely — the
	// invitation accept path overwrites the password hash, which is
	// the wrong UX when the parent already has a working password from
	// another school.
	if guardian.AccountID == nil && guardian.Email != nil && strings.TrimSpace(*guardian.Email) != "" {
		linked, err := s.attachExistingAccountIfPresent(ctx, guardian)
		if err != nil {
			return nil, fmt.Errorf("decision: attach existing account: %w", err)
		}
		if linked {
			s.logger.Info("decision: linked approval to existing global account",
				slog.Int64("guardian_profile_id", guardian.ID),
				slog.Int64("tenant_id", tenant.FromContext(ctx)),
				slog.Bool("profile_was_new", profileWasNew),
			)
		}
	}

	// 2. Person row for the child. DateOfBirth is required so a copy
	// is fine.
	dob := child.DateOfBirth
	person := &users.Person{
		FirstName: child.FirstName,
		LastName:  child.LastName,
		Birthday:  &dob,
	}
	if err := person.Validate(); err != nil {
		return nil, fmt.Errorf("decision: validate person: %w", err)
	}
	if err := s.personRepo.Create(ctx, person); err != nil {
		return nil, fmt.Errorf("decision: create person: %w", err)
	}

	// 3. Student row pinned to the phase's service window. Status
	// 'pending' lets the activate-students scheduler flip to 'active'
	// when ServiceStartDate arrives.
	schoolClass := s.gradeToClass(child.TargetGradeLevel)
	enrolledFrom := phase.ServiceStartDate
	enrolledUntil := phase.ServiceEndDate
	guardianEmail := request.GuardianEmail
	guardianPhone := request.GuardianPhone

	student := &users.Student{
		PersonID:      person.ID,
		SchoolClass:   schoolClass,
		Status:        users.StudentStatusPending,
		EnrolledFrom:  &enrolledFrom,
		EnrolledUntil: &enrolledUntil,
		GuardianEmail: &guardianEmail,
		GuardianPhone: guardianPhone,
	}
	if err := student.Validate(); err != nil {
		return nil, fmt.Errorf("decision: validate student: %w", err)
	}
	if err := s.studentRepo.Create(ctx, student); err != nil {
		return nil, fmt.Errorf("decision: create student: %w", err)
	}

	// 4. Link student ↔ guardian as the primary relationship.
	rel := &users.StudentGuardian{
		StudentID:          student.ID,
		GuardianProfileID:  guardian.ID,
		RelationshipType:   "guardian",
		IsPrimary:          true,
		IsEmergencyContact: true,
		CanPickup:          true,
	}
	if err := rel.Validate(); err != nil {
		return nil, fmt.Errorf("decision: validate student_guardian: %w", err)
	}
	if err := s.studentGuardianRepo.Create(ctx, rel); err != nil {
		return nil, fmt.Errorf("decision: create student_guardian: %w", err)
	}

	// 5. Materialize per-care-offering enrollments. Every offering the
	// parent picked that is bound to an activity_group becomes a row
	// in activities.student_enrollments. Offerings without an activity
	// group (pure schedule-only offerings) are skipped.
	if err := s.materializeEnrollments(ctx, child.ID, student.ID, phase); err != nil {
		return nil, err
	}

	// 6. Stamp the request_children row with the resulting student id
	// so the admin UI can link to the new student record. Failure is
	// fatal — if we can't link them, future revoke flows can't reverse
	// the approval cleanly.
	if err := s.linkCreatedStudent(ctx, child.ID, student.ID); err != nil {
		return nil, fmt.Errorf("decision: link created student: %w", err)
	}

	// 7. Decide whether to schedule a guardian invitation. Skip when
	// the guardian already has a portal account (per the design Q
	// answer: "when they already have an account we do not need to
	// create a new one").
	if !guardian.HasAccount && guardian.Email != nil && strings.TrimSpace(*guardian.Email) != "" {
		s.logger.Debug("decision: scheduling guardian invitation",
			slog.Int64("guardian_profile_id", guardian.ID),
			slog.Bool("profile_was_new", profileWasNew),
		)
		return &PendingGuardianInvite{
			GuardianProfileID: guardian.ID,
			CreatedBy:         reviewedBy,
		}, nil
	}
	return nil, nil
}

// resolveGuardianProfile finds an existing tenant-scoped guardian by
// email or creates a new one. Phone numbers from the submission are
// NOT migrated into guardian_phone_numbers in slice 2 — that's a
// separate hop the admin guardian editor already supports if they want
// to enrich the profile later.
func (s *decisionService) resolveGuardianProfile(
	ctx context.Context,
	request *enrollmentModels.Request,
) (*users.GuardianProfile, bool, error) {
	email := strings.TrimSpace(strings.ToLower(request.GuardianEmail))

	if email != "" {
		existing, err := s.guardianProfileRepo.FindByEmail(ctx, email)
		if err == nil && existing != nil {
			return existing, false, nil
		}
		// errors.Is(sql.ErrNoRows) and "not found" both flow through;
		// we don't distinguish — if the lookup fails we still create.
	}

	// Build a fresh profile.
	first := strings.TrimSpace(request.GuardianFirstName)
	last := strings.TrimSpace(request.GuardianLastName)

	profile := &users.GuardianProfile{
		FirstName:              first,
		LastName:               last,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	if email != "" {
		emailCopy := email
		profile.Email = &emailCopy
	}
	if err := profile.Validate(); err != nil {
		return nil, false, fmt.Errorf("decision: validate guardian profile: %w", err)
	}
	if err := s.guardianProfileRepo.Create(ctx, profile); err != nil {
		return nil, false, fmt.Errorf("decision: create guardian profile: %w", err)
	}
	return profile, true, nil
}

// gradeToClass renders the optional grade level into the student's
// school_class field. The student schema uses free-form text; we land
// "1", "2", … when the grade is provided and "" otherwise. Admins can
// rename via the student profile UI later.
func (s *decisionService) gradeToClass(grade *int16) string {
	if grade == nil || *grade == 0 {
		return ""
	}
	return strconv.Itoa(int(*grade))
}

// materializeEnrollments writes one activities.student_enrollments row
// per RequestChildOffering whose CareOffering points at an activity
// group. Offerings without an activity_group_id are skipped (e.g.
// schedule-only offerings have no group to enroll into).
func (s *decisionService) materializeEnrollments(
	ctx context.Context,
	requestChildID, studentID int64,
	phase *enrollmentModels.Phase,
) error {
	if s.requestChildOfferingRepo == nil || s.careOfferingRepo == nil ||
		s.studentEnrollmentRepo == nil {
		// Wired without the offering repos: skip silently. Approvals
		// will still create the student record; the admin can attach
		// activity groups later via the activity admin UI.
		s.logger.Warn("decision: enrollment repos missing; skipping activity materialization",
			slog.Int64("request_child_id", requestChildID),
			slog.Int64("student_id", studentID))
		return nil
	}

	links, err := s.requestChildOfferingRepo.ListByRequestChildID(ctx, requestChildID)
	if err != nil {
		return fmt.Errorf("decision: list child offerings: %w", err)
	}

	validFrom := phase.ServiceStartDate
	validUntil := phase.ServiceEndDate

	for _, link := range links {
		offering, err := s.careOfferingRepo.FindByID(ctx, link.CareOfferingID)
		if err != nil || offering == nil {
			s.logger.Warn("decision: care offering missing for child link",
				slog.Int64("request_child_id", requestChildID),
				slog.Int64("care_offering_id", link.CareOfferingID))
			continue
		}
		if offering.ActivityGroupID == nil || *offering.ActivityGroupID == 0 {
			// Schedule-only offering — no activity group, nothing to enroll into.
			continue
		}
		row := &activities.StudentEnrollment{
			StudentID:       studentID,
			ActivityGroupID: *offering.ActivityGroupID,
			ValidFrom:       validFrom,
			ValidUntil:      &validUntil,
		}
		if err := row.Validate(); err != nil {
			return fmt.Errorf("decision: validate enrollment: %w", err)
		}
		if err := s.studentEnrollmentRepo.Create(ctx, row); err != nil {
			return fmt.Errorf("decision: create enrollment: %w", err)
		}
	}
	return nil
}

// linkCreatedStudent stamps request_children.created_student_id so the
// admin UI can link from a historical request back to the new student.
func (s *decisionService) linkCreatedStudent(ctx context.Context, requestChildID, studentID int64) error {
	return s.requestChildRepo.LinkCreatedStudent(ctx, requestChildID, studentID)
}

// enqueueDecisionEmail enqueues a parent decision email matching the
// new status. Only approved/waitlisted/rejected get emails; transitions
// to under_review are admin-internal.
func (s *decisionService) enqueueDecisionEmail(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	phase *enrollmentModels.Phase,
	status DecisionStatus,
	reason *string,
) {
	if s.outboxEnqueuer == nil {
		return
	}

	var kind string
	switch status {
	case DecisionApproved:
		kind = platformModels.EmailKindEnrollmentApproved
	case DecisionWaitlisted:
		kind = platformModels.EmailKindEnrollmentWaitlisted
	case DecisionRejected:
		kind = platformModels.EmailKindEnrollmentRejected
	default:
		// under_review (and any future intermediate status) is
		// admin-internal — parent stays on the existing status email.
		return
	}

	logoURL := s.frontendURL + "/images/moto_transparent.png"
	statusURL := fmt.Sprintf("%s/enroll/status/%s", s.frontendURL, request.StatusToken)

	payload := map[string]any{
		EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
		EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
		EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
		EnrollmentPayloadStatusURL:         statusURL,
		EnrollmentPayloadLogoURL:           logoURL,
		EnrollmentPayloadChildNames:        []string{child.FirstName + " " + child.LastName},
		EnrollmentPayloadRecipientEmail:    request.GuardianEmail,
		"phase_name":                       phase.Name,
	}
	if phase != nil && phase.ShowStatusReasonToParent && reason != nil && *reason != "" {
		payload["status_reason"] = *reason
	}

	if err := s.outboxEnqueuer.Enqueue(ctx, OutboxEnqueueRequest{
		Kind:              kind,
		Payload:           payload,
		RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
		RelatedEntityID:   request.ID,
	}); err != nil {
		s.logger.Error("decision: enqueue parent decision email failed",
			slog.Int64("request_id", request.ID),
			slog.Int64("child_id", child.ID),
			slog.String("kind", kind),
			slog.String("error", err.Error()))
	}
}

func (s *decisionService) findChildByID(ctx context.Context, requestID, childID int64) (*enrollmentModels.RequestChild, error) {
	children, err := s.requestChildRepo.ListByRequestID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	for _, c := range children {
		if c.ID == childID {
			return c, nil
		}
	}
	return nil, ErrDecisionChildNotFound
}

// attachExistingAccountIfPresent looks up the parent email in the
// global auth.accounts table (no tenant_id — emails are unique
// platform-wide). If a row exists, it ensures the new tenant is
// represented in account_tenants + account_roles, and links the
// per-tenant guardian profile to that account_id. Returns true when
// the attachment happened so the caller can skip enqueueing an
// invitation.
//
// Why this exists (slice-2 follow-up): without this step, an admin
// approving the same parent at a second school would queue another
// guardian-invitation email, and the accept flow's
// createOrFindAccount overwrites the existing password hash. This
// surfaces as "I just got accepted at school B and now my school A
// password no longer works." Linking directly here keeps the parent
// silent and on the same credentials.
func (s *decisionService) attachExistingAccountIfPresent(
	ctx context.Context,
	guardian *users.GuardianProfile,
) (bool, error) {
	if s.accountRepo == nil || s.accountTenantRepo == nil ||
		s.accountRoleRepo == nil || s.roleRepo == nil {
		// Auth repos not wired — fall back to the original invitation
		// flow. Test factories that don't bring up the auth side will
		// hit this path.
		return false, nil
	}
	if guardian.Email == nil || strings.TrimSpace(*guardian.Email) == "" {
		return false, nil
	}

	email := strings.TrimSpace(strings.ToLower(*guardian.Email))
	account, err := s.accountRepo.FindByEmail(ctx, email)
	if err != nil {
		// Not-found is the common case (parent has no portal account
		// yet) — treat it as "nothing to attach", let the invitation
		// flow run. We don't import the auth package's notfound
		// detection here; instead we rely on the FindByEmail wrapper
		// returning a typed DatabaseError on real failures. Logging
		// at debug level covers both branches.
		s.logger.Debug("decision: account lookup result",
			slog.String("email", email),
			slog.String("error", err.Error()),
		)
		return false, nil
	}
	if account == nil {
		return false, nil
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return false, fmt.Errorf("attach: tenant not in context")
	}

	// 1. account_tenants mapping. Create is idempotent (ON CONFLICT
	// DO NOTHING on (account_id, tenant_id)).
	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   account.ID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.accountTenantRepo.Create(ctx, mapping); err != nil {
		return false, fmt.Errorf("attach: account_tenants: %w", err)
	}

	// 2. Guardian role for this tenant. AccountRoleRepo.Create has no
	// ON CONFLICT, so check first via FindByAccountAndRole (which
	// honours tenant scope from context) and only create when missing.
	if err := s.ensureGuardianRoleForTenant(ctx, account.ID); err != nil {
		return false, err
	}

	// 3. Link the per-tenant guardian profile row to the global
	// account. LinkAccount also flips has_account=true so future
	// approvals for the same profile see the linked state.
	if err := s.guardianProfileRepo.LinkAccount(ctx, guardian.ID, account.ID); err != nil {
		return false, fmt.Errorf("attach: link profile: %w", err)
	}
	guardian.AccountID = &account.ID
	guardian.HasAccount = true

	return true, nil
}

// ensureGuardianRoleForTenant assigns the guardian base role for the
// current tenant, idempotently. Mirrors the linkProfileToAccount step
// in services/auth.guardianInvitationService so a parent linked here
// gets the same role footprint as one who came in via the invite
// accept flow.
func (s *decisionService) ensureGuardianRoleForTenant(ctx context.Context, accountID int64) error {
	role, err := s.roleRepo.FindByName(ctx, guardianRoleName)
	if err != nil {
		return fmt.Errorf("attach: guardian role lookup: %w", err)
	}
	if role == nil {
		return fmt.Errorf("attach: guardian role not found")
	}

	existing, err := s.accountRoleRepo.FindByAccountAndRole(ctx, accountID, role.ID)
	if err == nil && existing != nil {
		// Already assigned for this tenant (FindByAccountAndRole
		// honours tenant scope) — nothing to do.
		return nil
	}

	assignment := &authModels.AccountRole{
		AccountID: accountID,
		RoleID:    role.ID,
	}
	if err := s.accountRoleRepo.Create(ctx, assignment); err != nil {
		return fmt.Errorf("attach: create account_role: %w", err)
	}
	return nil
}
