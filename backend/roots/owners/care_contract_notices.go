package owners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// CareContractNoticeSettings reads Träger-level texts in the caller's
// Träger transaction (ADR 0086).
type CareContractNoticeSettings interface {
	TraegerTexts(ctx context.Context, organizationID int64, keys []string) (map[string]string, error)
}

// NewCareContractNoticePorts binds the confirmation templates to the
// Träger level of the settings system and the confirmation mail to
// Delivery. The parents portal's binding uses it too.
func NewCareContractNoticePorts(settings CareContractNoticeSettings, delivery deliveryRuntime.Capability) careContractCompose.NoticePorts {
	return careContractCompose.NoticePorts{
		Templates: careContractNoticeTemplates{settings: settings},
		Post:      careContractNoticePost{delivery: delivery},
	}
}

// careContractNoticeTemplates reads the Träger's confirmation templates
// (care_contract.*_confirmation): its own value or the registry default,
// never an environment value.
type careContractNoticeTemplates struct {
	settings CareContractNoticeSettings
}

func (t careContractNoticeTemplates) NoticeTemplate(ctx context.Context, organizationID int64, kind string) (string, error) {
	key := configModels.KeyCareContractTerminationConfirmation
	switch kind {
	case carecontract.NoticeWithdrawal:
		key = configModels.KeyCareContractWithdrawalConfirmation
	case carecontract.TerminationExtraordinary:
		key = configModels.KeyCareContractExtraordinaryConfirmation
	}
	texts, err := t.settings.TraegerTexts(ctx, organizationID, []string{key})
	if err != nil {
		return "", err
	}
	return texts[key], nil
}

// careContractNoticePost enqueues the confirmation as a mail of the Träger
// (ADR 0110) in the caller's Träger transaction, related to the contract.
type careContractNoticePost struct {
	delivery deliveryRuntime.Capability
}

func (p careContractNoticePost) EnqueueNoticeMail(ctx context.Context, mail carecontract.NoticeMail) (int64, error) {
	payload, err := json.Marshal(mail)
	if err != nil {
		return 0, fmt.Errorf("contract notice payload: %w", err)
	}
	enqueued, err := p.delivery.EnqueueEmail(ctx, deliveryContract.EmailIntent{
		OrganizationID: mail.OrganizationID, Template: carecontract.MailKindNotice,
		Recipient: deliveryContract.EmailRecipient{Address: mail.Recipient, Name: mail.GuardianName},
		Payload:   payload, IdempotencyKey: mail.IdempotencyKey,
		Related: deliveryContract.RelatedEntity{Type: carecontract.MailRelatedContract, ID: mail.ContractID},
	})
	if err != nil {
		return 0, err
	}
	return enqueued.ID, nil
}

// parentContractNotices binds the parents portal's withdrawal and
// termination (#4682) to the Betreuungsvertrag in the Träger transaction the
// flow opens, and translates its refusals into the flow's words.
type parentContractNotices struct {
	notices carecontract.Notices
}

// NewParentContractNotices binds the parents' declarations over the
// confirmation templates of the Träger and Delivery's outbox.
func NewParentContractNotices(settings CareContractNoticeSettings, delivery deliveryRuntime.Capability) parentportal.ContractNoticesPort {
	return parentContractNotices{notices: careContractCompose.NewParentNotices(NewCareContractNoticePorts(settings, delivery))}
}

func (n parentContractNotices) ContractNoticeOptions(ctx context.Context, organizationID, contractID int64,
	reader parentportal.BillingDocumentReader) (parentportal.ContractNoticeOptions, error) {
	options, err := n.notices.ParentNoticeOptions(ctx, organizationID, contractID, carecontract.InboxReader(reader))
	return parentNoticeOptions(options), parentNoticeError(err)
}

func (n parentContractNotices) DeclareContractNotice(ctx context.Context, organizationID, contractID int64,
	reader parentportal.BillingDocumentReader, traeger string, input parentportal.ContractNoticeInput) (parentportal.ContractNoticeOptions, error) {
	options, err := n.notices.ParentDeclareNotice(ctx, organizationID, contractID, carecontract.InboxReader(reader), traeger,
		carecontract.NoticeInput{Kind: input.Kind, TerminationKind: input.TerminationKind, Reason: input.Reason, ExpectedEndsOn: input.ExpectedEndsOn})
	return parentNoticeOptions(options), parentNoticeError(err)
}

func parentNoticeOptions(options carecontract.NoticeOptions) parentportal.ContractNoticeOptions {
	result := parentportal.ContractNoticeOptions{
		ContractID: options.ContractID, Status: options.Status, ChildName: options.ChildName, Modules: options.Modules,
		SchoolName: options.SchoolName, Terms: options.Terms, OrdinaryEndsOn: options.OrdinaryEndsOn,
		SpecialEndsOn: options.SpecialEndsOn, WithdrawalUntil: options.WithdrawalUntil, EarlyStart: options.EarlyStart,
	}
	if notice := options.Notice; notice != nil {
		result.Notice = &parentportal.ContractNotice{
			Kind: notice.Kind, TerminationKind: notice.TerminationKind, Reason: notice.Reason, Channel: notice.Channel, DeclaredAt: notice.DeclaredAt,
			DispatchedOn: notice.DispatchedOn, ReceivedOn: notice.ReceivedOn, EndsOn: notice.EndsOn, Recipient: notice.Recipient, ConfirmationText: notice.ConfirmationText,
		}
	}
	return result
}

// parentNoticeError names a refusal of the Betreuungsvertrag in the flow's
// words; any other error stays as it is.
func parentNoticeError(err error) error {
	var invalid *carecontract.ValidationError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, carecontract.ErrInboxDocumentNotFound), errors.Is(err, carecontract.ErrContractNotFound):
		return fmt.Errorf("%w: %w", parentportal.ErrBillingDocumentNotFound, err)
	case errors.Is(err, carecontract.ErrContractNotEndable):
		return fmt.Errorf("%w: %w", parentportal.ErrContractNotEndable, err)
	case errors.Is(err, carecontract.ErrWithdrawalClosed):
		return fmt.Errorf("%w: %w", parentportal.ErrContractWithdrawalClosed, err)
	case errors.Is(err, carecontract.ErrSpecialTerminationClosed):
		return fmt.Errorf("%w: %w", parentportal.ErrContractSpecialTerminationClosed, err)
	case errors.Is(err, carecontract.ErrNoticeTemplateMissing):
		return fmt.Errorf("%w: %w", parentportal.ErrContractNoticeTemplateMissing, err)
	case errors.As(err, &invalid):
		return fmt.Errorf("%w: %w", parentportal.ErrContractNoticeInvalid, err)
	}
	return err
}

// NoticeDeliveries returns only positive provision evidence. Queueing, SMTP
// acceptance without a delivery receipt and bounces do not shorten the period.
func (p careContractNoticePost) NoticeDeliveries(ctx context.Context, organizationID, contractID int64) ([]carecontract.NoticeDelivery, error) {
	statuses, err := p.delivery.TraegerEmailStatuses(ctx, organizationID, deliveryContract.RelatedEntity{Type: carecontract.MailRelatedContract, ID: contractID})
	if err != nil {
		return nil, err
	}
	var deliveries []carecontract.NoticeDelivery
	for _, status := range statuses {
		if status.DeliveryState == deliveryContract.DeliveryDelivered && status.DeliveryStateAt != nil {
			deliveries = append(deliveries, carecontract.NoticeDelivery{OutboxID: status.ID, ProvidedAt: *status.DeliveryStateAt})
		}
	}
	return deliveries, nil
}
