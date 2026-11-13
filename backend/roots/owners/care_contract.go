package owners

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-ogs/project-phoenix/modules/carecontract/compose"
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	deliveryRuntime "github.com/moto-ogs/project-phoenix/modules/delivery/runtime"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	schedulerPorts "github.com/moto-ogs/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// The Betreuungsvertrag's ports both roots bind (ADR 0153, #4679): the
// serving root for its routes and the Worker for the reminders.

// CareContractPost binds the mail of a contract to Delivery: a mail of the
// Träger (ADR 0110) in the caller's Träger transaction, the contract as its
// related entity and the PDF in its payload (ADR 0123).
type CareContractPost struct {
	Delivery deliveryRuntime.Capability
	Outcomes deliveryRuntime.EmailOutcomes
}

func (p CareContractPost) EnqueueContractMail(ctx context.Context, mail carecontract.ContractMail) (int64, error) {
	payload, err := json.Marshal(mail)
	if err != nil {
		return 0, fmt.Errorf("contract mail payload: %w", err)
	}
	enqueued, err := p.Delivery.EnqueueEmail(ctx, deliveryContract.EmailIntent{
		OrganizationID: mail.OrganizationID, Template: carecontract.MailKindContract,
		Recipient: deliveryContract.EmailRecipient{Address: mail.Recipient, Name: mail.GuardianName},
		Payload:   payload, IdempotencyKey: mail.IdempotencyKey,
		Related: deliveryContract.RelatedEntity{Type: carecontract.MailRelatedContract, ID: mail.ContractID},
	})
	if err != nil {
		return 0, err
	}
	return enqueued.ID, nil
}

// contractMailState names Delivery's outcome in the contract's words. Only a mail
// that left and did not come back counts as sent: an unsubscribe or a complaint is
// no sent contract (#5934).
func contractMailState(outcome deliveryContract.EmailOutcome) string {
	switch outcome {
	case deliveryContract.EmailQueued:
		return carecontract.MailQueued
	case deliveryContract.EmailAccepted, deliveryContract.EmailDelivered, deliveryContract.EmailDelayed, deliveryContract.EmailOverdue:
		return carecontract.MailSent
	case deliveryContract.EmailOutcomeUnknown:
		return carecontract.MailUnknown
	default:
		return carecontract.MailFailed
	}
}

// ContractMailBlock reads what would hold a mail to the address now: a stop or a
// list of a school of the Träger, or the provider if the last mail to it left a block.
func (p CareContractPost) ContractMailBlock(ctx context.Context, organizationID int64, address, outcome string) (string, error) {
	listed, stopped, err := p.Outcomes.TraegerMailHolds(ctx, organizationID, address)
	if err != nil {
		return "", err
	}
	mark := deliveryContract.EmailAddressMark("")
	if listed {
		mark = deliveryContract.EmailMarkFailed
	}
	latest := deliveryContract.EmailOutcome(outcome)
	var provider deliveryContract.EmailProviderBlock
	if latest.HeldAtProvider() {
		provider = p.Outcomes.ProviderEmailBlocks(ctx, []string{address})[deliveryContract.NormalizedEmailAddress(address)]
	}
	return string(deliveryContract.EmailSendBlockOf(mark, stopped, latest, provider)), nil
}

// ContractMailBlocks answers ContractMailBlock for the first mail to each
// address, in one read: a candidate has no earlier mail whose outcome would
// make the provider worth asking (#4680).
func (p CareContractPost) ContractMailBlocks(ctx context.Context, organizationID int64, addresses []string) (map[string]string, error) {
	listed, stopped, err := p.Outcomes.TraegerMailHoldsOf(ctx, organizationID, addresses)
	if err != nil {
		return nil, err
	}
	blocks := map[string]string{}
	for _, address := range addresses {
		key := deliveryContract.NormalizedEmailAddress(address)
		mark := deliveryContract.EmailAddressMark("")
		if listed[key] {
			mark = deliveryContract.EmailMarkFailed
		}
		if block := deliveryContract.EmailSendBlockOf(mark, stopped[key], "", ""); block != "" {
			blocks[address] = string(block)
		}
	}
	return blocks, nil
}

// ShareMailBlockLimit lets one request share the limits of the provider's read.
func (CareContractPost) ShareMailBlockLimit(ctx context.Context) context.Context {
	return deliveryContract.WithProviderBlockBudget(ctx)
}

func (p CareContractPost) ContractMailStates(ctx context.Context, organizationID, contractID int64) ([]carecontract.MailState, error) {
	statuses, err := p.Delivery.TraegerEmailStatuses(ctx, organizationID,
		deliveryContract.RelatedEntity{Type: carecontract.MailRelatedContract, ID: contractID})
	if err != nil {
		return nil, err
	}
	states := make([]carecontract.MailState, len(statuses))
	for i, status := range statuses {
		states[i] = carecontract.MailState{OutboxID: status.ID, State: contractMailState(status.Outcome), Outcome: string(status.Outcome)}
	}
	return states, nil
}

// CareContractRuntime opens a school's tenant transaction or the Träger
// transaction, never both at once (ADR 0105).
type CareContractRuntime struct{}

func (CareContractRuntime) WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	school, err := tenant.NewTenantID(schoolID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, school, fn)
}

func (CareContractRuntime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(ctx, organizationID, fn)
}

// CareContractLinks are the one-time link tokens with the base of the
// parents portal the mails link to (#4679).
func CareContractLinks(parentsURL string) carecontract.SigningLinks {
	return carecontract.SigningLinks{Tokens: CareContractTokens{}, ParentsURL: parentsURL, Fingerprint: securityruntime.Fingerprint}
}

// CareContractLetterheadText reads the Träger's letterhead text.
type CareContractLetterheadText interface {
	TraegerTexts(ctx context.Context, organizationID int64, keys []string) (map[string]string, error)
}

// CareContractSender reads the letterhead of the Träger, which names it and
// carries its address, in the caller's Träger transaction.
type CareContractSender struct {
	Settings CareContractLetterheadText
}

func (s CareContractSender) TraegerLetterhead(ctx context.Context, organizationID int64) (string, error) {
	texts, err := s.Settings.TraegerTexts(ctx, organizationID, []string{configModels.KeyBrandingLetterheadText})
	if err != nil {
		return "", err
	}
	return texts[configModels.KeyBrandingLetterheadText], nil
}

// CareContractDocumentContact is Organisation & Tenancy's read of the
// contact person and the withdrawal contact, without an account.
type CareContractDocumentContact interface {
	DocumentContact(ctx context.Context, organizationID int64) (organizationtenancy.TraegerDocumentContact, error)
}

// CareContractSigningSender is the sender of the signature: the letterhead
// and, from the Träger's master data, the phone and e-mail the withdrawal
// notice names (#6200), in the caller's Träger transaction.
type CareContractSigningSender struct {
	CareContractSender
	MasterData CareContractDocumentContact
}

func (s CareContractSigningSender) WithdrawalReach(ctx context.Context, organizationID int64) (carecontract.WithdrawalReach, error) {
	contact, err := s.MasterData.DocumentContact(ctx, organizationID)
	if err != nil {
		return carecontract.WithdrawalReach{}, err
	}
	return carecontract.WithdrawalReach{
		ContactPhone: contact.Contact.Phone, ContactEmail: contact.Contact.Email,
		WithdrawalPhone: contact.Withdrawal.Phone, WithdrawalEmail: contact.Withdrawal.Email,
	}, nil
}

// CareContractOrganizations is Organisation & Tenancy as the reminders read
// the Träger.
type CareContractOrganizations interface {
	ListOrganizations(context.Context) ([]organizationtenancy.Organization, error)
}

// CareContractSettings are the Träger settings the reminders read: the
// reminder days and the letterhead.
type CareContractSettings interface {
	CareContractLetterheadText
	TraegerInts(ctx context.Context, organizationID int64, keys []string) (map[string]int, error)
}

// CareContractRemindersJob binds the Worker's reminders of unsigned care
// contracts (#4679): the active Träger of Organisation & Tenancy, the
// Träger's setting of the reminder days, and the ports of the signature.
// The Worker's context carries no unit of work; the job attaches unit, as the
// HTTP middleware does for a request.
func CareContractRemindersJob(unit tenant.UnitOfWork, organizations CareContractOrganizations, settings CareContractSettings,
	delivery deliveryRuntime.Capability, outcomes deliveryRuntime.EmailOutcomes, parentsURL string) schedulerPorts.CareContractReminders {
	reminders := careContractCompose.NewReminders(careContractCompose.ReminderPorts{
		Organizations: careContractTraeger{organizations: organizations},
		Settings:      careContractReminderSettings{settings: settings},
		Sender:        CareContractSender{Settings: settings},
		Post:          CareContractPost{Delivery: delivery, Outcomes: outcomes}, Runtime: CareContractRuntime{}, Links: CareContractLinks(parentsURL),
	})
	return func(ctx context.Context) error {
		return reminders.RemindDue(tenant.WithUnitOfWork(ctx, unit))
	}
}

// careContractTraeger lists the active, undeleted Träger.
type careContractTraeger struct {
	organizations CareContractOrganizations
}

func (t careContractTraeger) ActiveTraegerIDs(ctx context.Context) ([]int64, error) {
	organizations, err := t.organizations.ListOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(organizations))
	for _, organization := range organizations {
		if organization.Active && organization.DeletedAt == nil {
			ids = append(ids, organization.ID)
		}
	}
	return ids, nil
}

// careContractReminderSettings reads the Träger's reminder days.
type careContractReminderSettings struct {
	settings CareContractSettings
}

func (s careContractReminderSettings) SignatureReminderDays(ctx context.Context, organizationID int64) (int, error) {
	values, err := s.settings.TraegerInts(ctx, organizationID, []string{configModels.KeyCareContractSignatureReminderDays})
	if err != nil {
		return 0, err
	}
	return values[configModels.KeyCareContractSignatureReminderDays], nil
}
