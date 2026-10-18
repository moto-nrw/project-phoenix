package api

import (
	"context"
	"fmt"

	announcement "github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/delivery"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The adapters below bind Delivery to the owners it serves and reads: the
// People Directory names the guardians of a delivery, Communication records
// the deliveries of a parent announcement, and Enrollment counts and cancels
// the mails of an application (#4013).

// GuardianDisplayResolver names the guardians of the Delivery recipients.
type GuardianDisplayResolver struct{ Query GuardianDisplayQuery }

func (r GuardianDisplayResolver) ResolveGuardianDisplays(ctx context.Context, ids []int64) ([]delivery.GuardianDisplay, error) {
	if len(ids) == 0 {
		return []delivery.GuardianDisplay{}, nil
	}
	profiles, err := r.Query.GuardianDisplays(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("delivery: resolve guardian display data: %w", err)
	}
	result := make([]delivery.GuardianDisplay, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, delivery.GuardianDisplay{
			GuardianProfileID: profile.GuardianProfileID, FirstName: profile.FirstName, LastName: profile.LastName,
		})
	}
	return result, nil
}

type announcementDeliveryAdapter struct{ module *delivery.Module }

func (a announcementDeliveryAdapter) ReplaceForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64, rows []announcement.ParentAnnouncementEmailDelivery) error {
	values := make([]delivery.EmailDelivery, 0, len(rows))
	for _, row := range rows {
		values = append(values, delivery.EmailDelivery{
			OutboxID: row.OutboxID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			RecipientEmail: row.RecipientEmail, Reachability: row.Reachability,
		})
	}
	return a.module.ReplaceEmailDeliveries(ctx, tenantID, delivery.RelatedEntity{Type: relatedType, ID: relatedID}, values)
}

func (a announcementDeliveryAdapter) DeleteForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) (int64, error) {
	return a.module.DeleteEmailDeliveries(ctx, tenantID, delivery.RelatedEntity{Type: relatedType, ID: relatedID})
}

func (a announcementDeliveryAdapter) ListForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) ([]announcement.ParentAnnouncementEmailDeliveryStatus, error) {
	rows, err := a.module.EmailDeliveryStatuses(ctx, tenantID, delivery.RelatedEntity{Type: relatedType, ID: relatedID})
	if err != nil {
		return nil, err
	}
	result := make([]announcement.ParentAnnouncementEmailDeliveryStatus, 0, len(rows))
	for _, row := range rows {
		result = append(result, announcement.ParentAnnouncementEmailDeliveryStatus{
			DeliveryID: row.DeliveryID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			FirstName: row.FirstName, LastName: row.LastName, RecipientEmail: row.RecipientEmail,
			Reachability: row.Reachability, EmailStatus: row.EmailStatus, LastError: row.LastError,
			SentAt: row.SentAt, Attempts: row.Attempts,
		})
	}
	return result, nil
}

func (a announcementDeliveryAdapter) AttachOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	return a.module.AttachEmailOutbox(ctx, tenantID, deliveryID, outboxID)
}

func (a announcementDeliveryAdapter) ClaimFailedDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	return a.module.ClaimFailedEmailDelivery(ctx, tenantID, deliveryID)
}

// enrollmentDeliveryAdapter counts and cancels the mails of an application
// of the school in context.
type enrollmentDeliveryAdapter struct{ module *delivery.Module }

func (a enrollmentDeliveryAdapter) CountRelatedEmails(ctx context.Context, relatedType string, relatedID int64) (int, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	rows, err := a.module.Statuses(ctx, tenantID.Int64(), delivery.TransportEmail, delivery.RelatedEntity{Type: relatedType, ID: relatedID})
	return len(rows), err
}

func (a enrollmentDeliveryAdapter) CancelRelatedEmails(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	return a.module.Cancel(ctx, tenantID.Int64(), delivery.TransportEmail, delivery.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}

// GuardianDisplayRows names the guardians of the Delivery recipients from
// the People Directory's guardian rows, the way the guardian service reads
// them. Delivery is composed before the guardian service, which needs
// Identity & Access, which needs Delivery's outbox (ADR 0059).
type GuardianDisplayRows struct {
	Profiles peoplePorts.GuardianProfileRepository
}

func (r GuardianDisplayRows) ResolveGuardianDisplays(ctx context.Context, ids []int64) ([]delivery.GuardianDisplay, error) {
	if len(ids) == 0 {
		return []delivery.GuardianDisplay{}, nil
	}
	profiles, err := r.Profiles.FindByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("delivery: resolve guardian display data: %w", fmt.Errorf("resolve guardian displays: %w", err))
	}
	result := make([]delivery.GuardianDisplay, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, delivery.GuardianDisplay{
			GuardianProfileID: profile.ID, FirstName: profile.FirstName, LastName: profile.LastName,
		})
	}
	return result, nil
}
