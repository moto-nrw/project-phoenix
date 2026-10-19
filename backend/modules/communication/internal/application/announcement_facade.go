package application

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/ports"
)

func NewAnnouncementEngine(service *Service, observe ports.Observer) engine {
	return engine{service: service, observe: observe}
}
func NewAnnouncementObserver(observe ports.Observer) ports.Observer {
	return func(observation ports.Observation) {
		observation.Err = mapError(observation.Err, 0)
		observe(observation)
	}
}

type engine struct {
	service *Service
	observe ports.Observer
}

func (e engine) ObserveRejection(operation string, duration time.Duration, err error) {
	e.observe(ports.Observation{Operation: operation, Duration: duration, Err: err})
}

func (e engine) CreateAnnouncement(ctx context.Context, value *communication.Announcement, operatorID int64, clientIP net.IP) error {
	created, err := e.service.Create(ctx, toDomain(*value), auditEntry(operatorID, clientIP))
	if err != nil {
		return mapError(err, 0)
	}
	*value = toPublic(created)
	return nil
}

func (e engine) GetAnnouncement(ctx context.Context, id int64) (*communication.Announcement, error) {
	value, err := e.service.Get(ctx, id)
	if err != nil {
		return nil, mapError(err, id)
	}
	result := toPublic(value)
	return &result, nil
}

func (e engine) UpdateAnnouncement(ctx context.Context, value *communication.Announcement, operatorID int64, clientIP net.IP) error {
	updated, err := e.service.Update(ctx, toDomain(*value), auditEntry(operatorID, clientIP))
	if err != nil {
		return mapError(err, value.ID)
	}
	*value = toPublic(updated)
	return nil
}

func (e engine) DeleteAnnouncement(ctx context.Context, id, operatorID int64, clientIP net.IP) error {
	return mapError(e.service.Delete(ctx, id, auditEntry(operatorID, clientIP)), id)
}

func (e engine) ListAnnouncements(ctx context.Context, includeInactive bool) ([]*communication.Announcement, error) {
	values, err := e.service.List(ctx, includeInactive)
	return toPublicPointers(values), mapError(err, 0)
}

func (e engine) PublishAnnouncement(ctx context.Context, id, operatorID int64, clientIP net.IP) error {
	return mapError(e.service.Publish(ctx, id, auditEntry(operatorID, clientIP)), id)
}

func (e engine) UnpublishAnnouncement(ctx context.Context, id, operatorID int64, clientIP net.IP) error {
	return mapError(e.service.Unpublish(ctx, id, auditEntry(operatorID, clientIP)), id)
}

func (e engine) GetUnreadForUser(ctx context.Context, userID int64, roles []string, tenantID, orgID int64) ([]*communication.Announcement, error) {
	values, err := e.service.Unread(ctx, userID, roles, tenantID, orgID)
	return toPublicPointers(values), mapError(err, 0)
}

func (e engine) CountUnread(ctx context.Context, userID int64, roles []string, tenantID, orgID int64) (int, error) {
	count, err := e.service.CountUnread(ctx, userID, roles, tenantID, orgID)
	return count, mapError(err, 0)
}

func (e engine) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	return mapError(e.service.MarkSeen(ctx, userID, announcementID), announcementID)
}

func (e engine) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	return mapError(e.service.MarkDismissed(ctx, userID, announcementID), announcementID)
}

func (e engine) GetStats(ctx context.Context, id int64) (*communication.AnnouncementStats, error) {
	value, err := e.service.Stats(ctx, id)
	if err != nil {
		return nil, mapError(err, id)
	}
	return &communication.AnnouncementStats{
		AnnouncementID: value.AnnouncementID, TargetCount: value.TargetCount,
		SeenCount: value.SeenCount, DismissedCount: value.DismissedCount,
	}, nil
}

func (e engine) GetViewDetails(ctx context.Context, id int64) ([]*communication.AnnouncementViewDetail, error) {
	values, err := e.service.ViewDetails(ctx, id)
	if err != nil {
		return nil, mapError(err, id)
	}
	result := make([]*communication.AnnouncementViewDetail, 0, len(values))
	for _, value := range values {
		result = append(result, &communication.AnnouncementViewDetail{
			UserID: value.UserID, AccountEmail: value.AccountEmail, UserName: value.UserName,
			SeenAt: value.SeenAt, Dismissed: value.Dismissed,
		})
	}
	return result, nil
}

func auditEntry(operatorID int64, clientIP net.IP) domain.AuditEntry {
	return domain.AuditEntry{OperatorID: operatorID, RequestIP: clientIP}
}

func toDomain(value communication.Announcement) domain.Announcement {
	return domain.Announcement{
		ID: value.ID, Title: value.Title, Content: value.Content, Type: value.Type,
		Severity: value.Severity, Version: value.Version, Active: value.Active,
		PublishedAt: value.PublishedAt, ExpiresAt: value.ExpiresAt,
		TargetRoles: value.TargetRoles, TargetOrgIDs: value.TargetOrgIDs,
		TargetTenantIDs: value.TargetTenantIDs, CreatedBy: value.CreatedBy,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func toPublic(value domain.Announcement) communication.Announcement {
	return communication.Announcement{
		ID: value.ID, Title: value.Title, Content: value.Content, Type: value.Type,
		Severity: value.Severity, Version: value.Version, Active: value.Active,
		PublishedAt: value.PublishedAt, ExpiresAt: value.ExpiresAt,
		TargetRoles: value.TargetRoles, TargetOrgIDs: value.TargetOrgIDs,
		TargetTenantIDs: value.TargetTenantIDs, CreatedBy: value.CreatedBy,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func toPublicPointers(values []domain.Announcement) []*communication.Announcement {
	result := make([]*communication.Announcement, 0, len(values))
	for _, value := range values {
		converted := toPublic(value)
		result = append(result, &converted)
	}
	return result
}

func mapError(err error, id int64) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrAnnouncementNotFound):
		return &communication.AnnouncementNotFoundError{AnnouncementID: id}
	case errors.Is(err, domain.ErrInvalidTarget):
		return &communication.InvalidDataError{Err: err}
	default:
		return err
	}
}
