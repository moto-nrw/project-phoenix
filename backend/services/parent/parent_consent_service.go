package parent

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	ChildConsentStateGranted     = usersService.StudentConsentStateGranted
	ChildConsentStateWithdrawn   = usersService.StudentConsentStateWithdrawn
	ChildConsentStateNotRecorded = usersService.StudentConsentStateNotRecorded
)

type ChildConsent = usersService.StudentConsentState

func (s *service) GetChildConsents(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}
	if s.StudentRepo == nil {
		return nil, fmt.Errorf("parent: student repo not wired")
	}

	var consents []ChildConsent
	err = tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, loadErr := s.StudentRepo.FindByID(txCtx, studentID)
		if loadErr != nil {
			return loadErr
		}
		consents, loadErr = s.loadChildConsents(txCtx, student, child.hasPermission(authorize.GuardianPermissionConsentManage))
		return loadErr
	})
	if err != nil {
		return nil, fmt.Errorf("parent: get child consents: %w", err)
	}

	return consents, nil
}

// WithdrawPhotoConsent atomically clears the voluntary photo consent and its
// stored image. Repeated calls are successful without adding duplicate audit
// entries.
func (s *service) WithdrawPhotoConsent(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionConsentManage)
	if err != nil {
		return nil, err
	}
	if s.StudentRepo == nil || s.StudentGuardianRepo == nil || s.StudentConsents == nil {
		return nil, fmt.Errorf("parent: consent dependencies not wired")
	}

	var consents []ChildConsent
	err = tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		allowed, checkErr := s.StudentGuardianRepo.AccountHasStudentPermission(
			txCtx,
			accountID,
			studentID,
			child.tenantID,
			authorize.GuardianPermissionConsentManage,
		)
		if checkErr != nil {
			return checkErr
		}
		if !allowed {
			return ErrGuardianPermissionDenied
		}

		student, loadErr := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if loadErr != nil {
			return loadErr
		}
		if student.PhotoConsentGivenAt != nil {
			changedAt := s.now()
			before := *student
			storedURL := ""
			if student.PhotoPath != nil {
				storedURL = *student.PhotoPath
			}
			student.PhotoPath = nil
			student.PhotoConsentGivenAt = nil
			student.PhotoConsentGivenBy = nil
			if updateErr := s.StudentRepo.Update(txCtx, student); updateErr != nil {
				return updateErr
			}
			actorID := accountID
			if auditErr := s.StudentConsents.RecordTransitions(
				txCtx,
				&before,
				student,
				auditModels.StudentConsentSourceParentPortal,
				&actorID,
				changedAt,
			); auditErr != nil {
				return auditErr
			}
			if storedURL != "" {
				if s.StudentPhotos == nil {
					return fmt.Errorf("parent: student photo service not wired")
				}
				s.StudentPhotos.ScheduleUnlinkAfterCommit(txCtx, storedURL)
			}
			tenantID := child.tenantID
			tenant.RegisterAfterCommit(txCtx, func() {
				s.broadcastStudentUpdated(tenantID, studentID)
			})
		}

		consents, loadErr = s.loadChildConsents(txCtx, student, true)
		return loadErr
	})
	if err != nil {
		return nil, fmt.Errorf("parent: withdraw photo consent: %w", err)
	}
	return consents, nil
}

func (s *service) loadChildConsents(ctx context.Context, student *usersModels.Student, canManage bool) ([]ChildConsent, error) {
	if s.StudentConsents == nil {
		return nil, fmt.Errorf("parent: student consent service not wired")
	}
	return s.StudentConsents.CurrentStates(ctx, student, canManage)
}
