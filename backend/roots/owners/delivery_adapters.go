package owners

import (
	"context"
	"fmt"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"

	announcement "github.com/moto-ogs/project-phoenix/modules/communication"
	delivery "github.com/moto-ogs/project-phoenix/modules/delivery/runtime"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// GuardianDisplayResolver names the guardians of the Delivery recipients.
type GuardianDisplayResolver struct{ Query GuardianDisplayQuery }

func (r GuardianDisplayResolver) ResolveGuardianDisplays(ctx context.Context, ids []int64) ([]deliveryContract.GuardianDisplay, error) {
	if len(ids) == 0 {
		return []deliveryContract.GuardianDisplay{}, nil
	}
	profiles, err := r.Query.GuardianDisplays(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("delivery: resolve guardian display data: %w", err)
	}
	result := make([]deliveryContract.GuardianDisplay, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, deliveryContract.GuardianDisplay{
			GuardianProfileID: profile.GuardianProfileID, FirstName: profile.FirstName, LastName: profile.LastName,
		})
	}
	return result, nil
}

type announcementDeliveryAdapter struct {
	module   delivery.Capability
	outcomes delivery.EmailOutcomes
}

func (a announcementDeliveryAdapter) ReplaceForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64, rows []announcement.ParentAnnouncementEmailDelivery) error {
	values := make([]deliveryContract.EmailDelivery, 0, len(rows))
	for _, row := range rows {
		values = append(values, deliveryContract.EmailDelivery{
			OutboxID: row.OutboxID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			RecipientEmail: row.RecipientEmail, Reachability: row.Reachability,
		})
	}
	return a.module.ReplaceEmailDeliveries(ctx, tenantID, deliveryContract.RelatedEntity{Type: relatedType, ID: relatedID}, values)
}

func (a announcementDeliveryAdapter) DeleteForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) (int64, error) {
	return a.module.DeleteEmailDeliveries(ctx, tenantID, deliveryContract.RelatedEntity{Type: relatedType, ID: relatedID})
}

func (a announcementDeliveryAdapter) ListForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) ([]announcement.ParentAnnouncementEmailDeliveryStatus, error) {
	rows, err := a.module.EmailDeliveryStatuses(ctx, tenantID, deliveryContract.RelatedEntity{Type: relatedType, ID: relatedID})
	if err != nil {
		return nil, err
	}
	blocks := a.providerBlocks(ctx, rows)
	result := make([]announcement.ParentAnnouncementEmailDeliveryStatus, 0, len(rows))
	for _, row := range rows {
		result = append(result, announcement.ParentAnnouncementEmailDeliveryStatus{
			DeliveryID: row.DeliveryID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			FirstName: row.FirstName, LastName: row.LastName, RecipientEmail: row.RecipientEmail,
			Reachability: row.Reachability, EmailStatus: row.EmailStatus, LastError: row.LastError,
			SentAt: row.SentAt, Attempts: row.Attempts, MailOutcome: string(row.Outcome), AddressMark: string(row.AddressMark), Stopped: row.Stopped,
			StopShared: row.StopShared, MailedTo: row.MailedTo, ProviderBlock: letterProviderBlock(row, blocks),
			Departed: row.Departed,
		})
	}
	return result, nil
}

// providerBlocks asks the provider about the addresses another mail would go to,
// if a mail to exactly that address left a block there. A new address has no such
// history: it is no question for the provider.
func (a announcementDeliveryAdapter) providerBlocks(ctx context.Context, rows []deliveryContract.EmailDeliveryStatus) map[string]deliveryContract.EmailProviderBlock {
	var held []string
	for _, row := range rows {
		if asksTheProvider(row) {
			held = append(held, *row.RecipientEmail)
		}
	}
	if len(held) == 0 || a.outcomes == nil {
		return nil
	}
	return a.outcomes.ProviderEmailBlocks(ctx, held)
}

func letterProviderBlock(row deliveryContract.EmailDeliveryStatus, blocks map[string]deliveryContract.EmailProviderBlock) string {
	if !asksTheProvider(row) {
		return ""
	}
	return string(blocks[deliveryContract.NormalizedEmailAddress(*row.RecipientEmail)])
}

// asksTheProvider names a row whose current address has a history at the provider:
// the last mail went there and left a block, or an earlier one was refused there.
func asksTheProvider(row deliveryContract.EmailDeliveryStatus) bool {
	if row.RecipientEmail == nil {
		return false
	}
	return row.CurrentAddressRefused || (sameAddress(row.MailedTo, row.RecipientEmail) && row.Outcome.HeldAtProvider())
}

func sameAddress(a, b *string) bool {
	return a != nil && b != nil && deliveryContract.NormalizedEmailAddress(*a) == deliveryContract.NormalizedEmailAddress(*b)
}

func (a announcementDeliveryAdapter) AttachOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	return a.module.AttachEmailOutbox(ctx, tenantID, deliveryID, outboxID)
}

func (a announcementDeliveryAdapter) ClaimFailedDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	return a.module.ClaimFailedEmailDelivery(ctx, tenantID, deliveryID)
}

// enrollmentDeliveryAdapter counts and cancels the mails of an application
// of the school in context.
type enrollmentDeliveryAdapter struct {
	module   delivery.Capability
	outcomes delivery.EmailOutcomes
}

func (a enrollmentDeliveryAdapter) CountRelatedEmails(ctx context.Context, relatedType string, relatedID int64) (int, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	rows, err := a.module.Statuses(ctx, tenantID.Int64(), deliveryContract.TransportEmail, deliveryContract.RelatedEntity{Type: relatedType, ID: relatedID})
	return len(rows), err
}

// RelatedEmailOutcomes reports what became of the mails of an application.
func (a enrollmentDeliveryAdapter) RelatedEmailOutcomes(ctx context.Context, relatedType string, relatedID int64) ([]enrollment.RequestMailOutcome, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	reports, err := a.outcomes.EmailOutcomes(ctx, tenantID.Int64(), deliveryContract.EmailOutcomeQuery{RelatedType: relatedType, RelatedIDs: []int64{relatedID}})
	if err != nil {
		return nil, err
	}
	outcomes := make([]enrollment.RequestMailOutcome, 0, len(reports))
	for _, report := range reports {
		if deliveryContract.IsParentMail(report.Kind) {
			outcomes = append(outcomes, enrollment.RequestMailOutcome{Kind: report.Kind, Outcome: string(report.Outcome), AddressMark: string(report.Mark), Stopped: report.Stopped, OutcomeAt: report.OutcomeAt})
		}
	}
	return outcomes, nil
}

func (a enrollmentDeliveryAdapter) CancelRelatedEmails(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	return a.module.Cancel(ctx, tenantID.Int64(), deliveryContract.TransportEmail, deliveryContract.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}
