package application

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/ports"
)

const (
	actionCreate  = "create"
	actionUpdate  = "update"
	actionDelete  = "delete"
	actionPublish = "publish"
)

var errInvalidTarget = errors.New("communication: invalid announcement target")

// Service is the platform-announcement capability of Communication.
type Service struct {
	store    ports.Store
	audience ports.Audience
	targets  ports.Targets
	viewers  ports.ViewerNames
	audit    ports.Audit
	tx       ports.Transaction
	observe  ports.Observer
}

var _ communication.Capability = (*Service)(nil)

func New(store ports.Store, audience ports.Audience, targets ports.Targets, viewers ports.ViewerNames, audit ports.Audit, tx ports.Transaction, observe ports.Observer) *Service {
	if store == nil || audience == nil || targets == nil || viewers == nil || audit == nil || tx == nil || observe == nil {
		panic("communication application: all dependencies are required")
	}
	return &Service{store: store, audience: audience, targets: targets, viewers: viewers, audit: audit, tx: tx, observe: observe}
}

func (s *Service) CreateAnnouncement(ctx context.Context, value *communication.Announcement, operatorID int64, clientIP net.IP) error {
	started := time.Now()
	if value == nil {
		return s.reject("create", started, errors.New("announcement cannot be nil"))
	}
	// A stored announcement may have lost its author with the operator
	// account (#4047); a new one always names the operator creating it.
	if operatorID <= 0 {
		return s.reject("create", started, errors.New("created_by is required"))
	}
	value.CreatedBy = operatorID
	if err := value.Validate(); err != nil {
		return s.reject("create", started, err)
	}
	audit := auditEntry(operatorID, clientIP)
	var result communication.Announcement
	err := s.run(ctx, "create", func(txCtx context.Context) (stats domain.OperationStats, err error) {
		current, err := s.validateTargetingIDs(txCtx, value.TargetOrgIDs, value.TargetTenantIDs)
		stats.Add(current)
		if err != nil {
			return stats, err
		}
		result, current, err = s.store.Create(txCtx, *value)
		stats.Add(current)
		if err != nil {
			return stats, err
		}
		audit.Action = actionCreate
		audit.ResourceID = result.ID
		current, err = s.audit.Append(txCtx, audit)
		stats.Add(current)
		return stats, err
	})
	if err != nil {
		return err
	}
	*value = result
	return nil
}

func (s *Service) GetAnnouncement(ctx context.Context, id int64) (*communication.Announcement, error) {
	var result communication.Announcement
	err := s.run(ctx, "read", func(txCtx context.Context) (stats domain.OperationStats, err error) {
		result, stats, err = s.store.Get(txCtx, id)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) UpdateAnnouncement(ctx context.Context, value *communication.Announcement, operatorID int64, clientIP net.IP) error {
	started := time.Now()
	if value == nil {
		return s.reject("update", started, errors.New("announcement cannot be nil"))
	}
	if err := value.Validate(); err != nil {
		return s.reject("update", started, err)
	}
	audit := auditEntry(operatorID, clientIP)
	var result communication.Announcement
	err := s.run(ctx, "update", func(txCtx context.Context) (stats domain.OperationStats, err error) {
		existing, current, err := s.store.GetForMutation(txCtx, value.ID)
		stats.Add(current)
		if err != nil {
			return stats, err
		}
		current, err = s.validateNewTargetingIDs(txCtx, *value, existing)
		stats.Add(current)
		if err != nil {
			return stats, err
		}
		result, current, err = s.store.Update(txCtx, *value)
		stats.Add(current)
		if err != nil {
			return stats, err
		}
		audit.Action = actionUpdate
		audit.ResourceID = value.ID
		audit.Changes = map[string]any{
			"title_changed":             existing.Title != value.Title,
			"content_changed":           existing.Content != value.Content,
			"type_changed":              existing.Type != value.Type,
			"severity_changed":          existing.Severity != value.Severity,
			"target_org_ids_changed":    !slices.Equal(existing.TargetOrgIDs, value.TargetOrgIDs),
			"target_tenant_ids_changed": !slices.Equal(existing.TargetTenantIDs, value.TargetTenantIDs),
			"target_roles_changed":      !slices.Equal(existing.TargetRoles, value.TargetRoles),
		}
		current, err = s.audit.Append(txCtx, audit)
		stats.Add(current)
		return stats, err
	})
	if err != nil {
		return err
	}
	*value = result
	return nil
}

func (s *Service) DeleteAnnouncement(ctx context.Context, id, operatorID int64, clientIP net.IP) error {
	return s.mutateExisting(ctx, "delete", id, auditEntry(operatorID, clientIP), actionDelete, nil, s.store.Delete)
}

func (s *Service) PublishAnnouncement(ctx context.Context, id, operatorID int64, clientIP net.IP) error {
	return s.mutateExisting(ctx, "publish", id, auditEntry(operatorID, clientIP), actionPublish, nil, s.store.Publish)
}

func (s *Service) UnpublishAnnouncement(ctx context.Context, id, operatorID int64, clientIP net.IP) error {
	return s.mutateExisting(ctx, "unpublish", id, auditEntry(operatorID, clientIP), actionUpdate, map[string]any{"action": "unpublish"}, s.store.Unpublish)
}

func (s *Service) mutateExisting(
	ctx context.Context,
	operation string,
	id int64,
	audit domain.AuditEntry,
	action string,
	changes map[string]any,
	mutate func(context.Context, int64) (domain.OperationStats, error),
) error {
	return s.run(ctx, operation, func(txCtx context.Context) (stats domain.OperationStats, err error) {
		_, current, err := s.store.GetForMutation(txCtx, id)
		stats.Add(current)
		if err != nil {
			return stats, err
		}
		current, err = mutate(txCtx, id)
		stats.Add(current)
		if err != nil {
			return stats, err
		}
		audit.Action = action
		audit.ResourceID = id
		audit.Changes = changes
		current, err = s.audit.Append(txCtx, audit)
		stats.Add(current)
		return stats, err
	})
}

func (s *Service) ListAnnouncements(ctx context.Context, includeInactive bool) ([]*communication.Announcement, error) {
	var result []communication.Announcement
	err := s.run(ctx, "read_list", func(txCtx context.Context) (stats domain.OperationStats, err error) {
		result, stats, err = s.store.List(txCtx, includeInactive)
		return stats, err
	})
	return pointers(result), err
}

func (s *Service) GetUnreadForUser(ctx context.Context, userID int64, roles []string, tenantID, orgID int64) ([]*communication.Announcement, error) {
	var result []communication.Announcement
	err := s.run(ctx, "read_unread", func(txCtx context.Context) (stats domain.OperationStats, err error) {
		result, stats, err = s.audience.Unread(txCtx, userID, roles, tenantID, orgID)
		return stats, err
	})
	return pointers(result), err
}

func (s *Service) CountUnread(ctx context.Context, userID int64, roles []string, tenantID, orgID int64) (result int, err error) {
	err = s.run(ctx, "count_unread", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		result, stats, err = s.audience.CountUnread(txCtx, userID, roles, tenantID, orgID)
		return stats, err
	})
	return result, err
}

func (s *Service) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	return s.run(ctx, "mark_seen", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.store.MarkSeen(txCtx, userID, announcementID)
	})
}

func (s *Service) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	return s.run(ctx, "mark_dismissed", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.store.MarkDismissed(txCtx, userID, announcementID)
	})
}

func (s *Service) GetStats(ctx context.Context, id int64) (*communication.AnnouncementStats, error) {
	var result communication.AnnouncementStats
	err := s.run(ctx, "read_stats", func(txCtx context.Context) (stats domain.OperationStats, err error) {
		value, current, err := s.store.Get(txCtx, id)
		stats.Add(current)
		if err != nil {
			return stats, err
		}
		result, current, err = s.store.ViewStats(txCtx, id)
		stats.Add(current)
		if err != nil {
			return stats, err
		}
		result.TargetCount, current, err = s.audience.TargetCount(txCtx, value)
		stats.Add(current)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) GetViewDetails(ctx context.Context, id int64) ([]*communication.AnnouncementViewDetail, error) {
	var result []communication.AnnouncementViewDetail
	err := s.run(ctx, "read_view_details", func(txCtx context.Context) (stats domain.OperationStats, err error) {
		_, current, err := s.store.Get(txCtx, id)
		stats.Add(current)
		if err != nil {
			return stats, err
		}
		result, current, err = s.audience.ViewDetails(txCtx, id)
		stats.Add(current)
		if err != nil || len(result) == 0 {
			return stats, err
		}
		accountIDs := make([]int64, 0, len(result))
		for _, detail := range result {
			accountIDs = append(accountIDs, detail.UserID)
		}
		names, current, err := s.viewers.NamesByAccount(txCtx, accountIDs)
		stats.Add(current)
		if err != nil {
			return stats, fmt.Errorf("load announcement viewer names: %w", err)
		}
		for index := range result {
			result[index].UserName = result[index].AccountEmail
			if name := names[result[index].UserID]; name != "" {
				result[index].UserName = name
			}
		}
		return stats, nil
	})
	if err != nil {
		return nil, err
	}
	return pointers(result), nil
}

func (s *Service) validateTargetingIDs(ctx context.Context, orgIDs, schoolIDs []int64) (stats domain.OperationStats, err error) {
	if len(orgIDs) > 0 {
		unique := deduplicate(orgIDs)
		count, current, err := s.targets.CountOrganizationsByID(ctx, unique)
		stats.Add(current)
		if err != nil {
			return stats, invalidTarget(fmt.Errorf("failed to verify organizations: %w", err))
		}
		if count != len(unique) {
			return stats, invalidTarget(fmt.Errorf("%w: one or more organization IDs do not exist or are deleted (requested %d, found %d)", errInvalidTarget, len(unique), count))
		}
	}
	if len(schoolIDs) > 0 {
		unique := deduplicate(schoolIDs)
		count, current, err := s.targets.CountSchoolsByID(ctx, unique)
		stats.Add(current)
		if err != nil {
			return stats, invalidTarget(fmt.Errorf("failed to verify schools: %w", err))
		}
		if count != len(unique) {
			return stats, invalidTarget(fmt.Errorf("%w: one or more school (tenant) IDs do not exist or are deleted (requested %d, found %d)", errInvalidTarget, len(unique), count))
		}
	}
	return stats, nil
}

func (s *Service) validateNewTargetingIDs(ctx context.Context, value, existing communication.Announcement) (domain.OperationStats, error) {
	return s.validateTargetingIDs(ctx, difference(value.TargetOrgIDs, existing.TargetOrgIDs), difference(value.TargetTenantIDs, existing.TargetTenantIDs))
}

// invalidTarget reports a target the platform does not know as invalid
// announcement data; any other failure stays as it is.
func invalidTarget(err error) error {
	if errors.Is(err, errInvalidTarget) {
		return &communication.InvalidDataError{Err: err}
	}
	return err
}

// reject observes and returns input the operation refused before it ran.
func (s *Service) reject(operation string, started time.Time, err error) error {
	invalid := &communication.InvalidDataError{Err: err}
	s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Err: invalid})
	return invalid
}

func auditEntry(operatorID int64, clientIP net.IP) domain.AuditEntry {
	return domain.AuditEntry{OperatorID: operatorID, RequestIP: clientIP}
}

func pointers[T any](values []T) []*T {
	result := make([]*T, 0, len(values))
	for index := range values {
		result = append(result, &values[index])
	}
	return result
}

func deduplicate(ids []int64) []int64 {
	result := slices.Clone(ids)
	slices.Sort(result)
	return slices.Compact(result)
}

func difference(values, existing []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	known := make(map[int64]struct{}, len(existing))
	for _, id := range existing {
		known[id] = struct{}{}
	}
	result := make([]int64, 0, len(values))
	for _, id := range values {
		if _, ok := known[id]; !ok {
			result = append(result, id)
		}
	}
	return result
}

func (s *Service) run(ctx context.Context, operation string, fn func(context.Context) (domain.OperationStats, error)) (err error) {
	started := time.Now()
	var stats domain.OperationStats
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	execute := func(txCtx context.Context) error {
		current, runErr := fn(txCtx)
		stats.Queries += current.Queries
		stats.Rows = current.Rows
		stats.StatementDuration += current.StatementDuration
		stats.DuplicatePreventionConflicts += current.DuplicatePreventionConflicts
		return runErr
	}
	if s.tx.HasTransaction(ctx) {
		err = execute(ctx)
	} else {
		err = s.tx.RunAdmin(ctx, execute)
	}
	if err != nil {
		stats.Rows = 0
	}
	return err
}
