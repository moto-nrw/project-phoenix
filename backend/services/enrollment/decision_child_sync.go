package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ListOfferingAdjustments lists the audit trail of one child's offering
// adjustments for the admin request detail.
func (s *decisionService) ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*auditModels.EnrollmentOfferingAdjustment, error) {
	if s.OfferingAdjustmentRepo == nil {
		return nil, fmt.Errorf("decision: offering adjustment repo not configured")
	}
	if requestID <= 0 || requestChildID <= 0 {
		return nil, fmt.Errorf("%w: request_id and child_id are required", ErrOfferingAdjustmentInvalid)
	}
	child, err := offeringChildByID(ctx, s.Children, requestChildID)
	if err != nil || child == nil || child.RequestID != requestID {
		return nil, ErrDecisionChildNotFound
	}
	return s.OfferingAdjustmentRepo.ListByRequestChildID(ctx, requestChildID)
}

func (s *decisionService) SyncApprovedChildData(ctx context.Context, input SyncApprovedChildDataInput) (*RequestChild, error) {
	if input.RequestID <= 0 || input.ChildID <= 0 {
		return nil, fmt.Errorf("%w: request_id and child_id are required", ErrOfferingAdjustmentInvalid)
	}
	if s.Requests == nil || s.Children == nil || s.StudentRepo == nil || s.PersonRepo == nil {
		return nil, fmt.Errorf("decision: approved child sync dependencies are not configured")
	}

	req, err := intakeRequestByID(ctx, s.Requests, input.RequestID, false)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	child, err := offeringChildByID(ctx, s.Children, input.ChildID)
	if err != nil || child == nil || child.RequestID != req.ID {
		return nil, ErrDecisionChildNotFound
	}
	if child.Status != enrollmentModels.ChildStatusApproved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
		return nil, fmt.Errorf("%w: only approved children with a linked student can be synced", ErrOfferingAdjustmentInvalid)
	}

	// Tenant-wide gates BEFORE the first student row lock, in the project-wide
	// class-change order (shared class-writes gate first, recurrence gate
	// second, row locks last) — the same order the direct school_class PUT
	// takes in acquirePreRowLockGates. Locking the row first and the
	// recurrence gate only later (in the class-change branch below) let this
	// sync and a concurrent direct PUT on the same child wait on each other
	// cyclically until PostgreSQL aborted one (#2147 review round 15). The
	// gates are taken unconditionally because whether the confirmed edit
	// actually changes the class is only known once the row is locked.
	if err := s.LockOfferingDerivedWrites(ctx); err != nil {
		return nil, err
	}

	student, err := s.readEnrollmentStudent(ctx, *child.CreatedStudentID, "update")
	if errors.Is(err, sql.ErrNoRows) || (err == nil && student == nil) {
		return nil, ErrDecisionStudentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("decision: load approved child student: %w", err)
	}
	person, err := s.PersonRepo.FindByID(ctx, student.PersonID)
	if err != nil || person == nil {
		return nil, fmt.Errorf("decision: load approved child person: %w", err)
	}

	person.FirstName = child.FirstName
	person.LastName = child.LastName
	dob := timezone.Date(child.DateOfBirth)
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
	previousSchoolClass := student.SchoolClass
	student.SchoolClass = s.resolveRolloverSchoolClass(child, student.SchoolClass)
	if s.StudentEnrollment == nil {
		return nil, fmt.Errorf("decision: student enrollment capability is required")
	}
	if err := s.StudentEnrollment.RenewEnrollmentStudent(ctx, student.ID, enrollmentStudentInput(student)); err != nil {
		return nil, fmt.Errorf("decision: sync approved child student: %w", err)
	}

	// A confirmed edit can move the child into another Jahrgang, exactly like
	// a direct school_class edit or a grade transition, so the Jahrgang-
	// filtered offering-sourced Regeltermine and their materialized future
	// occurrences must follow in the same transaction (#2147 review round 13).
	// The recurrence gate is already held — taken with the shared class-writes
	// gate before the first row lock above.
	if student.SchoolClass != previousSchoolClass {
		if err := s.ResyncOfferingSourcedTemplates(ctx, s.todayDate()); err != nil {
			return nil, fmt.Errorf("decision: resync sourced templates after class change: %w", err)
		}
	}

	guardianRequest := req
	if s.GuardianProfileRepo != nil {
		guardianRequest, err = s.guardianIdentityRequest(ctx, req)
		if err != nil {
			return nil, err
		}
	}

	var guardian *users.GuardianProfile
	if input.ReplaceTargetedData && s.GuardianProfileRepo != nil {
		guardian, err = s.reconcilePrimaryGuardianLink(ctx, guardianRequest, student.ID, true, input.ActorAccountID)
		if err != nil {
			return nil, err
		}
	}

	keepGuardianProfileIDs := map[int64]bool{}
	if input.ReplaceTargetedData {
		var relinkErr error
		keepGuardianProfileIDs, relinkErr = s.reconcileApprovedChildGuardians(ctx, req, student.ID, input.PreviousRequestGuardians, input.ActorAccountID)
		if relinkErr != nil {
			return nil, relinkErr
		}
	}

	if s.GuardianProfileRepo != nil {
		if guardian == nil {
			resolved, _, gerr := s.resolveGuardianProfile(ctx, guardianRequest)
			if gerr == nil {
				guardian = resolved
			}
		}
		if guardian != nil {
			beforeTargetedSync := *student
			planSynced, terr := s.applyTargetedFields(ctx, req, child, student, guardian, input.ActorAccountID, targetedFieldSyncOptions{
				Replace:                input.ReplaceTargetedData,
				PreviousSnapshot:       input.PreviousSnapshot,
				KeepGuardianProfileIDs: keepGuardianProfileIDs,
			})
			if terr != nil {
				s.Logger.Warn("decision: approved child targeted-field sync had errors",
					slog.Int64("request_id", req.ID),
					slog.Int64("child_id", child.ID),
					slog.String("error", terr.Error()),
				)
				// The two companion sentinels are NOT best-effort field noise:
				// they mean the departure-plan sync was REFUSED, either because
				// it would strand a linked child without an allowed Heimweg or
				// because another editor holds the linked child's row. Swallowing
				// them outside the replacement path (as every other field error is
				// swallowed) would report success for a correction that never
				// landed. Propagate so the tenant transaction rolls back and the
				// handler answers with the actionable 400/409 the student PUT
				// gives (#1694).
				if input.ReplaceTargetedData {
					return nil, fmt.Errorf("decision: approved child targeted-field replacement sync: %w", terr)
				}
				if errors.Is(terr, users.ErrCompanionWouldLoseDeparture) ||
					errors.Is(terr, users.ErrCompanionLockBusy) {
					return nil, fmt.Errorf("decision: approved child departure sync refused: %w", terr)
				}
			}
			if s.StudentAudit != nil {
				afterTargetedSync := student
				if terr != nil {
					persistedStudent, reloadErr := s.readEnrollmentStudent(ctx, student.ID, "")
					if reloadErr != nil {
						return nil, fmt.Errorf("decision: reload approved child after partial targeted-field sync: %w", reloadErr)
					}
					afterTargetedSync = persistedStudent
				}
				if auditErr := s.StudentAudit.RecordChangesForActor(
					ctx,
					&beforeTargetedSync,
					afterTargetedSync,
					input.ActorAccountID,
				); auditErr != nil {
					return nil, fmt.Errorf("decision: audit approved child targeted-field sync: %w", auditErr)
				}
			}
			// A departure-plan sync that actually TRIMMED a link changed rows on
			// ANOTHER child's card too. Announce it exactly like the student PUT,
			// care-request and master-data-review writers do, or open detail cards
			// and the Laufgemeinschaft search keep showing the pre-sync links.
			// After the refusal returns above, so a rolled-back sync stays silent.
			if planSynced {
				s.deferStudentPlanBroadcasts(ctx, student.ID)
			}
		}
	}
	if input.ReplaceTargetedData {
		if _, relinkErr := s.reconcileApprovedChildGuardians(ctx, req, student.ID, input.PreviousRequestGuardians, input.ActorAccountID); relinkErr != nil {
			return nil, relinkErr
		}
	}

	return offeringChildByID(ctx, s.Children, child.ID)
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
//   - student_companions_changed: a narrowed plan makes the repository trim
//     "läuft mit" links, which are rows on ANOTHER child's card too — the
//     signal every mounted companion view refetches on.
//
// Fire-and-forget: a lost event costs a stale card, never data.
func (s *decisionService) deferStudentPlanBroadcasts(ctx context.Context, studentID int64) {
	if s.Broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if tenantID <= 0 {
			return
		}
		source := "enrollment_sync"
		studentEvent := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
		if err := s.Broadcaster.BroadcastToTenant(tenantID, studentEvent); err != nil {
			s.Logger.Warn("decision: failed to broadcast student update",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
		companionEvent := realtime.NewEvent(realtime.EventStudentCompanionsChanged, "", realtime.EventData{Source: &source})
		if err := s.Broadcaster.BroadcastToTenant(tenantID, companionEvent); err != nil {
			s.Logger.Warn("decision: failed to broadcast student companions change",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
	})
}
