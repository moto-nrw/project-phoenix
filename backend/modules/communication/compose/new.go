package compose

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/audience"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type AuditEntry struct {
	OperatorID int64
	Action     string
	ResourceID int64
	RequestIP  net.IP
	Changes    map[string]any
}

type Audit interface {
	AppendAnnouncementAudit(context.Context, AuditEntry) error
}

type AuditFunc func(context.Context, AuditEntry) error

func (f AuditFunc) AppendAnnouncementAudit(ctx context.Context, entry AuditEntry) error {
	return f(ctx, entry)
}

type Observation = ports.Observation

type Dependencies struct {
	DB            *bun.DB
	Organizations organizationtenancy.Capability
	People        peopledirectory.Query
	Audit         Audit
	Observe       func(Observation)
}

func New(dependencies Dependencies) (*communication.Module, error) {
	if dependencies.DB == nil || dependencies.Organizations == nil || dependencies.People == nil || dependencies.Audit == nil || dependencies.Observe == nil {
		return nil, errors.New("communication compose: all dependencies are required")
	}
	database := func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("communication postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("communication postgres: unsupported transaction %T", transaction)
		}
		return tx, nil
	}
	store := postgres.New(database)
	service := application.New(
		store,
		audience.New(database),
		targets{capability: dependencies.Organizations},
		viewerNames{query: dependencies.People},
		audit{port: dependencies.Audit},
		transaction{},
		func(observation ports.Observation) {
			observation.Err = mapError(observation.Err, 0)
			dependencies.Observe(observation)
		},
	)
	return communication.NewModule(engine{service: service, observe: dependencies.Observe}), nil
}

type viewerNames struct{ query peopledirectory.Query }

func (v viewerNames) NamesByAccount(ctx context.Context, accountIDs []int64) (map[int64]string, domain.OperationStats, error) {
	people, err := v.query.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, domain.OperationStats{Queries: 1}, err
	}
	result := make(map[int64]string, len(people))
	for _, person := range people {
		if person.AccountID == nil {
			continue
		}
		name := person.FullName()
		if name != " " {
			result[*person.AccountID] = name
		}
	}
	return result, domain.OperationStats{Queries: 1, Rows: int64(len(people))}, nil
}

type transaction struct{}

func (transaction) RunAdmin(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinAdminRetry(ctx, callback)
}

type targets struct {
	capability organizationtenancy.Capability
}

func (t targets) CountOrganizationsByID(ctx context.Context, ids []int64) (int, domain.OperationStats, error) {
	count, err := t.capability.CountOrganizationsByID(ctx, ids)
	stats := domain.OperationStats{Queries: 1, Rows: int64(count)}
	if errors.Is(err, organizationtenancy.ErrInvalidOrganization) {
		return 0, stats, fmt.Errorf("%w: %v", domain.ErrInvalidTarget, err)
	}
	return count, stats, err
}

func (t targets) CountSchoolsByID(ctx context.Context, ids []int64) (int, domain.OperationStats, error) {
	count, err := t.capability.CountSchoolsByID(ctx, ids)
	stats := domain.OperationStats{Queries: 1, Rows: int64(count)}
	if errors.Is(err, organizationtenancy.ErrInvalidSchool) {
		return 0, stats, fmt.Errorf("%w: %v", domain.ErrInvalidTarget, err)
	}
	return count, stats, err
}

type audit struct{ port Audit }

func (a audit) Append(ctx context.Context, entry domain.AuditEntry) (domain.OperationStats, error) {
	started := time.Now()
	err := a.port.AppendAnnouncementAudit(ctx, AuditEntry{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceID: entry.ResourceID,
		RequestIP: entry.RequestIP, Changes: entry.Changes,
	})
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err == nil {
		stats.Rows = 1
	}
	return stats, err
}

type engine struct {
	service *application.Service
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
