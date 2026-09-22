package presence

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// validateStudentExists checks if a student exists, returning appropriate errors
func (s *service) validateStudentExists(ctx context.Context, studentID int64) error {
	if _, err := s.StudentRepo.FindByID(ctx, studentID); err != nil {
		if base.IsNoRows(err) {
			return ErrStudentNotFound
		}
		return err
	}
	return nil
}

// ensureStudentCheckinAllowed validates the student exists and is not a
// graduated (alumnus) soft-deleted record, taking a FOR UPDATE row lock in the
// process. The lock is held for the caller's request transaction, so it
// serializes against a concurrent grade-transition apply that flips the same
// student to alumnus: the two transactions block on the shared row rather than
// interleaving into a stranded open attendance/visit the kiosk can no longer
// close (#405). Nil-safe for the partial unit-test services that never wire a
// StudentRepo; production always does.
func (s *service) ensureStudentCheckinAllowed(ctx context.Context, studentID int64) error {
	if s.StudentRepo == nil {
		return nil
	}
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		if base.IsNoRows(err) {
			return ErrStudentNotFound
		}
		return err
	}
	if student.IsAlumnus() {
		return ErrStudentGraduated
	}
	// A child whose care ended yesterday cannot start a new day here (#2487).
	// Checked against the enrollment interval rather than the lifecycle
	// status: the status only follows once the scheduler ticks, and the kiosk
	// must not let a departed child in during that window.
	if student.CareEndedOn(s.todayDate()) {
		return ErrStudentCareEnded
	}
	return nil
}

// validateActiveGroupExists checks if an active group exists, returning appropriate errors
func (s *service) validateActiveGroupExists(ctx context.Context, groupID int64) error {
	if _, err := s.GroupRepo.FindByID(ctx, groupID); err != nil {
		if base.IsNoRows(err) {
			return ErrActiveGroupNotFound
		}
		return err
	}
	return nil
}

// validateActiveGroupOpenForUpdate locks the target group and rejects ended
// sessions. Tenant HTTP requests and scheduler operations already run inside a
// tenant transaction, so the lock remains held through the subsequent visit
// or supervisor INSERT. This prevents a row selected before a concurrent
// absorption from attaching to the group after that absorption ends it.
func (s *service) validateActiveGroupOpenForUpdate(ctx context.Context, groupID int64) error {
	_, err := s.lockActiveGroupOpenForUpdate(ctx, groupID)
	return err
}

func (s *service) lockActiveGroupOpenForUpdate(ctx context.Context, groupID int64) (*ports.ActiveGroup, error) {
	group, err := s.GroupRepo.FindByIDForUpdate(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group == nil || group.EndTime != nil {
		return nil, ErrActiveGroupNotFound
	}
	return group, nil
}

// validateStaffExists locks a live staff member and maps missing-row errors.
func (s *service) validateStaffExists(ctx context.Context, staffID int64) error {
	found, err := s.StaffRepo.LockStaffExists(ctx, staffID)
	if base.IsNoRows(err) || (err == nil && !found) {
		return ErrStaffNotFound
	}
	return err
}

// lockStaffForSupervision serializes operational supervision writes with
// Membership retirement. A foreign key alone cannot reject a staff tombstone.
func (s *service) lockStaffForSupervision(ctx context.Context, staffID int64) error {
	// Supervision may auto-open a work session later in this transaction.
	// Take its balance lock before the staff row, matching offboarding.
	if tenant.FromContext(ctx) <= 0 || staffID <= 0 {
		return ErrStaffNotFound
	}
	if err := tenant.AcquireLock(ctx, fmt.Sprintf("staff-balance:%d:%d", tenant.FromContext(ctx), staffID), false); err != nil {
		return err
	}
	return s.validateStaffExists(ctx, staffID)
}

// extractContextIDs extracts device and staff IDs from context
func (s *service) extractContextIDs(ctx context.Context) (deviceID, staffID int64) {
	principal := s.attendancePrincipal(ctx)
	return principal.DeviceID, principal.StaffID
}
