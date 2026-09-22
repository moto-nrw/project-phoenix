package care

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
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

func (s *Service) GetChildConsents(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}
	if s.StudentRepo == nil {
		return nil, fmt.Errorf("parent: student repo not wired")
	}

	var consents []ChildConsent
	err = InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
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
func (s *Service) WithdrawPhotoConsent(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error) {
	return s.setPhotoConsent(ctx, accountID, studentID, photoConsentActionWithdraw)
}

// GrantPhotoConsent records a new voluntary photo consent after a withdrawal.
// It never restores a photo deleted by the earlier withdrawal.
func (s *Service) GrantPhotoConsent(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error) {
	return s.setPhotoConsent(ctx, accountID, studentID, photoConsentActionGrant)
}

func (s *Service) setPhotoConsent(ctx context.Context, accountID, studentID int64, action photoConsentAction) ([]ChildConsent, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionConsentManage)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if s.StudentRepo == nil || s.StudentGuardianRepo == nil || s.StudentConsents == nil || s.Students == nil {
		return nil, fmt.Errorf("parent: consent dependencies not wired")
	}

	var consents []ChildConsent
	err = InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		var txErr error
		consents, txErr = s.changePhotoConsent(txCtx, child.TenantID, accountID, studentID, action)
		return txErr
	})
	if err != nil {
		return nil, fmt.Errorf("parent: set photo consent: %w", err)
	}
	return consents, nil
}

// changePhotoConsent re-checks the relationship's consent permission and the
// care interval under the student lock, then grants or withdraws the voluntary
// photo consent. A repeated call changes nothing and appends no audit entry.
func (s *Service) changePhotoConsent(ctx context.Context, tenantID, accountID, studentID int64, action photoConsentAction) ([]ChildConsent, error) {
	allowed, err := s.StudentGuardianRepo.AccountHasStudentPermission(
		ctx,
		accountID,
		studentID,
		tenantID,
		authorize.GuardianPermissionConsentManage,
	)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrGuardianPermissionDenied
	}

	student, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if student.CareEndedOn(s.todayDate()) {
		return nil, ErrChildCareEnded
	}
	granting := action == photoConsentActionGrant
	if granting && student.PhotoConsentGivenAt == nil {
		if err := s.requirePhotoConsentWithdrawn(ctx, student); err != nil {
			return nil, err
		}
	}
	stateChanged := (granting && student.PhotoConsentGivenAt == nil) || (!granting && student.PhotoConsentGivenAt != nil)
	if stateChanged {
		if err := s.writePhotoConsent(ctx, tenantID, accountID, student, granting); err != nil {
			return nil, err
		}
	}
	return s.loadChildConsents(ctx, student, true)
}

// requirePhotoConsentWithdrawn keeps the re-grant path from becoming a
// first-time consent: it needs an earlier withdrawal in the consent history.
func (s *Service) requirePhotoConsentWithdrawn(ctx context.Context, student *usersModels.Student) error {
	currentStates, err := s.StudentConsents.CurrentStates(ctx, student, true)
	if err != nil {
		return err
	}
	for _, consent := range currentStates {
		if consent.Key == StudentConsentPhoto {
			if consent.State == usersModels.StudentConsentStateWithdrawn {
				return nil
			}
			break
		}
	}
	return ErrPhotoConsentNotWithdrawn
}

// writePhotoConsent stores the new consent state through People Directory,
// appends the consent trail, and removes a withdrawn photo after the commit.
func (s *Service) writePhotoConsent(ctx context.Context, tenantID, accountID int64, student *usersModels.Student, granting bool) error {
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
	if err := s.Students.SetStudentPhotoConsent(ctx, StudentPhotoState{
		StudentID: student.ID, PhotoPath: student.PhotoPath,
		PhotoConsentGivenAt: student.PhotoConsentGivenAt, PhotoConsentGivenBy: student.PhotoConsentGivenBy,
	}); err != nil {
		return err
	}
	if err := s.StudentConsents.RecordTransitions(
		ctx,
		&before,
		student,
		StudentConsentSourceParentPortal,
		&actorID,
		changedAt,
	); err != nil {
		return err
	}
	if !granting && storedURL != "" {
		var photos StudentPhotoUnlinker
		if s.StudentPhotos != nil {
			photos = s.StudentPhotos()
		}
		if photos == nil {
			return fmt.Errorf("parent: student photo service not wired")
		}
		photos.ScheduleUnlinkAfterCommit(ctx, storedURL)
	}
	tenant.RegisterAfterCommit(ctx, func() {
		s.broadcastStudentUpdated(tenantID, student.ID)
	})
	return nil
}

func (s *Service) loadChildConsents(ctx context.Context, student *usersModels.Student, canManage bool) ([]ChildConsent, error) {
	if s.StudentConsents == nil {
		return nil, fmt.Errorf("parent: student consent service not wired")
	}
	return s.StudentConsents.CurrentStates(ctx, student, canManage)
}
