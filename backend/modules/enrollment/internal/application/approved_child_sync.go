package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// ListOfferingAdjustments lists the audit trail of one child's offering
// adjustments for the admin request detail.
func (d *Decisions) ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*enrollment.OfferingAdjustmentRecord, error) {
	if d.deps.Adjustments == nil {
		return nil, fmt.Errorf("decision: offering adjustment repo not configured")
	}
	if requestID <= 0 || requestChildID <= 0 {
		return nil, fmt.Errorf("%w: request_id and child_id are required", careplan.ErrOfferingAdjustmentInvalid)
	}
	// The child must belong to the request in the path: a mismatched pair
	// answers not found instead of another request's trail.
	raw, err := d.deps.Children.ChildByID(ctx, requestChildID)
	if err == nil && raw != nil {
		_, err = childValue(raw)
	}
	if err != nil || raw == nil || raw.RequestID != requestID {
		return nil, careplan.ErrBookingChildNotFound
	}
	return d.deps.Adjustments.ListOfferingAdjustments(ctx, requestChildID)
}

// approvedChildSync is one run of SyncApprovedChildData.
type approvedChildSync struct {
	input    enrollment.ApprovedChildSync
	snapshot map[string]any
	request  *enrollmentModels.Request
	child    *RequestChild
	student  *Student
}

// SyncApprovedChildData carries the confirmed request and child data onto the
// student the approval created: the person's name and birthday, the class,
// the guardian links and the targeted form fields.
func (d *Decisions) SyncApprovedChildData(ctx context.Context, input enrollment.ApprovedChildSync) (*enrollment.RequestChild, error) {
	if input.RequestID <= 0 || input.ChildID <= 0 {
		return nil, fmt.Errorf("%w: request_id and child_id are required", careplan.ErrOfferingAdjustmentInvalid)
	}
	if d.deps.Requests == nil || d.deps.Children == nil || d.deps.People.Students == nil || d.deps.People.Persons == nil {
		return nil, fmt.Errorf("decision: approved child sync dependencies are not configured")
	}
	run := &approvedChildSync{input: input}
	if len(input.PreviousSnapshot) > 0 {
		if err := json.Unmarshal(input.PreviousSnapshot, &run.snapshot); err != nil {
			return nil, fmt.Errorf("decision: decode previous snapshot: %w", err)
		}
	}
	if err := d.loadApprovedChildForSync(ctx, run); err != nil {
		return nil, err
	}
	if err := d.syncApprovedChildStudent(ctx, run); err != nil {
		return nil, err
	}
	if err := d.syncApprovedChildGuardians(ctx, run); err != nil {
		return nil, err
	}
	child, err := d.deps.Children.ChildByID(ctx, run.child.ID)
	if err == nil {
		_, err = childValue(child)
	}
	if err != nil {
		return nil, err
	}
	return child, nil
}

func (d *Decisions) loadApprovedChildForSync(ctx context.Context, run *approvedChildSync) error {
	req, err := decodedRequestByID(ctx, d.deps.Requests, run.input.RequestID, false)
	if err != nil {
		return careplan.ErrBookingRequestNotFound
	}
	child, err := decodedChildByID(ctx, d.deps.Children, run.input.ChildID)
	if err != nil || child == nil || child.RequestID != req.ID {
		return careplan.ErrBookingChildNotFound
	}
	if child.Status != enrollmentModels.ChildStatusApproved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
		return fmt.Errorf("%w: only approved children with a linked student can be synced", careplan.ErrOfferingAdjustmentInvalid)
	}
	run.request, run.child = req, child
	return nil
}

// syncApprovedChildStudent takes the tenant-wide gates BEFORE the first
// student row lock, in the project-wide class-change order (shared
// class-writes gate first, recurrence gate second, row locks last) — the same
// order the direct school_class PUT takes. Locking the row first and the
// recurrence gate only later let this sync and a concurrent direct PUT on the
// same child wait on each other cyclically until PostgreSQL aborted one
// (#2147 review round 15). The gates are taken unconditionally because
// whether the confirmed edit actually changes the class is only known once
// the row is locked.
func (d *Decisions) syncApprovedChildStudent(ctx context.Context, run *approvedChildSync) error {
	if err := d.lockOfferingDerivedWrites(ctx); err != nil {
		return err
	}
	student, err := d.readEnrollmentStudent(ctx, *run.child.CreatedStudentID, "update")
	if (err != nil && d.deps.Runtime.NotFound(err)) || (err == nil && student == nil) {
		return enrollment.ErrDecisionStudentNotFound
	}
	if err != nil {
		return fmt.Errorf("decision: load approved child student: %w", err)
	}
	if err := d.syncApprovedChildPerson(ctx, student.PersonID, run.child); err != nil {
		return err
	}
	run.student = student
	return d.syncApprovedChildClass(ctx, run)
}

func (d *Decisions) syncApprovedChildPerson(ctx context.Context, personID int64, child *RequestChild) error {
	person, err := d.deps.People.Persons.PersonByID(ctx, personID)
	if err != nil || person == nil {
		return fmt.Errorf("decision: load approved child person: %w", err)
	}
	person.FirstName = child.FirstName
	person.LastName = child.LastName
	dob := child.DateOfBirth
	person.Birthday = &dob
	if err := d.deps.People.Persons.UpdatePerson(ctx, person); err != nil {
		return fmt.Errorf("decision: sync approved child person: %w", err)
	}
	return nil
}

// syncApprovedChildClass re-derives the school_class exactly like the
// rollover approval path: the concrete class carried by the edit wins;
// otherwise a bare grade placeholder tracks a grade change ("1" -> "2"), a
// concrete class whose grade still matches is kept ("3b" at grade 3), and a
// concrete class stranded on a now-stale grade ("2a" while the edit bumps to
// grade 3) falls back to the new bare grade rather than leaving the student
// in a mismatched class (#1833).
//
// A confirmed edit can move the child into another Jahrgang, exactly like a
// direct school_class edit or a grade transition, so the Jahrgang-filtered
// offering-sourced Regeltermine and their materialized future occurrences
// must follow in the same transaction (#2147 review round 13). The
// recurrence gate is already held.
func (d *Decisions) syncApprovedChildClass(ctx context.Context, run *approvedChildSync) error {
	student := run.student
	previousSchoolClass := student.SchoolClass
	student.SchoolClass = resolveRolloverSchoolClass(run.child, student.SchoolClass)
	if d.deps.StudentEnrollment == nil {
		return fmt.Errorf("decision: student enrollment capability is required")
	}
	if err := d.deps.StudentEnrollment.RenewEnrollmentStudent(ctx, student.ID, enrollmentStudentInput(student)); err != nil {
		return fmt.Errorf("decision: sync approved child student: %w", err)
	}
	if student.SchoolClass == previousSchoolClass {
		return nil
	}
	if err := d.resyncOfferingSourcedTemplates(ctx, d.todayDate()); err != nil {
		return fmt.Errorf("decision: resync sourced templates after class change: %w", err)
	}
	return nil
}

// syncApprovedChildGuardians reconciles the primary guardian and the
// co-guardians of a replacing edit and dispatches the targeted form fields.
func (d *Decisions) syncApprovedChildGuardians(ctx context.Context, run *approvedChildSync) error {
	replace := run.input.ReplaceTargetedData
	guardianRequest, guardian, err := d.approvedChildPrimaryGuardian(ctx, run)
	if err != nil {
		return err
	}
	keepGuardianProfileIDs := map[int64]bool{}
	if replace {
		if keepGuardianProfileIDs, err = d.reconcileApprovedChildGuardians(ctx, run.request, run.student.ID, run.input.PreviousRequestGuardians, run.input.ActorAccountID); err != nil {
			return err
		}
	}
	if err := d.syncApprovedChildGuardianFields(ctx, run, guardianRequest, guardian, keepGuardianProfileIDs); err != nil {
		return err
	}
	if replace {
		if _, err := d.reconcileApprovedChildGuardians(ctx, run.request, run.student.ID, run.input.PreviousRequestGuardians, run.input.ActorAccountID); err != nil {
			return err
		}
	}
	return nil
}

// approvedChildPrimaryGuardian resolves the request identity that may hold
// the primary guardian link and, on a replacing edit, reconciles that link.
// Without guardian profiles the request itself is the identity and no link
// is touched.
func (d *Decisions) approvedChildPrimaryGuardian(ctx context.Context, run *approvedChildSync) (*enrollmentModels.Request, *GuardianProfile, error) {
	if d.deps.People.GuardianProfiles == nil {
		return run.request, nil, nil
	}
	guardianRequest, err := d.guardianIdentityRequest(ctx, run.request)
	if err != nil {
		return nil, nil, err
	}
	if !run.input.ReplaceTargetedData {
		return guardianRequest, nil, nil
	}
	guardian, err := d.reconcilePrimaryGuardianLink(ctx, guardianRequest, run.student.ID, true, run.input.ActorAccountID)
	if err != nil {
		return nil, nil, err
	}
	return guardianRequest, guardian, nil
}

// syncApprovedChildGuardianFields dispatches the targeted form fields for
// the primary guardian, resolving the guardian when the edit did not
// reconcile the link. A guardian that cannot be resolved skips the dispatch.
func (d *Decisions) syncApprovedChildGuardianFields(ctx context.Context, run *approvedChildSync, guardianRequest *enrollmentModels.Request, guardian *GuardianProfile, keep map[int64]bool) error {
	if d.deps.People.GuardianProfiles == nil {
		return nil
	}
	if guardian == nil {
		if resolved, _, err := d.resolveGuardianProfile(ctx, guardianRequest); err == nil {
			guardian = resolved
		}
	}
	if guardian == nil {
		return nil
	}
	return d.syncApprovedChildTargetedFields(ctx, run, guardian, keep)
}

// syncApprovedChildTargetedFields dispatches the targeted form fields of the
// edit, audits the resulting student change and announces a trimmed
// departure plan.
func (d *Decisions) syncApprovedChildTargetedFields(ctx context.Context, run *approvedChildSync, guardian *GuardianProfile, keep map[int64]bool) error {
	student := run.student
	beforeTargetedSync := *student
	planSynced, terr := d.applyTargetedFields(ctx, run.request, run.child, student, guardian, run.input.ActorAccountID, targetedFieldSyncOptions{
		Replace:                run.input.ReplaceTargetedData,
		PreviousSnapshot:       run.snapshot,
		KeepGuardianProfileIDs: keep,
	})
	if err := d.checkTargetedSyncErrors(run, terr); err != nil {
		return err
	}
	if d.deps.StudentAudit != nil {
		afterTargetedSync := student
		if terr != nil {
			persistedStudent, reloadErr := d.readEnrollmentStudent(ctx, student.ID, "")
			if reloadErr != nil {
				return fmt.Errorf("decision: reload approved child after partial targeted-field sync: %w", reloadErr)
			}
			afterTargetedSync = persistedStudent
		}
		if err := d.deps.StudentAudit.RecordChangesForActor(ctx, &beforeTargetedSync, afterTargetedSync, run.input.ActorAccountID); err != nil {
			return fmt.Errorf("decision: audit approved child targeted-field sync: %w", err)
		}
	}
	// A departure-plan sync that actually TRIMMED a link changed rows on
	// ANOTHER child's card too. Announce it exactly like the student PUT,
	// care-request and master-data-review writers do, or open detail cards
	// and the Laufgemeinschaft search keep showing the pre-sync links. After
	// the refusal returns above, so a rolled-back sync stays silent.
	if planSynced {
		d.deferStudentPlanBroadcasts(ctx, student.ID)
	}
	return nil
}

// checkTargetedSyncErrors logs the dispatch errors of an approved child sync
// and fails the sync where they matter. The two companion sentinels are NOT
// best-effort field noise: they mean the departure-plan sync was REFUSED,
// either because it would strand a linked child without an allowed Heimweg
// or because another editor holds the linked child's row. Swallowing them
// outside the replacement path would report success for a correction that
// never landed; propagating them rolls the tenant transaction back and lets
// the handler answer with the actionable 400/409 the student PUT gives
// (#1694).
func (d *Decisions) checkTargetedSyncErrors(run *approvedChildSync, terr error) error {
	if terr == nil {
		return nil
	}
	d.logger().Warn("decision: approved child targeted-field sync had errors",
		slog.Int64("request_id", run.request.ID),
		slog.Int64("child_id", run.child.ID),
		slog.String("error", terr.Error()),
	)
	if run.input.ReplaceTargetedData {
		return fmt.Errorf("decision: approved child targeted-field replacement sync: %w", terr)
	}
	if d.isCompanionRefusal(terr) {
		return fmt.Errorf("decision: approved child departure sync refused: %w", terr)
	}
	return nil
}

// deferStudentPlanBroadcasts announces, after the surrounding tenant
// transaction commits, that an enrollment sync replaced a child's departure
// plan. Two events, because they invalidate different caches — exactly like
// the master-data-review and care-request emitters:
//
//   - student_updated: the plan itself lives on the student record, so the
//     student detail and database caches that feed an OPEN staff editor must
//     refetch. Without it the editor keeps the pre-sync plan and resubmits it
//     whole on the next unrelated save (an address edit), reverting the
//     approved change.
//   - student_companions_changed: a narrowed plan makes the owner trim
//     "läuft mit" links, which are rows on ANOTHER child's card too — the
//     signal every mounted companion view refetches on.
//
// Fire-and-forget: a lost event costs a stale card, never data.
func (d *Decisions) deferStudentPlanBroadcasts(ctx context.Context, studentID int64) {
	broadcasts := d.deps.Broadcasts
	if broadcasts == nil {
		return
	}
	tenantID := d.deps.Runtime.TenantID(ctx)
	d.deps.Runtime.AfterCommit(ctx, func() {
		if tenantID <= 0 {
			return
		}
		source := "enrollment_sync"
		if err := broadcasts.BroadcastStudentUpdated(tenantID, source); err != nil {
			d.logger().Warn("decision: failed to broadcast student update",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
		if err := broadcasts.BroadcastStudentCompanionsChanged(tenantID, source); err != nil {
			d.logger().Warn("decision: failed to broadcast student companions change",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
	})
}
