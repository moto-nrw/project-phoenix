// Package communication is the public Communication capability for platform
// announcements and staff-facing conversations.
package communication

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	TypeAnnouncement = "announcement"
	TypeRelease      = "release"
	TypeMaintenance  = "maintenance"

	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"

	MaxTargetIDs  = 100
	maxTitleLen   = 200
	maxVersionLen = 50
)

var (
	ErrAnnouncementNotFound = errors.New("announcement not found")
	ErrInvalidAnnouncement  = errors.New("invalid announcement")
)

type AnnouncementNotFoundError struct{ AnnouncementID int64 }

func (e *AnnouncementNotFoundError) Error() string {
	return fmt.Sprintf("announcement with ID %d not found", e.AnnouncementID)
}
func (e *AnnouncementNotFoundError) Unwrap() error { return ErrAnnouncementNotFound }

type InvalidDataError struct{ Err error }

func (e *InvalidDataError) Error() string   { return fmt.Sprintf("invalid data: %v", e.Err) }
func (e *InvalidDataError) Unwrap() []error { return []error{ErrInvalidAnnouncement, e.Err} }

type Announcement struct {
	ID              int64
	Title           string
	Content         string
	Type            string
	Severity        string
	Version         *string
	Active          bool
	PublishedAt     *time.Time
	ExpiresAt       *time.Time
	TargetRoles     []string
	TargetOrgIDs    []int64
	TargetTenantIDs []int64
	CreatedBy       int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (a *Announcement) Validate() error {
	if a == nil {
		return errors.New("announcement cannot be nil")
	}
	a.Title = strings.TrimSpace(a.Title)
	a.Content = strings.TrimSpace(a.Content)
	if a.Title == "" {
		return errors.New("title is required")
	}
	if len(a.Title) > maxTitleLen {
		return fmt.Errorf("title must not exceed %d characters", maxTitleLen)
	}
	if a.Content == "" {
		return errors.New("content is required")
	}
	if !IsValidAnnouncementType(a.Type) {
		return errors.New("invalid announcement type")
	}
	if !IsValidSeverity(a.Severity) {
		return errors.New("invalid severity")
	}
	if a.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	if a.Version != nil && len(*a.Version) > maxVersionLen {
		return fmt.Errorf("version must not exceed %d characters", maxVersionLen)
	}
	if a.TargetRoles == nil {
		a.TargetRoles = []string{}
	}
	if a.TargetOrgIDs == nil {
		a.TargetOrgIDs = []int64{}
	}
	if a.TargetTenantIDs == nil {
		a.TargetTenantIDs = []int64{}
	}
	if len(a.TargetOrgIDs) > MaxTargetIDs {
		return fmt.Errorf("target_org_ids exceeds maximum of %d entries", MaxTargetIDs)
	}
	if len(a.TargetTenantIDs) > MaxTargetIDs {
		return fmt.Errorf("target_tenant_ids exceeds maximum of %d entries", MaxTargetIDs)
	}
	return nil
}

func IsValidAnnouncementType(value string) bool {
	switch value {
	case TypeAnnouncement, TypeRelease, TypeMaintenance:
		return true
	default:
		return false
	}
}

func IsValidSeverity(value string) bool {
	switch value {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	default:
		return false
	}
}

func (a *Announcement) IsDraft() bool { return a.PublishedAt == nil }

func IsAnnouncementExpired(a *Announcement, now time.Time) bool {
	return a != nil && a.ExpiresAt != nil && a.ExpiresAt.Before(now)
}

type AnnouncementStats struct {
	AnnouncementID int64 `json:"announcement_id"`
	TargetCount    int   `json:"target_count"`
	SeenCount      int   `json:"seen_count"`
	DismissedCount int   `json:"dismissed_count"`
}

type AnnouncementViewDetail struct {
	UserID       int64
	AccountEmail string
	UserName     string
	SeenAt       time.Time
	Dismissed    bool
}

type engine interface {
	CreateAnnouncement(context.Context, *Announcement, int64, net.IP) error
	GetAnnouncement(context.Context, int64) (*Announcement, error)
	UpdateAnnouncement(context.Context, *Announcement, int64, net.IP) error
	DeleteAnnouncement(context.Context, int64, int64, net.IP) error
	ListAnnouncements(context.Context, bool) ([]*Announcement, error)
	PublishAnnouncement(context.Context, int64, int64, net.IP) error
	UnpublishAnnouncement(context.Context, int64, int64, net.IP) error
	GetUnreadForUser(context.Context, int64, []string, int64, int64) ([]*Announcement, error)
	CountUnread(context.Context, int64, []string, int64, int64) (int, error)
	MarkSeen(context.Context, int64, int64) error
	MarkDismissed(context.Context, int64, int64) error
	GetStats(context.Context, int64) (*AnnouncementStats, error)
	GetViewDetails(context.Context, int64) ([]*AnnouncementViewDetail, error)
	ObserveRejection(string, time.Duration, error)
}

// Query exposes platform-announcement reads without write authority.
type Query interface {
	GetAnnouncement(context.Context, int64) (*Announcement, error)
	ListAnnouncements(context.Context, bool) ([]*Announcement, error)
	GetUnreadForUser(context.Context, int64, []string, int64, int64) ([]*Announcement, error)
	CountUnread(context.Context, int64, []string, int64, int64) (int, error)
	GetStats(context.Context, int64) (*AnnouncementStats, error)
	GetViewDetails(context.Context, int64) ([]*AnnouncementViewDetail, error)
}

// Command owns platform-announcement lifecycle and read-state writes.
type Command interface {
	CreateAnnouncement(context.Context, *Announcement, int64, net.IP) error
	UpdateAnnouncement(context.Context, *Announcement, int64, net.IP) error
	DeleteAnnouncement(context.Context, int64, int64, net.IP) error
	PublishAnnouncement(context.Context, int64, int64, net.IP) error
	UnpublishAnnouncement(context.Context, int64, int64, net.IP) error
	MarkSeen(context.Context, int64, int64) error
	MarkDismissed(context.Context, int64, int64) error
}

// Capability is the combined Communication platform-announcement facade.
type Capability interface {
	Query
	Command
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("communication: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) CreateAnnouncement(ctx context.Context, value *Announcement, operatorID int64, clientIP net.IP) error {
	started := time.Now()
	if value == nil {
		err := &InvalidDataError{Err: errors.New("announcement cannot be nil")}
		m.engine.ObserveRejection("create", time.Since(started), err)
		return err
	}
	value.CreatedBy = operatorID
	if err := value.Validate(); err != nil {
		wrapped := &InvalidDataError{Err: err}
		m.engine.ObserveRejection("create", time.Since(started), wrapped)
		return wrapped
	}
	return m.engine.CreateAnnouncement(ctx, value, operatorID, clientIP)
}

func (m *Module) GetAnnouncement(ctx context.Context, id int64) (*Announcement, error) {
	return m.engine.GetAnnouncement(ctx, id)
}

func (m *Module) UpdateAnnouncement(ctx context.Context, value *Announcement, operatorID int64, clientIP net.IP) error {
	started := time.Now()
	if value == nil {
		err := &InvalidDataError{Err: errors.New("announcement cannot be nil")}
		m.engine.ObserveRejection("update", time.Since(started), err)
		return err
	}
	if err := value.Validate(); err != nil {
		wrapped := &InvalidDataError{Err: err}
		m.engine.ObserveRejection("update", time.Since(started), wrapped)
		return wrapped
	}
	return m.engine.UpdateAnnouncement(ctx, value, operatorID, clientIP)
}

func (m *Module) DeleteAnnouncement(ctx context.Context, id, operatorID int64, clientIP net.IP) error {
	return m.engine.DeleteAnnouncement(ctx, id, operatorID, clientIP)
}
func (m *Module) ListAnnouncements(ctx context.Context, includeInactive bool) ([]*Announcement, error) {
	return m.engine.ListAnnouncements(ctx, includeInactive)
}
func (m *Module) PublishAnnouncement(ctx context.Context, id, operatorID int64, clientIP net.IP) error {
	return m.engine.PublishAnnouncement(ctx, id, operatorID, clientIP)
}
func (m *Module) UnpublishAnnouncement(ctx context.Context, id, operatorID int64, clientIP net.IP) error {
	return m.engine.UnpublishAnnouncement(ctx, id, operatorID, clientIP)
}
func (m *Module) GetUnreadForUser(ctx context.Context, userID int64, roles []string, tenantID, orgID int64) ([]*Announcement, error) {
	return m.engine.GetUnreadForUser(ctx, userID, roles, tenantID, orgID)
}
func (m *Module) CountUnread(ctx context.Context, userID int64, roles []string, tenantID, orgID int64) (int, error) {
	return m.engine.CountUnread(ctx, userID, roles, tenantID, orgID)
}
func (m *Module) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	return m.engine.MarkSeen(ctx, userID, announcementID)
}
func (m *Module) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	return m.engine.MarkDismissed(ctx, userID, announcementID)
}
func (m *Module) GetStats(ctx context.Context, announcementID int64) (*AnnouncementStats, error) {
	return m.engine.GetStats(ctx, announcementID)
}
func (m *Module) GetViewDetails(ctx context.Context, announcementID int64) ([]*AnnouncementViewDetail, error) {
	return m.engine.GetViewDetails(ctx, announcementID)
}

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrAnnouncementNotFound):
		return "announcement_not_found"
	case errors.Is(err, ErrInvalidAnnouncement):
		return "invalid_announcement"
	case errors.Is(err, ErrParentMessageThreadNotFound), errors.Is(err, ErrStaffMessageThreadNotFound):
		return "message_thread_not_found"
	case errors.Is(err, ErrParentMessagingForbidden), errors.Is(err, ErrStaffMessagingNotParticipant), errors.Is(err, ErrStaffMessagingNoActor):
		return "forbidden"
	case errors.Is(err, ErrParentMessagingDisabled), errors.Is(err, ErrStaffMessagingDisabled):
		return "messaging_disabled"
	case errors.Is(err, ErrParentMessageEmptyBody), errors.Is(err, ErrStaffMessageEmptyBody):
		return "empty_message"
	case errors.Is(err, ErrParentMessageBodyTooLong), errors.Is(err, ErrStaffMessageBodyTooLong):
		return "message_too_long"
	case errors.Is(err, ErrParentMessageHandledBoundaryRequired):
		return "handled_boundary_required"
	case errors.Is(err, ErrParentMessageInvalidGuardian):
		return "invalid_guardian"
	case errors.Is(err, ErrParentMessageGuardianAccessRevoked):
		return "guardian_access_revoked"
	case errors.Is(err, ErrStaffMessageRecipientNotAvailable):
		return "recipient_unavailable"
	case errors.Is(err, ErrStaffMessageCounterpartUnavailable):
		return "counterpart_unavailable"
	case errors.Is(err, ErrStaffMessageSelfConversation):
		return "self_conversation"
	case errors.Is(err, ErrStaffMessageRetentionUnresolved):
		return "retention_unresolved"
	default:
		return "internal_error"
	}
}

var _ Capability = (*Module)(nil)
