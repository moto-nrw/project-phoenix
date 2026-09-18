package legacy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	ChildConsentStateGranted     = usersModels.StudentConsentStateGranted
	ChildConsentStateWithdrawn   = usersModels.StudentConsentStateWithdrawn
	ChildConsentStateNotRecorded = usersModels.StudentConsentStateNotRecorded
)

type ChildConsent = usersModels.StudentConsentState

// StudentConsentService is the consent surface this portal needs: the shared
// projection People Directory folds together, and the Audit Platform trail
// every effective change appends to. The composition root binds both (#3349).
type StudentConsentService interface {
	CurrentStates(ctx context.Context, student *usersModels.Student, canManagePhoto bool) ([]ChildConsent, error)
	RecordTransitions(
		ctx context.Context,
		before, after *usersModels.Student,
		source string,
		actorAccountID *int64,
		changedAt time.Time,
	) error
}

type photoConsentAction string

const (
	photoConsentActionWithdraw photoConsentAction = "withdraw"
	photoConsentActionGrant    photoConsentAction = "grant"
)

// ErrPhotoConsentNotWithdrawn prevents the re-grant endpoint from becoming a
// separate first-time consent path. A new grant here requires an append-only
// withdrawal event from the existing consent history.
var ErrPhotoConsentNotWithdrawn = errors.New("parent: photo consent was not withdrawn")

func (s *service) GetChildConsents(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}
	if s.StudentRepo == nil {
		return nil, fmt.Errorf("parent: student repo not wired")
	}

	var consents []ChildConsent
	err = tenant.WithTenantTx(ctx, s.DB, child.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, loadErr := s.StudentRepo.FindByID(txCtx, studentID)
		if loadErr != nil {
			return loadErr
		}
		consents, loadErr = s.loadChildConsents(txCtx, student, child.HasPermission(authorize.GuardianPermissionConsentManage))
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
	return s.setPhotoConsent(ctx, accountID, studentID, photoConsentActionWithdraw)
}

// GrantPhotoConsent records a new voluntary photo consent after a withdrawal.
// It never restores a photo deleted by the earlier withdrawal.
func (s *service) GrantPhotoConsent(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error) {
	return s.setPhotoConsent(ctx, accountID, studentID, photoConsentActionGrant)
}

func (s *service) setPhotoConsent(ctx context.Context, accountID, studentID int64, action photoConsentAction) ([]ChildConsent, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionConsentManage)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if s.StudentRepo == nil || s.StudentGuardianRepo == nil || s.StudentConsents == nil {
		return nil, fmt.Errorf("parent: consent dependencies not wired")
	}

	var consents []ChildConsent
	err = tenant.WithTenantTx(ctx, s.DB, child.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		allowed, checkErr := s.StudentGuardianRepo.AccountHasStudentPermission(
			txCtx,
			accountID,
			studentID,
			child.TenantID,
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
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		granting := action == photoConsentActionGrant
		if granting && student.PhotoConsentGivenAt == nil {
			currentStates, stateErr := s.StudentConsents.CurrentStates(txCtx, student, true)
			if stateErr != nil {
				return stateErr
			}
			wasWithdrawn := false
			for _, consent := range currentStates {
				if consent.Key == auditModels.StudentConsentPhoto {
					wasWithdrawn = consent.State == usersModels.StudentConsentStateWithdrawn
					break
				}
			}
			if !wasWithdrawn {
				return ErrPhotoConsentNotWithdrawn
			}
		}
		stateChanged := (granting && student.PhotoConsentGivenAt == nil) || (!granting && student.PhotoConsentGivenAt != nil)
		if stateChanged {
			changedAt := s.now()
			before := *student
			storedURL := ""
			actorID := accountID
			if granting {
				student.PhotoConsentGivenAt = &changedAt
				student.PhotoConsentGivenBy = &actorID
			} else {
				if student.PhotoPath != nil {
					storedURL = *student.PhotoPath
				}
				student.PhotoPath = nil
				student.PhotoConsentGivenAt = nil
				student.PhotoConsentGivenBy = nil
			}
			if updateErr := s.StudentRepo.Update(txCtx, student); updateErr != nil {
				return updateErr
			}
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
			if !granting && storedURL != "" {
				var photos StudentPhotoUnlinker
				if s.StudentPhotos != nil {
					photos = s.StudentPhotos()
				}
				if photos == nil {
					return fmt.Errorf("parent: student photo service not wired")
				}
				photos.ScheduleUnlinkAfterCommit(txCtx, storedURL)
			}
			tenantID := child.TenantID
			tenant.RegisterAfterCommit(txCtx, func() {
				s.broadcastStudentUpdated(tenantID, studentID)
			})
		}

		consents, loadErr = s.loadChildConsents(txCtx, student, true)
		return loadErr
	})
	if err != nil {
		return nil, fmt.Errorf("parent: set photo consent: %w", err)
	}
	return consents, nil
}

func (s *service) loadChildConsents(ctx context.Context, student *usersModels.Student, canManage bool) ([]ChildConsent, error) {
	if s.StudentConsents == nil {
		return nil, fmt.Errorf("parent: student consent service not wired")
	}
	return s.StudentConsents.CurrentStates(ctx, student, canManage)
}
