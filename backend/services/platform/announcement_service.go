package platform

import (
	"context"
	"fmt"
	"net"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

// AnnouncementService handles platform announcements
type AnnouncementService interface {
	// CRUD operations (for operators)
	CreateAnnouncement(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error
	GetAnnouncement(ctx context.Context, id int64) (*platform.Announcement, error)
	UpdateAnnouncement(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error
	DeleteAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error

	// Listing (for operators - includes drafts)
	ListAnnouncements(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error)

	// Publishing
	PublishAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error
	UnpublishAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error

	// User-facing operations
	GetUnreadForUser(ctx context.Context, userID int64) ([]*platform.Announcement, error)
	MarkSeen(ctx context.Context, userID, announcementID int64) error
	MarkDismissed(ctx context.Context, userID, announcementID int64) error
}

type announcementService struct {
	announcementRepo     platform.AnnouncementRepository
	announcementViewRepo platform.AnnouncementViewRepository
	auditLogRepo         platform.OperatorAuditLogRepository
	db                   *bun.DB
}

// AnnouncementServiceConfig holds configuration for the announcement service
type AnnouncementServiceConfig struct {
	AnnouncementRepo     platform.AnnouncementRepository
	AnnouncementViewRepo platform.AnnouncementViewRepository
	AuditLogRepo         platform.OperatorAuditLogRepository
	DB                   *bun.DB
}

// NewAnnouncementService creates a new announcement service
func NewAnnouncementService(cfg AnnouncementServiceConfig) AnnouncementService {
	return &announcementService{
		announcementRepo:     cfg.AnnouncementRepo,
		announcementViewRepo: cfg.AnnouncementViewRepo,
		auditLogRepo:         cfg.AuditLogRepo,
		db:                   cfg.DB,
	}
}

// CreateAnnouncement creates a new announcement
func (s *announcementService) CreateAnnouncement(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
	if announcement == nil {
		return &InvalidDataError{Err: fmt.Errorf("announcement cannot be nil")}
	}

	announcement.CreatedBy = operatorID

	if err := announcement.Validate(); err != nil {
		return &InvalidDataError{Err: err}
	}

	if err := s.announcementRepo.Create(ctx, announcement); err != nil {
		return err
	}

	// Audit log
	s.logAction(ctx, operatorID, platform.ActionCreate, platform.ResourceAnnouncement, &announcement.ID, clientIP, nil)

	return nil
}

// GetAnnouncement retrieves an announcement by ID
func (s *announcementService) GetAnnouncement(ctx context.Context, id int64) (*platform.Announcement, error) {
	announcement, err := s.announcementRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if announcement == nil {
		return nil, &AnnouncementNotFoundError{AnnouncementID: id}
	}
	return announcement, nil
}

// UpdateAnnouncement updates an announcement
func (s *announcementService) UpdateAnnouncement(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
	if announcement == nil {
		return &InvalidDataError{Err: fmt.Errorf("announcement cannot be nil")}
	}

	existing, err := s.announcementRepo.FindByID(ctx, announcement.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return &AnnouncementNotFoundError{AnnouncementID: announcement.ID}
	}

	if err := announcement.Validate(); err != nil {
		return &InvalidDataError{Err: err}
	}

	if err := s.announcementRepo.Update(ctx, announcement); err != nil {
		return err
	}

	// Audit log
	changes := map[string]any{
		"title_changed":    existing.Title != announcement.Title,
		"content_changed":  existing.Content != announcement.Content,
		"type_changed":     existing.Type != announcement.Type,
		"severity_changed": existing.Severity != announcement.Severity,
	}
	s.logAction(ctx, operatorID, platform.ActionUpdate, platform.ResourceAnnouncement, &announcement.ID, clientIP, changes)

	return nil
}

// DeleteAnnouncement deletes an announcement
func (s *announcementService) DeleteAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	existing, err := s.announcementRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return &AnnouncementNotFoundError{AnnouncementID: id}
	}

	if err := s.announcementRepo.Delete(ctx, id); err != nil {
		return err
	}

	// Audit log
	s.logAction(ctx, operatorID, platform.ActionDelete, platform.ResourceAnnouncement, &id, clientIP, nil)

	return nil
}

// ListAnnouncements lists all announcements
func (s *announcementService) ListAnnouncements(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
	return s.announcementRepo.List(ctx, includeInactive)
}

// PublishAnnouncement publishes an announcement
func (s *announcementService) PublishAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	existing, err := s.announcementRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return &AnnouncementNotFoundError{AnnouncementID: id}
	}

	if err := s.announcementRepo.Publish(ctx, id); err != nil {
		return err
	}

	// Audit log
	s.logAction(ctx, operatorID, platform.ActionPublish, platform.ResourceAnnouncement, &id, clientIP, nil)

	return nil
}

// UnpublishAnnouncement unpublishes an announcement
func (s *announcementService) UnpublishAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	existing, err := s.announcementRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return &AnnouncementNotFoundError{AnnouncementID: id}
	}

	if err := s.announcementRepo.Unpublish(ctx, id); err != nil {
		return err
	}

	// Audit log
	changes := map[string]any{"action": "unpublish"}
	s.logAction(ctx, operatorID, platform.ActionUpdate, platform.ResourceAnnouncement, &id, clientIP, changes)

	return nil
}

// GetUnreadForUser retrieves unread announcements for a user
func (s *announcementService) GetUnreadForUser(ctx context.Context, userID int64) ([]*platform.Announcement, error) {
	return s.announcementViewRepo.GetUnreadForUser(ctx, userID)
}

// MarkSeen marks an announcement as seen by a user
func (s *announcementService) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	return s.announcementViewRepo.MarkSeen(ctx, userID, announcementID)
}

// MarkDismissed marks an announcement as dismissed by a user
func (s *announcementService) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	return s.announcementViewRepo.MarkDismissed(ctx, userID, announcementID)
}

// logAction logs an audit entry
func (s *announcementService) logAction(ctx context.Context, operatorID int64, action, resourceType string, resourceID *int64, clientIP net.IP, changes map[string]any) {
	entry := &platform.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		RequestIP:    clientIP,
	}

	if changes != nil {
		if err := entry.SetChanges(changes); err != nil {
			fmt.Printf("failed to set audit log changes: %v\n", err)
		}
	}

	if err := s.auditLogRepo.Create(ctx, entry); err != nil {
		fmt.Printf("failed to create audit log: %v\n", err)
	}
}
