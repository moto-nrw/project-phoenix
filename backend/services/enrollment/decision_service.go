package enrollment

import (
	"context"
	"encoding/json"
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
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	importsvc "github.com/moto-nrw/project-phoenix/services/import"
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
	// ErrDecisionInvalidData marks an approval that failed because the
	// parent-supplied request data (e.g. guardian phone) doesn't pass the
	// student/person validators. Mapped to 400, not 500 — submit/edit now
	// validate up front, so this is defense-in-depth for legacy rows.
	ErrDecisionInvalidData = errors.New("enrollment request data is invalid")
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

	// ListChildOfferings returns the request_child_offerings rows for
	// every child under requestID, joined to the offering's name +
	// description so the admin detail page can render labels without
	// a second per-offering fetch. Map key is request_child_id.
	ListChildOfferings(ctx context.Context, requestID int64) (map[int64][]ChildOfferingRow, error)
}

// ChildOfferingRow is one care-offering selection for a child, as
// surfaced by ListChildOfferings. SelectedDays mirrors the DB column
// — nil when the offering runs in admin-fixed mode, non-nil only when
// the parent picked specific days.
type ChildOfferingRow struct {
	OfferingID     int64
	OfferingName   string
	DaysOfWeekMode string
	SelectedDays   []string
	AvailableDays  []string
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
	FormSchemaRepo           enrollmentModels.FormSchemaRepository // needed to look up FormField.Target for each submitted answer
	SchoolRepo               platformModels.SchoolRepository
	PersonRepo               users.PersonRepository
	StudentRepo              users.StudentRepository
	StudentGuardianRepo      users.StudentGuardianRepository
	GuardianProfileRepo      users.GuardianProfileRepository
	GuardianPhoneRepo        users.GuardianPhoneNumberRepository             // target: guardian.phone_numbers / contact.phone_numbers
	PickupScheduleRepo       scheduleModels.StudentPickupScheduleRepository  // target: schedule.pickup
	ArrivalScheduleRepo      scheduleModels.StudentArrivalScheduleRepository // target: schedule.arrival
	StudentEnrollmentRepo    activities.StudentEnrollmentRepository
	AccountRepo              authModels.AccountRepository
	AccountTenantRepo        authModels.AccountTenantRepository
	AccountRoleRepo          authModels.AccountRoleRepository
	RoleRepo                 authModels.RoleRepository
	OutboxEnqueuer           OutboxEnqueuer
	FrontendURL              string // not used by parent-facing emails today; kept for future admin links
	ParentsURL               string // status link in approved/waitlisted/rejected emails. Falls back to FrontendURL when empty.
	Logger                   *slog.Logger
}

type decisionService struct {
	requestRepo              enrollmentModels.RequestRepository
	requestChildRepo         enrollmentModels.RequestChildRepository
	requestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	careOfferingRepo         enrollmentModels.CareOfferingRepository
	phaseRepo                enrollmentModels.PhaseRepository
	formSchemaRepo           enrollmentModels.FormSchemaRepository
	schoolRepo               platformModels.SchoolRepository
	personRepo               users.PersonRepository
	studentRepo              users.StudentRepository
	studentGuardianRepo      users.StudentGuardianRepository
	guardianProfileRepo      users.GuardianProfileRepository
	guardianPhoneRepo        users.GuardianPhoneNumberRepository
	pickupScheduleRepo       scheduleModels.StudentPickupScheduleRepository
	arrivalScheduleRepo      scheduleModels.StudentArrivalScheduleRepository
	studentEnrollmentRepo    activities.StudentEnrollmentRepository
	accountRepo              authModels.AccountRepository
	accountTenantRepo        authModels.AccountTenantRepository
	accountRoleRepo          authModels.AccountRoleRepository
	roleRepo                 authModels.RoleRepository
	outboxEnqueuer           OutboxEnqueuer
	frontendURL              string
	parentsURL               string
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
		formSchemaRepo:           cfg.FormSchemaRepo,
		schoolRepo:               cfg.SchoolRepo,
		personRepo:               cfg.PersonRepo,
		studentRepo:              cfg.StudentRepo,
		studentGuardianRepo:      cfg.StudentGuardianRepo,
		guardianProfileRepo:      cfg.GuardianProfileRepo,
		guardianPhoneRepo:        cfg.GuardianPhoneRepo,
		pickupScheduleRepo:       cfg.PickupScheduleRepo,
		arrivalScheduleRepo:      cfg.ArrivalScheduleRepo,
		studentEnrollmentRepo:    cfg.StudentEnrollmentRepo,
		accountRepo:              cfg.AccountRepo,
		accountTenantRepo:        cfg.AccountTenantRepo,
		accountRoleRepo:          cfg.AccountRoleRepo,
		roleRepo:                 cfg.RoleRepo,
		outboxEnqueuer:           cfg.OutboxEnqueuer,
		frontendURL:              cfg.FrontendURL,
		parentsURL: func() string {
			parents := strings.TrimRight(strings.TrimSpace(cfg.ParentsURL), "/")
			if parents != "" {
				return parents
			}
			return strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/")
		}(),
		logger: logger,
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
		// Phase may have been deleted under us - surface as "phase
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

// ListChildOfferings returns the offerings each child in this request
// picked. Per-child rows are keyed by request_child_id; offerings
// missing a parent_choice day picker land with SelectedDays == nil.
// Used by the admin detail endpoint to render the Betreuungsangebote
// next to each child for the decision UI.
func (s *decisionService) ListChildOfferings(ctx context.Context, requestID int64) (map[int64][]ChildOfferingRow, error) {
	if requestID <= 0 {
		return nil, fmt.Errorf("decision: request_id required")
	}
	children, err := s.requestChildRepo.ListByRequestID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("decision: list children for offerings: %w", err)
	}
	out := make(map[int64][]ChildOfferingRow, len(children))
	for _, child := range children {
		links, lerr := s.requestChildOfferingRepo.ListByRequestChildID(ctx, child.ID)
		if lerr != nil {
			return nil, fmt.Errorf("decision: list offerings for child %d: %w", child.ID, lerr)
		}
		rows := make([]ChildOfferingRow, 0, len(links))
		for _, link := range links {
			row := ChildOfferingRow{
				OfferingID:   link.CareOfferingID,
				SelectedDays: link.SelectedDays,
			}
			if s.careOfferingRepo != nil {
				if off, err := s.careOfferingRepo.FindByID(ctx, link.CareOfferingID); err == nil && off != nil {
					row.OfferingName = off.Name
					row.DaysOfWeekMode = off.DaysOfWeekMode
					row.AvailableDays = off.AvailableDays
				}
			}
			rows = append(rows, row)
		}
		out[child.ID] = rows
	}
	return out, nil
}

// Decide updates a single child's status. When status==approved the
// service also creates the downstream records (Person + Student +
// GuardianProfile + StudentGuardian + StudentEnrollment[s]) inside the
// same tenant tx the handler provides - failure of any one rolls the
// whole approval back. Parent decision emails are enqueued via the
// outbox in the same tx; guardian invitation creation is returned as a
// PendingGuardianInvite for the handler to fire post-commit.
//
// Idempotency: applying the same status twice is a no-op. Re-applying
// any new status to an already-terminal child (approved/rejected/
// withdrawn) returns ErrDecisionAlreadyTerminal - admins must use
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
	// so a hard failure WILL roll back - log+swallow keeps the
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
// implies. Runs inside the outer tenant tx the handler provides - every
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

	// Rollover branch (migration 1.15.62): when this request_child was
	// carried forward from a previous year's approved enrollment, we
	// already have a Person + Student row for this human. Update the
	// existing student (new school year, possibly bumped class) and
	// skip Person/Student creation entirely. Materialize the new
	// year's care offerings and link the new request_child to the
	// same student so the admin UI still navigates correctly.
	if child.RolloverSourceChildID != nil {
		return s.applyApprovalRollover(ctx, request, child, phase)
	}

	// 1. Resolve or create the guardian profile (per-tenant).
	guardian, profileWasNew, err := s.resolveGuardianProfile(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("decision: resolve guardian: %w", err)
	}

	// 1b. Cross-tenant account check. If the email already has a global
	// auth.accounts row (from another school's enrollment, an admin
	// invitation, etc.), attach the new tenant + this profile to it
	// directly. This bypasses the invitation flow entirely - the
	// invitation accept path overwrites the password hash, which is
	// the wrong UX when the parent already has a working password from
	// another school.
	//
	// PR 11/4: when the request carries guardian_account_id (parent
	// submitted while logged in), prefer the by-ID lookup over the
	// email lookup. A parent who edits their email in the form would
	// otherwise miss the attach step and trigger an invitation that
	// overwrites their existing password. The by-ID path is also
	// strictly cheaper - no platform-wide email index hit.
	if guardian.AccountID == nil {
		var (
			linked bool
			err    error
		)
		switch {
		case request.GuardianAccountID != nil && *request.GuardianAccountID > 0:
			linked, err = s.attachExistingAccountByID(ctx, guardian, *request.GuardianAccountID)
		case guardian.Email != nil && strings.TrimSpace(*guardian.Email) != "":
			linked, err = s.attachExistingAccountIfPresent(ctx, guardian)
		}
		if err != nil {
			return nil, fmt.Errorf("decision: attach existing account: %w", err)
		}
		if linked {
			s.logger.Info("decision: linked approval to existing global account",
				slog.Int64("guardian_profile_id", guardian.ID),
				slog.Int64("tenant_id", tenant.FromContext(ctx)),
				slog.Bool("profile_was_new", profileWasNew),
				slog.Bool("via_request_account_id", request.GuardianAccountID != nil),
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
		return nil, fmt.Errorf("decision: validate person: %w: %w", ErrDecisionInvalidData, err)
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
		return nil, fmt.Errorf("decision: validate student: %w: %w", ErrDecisionInvalidData, err)
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

	// 4b. Dispatch every targeted form field onto the right downstream
	// record. Scalar targets (health_info, extra_info, bus, photo_
	// consent, pickup_status) update the Student row in place;
	// structured targets (phone_list, weekday_schedule, contact_list)
	// create association rows. Failures inside one field don't abort
	// the approval — the targeted-field path is best-effort, the same
	// philosophy the invitation-email enqueue uses elsewhere in this
	// service.
	if err := s.applyTargetedFields(ctx, request, child, student, guardian, reviewedBy); err != nil {
		s.logger.Warn("decision: targeted-field dispatch had errors",
			slog.Int64("request_id", request.ID),
			slog.Int64("child_id", child.ID),
			slog.String("error", err.Error()),
		)
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
	// fatal - if we can't link them, future revoke flows can't reverse
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

// applyApprovalRollover is the abbreviated approval path for
// rolled-over enrollments. The student row already exists from last
// year's approval — we update its school_class + enrollment window,
// materialize the new year's care offerings, and link the new
// request_child to that same student.
//
// Falls back to the full applyApproval path when the source row
// doesn't have a created_student_id (defensive — the migration's
// unique index already prevents source-row reuse so this is rare).
func (s *decisionService) applyApprovalRollover(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	phase *enrollmentModels.Phase,
) (*PendingGuardianInvite, error) {
	source, err := s.requestChildRepo.FindByID(ctx, *child.RolloverSourceChildID)
	if err != nil || source == nil || source.CreatedStudentID == nil {
		s.logger.Warn("decision: rollover source has no created_student, falling back to fresh approval",
			slog.Int64("request_child_id", child.ID),
			slog.Any("source_id", child.RolloverSourceChildID),
		)
		// Falling back means we'd re-enter applyApproval, which would
		// loop back here because child.RolloverSourceChildID is still
		// set. To break the loop, clear it in-memory for this call
		// only — the DB row is unchanged, so the audit trail still
		// shows the row was a rollover.
		clone := *child
		clone.RolloverSourceChildID = nil
		// reviewedBy isn't tracked on this code path; falling back to
		// 0 keeps the audit row consistent (UpdateStatus already
		// handles 0 by skipping the column).
		return s.applyApproval(ctx, request, &clone, phase, 0)
	}

	studentID := *source.CreatedStudentID
	existing, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("decision: rollover load existing student %d: %w", studentID, err)
	}

	// Update school_class / enrollment window. Status is left at
	// whatever the scheduler had it on — typically 'active'. The
	// activate-students scheduler keeps the lifecycle in sync.
	existing.SchoolClass = s.gradeToClass(child.TargetGradeLevel)
	enrolledFrom := phase.ServiceStartDate
	enrolledUntil := phase.ServiceEndDate
	existing.EnrolledFrom = &enrolledFrom
	existing.EnrolledUntil = &enrolledUntil
	if err := s.studentRepo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("decision: rollover update student: %w", err)
	}

	// Materialize the new year's care offerings under this student.
	if err := s.materializeEnrollments(ctx, child.ID, studentID, phase); err != nil {
		return nil, err
	}

	// Link the new request_child to the same student so the admin UI
	// can navigate from either year's submission to one student row.
	if err := s.linkCreatedStudent(ctx, child.ID, studentID); err != nil {
		return nil, fmt.Errorf("decision: rollover link student: %w", err)
	}

	s.logger.Info("decision: rollover approval — updated existing student",
		slog.Int64("request_child_id", child.ID),
		slog.Int64("student_id", studentID),
	)

	// Skip guardian invitation logic — by definition a rolled-over
	// child's parent already had an enrollment last year, so they
	// either already have a portal account or they were already
	// offered one last year. No new invite here.
	return nil, nil
}

// resolveGuardianProfile finds an existing tenant-scoped guardian by
// email or creates a new one. Phone numbers from the submission are
// NOT migrated into guardian_phone_numbers in slice 2 - that's a
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
		// we don't distinguish - if the lookup fails we still create.
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
			// Schedule-only offering - no activity group, nothing to enroll into.
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
		// admin-internal - parent stays on the existing status email.
		return
	}

	schoolName, logoURL := emailBrandForSchool(ctx, s.schoolRepo, request.TenantID, s.parentsURL)
	footerLogoURL := motoLogoURL(s.parentsURL)
	statusURL := fmt.Sprintf("%s/enroll/status/%s", s.parentsURL, request.StatusToken)
	phaseName := ""
	if phase != nil {
		phaseName = phase.Name
	}

	payload := map[string]any{
		EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
		EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
		EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
		EnrollmentPayloadSchoolName:        schoolName,
		EnrollmentPayloadStatusURL:         statusURL,
		EnrollmentPayloadLogoURL:           logoURL,
		EnrollmentPayloadMotoLogoURL:       footerLogoURL,
		EnrollmentPayloadChildNames:        []string{child.FirstName + " " + child.LastName},
		EnrollmentPayloadRecipientEmail:    request.GuardianEmail,
		"phase_name":                       phaseName,
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
// global auth.accounts table (no tenant_id - emails are unique
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
		// Auth repos not wired - fall back to the original invitation
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
		// yet) - treat it as "nothing to attach", let the invitation
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

// attachExistingAccountByID links the guardian profile to the account
// identified by accountID directly, bypassing the email lookup that
// attachExistingAccountIfPresent uses. Called when the enrollment
// request was submitted by an authenticated parent (PR 11) - the
// JWT-derived account_id is more authoritative than the email field
// (which the parent could have typed differently in the form).
//
// Same downstream steps as the email-based path: account_tenants
// mapping + guardian role for the new tenant + LinkAccount on the
// per-tenant profile. Returns true on success so the caller skips the
// invitation enqueue.
func (s *decisionService) attachExistingAccountByID(
	ctx context.Context,
	guardian *users.GuardianProfile,
	accountID int64,
) (bool, error) {
	if s.accountRepo == nil || s.accountTenantRepo == nil ||
		s.accountRoleRepo == nil || s.roleRepo == nil {
		return false, nil
	}
	account, err := s.accountRepo.FindByID(ctx, accountID)
	if err != nil || account == nil {
		// Account was deleted between submission and decision - fall
		// back to email lookup so the approval still goes through.
		s.logger.Warn("decision: request guardian_account_id no longer resolvable, falling back to email",
			slog.Int64("guardian_account_id", accountID),
		)
		if guardian.Email != nil && strings.TrimSpace(*guardian.Email) != "" {
			return s.attachExistingAccountIfPresent(ctx, guardian)
		}
		return false, nil
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return false, fmt.Errorf("attach by id: tenant not in context")
	}

	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   account.ID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.accountTenantRepo.Create(ctx, mapping); err != nil {
		return false, fmt.Errorf("attach by id: account_tenants: %w", err)
	}
	if err := s.ensureGuardianRoleForTenant(ctx, account.ID); err != nil {
		return false, err
	}
	if err := s.guardianProfileRepo.LinkAccount(ctx, guardian.ID, account.ID); err != nil {
		return false, fmt.Errorf("attach by id: link profile: %w", err)
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
		// honours tenant scope) - nothing to do.
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

// applyTargetedFields walks the request's pinned schema and dispatches
// every field carrying a non-empty Target onto the appropriate
// downstream record. The student row may be mutated in place for
// scalar targets and persisted at the end via studentRepo.Update.
//
// Best-effort overall: per-field errors are collected and returned in
// one combined error string but never abort the approval. The student
// + per-child records have already been written by the caller.
func (s *decisionService) applyTargetedFields(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	student *users.Student,
	guardian *users.GuardianProfile,
	reviewedBy int64,
) error {
	if s.formSchemaRepo == nil || request.SchemaID == nil {
		return nil
	}
	schema, err := s.formSchemaRepo.FindByID(ctx, *request.SchemaID)
	if err != nil || schema == nil {
		return nil
	}

	var errs []string
	studentDirty := false

	for i := range schema.Fields {
		field := schema.Fields[i]
		if field.Target == "" {
			continue
		}
		raw := s.readFieldValue(request, child, &field)
		if raw == nil {
			continue
		}

		switch field.Target {
		case enrollmentModels.TargetStudentHealthInfo:
			if str := stringValue(raw); str != "" {
				student.HealthInfo = &str
				studentDirty = true
			}
		case enrollmentModels.TargetStudentExtraInfo:
			if str := stringValue(raw); str != "" {
				student.ExtraInfo = &str
				studentDirty = true
			}
		case enrollmentModels.TargetStudentBus:
			if b, ok := raw.(bool); ok {
				student.Bus = &b
				studentDirty = true
			}
		case enrollmentModels.TargetStudentPickupStatus:
			if str := stringValue(raw); str != "" {
				student.PickupStatus = &str
				studentDirty = true
			}
		case enrollmentModels.TargetSchedulePickup:
			if err := s.dispatchWeekdaySchedule(ctx, raw, student.ID, reviewedBy, true); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", field.Target, err))
			}
		case enrollmentModels.TargetScheduleArrival:
			if err := s.dispatchWeekdaySchedule(ctx, raw, student.ID, reviewedBy, false); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", field.Target, err))
			}
		case enrollmentModels.TargetStudentContacts:
			if err := s.dispatchContactList(ctx, raw, student.ID); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", field.Target, err))
			}
		}
	}

	// Auto-flow from core base-form fields. These don't appear in the
	// Stammdaten-target picker (photo consent + guardian_phone are
	// already collected by the public base form via consent_flags.photo
	// and the dedicated guardian_phone input), but their values still
	// need to land in the right downstream rows on approval.
	//
	// All four consent flags get copied onto the student row so staff
	// looking at a single child see the consent state without joining
	// back to enrollment.requests. AGB / Datenschutz / E-Mail are
	// required at submission and will always be true here; photo is
	// optional. Each stamp records the approval moment, not the parent
	// submission moment — the parent submission timestamp lives on
	// enrollment.requests if a more precise audit is ever needed.
	if request.ConsentFlags != nil {
		now := time.Now()
		if photo, ok := request.ConsentFlags["photo"].(bool); ok && photo {
			student.PhotoConsentGivenAt = &now
			if reviewedBy > 0 {
				rb := reviewedBy
				student.PhotoConsentGivenBy = &rb
			}
			studentDirty = true
		}
		if agb, ok := request.ConsentFlags["agb"].(bool); ok && agb {
			student.AGBAcceptedAt = &now
			studentDirty = true
		}
		if dp, ok := request.ConsentFlags["data_processing"].(bool); ok && dp {
			student.DataProcessingAcceptedAt = &now
			studentDirty = true
		}
		if email, ok := request.ConsentFlags["email_contact"].(bool); ok && email {
			student.EmailContactAcceptedAt = &now
			studentDirty = true
		}
	}
	if request.GuardianPhone != nil && s.guardianPhoneRepo != nil {
		phone := strings.TrimSpace(*request.GuardianPhone)
		if phone != "" {
			row := &users.GuardianPhoneNumber{
				GuardianProfileID: guardian.ID,
				PhoneNumber:       phone,
				PhoneType:         users.PhoneType("mobile"),
				IsPrimary:         true,
			}
			if err := s.guardianPhoneRepo.Create(ctx, row); err != nil {
				// Unique-violation = guardian already had this number
				// on file from a previous enrollment — benign.
				if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate") {
					errs = append(errs, fmt.Sprintf("auto guardian_phone: %v", err))
				}
			}
		}
	}

	if studentDirty {
		if err := s.studentRepo.Update(ctx, student); err != nil {
			errs = append(errs, fmt.Sprintf("update student: %v", err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// readFieldValue pulls the submission value for a field. Guardian-level
// fields live on request.CustomData; per-child fields on
// request_children.custom_data.
func (s *decisionService) readFieldValue(
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	field *enrollmentModels.FormField,
) any {
	if field.AppliesToCh {
		if child == nil || child.CustomData == nil {
			return nil
		}
		return child.CustomData[field.Key]
	}
	if request.CustomData == nil {
		return nil
	}
	return request.CustomData[field.Key]
}

// stringValue extracts a trimmed string from a raw any value. Returns
// "" for non-string or whitespace-only inputs.
func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// decodeStructured marshals raw → JSON → out so we can read interface{}
// values pulled out of a JSONB column into the typed structs declared
// in models/enrollment/form_schema.go without writing per-type
// destructuring code.
func decodeStructured(raw any, out any) error {
	bs, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, out)
}

// dispatchWeekdaySchedule inserts one pickup or arrival schedule row
// per non-empty weekday entry. isPickup=true targets pickup_schedules,
// false targets arrival_schedules.
func (s *decisionService) dispatchWeekdaySchedule(ctx context.Context, raw any, studentID int64, reviewedBy int64, isPickup bool) error {
	if (isPickup && s.pickupScheduleRepo == nil) || (!isPickup && s.arrivalScheduleRepo == nil) {
		return nil
	}
	var sched enrollmentModels.WeekdaySchedule
	if err := decodeStructured(raw, &sched); err != nil {
		return fmt.Errorf("decode weekday_schedule: %w", err)
	}
	if err := sched.Validate(); err != nil {
		return err
	}
	weekdayInt := map[string]int{
		"mon": scheduleModels.WeekdayMonday,
		"tue": scheduleModels.WeekdayTuesday,
		"wed": scheduleModels.WeekdayWednesday,
		"thu": scheduleModels.WeekdayThursday,
		"fri": scheduleModels.WeekdayFriday,
	}
	createdBy := reviewedBy
	if createdBy <= 0 {
		createdBy = 1 // fallback for legacy tests with no actor
	}
	for day, hhmm := range sched {
		hhmm = strings.TrimSpace(hhmm)
		if hhmm == "" {
			continue
		}
		t, err := time.Parse("15:04", hhmm)
		if err != nil {
			return fmt.Errorf("parse %s time %q: %w", day, hhmm, err)
		}
		if isPickup {
			row := &scheduleModels.StudentPickupSchedule{
				StudentID:  studentID,
				Weekday:    weekdayInt[day],
				PickupTime: t,
				CreatedBy:  createdBy,
			}
			if err := s.pickupScheduleRepo.UpsertSchedule(ctx, row); err != nil {
				return fmt.Errorf("upsert pickup %s: %w", day, err)
			}
		} else {
			row := &scheduleModels.StudentArrivalSchedule{
				StudentID:       studentID,
				Weekday:         weekdayInt[day],
				ExpectedArrival: t,
				CreatedBy:       createdBy,
			}
			if err := s.arrivalScheduleRepo.Create(ctx, row); err != nil {
				return fmt.Errorf("create arrival %s: %w", day, err)
			}
		}
	}
	return nil
}

// dispatchContactList creates one additional guardian_profile (or
// reuses an existing one matched by email) per submitted contact,
// links it to the student via users.students_guardians, and inserts
// any submitted phone numbers. Mirrors the dedup-by-email behaviour
// of the CSV importer at services/import/student_import_config.go.
func (s *decisionService) dispatchContactList(ctx context.Context, raw any, studentID int64) error {
	if s.guardianProfileRepo == nil || s.studentGuardianRepo == nil {
		return nil
	}
	var entries []enrollmentModels.ContactEntry
	if err := decodeStructured(raw, &entries); err != nil {
		return fmt.Errorf("decode contact_list: %w", err)
	}
	for i := range entries {
		c := entries[i]
		if err := c.Validate(); err != nil {
			return err
		}

		var profile *users.GuardianProfile
		emailLC := strings.ToLower(strings.TrimSpace(c.Email))
		if emailLC != "" {
			existing, _ := s.guardianProfileRepo.FindByEmail(ctx, emailLC)
			profile = existing
		}
		if profile == nil {
			profile = &users.GuardianProfile{
				FirstName:              c.FirstName,
				LastName:               c.LastName,
				PreferredContactMethod: "phone",
				LanguagePreference:     "de",
			}
			if emailLC != "" {
				profile.Email = &emailLC
			}
			if err := s.guardianProfileRepo.Create(ctx, profile); err != nil {
				return fmt.Errorf("create contact profile %s %s: %w", c.FirstName, c.LastName, err)
			}
		}

		// Phone numbers — append, dedup by unique index.
		if s.guardianPhoneRepo != nil {
			for j := range c.PhoneNumbers {
				p := c.PhoneNumbers[j]
				label := p.Label
				phone := &users.GuardianPhoneNumber{
					GuardianProfileID: profile.ID,
					PhoneNumber:       p.PhoneNumber,
					PhoneType:         users.PhoneType(p.PhoneType),
					IsPrimary:         p.IsPrimary,
				}
				if label != "" {
					phone.Label = &label
				}
				if err := s.guardianPhoneRepo.Create(ctx, phone); err != nil {
					if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate") {
						return fmt.Errorf("create contact phone: %w", err)
					}
				}
			}
		}

		// students_guardians link with the parent-submitted flags.
		// Relationship type goes through the same German→enum mapping
		// the CSV importer uses; unknown values land on "other".
		rel := &users.StudentGuardian{
			StudentID:          studentID,
			GuardianProfileID:  profile.ID,
			RelationshipType:   importsvc.MapRelationshipType(c.RelationshipType),
			IsPrimary:          false,
			IsEmergencyContact: c.IsEmergencyContact,
			CanPickup:          c.CanPickup,
		}
		if c.EmergencyPriority > 0 {
			rel.EmergencyPriority = c.EmergencyPriority
		}
		if err := s.studentGuardianRepo.Create(ctx, rel); err != nil {
			return fmt.Errorf("link contact to student: %w", err)
		}
	}
	return nil
}
