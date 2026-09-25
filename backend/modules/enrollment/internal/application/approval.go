package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// applyApproval creates the downstream records that an approval
// implies. Runs inside the outer tenant tx the handler provides.
//
// Returns a PendingGuardianInvite when the guardian needs an invitation
// (no existing portal account) so the handler can fire it post-commit.
func (d *Decisions) applyApproval(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *RequestChild,
	phase *enrollment.Phase,
	reviewedBy int64,
) (*enrollment.PendingGuardianInvite, error) {
	people := d.deps.People
	if people.Persons == nil || people.GuardianProfiles == nil || people.StudentGuardians == nil {
		return nil, fmt.Errorf("decision: approval requires user repos (person/student/guardian)")
	}
	if d.deps.GuardianAccess == nil {
		return nil, errDecisionGuardianAccessRequired
	}
	if d.deps.StudentEnrollment == nil {
		return nil, errors.New("decision: student enrollment capability is required")
	}

	// Rollover branch (migration 1.15.62): when this request_child was
	// carried forward from a previous year's approved enrollment, we
	// already have a Person + Student row for this human. Update the
	// existing student (new school year, possibly bumped class) and
	// skip Person/Student creation entirely.
	if child.RolloverSourceChildID != nil {
		return d.applyApprovalRollover(ctx, request, child, phase, reviewedBy)
	}

	// Existing-student re-enrollment branch (migration 1.15.221): an
	// existing_students phase matched this child to an already-enrolled
	// student at submission and pinned its id. Renew that student instead of
	// creating a duplicate Person + Student (#1663). A student deleted between
	// submission and approval nulls this reference via ON DELETE SET NULL, so
	// we only reach here with a live student and otherwise fall through to a
	// fresh create.
	if child.MatchedStudentID != nil {
		d.logger().Info("decision: existing-student re-enrollment — updating matched student",
			slog.Int64("request_child_id", child.ID),
			slog.Int64("student_id", *child.MatchedStudentID),
		)
		// syncTargetedFields=true: an existing_students submission is a full
		// parent form, so the submitted targeted fields must land on the
		// matched student — unlike the annual rollover, which carries no fresh
		// form (#1663).
		return d.attachApprovalToExistingStudent(ctx, request, child, phase, *child.MatchedStudentID, reviewedBy, true)
	}
	return d.approveNewStudent(ctx, request, child, phase, reviewedBy)
}

// approveNewStudent is the fresh-create approval: guardian profile, person,
// student, the primary and additional guardian links, the targeted form
// fields, the student link on the request child, the care-offering
// materialization and the activation plan.
func (d *Decisions) approveNewStudent(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *RequestChild,
	phase *enrollment.Phase,
	reviewedBy int64,
) (*enrollment.PendingGuardianInvite, error) {
	// For an accountless late-invite submission, the invite recipient remains
	// the access identity even when the submitted contact email was corrected.
	guardianRequest, err := d.guardianIdentityRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	guardian, profileWasNew, err := d.resolveGuardianProfile(ctx, guardianRequest)
	if err != nil {
		return nil, fmt.Errorf("decision: resolve guardian: %w", err)
	}
	if err := d.attachGuardianAccountIfPresent(ctx, guardianRequest, guardian, profileWasNew); err != nil {
		return nil, err
	}
	person, err := d.createChildPerson(ctx, child)
	if err != nil {
		return nil, err
	}
	activationPlan := d.approvalActivationPlan(ctx, phase)
	student, err := d.createApprovedStudent(ctx, child, phase, person.ID, activationPlan)
	if err != nil {
		return nil, err
	}
	if err := d.linkPrimaryGuardian(ctx, student.ID, guardian.ID); err != nil {
		return nil, err
	}
	// Co-guardians the parent added beyond the primary are mapped identically
	// except IsPrimary=false; contact-only, so no account attach/invitation.
	if err := d.linkAdditionalGuardians(ctx, request, student.ID); err != nil {
		return nil, fmt.Errorf("decision: link additional guardians: %w", err)
	}
	if err := d.applyFreshTargetedFields(ctx, request, child, student, guardian, reviewedBy); err != nil {
		return nil, err
	}
	if err := d.linkAdditionalGuardians(ctx, request, student.ID); err != nil {
		return nil, fmt.Errorf("decision: relink additional guardians after targeted fields: %w", err)
	}
	// Stamp the request_children row with the resulting student id so the
	// admin UI can link to the new student record. Stamped BEFORE the
	// enrollment materialization: the multi-source union resync resolves the
	// child's student through created_student_id, so a later stamp would hide
	// the just-approved child from the resync inside this same transaction.
	if err := d.deps.Children.LinkCreatedStudent(ctx, child.ID, student.ID); err != nil {
		return nil, fmt.Errorf("decision: link created student: %w", err)
	}
	if err := d.materializeApprovedOfferings(ctx, child.ID, student.ID, phase); err != nil {
		return nil, err
	}
	if err := d.stampActivationPlan(ctx, child.ID, activationPlan); err != nil {
		return nil, err
	}
	return d.pendingGuardianInvite(guardian, reviewedBy, profileWasNew), nil
}

// createChildPerson creates the person row of the child. DateOfBirth is
// required so a copy is fine.
func (d *Decisions) createChildPerson(ctx context.Context, child *RequestChild) (*Person, error) {
	dob := child.DateOfBirth
	person := &Person{FirstName: child.FirstName, LastName: child.LastName, Birthday: &dob}
	if err := d.deps.People.Rules.ValidatePerson(person); err != nil {
		return nil, fmt.Errorf("decision: validate person: %w: %w", enrollment.ErrDecisionInvalidData, err)
	}
	if err := d.deps.People.Persons.CreatePerson(ctx, person); err != nil {
		return nil, fmt.Errorf("decision: create person: %w", err)
	}
	return person, nil
}

// createApprovedStudent creates the student pinned to the phase's service
// window. The enrollment.default_activation_mode setting decides the
// initial status: "scheduled" (default) keeps it pending until the
// activate-students scheduler flips it on enrolled_from; "immediate" makes it
// active right away. enrolled_from stays the phase's ServiceStartDate in BOTH
// modes — it is the official start date and is no longer read once a student
// is active; only enrolled_until drives later deactivation.
func (d *Decisions) createApprovedStudent(ctx context.Context, child *RequestChild, phase *enrollment.Phase, personID int64, plan approvalActivationPlan) (*Student, error) {
	enrolledFrom := calendar.Date(phase.ServiceStartDate)
	enrolledUntil := calendar.Date(phase.ServiceEndDate)
	student := &Student{
		PersonID:      personID,
		SchoolClass:   resolveSchoolClass(child),
		Status:        plan.StudentStatus,
		EnrolledFrom:  &enrolledFrom,
		EnrolledUntil: &enrolledUntil,
	}
	if err := d.deps.People.Rules.ValidateStudent(student); err != nil {
		return nil, fmt.Errorf("decision: validate student: %w: %w", enrollment.ErrDecisionInvalidData, err)
	}
	created, err := d.deps.StudentEnrollment.CreateEnrollmentStudent(ctx, enrollmentStudentInput(student))
	if err != nil {
		return nil, fmt.Errorf("decision: create student: %w", err)
	}
	student.ID, student.CreatedAt, student.UpdatedAt = created.ID, created.CreatedAt, created.UpdatedAt
	student.TenantID = created.TenantID
	return student, nil
}

// linkPrimaryGuardian links student ↔ guardian as the primary relationship.
func (d *Decisions) linkPrimaryGuardian(ctx context.Context, studentID, guardianProfileID int64) error {
	rel := &StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  guardianProfileID,
		RelationshipType:   "guardian",
		IsPrimary:          true,
		IsEmergencyContact: true,
		CanPickup:          true,
	}
	d.deps.People.Rules.ApplyGuardianRole(rel, GuardianRolePrimary)
	if err := d.deps.People.Rules.ValidateStudentGuardian(rel); err != nil {
		return fmt.Errorf("decision: validate student_guardian: %w", err)
	}
	if err := d.deps.People.StudentGuardians.CreateGuardianLink(ctx, rel); err != nil {
		return fmt.Errorf("decision: create student_guardian: %w", err)
	}
	return nil
}

// applyFreshTargetedFields dispatches every targeted form field onto the new
// student. Ordinary field failures don't abort the approval. Consent-audit
// failures are different: committing a consent without its required history
// would break the append-only audit contract, so they abort and let the
// surrounding tenant transaction roll back. The plan-synced flag is
// deliberately dropped: this student row was created moments ago in this
// transaction, so no "läuft mit" link can exist yet and a companion broadcast
// would only wake every open editor once per mass approval.
func (d *Decisions) applyFreshTargetedFields(ctx context.Context, request *enrollmentModels.Request, child *RequestChild, student *Student, guardian *GuardianProfile, reviewedBy int64) error {
	_, err := d.applyTargetedFields(ctx, request, child, student, guardian, reviewedBy, targetedFieldSyncOptions{})
	if err == nil {
		return nil
	}
	if errors.Is(err, errStudentConsentAuditRequired) {
		return fmt.Errorf("decision: record consent history: %w", err)
	}
	d.logger().Warn("decision: targeted-field dispatch had errors",
		slog.Int64("request_id", request.ID),
		slog.Int64("child_id", child.ID),
		slog.String("error", err.Error()),
	)
	return nil
}

// materializeApprovedOfferings writes the roster rows of the child's
// bookings when the tenant uses care offerings.
func (d *Decisions) materializeApprovedOfferings(ctx context.Context, requestChildID, studentID int64, phase *enrollment.Phase) error {
	careOfferingsEnabled, err := d.careOfferingsEnabled(ctx)
	if err != nil {
		return fmt.Errorf("decision: resolve care offerings setting: %w", err)
	}
	if !careOfferingsEnabled {
		return nil
	}
	return d.materializeEnrollmentsForApproval(ctx, requestChildID, studentID, phase)
}

// attachGuardianAccountIfPresent runs the cross-tenant account check for a
// resolved guardian profile: if the email (or the submitting parent's
// JWT-derived account id) already has a global auth.accounts row, attach the
// new tenant + this profile to it directly. This bypasses the invitation flow
// entirely — the invitation accept path overwrites the password hash, which is
// the wrong UX when the parent already has a working password from another
// school.
//
// The by-ID lookup wins when the request carries guardian_account_id (parent
// submitted while logged in): a parent who edits their email in the form would
// otherwise miss the attach step and trigger an invitation that overwrites
// their existing password. It is also strictly cheaper — no platform-wide
// email index hit.
//
// Shared by the fresh-create approval and the existing-student re-enrollment
// approval; profileWasNew is logging context only.
func (d *Decisions) attachGuardianAccountIfPresent(
	ctx context.Context,
	request *enrollmentModels.Request,
	guardian *GuardianProfile,
	profileWasNew bool,
) error {
	if guardian == nil {
		return nil
	}
	// A profile that already carries an account_id needs no attach — but the
	// link alone does NOT prove the account can reach this school. account_id
	// survives an offboarding that flipped auth.account_tenants to inactive,
	// and pendingGuardianInvite deliberately sends nothing for a linked
	// profile, so no other step would repair the mapping. Approving is the
	// administrative act that grants access, so re-assert it here.
	if guardian.AccountID != nil {
		return d.ensureGuardianTenantAccess(ctx, *guardian.AccountID, "ensure linked guardian access")
	}
	var (
		linked bool
		err    error
	)
	switch {
	case request.GuardianAccountID != nil && *request.GuardianAccountID > 0:
		linked, err = d.attachExistingAccountByID(ctx, guardian, *request.GuardianAccountID)
	case guardian.Email != nil && strings.TrimSpace(*guardian.Email) != "":
		linked, err = d.attachExistingAccountIfPresent(ctx, guardian)
	}
	if err != nil {
		return fmt.Errorf("decision: attach existing account: %w", err)
	}
	if linked {
		d.logger().Info("decision: linked approval to existing global account",
			slog.Int64("guardian_profile_id", guardian.ID),
			slog.Int64("tenant_id", d.deps.Runtime.TenantID(ctx)),
			slog.Bool("profile_was_new", profileWasNew),
			slog.Bool("via_request_account_id", request.GuardianAccountID != nil),
		)
	}
	return nil
}

// pendingGuardianInvite reports the post-commit invitation an approval owes the
// submitted primary guardian: none when they already hold a portal account (per
// the design Q answer: "when they already have an account we do not need to
// create a new one") or when there is no address to invite.
func (d *Decisions) pendingGuardianInvite(
	guardian *GuardianProfile,
	reviewedBy int64,
	profileWasNew bool,
) *enrollment.PendingGuardianInvite {
	if guardian == nil || guardian.HasAccount {
		return nil
	}
	if guardian.Email == nil || strings.TrimSpace(*guardian.Email) == "" {
		return nil
	}
	d.logger().Debug("decision: scheduling guardian invitation",
		slog.Int64("guardian_profile_id", guardian.ID),
		slog.Bool("profile_was_new", profileWasNew),
	)
	return &enrollment.PendingGuardianInvite{
		GuardianProfileID: guardian.ID,
		CreatedBy:         reviewedBy,
	}
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
func (d *Decisions) applyApprovalRollover(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *RequestChild,
	phase *enrollment.Phase,
	reviewedBy int64,
) (*enrollment.PendingGuardianInvite, error) {
	source, err := decodedChildByID(ctx, d.deps.Children, *child.RolloverSourceChildID)
	if err != nil || source == nil || source.CreatedStudentID == nil {
		d.logger().Warn("decision: rollover source has no created_student, falling back to fresh approval",
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
		return d.applyApproval(ctx, request, &clone, phase, reviewedBy)
	}

	d.logger().Info("decision: rollover approval — updating existing student",
		slog.Int64("request_child_id", child.ID),
		slog.Int64("student_id", *source.CreatedStudentID),
	)
	// syncTargetedFields=false: a rolled-over request_child is carried forward
	// from last year's approval without a fresh parent submission, so there are
	// no newly submitted targeted fields to dispatch. reviewedBy isn't tracked
	// on this path — pass 0.
	return d.attachApprovalToExistingStudent(ctx, request, child, phase, *source.CreatedStudentID, 0, false)
}
