package care

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	ChildConsentStateGranted     = "granted"
	ChildConsentStateWithdrawn   = "withdrawn"
	ChildConsentStateNotRecorded = "not_recorded"
)

type ChildConsent struct {
	Key         string
	State       string
	ChangedAt   *time.Time
	CanWithdraw bool
	CanGrant    bool
}

// StudentConsentService is the consent surface this portal needs: the shared
// projection People Directory folds together, and the Audit Platform trail
// every effective change appends to. The composition root binds both (#3349).
type StudentConsentService interface {
	CurrentStates(ctx context.Context, student *PortalStudent, canManagePhoto bool) ([]ChildConsent, error)
	RecordTransitions(
		ctx context.Context,
		before, after *PortalStudent,
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
	var consents []ChildConsent
	err := s.WithPermittedChild(ctx, accountID, studentID, GuardianPermissionPortalAccess, func(*Child) error {
		if s.StudentRepo == nil {
			return fmt.Errorf("parent: student repo not wired")
		}
		return nil
	}, func(txCtx context.Context, child *Child) error {
		student, loadErr := s.StudentRepo.FindByID(txCtx, studentID)
		if loadErr == nil {
			consents, loadErr = s.loadChildConsents(txCtx, student, child.HasPermission(GuardianPermissionConsentManage))
		}
		if loadErr == nil && student.CareEndedOn(s.todayDate()) {
			withoutPhotoGrant(consents)
		}
		if loadErr != nil {
			return fmt.Errorf("parent: get child consents: %w", loadErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return consents, nil
}

// WithdrawPhotoConsent atomically clears the voluntary photo consent and its
// stored image. Repeated calls are successful without adding duplicate audit
// entries. The end of care does not end the consent, so a guardian with the
// consent permission withdraws it after the care ended as well (#6494): the
// withdrawal must stay as easy as giving the consent (Art. 7 Abs. 3 DSGVO).
func (s *Service) WithdrawPhotoConsent(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error) {
	return s.setPhotoConsent(ctx, accountID, studentID, photoConsentActionWithdraw)
}

// GrantPhotoConsent records a new voluntary photo consent after a withdrawal.
// It never restores a photo deleted by the earlier withdrawal and is refused
// once the child's care ended.
func (s *Service) GrantPhotoConsent(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error) {
	return s.setPhotoConsent(ctx, accountID, studentID, photoConsentActionGrant)
}

func (s *Service) setPhotoConsent(ctx context.Context, accountID, studentID int64, action photoConsentAction) ([]ChildConsent, error) {
	var consents []ChildConsent
	err := s.WithPermittedChildForWrite(ctx, accountID, studentID, GuardianPermissionConsentManage, func(child *Child) error {
		if action == photoConsentActionGrant {
			if err := child.RequireCareRunning(); err != nil {
				return err
			}
		}
		if s.StudentRepo == nil || s.StudentConsents == nil || s.Students == nil {
			return fmt.Errorf("parent: consent dependencies not wired")
		}
		return nil
	}, func(txCtx context.Context, child *Child) error {
		var txErr error
		consents, txErr = s.changePhotoConsent(txCtx, child.TenantID, accountID, studentID, action)
		if txErr != nil {
			return fmt.Errorf("parent: set photo consent: %w", txErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return consents, nil
}

// changePhotoConsent grants or withdraws the voluntary photo consent under
// the student lock. A grant re-checks the care interval there; a withdrawal
// does not, it ends a consent the guardian gave (#6494). The relationship's
// consent permission is held by the guard of WithPermittedChildForWrite. A
// repeated call changes nothing and appends no audit entry.
func (s *Service) changePhotoConsent(ctx context.Context, tenantID, accountID, studentID int64, action photoConsentAction) ([]ChildConsent, error) {
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return nil, err
	}
	granting := action == photoConsentActionGrant
	careEnded := student.CareEndedOn(s.todayDate())
	if granting && careEnded {
		return nil, ErrChildCareEnded
	}
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
	consents, err := s.loadChildConsents(ctx, student, true)
	if err == nil && careEnded {
		withoutPhotoGrant(consents)
	}
	return consents, err
}

// withoutPhotoGrant drops the offer to give the photo consent again once the
// child's care ended; a standing consent stays withdrawable.
func withoutPhotoGrant(consents []ChildConsent) {
	for i := range consents {
		if consents[i].Key == StudentConsentPhoto {
			consents[i].CanGrant = false
		}
	}
}

// requirePhotoConsentWithdrawn keeps the re-grant path from becoming a
// first-time consent: it needs an earlier withdrawal in the consent history.
func (s *Service) requirePhotoConsentWithdrawn(ctx context.Context, student *PortalStudent) error {
	currentStates, err := s.StudentConsents.CurrentStates(ctx, student, true)
	if err != nil {
		return err
	}
	for _, consent := range currentStates {
		if consent.Key == StudentConsentPhoto {
			if consent.State == "withdrawn" {
				return nil
			}
			break
		}
	}
	return ErrPhotoConsentNotWithdrawn
}

// writePhotoConsent stores the new consent state through People Directory,
// appends the consent trail, and removes a withdrawn photo after the commit.
func (s *Service) writePhotoConsent(ctx context.Context, tenantID, accountID int64, student *PortalStudent, granting bool) error {
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
		if s.StudentPhotos == nil {
			return fmt.Errorf("parent: student photo service not wired")
		}
		if err := s.StudentPhotos.ScheduleUnlinkAfterCommit(ctx, storedURL); err != nil {
			return err
		}
	}
	s.Runtime.RegisterAfterCommit(ctx, func() {
		s.broadcastStudentUpdated(tenantID, student.ID)
	})
	return nil
}

func (s *Service) loadChildConsents(ctx context.Context, student *PortalStudent, canManage bool) ([]ChildConsent, error) {
	if s.StudentConsents == nil {
		return nil, fmt.Errorf("parent: student consent service not wired")
	}
	return s.StudentConsents.CurrentStates(ctx, student, canManage)
}
