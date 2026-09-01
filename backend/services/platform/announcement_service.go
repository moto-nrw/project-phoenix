package platform

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// IsAnnouncementExpired reports whether the announcement has expired at the
// given instant: it has an ExpiresAt timestamp in the past. The clock is
// injected via `now` so the expiry decision stays testable.
func IsAnnouncementExpired(a *platform.Announcement, now time.Time) bool {
	return a.ExpiresAt != nil && a.ExpiresAt.Before(now)
}

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

	// User-facing operations (scoped to the current session tenant/org)
	GetUnreadForUser(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platform.Announcement, error)
	CountUnread(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) (int, error)
	MarkSeen(ctx context.Context, userID, announcementID int64) error
	MarkDismissed(ctx context.Context, userID, announcementID int64) error

	// Statistics
	GetStats(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error)
	GetViewDetails(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error)
}

type announcementService struct {
	AnnouncementServiceConfig
}

type OrganizationTargetQuery interface {
	CountOrganizationsByID(context.Context, []int64) (int, error)
}

// AnnouncementServiceConfig holds configuration for the announcement service
type AnnouncementServiceConfig struct {
	AnnouncementRepo     platform.AnnouncementRepository
	AnnouncementViewRepo platform.AnnouncementViewRepository
	AuditLogRepo         platform.OperatorAuditLogRepository
	Organizations        OrganizationTargetQuery
	SchoolRepo           platform.SchoolRepository
	DB                   *bun.DB
	Logger               *slog.Logger
}

// NewAnnouncementService creates a new announcement service
func NewAnnouncementService(cfg AnnouncementServiceConfig) AnnouncementService {
	return &announcementService{AnnouncementServiceConfig: cfg}
}

func (s *announcementService) getLogger() *slog.Logger {
	return cmp.Or(s.Logger, slog.Default())
}

// deduplicateInt64 returns a sorted copy of the slice with duplicates removed.
// The input slice is not modified.
func deduplicateInt64(ids []int64) []int64 {
	if len(ids) == 0 {
		return ids
	}
	cp := slices.Clone(ids)
	if len(cp) == 1 {
		return cp
	}
	slices.Sort(cp)
	return slices.Compact(cp)
}

// diffInt64 returns the IDs in `new` that are not present in `existing`.
// The inputs are not modified.
func diffInt64(newIDs, existingIDs []int64) []int64 {
	if len(newIDs) == 0 {
		return nil
	}
	if len(existingIDs) == 0 {
		return slices.Clone(newIDs)
	}
	keep := make(map[int64]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		keep[id] = struct{}{}
	}
	added := make([]int64, 0, len(newIDs))
	for _, id := range newIDs {
		if _, ok := keep[id]; !ok {
			added = append(added, id)
		}
	}
	return added
}

// validateTargetingIDs checks that all referenced org and tenant IDs exist in
// the database (and are not soft-deleted) using batch queries instead of N+1
// individual lookups. `CountByIDs` excludes soft-deleted rows, so passing a
// soft-deleted ID here will fail validation.
//
// Callers responsible for updates should only pass the NEWLY-ADDED IDs (diffed
// against the existing announcement) so that historical targets pointing at
// now-deleted orgs or schools can still round-trip through an unrelated edit.
// Use validateNewTargetingIDs for that.
func (s *announcementService) validateTargetingIDs(ctx context.Context, orgIDs, tenantIDs []int64) error {
	if len(orgIDs) > 0 {
		unique := deduplicateInt64(orgIDs)
		count, err := s.Organizations.CountOrganizationsByID(ctx, unique)
		if err != nil {
			if errors.Is(err, organizationtenancy.ErrInvalidOrganization) {
				return &InvalidDataError{Err: err}
			}
			return fmt.Errorf("failed to verify organizations: %w", err)
		}
		if count != len(unique) {
			return &InvalidDataError{Err: fmt.Errorf("one or more organization IDs do not exist or are deleted (requested %d, found %d)", len(unique), count)}
		}
	}
	if len(tenantIDs) > 0 {
		unique := deduplicateInt64(tenantIDs)
		count, err := s.SchoolRepo.CountByIDs(ctx, unique)
		if err != nil {
			return fmt.Errorf("failed to verify schools: %w", err)
		}
		if count != len(unique) {
			return &InvalidDataError{Err: fmt.Errorf("one or more school (tenant) IDs do not exist or are deleted (requested %d, found %d)", len(unique), count)}
		}
	}
	return nil
}

// validateNewTargetingIDs validates only the IDs that are new relative to the
// existing announcement. Historical IDs already on the record are allowed to
// persist even if their org/school has since been soft-deleted — the operator
// cannot add a new deleted target, but an unrelated edit will not blow up on
// targets that were legitimate when originally set.
func (s *announcementService) validateNewTargetingIDs(ctx context.Context, newOrgIDs, existingOrgIDs, newTenantIDs, existingTenantIDs []int64) error {
	addedOrgs := diffInt64(newOrgIDs, existingOrgIDs)
	addedTenants := diffInt64(newTenantIDs, existingTenantIDs)
	return s.validateTargetingIDs(ctx, addedOrgs, addedTenants)
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

	if err := s.validateTargetingIDs(ctx, announcement.TargetOrgIDs, announcement.TargetTenantIDs); err != nil {
		return err
	}

	if err := s.AnnouncementRepo.Create(ctx, announcement); err != nil {
		return err
	}

	// Audit log
	s.logAction(ctx, operatorID, platform.ActionCreate, platform.ResourceAnnouncement, &announcement.ID, clientIP, nil)

	return nil
}

// GetAnnouncement retrieves an announcement by ID
func (s *announcementService) GetAnnouncement(ctx context.Context, id int64) (*platform.Announcement, error) {
	announcement, err := s.AnnouncementRepo.FindByID(ctx, id)
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

	existing, err := s.AnnouncementRepo.FindByID(ctx, announcement.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return &AnnouncementNotFoundError{AnnouncementID: announcement.ID}
	}

	if err := announcement.Validate(); err != nil {
		return &InvalidDataError{Err: err}
	}

	if err := s.validateNewTargetingIDs(
		ctx,
		announcement.TargetOrgIDs, existing.TargetOrgIDs,
		announcement.TargetTenantIDs, existing.TargetTenantIDs,
	); err != nil {
		return err
	}

	if err := s.AnnouncementRepo.Update(ctx, announcement); err != nil {
		return err
	}

	// Audit log
	changes := map[string]any{
		"title_changed":             existing.Title != announcement.Title,
		"content_changed":           existing.Content != announcement.Content,
		"type_changed":              existing.Type != announcement.Type,
		"severity_changed":          existing.Severity != announcement.Severity,
		"target_org_ids_changed":    !slices.Equal(existing.TargetOrgIDs, announcement.TargetOrgIDs),
		"target_tenant_ids_changed": !slices.Equal(existing.TargetTenantIDs, announcement.TargetTenantIDs),
		"target_roles_changed":      !slices.Equal(existing.TargetRoles, announcement.TargetRoles),
	}
	s.logAction(ctx, operatorID, platform.ActionUpdate, platform.ResourceAnnouncement, &announcement.ID, clientIP, changes)

	return nil
}

// DeleteAnnouncement deletes an announcement
func (s *announcementService) DeleteAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	existing, err := s.AnnouncementRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return &AnnouncementNotFoundError{AnnouncementID: id}
	}

	if err := s.AnnouncementRepo.Delete(ctx, id); err != nil {
		return err
	}

	// Audit log
	s.logAction(ctx, operatorID, platform.ActionDelete, platform.ResourceAnnouncement, &id, clientIP, nil)

	return nil
}

// ListAnnouncements lists all announcements
func (s *announcementService) ListAnnouncements(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
	return s.AnnouncementRepo.List(ctx, includeInactive)
}

// PublishAnnouncement publishes an announcement
func (s *announcementService) PublishAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	existing, err := s.AnnouncementRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return &AnnouncementNotFoundError{AnnouncementID: id}
	}

	if err := s.AnnouncementRepo.Publish(ctx, id); err != nil {
		return err
	}

	// Audit log
	s.logAction(ctx, operatorID, platform.ActionPublish, platform.ResourceAnnouncement, &id, clientIP, nil)

	return nil
}

// UnpublishAnnouncement unpublishes an announcement
func (s *announcementService) UnpublishAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	existing, err := s.AnnouncementRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return &AnnouncementNotFoundError{AnnouncementID: id}
	}

	if err := s.AnnouncementRepo.Unpublish(ctx, id); err != nil {
		return err
	}

	// Audit log
	changes := map[string]any{"action": "unpublish"}
	s.logAction(ctx, operatorID, platform.ActionUpdate, platform.ResourceAnnouncement, &id, clientIP, changes)

	return nil
}

// GetUnreadForUser retrieves unread announcements for a user scoped to the current session tenant/org.
// Base-role expansion (custom role → system role matching) is handled at the SQL level
// via an EXISTS subquery in the repository, consistent with the GetStats query strategy.
func (s *announcementService) GetUnreadForUser(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platform.Announcement, error) {
	return s.AnnouncementViewRepo.GetUnreadForUser(ctx, userID, userRoles, tenantID, orgID)
}

// CountUnread counts unread announcements for a user scoped to the current session tenant/org
func (s *announcementService) CountUnread(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) (int, error) {
	return s.AnnouncementViewRepo.CountUnread(ctx, userID, userRoles, tenantID, orgID)
}

// GetStats retrieves view statistics for an announcement
func (s *announcementService) GetStats(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error) {
	// Verify announcement exists
	ann, err := s.AnnouncementRepo.FindByID(ctx, announcementID)
	if err != nil {
		return nil, err
	}
	if ann == nil {
		return nil, &AnnouncementNotFoundError{AnnouncementID: announcementID}
	}
	return s.AnnouncementViewRepo.GetStats(ctx, announcementID)
}

// MarkSeen marks an announcement as seen by a user
func (s *announcementService) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	return s.AnnouncementViewRepo.MarkSeen(ctx, userID, announcementID)
}

// MarkDismissed marks an announcement as dismissed by a user
func (s *announcementService) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	return s.AnnouncementViewRepo.MarkDismissed(ctx, userID, announcementID)
}

// GetViewDetails retrieves detailed view information for an announcement
func (s *announcementService) GetViewDetails(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error) {
	// Verify announcement exists
	ann, err := s.AnnouncementRepo.FindByID(ctx, announcementID)
	if err != nil {
		return nil, err
	}
	if ann == nil {
		return nil, &AnnouncementNotFoundError{AnnouncementID: announcementID}
	}
	return s.AnnouncementViewRepo.GetViewDetails(ctx, announcementID)
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

	if err := s.AuditLogRepo.Create(ctx, entry); err != nil {
		s.getLogger().Error("failed to create audit log",
			"operator_id", operatorID,
			"action", action,
			"resource_type", resourceType,
			"error", err,
		)
	}
}
