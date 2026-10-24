package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

func (s *Service) answerableAnnouncement(adminCtx context.Context, announcementID, accountID, studentID int64) (*ParentAnnouncement, error) {
	a, err := s.AnnouncementRepo.FindByID(adminCtx, announcementID)
	if err != nil {
		return nil, fmt.Errorf("parent: load announcement: %w", err)
	}
	if a == nil || !announcementIsLive(a) {
		return nil, ErrAnnouncementNotFound
	}
	// Authorize the CHILD, not just the account: AccountMayAnswerForStudent
	// checks the guardian link and the poll's audience in one query, so a
	// guardian can neither answer for someone else's child nor for a child the
	// poll does not target.
	allowed, err := s.AnnouncementRepo.AccountMayAnswerForStudent(adminCtx, a.GetTenantID(), announcementID, accountID, studentID)
	if err != nil {
		return nil, fmt.Errorf("parent: check answer permission: %w", err)
	}
	if !allowed {
		// A guardian outside the audience entirely must not be able to tell
		// "wrong child" from "no such announcement", so both are 404 — but a
		// guardian who IS in the audience gets the more useful child error.
		matched, matchErr := s.AnnouncementRepo.AccountMatchesAnnouncement(adminCtx, a.GetTenantID(), announcementID, accountID)
		if matchErr != nil {
			return nil, fmt.Errorf("parent: match announcement audience: %w", matchErr)
		}
		if !matched {
			return nil, ErrAnnouncementNotFound
		}
		return nil, ErrChildNotAnswerable
	}

	return a, nil
}

func (s *Service) stampableAnnouncement(adminCtx context.Context, announcementID, accountID int64) (*ParentAnnouncement, error) {
	a, err := s.AnnouncementRepo.FindByID(adminCtx, announcementID)
	if err != nil {
		return nil, fmt.Errorf("parent: load announcement: %w", err)
	}
	if a == nil || !announcementIsLive(a) {
		return nil, ErrAnnouncementNotFound
	}
	// Authorize audience membership BEFORE any other 4xx: an unmatched caller
	// must always get 404 so a parent token cannot enumerate live announcement
	// IDs (or their ack flags) by distinguishing ack-not-required (400) from
	// not-found (404) responses across tenants.
	matched, err := s.AnnouncementRepo.AccountMatchesAnnouncement(adminCtx, a.GetTenantID(), announcementID, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: match announcement audience: %w", err)
	}
	if !matched {
		return nil, ErrAnnouncementNotFound
	}

	return a, nil
}

func (s *Service) writeAnnouncementStamp(adminCtx context.Context, announcementTenantID, announcementID, accountID int64, expectedPublishedAt time.Time, ack bool) error {
	if ack {
		applied, err := s.AnnouncementRepo.MarkAcknowledged(adminCtx, announcementTenantID, announcementID, accountID, expectedPublishedAt)
		if err != nil {
			return err
		}
		if !applied {
			return ErrAnnouncementStale
		}
		s.Logger.Info("parent acknowledged announcement",
			slog.Int64("account_id", accountID),
			slog.Int64("announcement_id", announcementID),
		)
		return nil
	}
	applied, err := s.AnnouncementRepo.MarkRead(adminCtx, announcementTenantID, announcementID, accountID, expectedPublishedAt)
	if err != nil {
		return err
	}
	if !applied {
		return ErrAnnouncementStale
	}
	return nil
}

func (s *Service) writeAnnouncementResponse(adminCtx context.Context, announcementTenantID, announcementID, accountID, studentID int64, optionIDs []int64, expectedPublishedAt time.Time) error {
	// Poll authorization above uses a snapshot. Lock and re-read the student
	// in this write transaction so a concurrent care exit cannot leave a new
	// response behind after the child became read-only.
	studentCtx := s.Runtime.WithTenantID(adminCtx, announcementTenantID)
	student, err := s.StudentRepo.FindByIDForUpdate(studentCtx, studentID)
	if err != nil {
		return err
	}
	if student.CareEndedOn(s.todayDate()) {
		return care.ErrChildCareEnded
	}
	applied, err := s.AnnouncementRepo.SetResponse(adminCtx, announcementTenantID, announcementID, studentID, accountID, optionIDs, expectedPublishedAt)
	if err != nil {
		return err
	}
	if !applied {
		return ErrAnnouncementStale
	}
	s.Logger.Info("parent answered announcement poll",
		slog.Int64("account_id", accountID),
		slog.Int64("announcement_id", announcementID),
		slog.Int64("student_id", studentID),
		slog.Int("option_count", len(optionIDs)),
	)
	return nil
}

func addEnrollmentAnnouncementTenants(requests []*care.EnrollmentRequestSummary, add func(int64)) {
	for _, req := range requests {
		if req == nil || req.WithdrawnAt != nil {
			continue
		}
		add(req.TenantID)
	}
}
