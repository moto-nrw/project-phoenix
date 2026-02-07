package platform

import (
	"context"
	"fmt"
	"log/slog"
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
	GetUnreadForUser(ctx context.Context, userID int64, userRoles []string) ([]*platform.Announcement, error)
	CountUnread(ctx context.Context, userID int64, userRoles []string) (int, error)
	MarkSeen(ctx context.Context, userID, announcementID int64) error
	MarkDismissed(ctx context.Context, userID, announcementID int64) error

	// Statistics
	GetStats(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error)
	GetViewDetails(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error)
}

type announcementService struct {
	announcementRepo     platform.AnnouncementRepository
	announcementViewRepo platform.AnnouncementViewRepository
	auditLogRepo         platform.OperatorAuditLogRepository
	db                   *bun.DB
	logger               *slog.Logger
}

// AnnouncementServiceConfig holds configuration for the announcement service
type AnnouncementServiceConfig struct {
	AnnouncementRepo     platform.AnnouncementRepository
	AnnouncementViewRepo platform.AnnouncementViewRepository
	AuditLogRepo         platform.OperatorAuditLogRepository
	DB                   *bun.DB
	Logger               *slog.Logger
}

// NewAnnouncementService creates a new announcement service
func NewAnnouncementService(cfg AnnouncementServiceConfig) AnnouncementService {
	return &announcementService{
		announcementRepo:     cfg.AnnouncementRepo,
		announcementViewRepo: cfg.AnnouncementViewRepo,
		auditLogRepo:         cfg.AuditLogRepo,
		db:                   cfg.DB,
		logger:               cfg.Logger,
	}
}

func (s *announcementService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
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

// GetUnreadForUser retrieves unread announcements for a user filtered by roles
func (s *announcementService) GetUnreadForUser(ctx context.Context, userID int64, userRoles []string) ([]*platform.Announcement, error) {
	return s.announcementViewRepo.GetUnreadForUser(ctx, userID, userRoles)
}

// CountUnread counts unread announcements for a user filtered by roles
func (s *announcementService) CountUnread(ctx context.Context, userID int64, userRoles []string) (int, error) {
	return s.announcementViewRepo.CountUnread(ctx, userID, userRoles)
}

// GetStats retrieves view statistics for an announcement
func (s *announcementService) GetStats(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error) {
	// Verify announcement exists
	ann, err := s.announcementRepo.FindByID(ctx, announcementID)
	if err != nil {
		return nil, err
	}
	if ann == nil {
		return nil, &AnnouncementNotFoundError{AnnouncementID: announcementID}
	}
	return s.announcementViewRepo.GetStats(ctx, announcementID)
}

// MarkSeen marks an announcement as seen by a user
func (s *announcementService) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	return s.announcementViewRepo.MarkSeen(ctx, userID, announcementID)
}

// MarkDismissed marks an announcement as dismissed by a user
func (s *announcementService) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	return s.announcementViewRepo.MarkDismissed(ctx, userID, announcementID)
}

// GetViewDetails retrieves detailed view information for an announcement
func (s *announcementService) GetViewDetails(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error) {
	// Verify announcement exists
	ann, err := s.announcementRepo.FindByID(ctx, announcementID)
	if err != nil {
		return nil, err
	}
	if ann == nil {
		return nil, &AnnouncementNotFoundError{AnnouncementID: announcementID}
	}
	return s.announcementViewRepo.GetViewDetails(ctx, announcementID)
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
			s.getLogger().Error("failed to set audit log changes",
				"operator_id", operatorID,
				"action", action,
				"error", err,
			)
		}
	}

	if err := s.auditLogRepo.Create(ctx, entry); err != nil {
		s.getLogger().Error("failed to create audit log",
			"operator_id", operatorID,
			"action", action,
			"resource_type", resourceType,
			"error", err,
		)
	}
}
