package owners

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// The pushes „Neues Dokument vom Träger“ of a sent care contract and of a
// release for parents (#5941) share the letters' notifier, notification
// type, consent and copy. Each kind keeps its own outbox kind, related type
// and key prefix, so a contract, a release and a letter with the same
// numeric ID never collide.

// documentPushIdentity names the key prefix and the related type of a kind.
func documentPushIdentity(kind string) (string, string, error) {
	switch kind {
	case parentportal.DocumentPushBilling:
		return "parent-billing-document", parentbilling.PushRelatedBillingDocument, nil
	case parentportal.DocumentPushContract:
		return "parent-care-contract-document", carecontract.PushRelatedContractDocument, nil
	case parentportal.DocumentPushRelease:
		return "parent-traeger-document-release", organizationtenancy.PushRelatedTraegerDocumentRelease, nil
	default:
		return "", "", fmt.Errorf("parent document push: unknown kind %q", kind)
	}
}

// isTraegerDocumentPush tells the outbox kinds the parents portal sends.
func isTraegerDocumentPush(template string) bool {
	switch template {
	case parentbilling.PushKindBillingDocument, carecontract.PushKindContractDocument, organizationtenancy.PushKindTraegerDocumentRelease,
		pushKindTraegerDocumentReleaseAccount:
		return true
	}
	return false
}

// route sends a claimed document push by its outbox kind. A payload that
// does not match the intent's Träger and related entity is cancelled, never
// sent to anyone.
func (p parentDocumentPushes) route(ctx context.Context, intent deliveryContract.ClaimedIntent) error {
	switch intent.Template {
	case carecontract.PushKindContractDocument:
		var notice carecontract.ContractDocumentNotice
		if err := p.scoped(intent, carecontract.PushRelatedContractDocument, &notice, func() (int64, int64) { return notice.OrganizationID, notice.ContractID }); err != nil {
			return err
		}
		return p.notices.NotifyContractDocument(ctx, notice.OrganizationID, notice.ContractID, notice.RecipientEmail)
	case organizationtenancy.PushKindTraegerDocumentRelease:
		var notice organizationtenancy.TraegerDocumentReleaseNotice
		if err := p.scoped(intent, organizationtenancy.PushRelatedTraegerDocumentRelease, &notice, func() (int64, int64) { return notice.OrganizationID, notice.ReleaseID }); err != nil {
			return err
		}
		return p.notices.NotifyTraegerDocumentRelease(ctx, notice.OrganizationID, notice.ReleaseID)
	case pushKindTraegerDocumentReleaseAccount:
		var notice releaseAccountNotice
		if err := p.scoped(intent, organizationtenancy.PushRelatedTraegerDocumentRelease, &notice, func() (int64, int64) { return notice.OrganizationID, notice.ReleaseID }); err != nil {
			return err
		}
		if notice.AccountID <= 0 {
			return fmt.Errorf("%w: release notice without an account", deliveryContract.ErrCancelled)
		}
		return p.notices.NotifyTraegerDocumentReleaseAccount(ctx, notice.OrganizationID, notice.ReleaseID, notice.AccountID)
	default:
		return p.send(ctx, intent)
	}
}

// scoped decodes the payload and checks it names the intent's Träger and
// related entity of relatedType.
func (parentDocumentPushes) scoped(intent deliveryContract.ClaimedIntent, relatedType string, payload any, scope func() (int64, int64)) error {
	if err := json.Unmarshal(intent.DeferredPayload, payload); err != nil {
		return fmt.Errorf("%w: document notice payload: %w", deliveryContract.ErrCancelled, err)
	}
	organizationID, relatedID := scope()
	if intent.OrganizationID <= 0 || organizationID != intent.OrganizationID ||
		intent.RelatedEntityID == nil || *intent.RelatedEntityID != relatedID ||
		intent.RelatedEntityType == nil || *intent.RelatedEntityType != relatedType {
		return fmt.Errorf("%w: document notice scope mismatch", deliveryContract.ErrCancelled)
	}
	return nil
}

// CareContractDocumentNotices binds the push of a sent contract to
// Delivery's outbox in the caller's Träger transaction (#5941). Its audience
// is found when the worker sends it.
type CareContractDocumentNotices struct {
	Delivery DeferredPushOutbox
}

// DeferredPushOutbox enqueues a deferred push of a Träger.
type DeferredPushOutbox interface {
	EnqueueDeferredPush(ctx context.Context, intent deliveryContract.DeferredPushIntent) (deliveryContract.Enqueued, error)
}

func (n CareContractDocumentNotices) EnqueueContractDocumentNotice(ctx context.Context, notice carecontract.ContractDocumentNotice) error {
	payload, err := json.Marshal(notice)
	if err != nil {
		return fmt.Errorf("contract document notice payload: %w", err)
	}
	_, err = n.Delivery.EnqueueDeferredPush(ctx, deliveryContract.DeferredPushIntent{
		OrganizationID: notice.OrganizationID, Template: carecontract.PushKindContractDocument,
		IdempotencyKey: contractDocumentNoticeKey(notice.ContractID, notice.RecipientEmail), Payload: payload,
		Related: deliveryContract.RelatedEntity{Type: carecontract.PushRelatedContractDocument, ID: notice.ContractID},
	})
	return err
}

// TraegerDocumentReleaseNotices binds the push of a release for parents to
// Delivery's outbox in the caller's Träger transaction (#5941). The key names
// the release: a new release after a revocation is a new push, a retry of
// the same release is not.
type TraegerDocumentReleaseNotices struct {
	Delivery DeferredPushOutbox
}

func (n TraegerDocumentReleaseNotices) EnqueueReleaseNotice(ctx context.Context, notice organizationtenancy.TraegerDocumentReleaseNotice) error {
	payload, err := json.Marshal(notice)
	if err != nil {
		return fmt.Errorf("release notice payload: %w", err)
	}
	_, err = n.Delivery.EnqueueDeferredPush(ctx, deliveryContract.DeferredPushIntent{
		OrganizationID: notice.OrganizationID, Template: organizationtenancy.PushKindTraegerDocumentRelease,
		IdempotencyKey: fmt.Sprintf("traeger-document-release:%d", notice.ReleaseID), Payload: payload,
		Related: deliveryContract.RelatedEntity{Type: organizationtenancy.PushRelatedTraegerDocumentRelease, ID: notice.ReleaseID},
	})
	return err
}

// contractDocumentNoticeKey names the contract and the address without
// spelling the address out: the key travels into Delivery's logs.
func contractDocumentNoticeKey(contractID int64, address string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(address))))
	return fmt.Sprintf("care-contract-document:%d:%s", contractID, hex.EncodeToString(sum[:8]))
}

// pushKindTraegerDocumentReleaseAccount is the outbox kind of the push of a
// release to one account: the release fans out to one entry per candidate,
// so every account has its own time limit and retries (#5941).
const pushKindTraegerDocumentReleaseAccount = "traeger_document_release_account"

// releaseAccountNotice is the payload of that entry.
type releaseAccountNotice struct {
	OrganizationID int64 `json:"organization_id"`
	ReleaseID      int64 `json:"release_id"`
	AccountID      int64 `json:"account_id"`
}

// ParentReleaseFanOut binds the fan-out of a release to Delivery's outbox in
// the caller's Träger transaction. The key names the release and the
// account, so a retried fan-out adds no entry twice.
type ParentReleaseFanOut struct {
	Delivery DeferredPushOutbox
}

func (f ParentReleaseFanOut) EnqueueReleaseAccounts(ctx context.Context, organizationID, releaseID int64, accountIDs []int64) error {
	for _, accountID := range accountIDs {
		payload, err := json.Marshal(releaseAccountNotice{OrganizationID: organizationID, ReleaseID: releaseID, AccountID: accountID})
		if err != nil {
			return fmt.Errorf("release account notice payload: %w", err)
		}
		if _, err := f.Delivery.EnqueueDeferredPush(ctx, deliveryContract.DeferredPushIntent{
			OrganizationID: organizationID, Template: pushKindTraegerDocumentReleaseAccount,
			IdempotencyKey: fmt.Sprintf("traeger-document-release:%d:account:%d", releaseID, accountID), Payload: payload,
			Related: deliveryContract.RelatedEntity{Type: organizationtenancy.PushRelatedTraegerDocumentRelease, ID: releaseID},
		}); err != nil {
			return err
		}
	}
	return nil
}
