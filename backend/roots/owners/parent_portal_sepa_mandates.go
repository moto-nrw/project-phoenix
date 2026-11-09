package owners

import (
	"context"
	"errors"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// The SEPA mandate of the parents (#4696): the Elternabrechnung keeps the
// mandate and its proof in the Träger transaction the parents portal opens;
// these bindings give it the Träger's switch and address from the Träger
// level of the settings, the contract party from the Betreuungsvertrag and
// the bank-data ring, and translate its values and refusals into the flow's.

// ParentMandateSettings is the Träger level of the settings as the online
// mandate reads it.
type ParentMandateSettings interface {
	TraegerBools(ctx context.Context, organizationID int64, keys []string) (map[string]bool, error)
	TraegerTexts(ctx context.Context, organizationID int64, keys []string) (map[string]string, error)
}

type parentOnlineMandateSettings struct {
	settings ParentMandateSettings
}

// OnlineMandateTerms reads the switch and the letterhead text, which names
// the Träger with its postal address (ADR 0112).
func (s parentOnlineMandateSettings) OnlineMandateTerms(ctx context.Context, organizationID int64) (parentbilling.OnlineMandateTerms, error) {
	values, err := s.settings.TraegerBools(ctx, organizationID, []string{configModels.KeyParentBillingOnlineMandate})
	if err != nil {
		return parentbilling.OnlineMandateTerms{}, err
	}
	if !values[configModels.KeyParentBillingOnlineMandate] {
		return parentbilling.OnlineMandateTerms{}, nil
	}
	texts, err := s.settings.TraegerTexts(ctx, organizationID, []string{configModels.KeyBrandingLetterheadText})
	if err != nil {
		return parentbilling.OnlineMandateTerms{}, err
	}
	return parentbilling.OnlineMandateTerms{Allowed: true, CreditorAddress: texts[configModels.KeyBrandingLetterheadText]}, nil
}

type parentMandateParties struct {
	billing carecontract.ContractBilling
}

func (p parentMandateParties) Recipients(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64]parentbilling.CareRecipient, error) {
	parties, err := p.billing.BillingRecipients(ctx, organizationID, contractIDs)
	if err != nil {
		return nil, err
	}
	recipients := make(map[int64]parentbilling.CareRecipient, len(parties))
	for contractID, party := range parties {
		recipients[contractID] = parentbilling.CareRecipient{FirstName: party.FirstName, LastName: party.LastName, Email: party.Email}
	}
	return recipients, nil
}

type parentSEPAMandates struct {
	mandates parentbilling.ParentSEPAMandates
}

// NewParentSEPAMandates binds the parents' mandates to the Elternabrechnung.
func NewParentSEPAMandates(settings ParentMandateSettings, cipher BankDataCipher) parentportal.SEPAMandatesPort {
	return parentSEPAMandates{mandates: parentBillingCompose.NewParentSEPAMandates(
		parentMandateParties{billing: careContractCompose.NewContractVersions()}, cipher, parentOnlineMandateSettings{settings: settings}, SecurityFingerprints{})}
}

func (m parentSEPAMandates) SEPAMandate(ctx context.Context, organizationID int64, reader parentportal.SEPAMandateReader) (parentportal.SEPAMandateView, error) {
	view, err := m.mandates.ParentSEPAMandate(ctx, organizationID, parentbilling.ParentMandateReader(reader))
	return parentMandateView(view), parentMandateError(err)
}

func (m parentSEPAMandates) PreviewSEPAMandate(ctx context.Context, organizationID int64, reader parentportal.SEPAMandateReader,
	input parentportal.SEPAMandateInput,
) (parentportal.SEPAMandateText, error) {
	text, err := m.mandates.PreviewParentSEPAMandate(ctx, organizationID, parentbilling.ParentMandateReader(reader), parentbilling.ParentMandateInput(input))
	return parentportal.SEPAMandateText(text), parentMandateError(err)
}

func (m parentSEPAMandates) GrantSEPAMandate(ctx context.Context, organizationID int64, reader parentportal.SEPAMandateReader,
	input parentportal.SEPAMandateInput,
) (parentportal.SEPAMandateView, error) {
	view, err := m.mandates.GrantParentSEPAMandate(ctx, organizationID, parentbilling.ParentMandateReader(reader), parentbilling.ParentMandateInput(input))
	return parentMandateView(view), parentMandateError(err)
}

func (m parentSEPAMandates) RevokeSEPAMandate(ctx context.Context, organizationID int64, reader parentportal.SEPAMandateReader) (parentportal.SEPAMandateView, error) {
	view, err := m.mandates.RevokeParentSEPAMandate(ctx, organizationID, parentbilling.ParentMandateReader(reader))
	return parentMandateView(view), parentMandateError(err)
}

func parentMandateView(view parentbilling.ParentMandate) parentportal.SEPAMandateView {
	out := parentportal.SEPAMandateView{Enabled: view.Enabled, CreditorMissing: view.CreditorMissing}
	if view.Creditor != nil {
		creditor := parentportal.SEPAMandateCreditor(*view.Creditor)
		out.Creditor = &creditor
	}
	if view.Prefill != nil {
		prefill := parentportal.SEPAMandatePrefill(*view.Prefill)
		out.Prefill = &prefill
	}
	if mandate := view.Mandate; mandate != nil {
		summary := parentportal.SEPAMandateSummary{ID: mandate.ID, Reference: mandate.Reference, SignedOn: mandate.SignedOn,
			Kind: mandate.Kind, AccountHolder: mandate.AccountHolder, IBANMasked: mandate.IBANMasked, BIC: mandate.BIC,
			Source: mandate.Source, CreditorID: mandate.CreditorID, RevokedOn: mandate.RevokedOn}
		if mandate.Proof != nil {
			proof := parentportal.SEPAMandateProof(*mandate.Proof)
			summary.Proof = &proof
		}
		out.Mandate = &summary
	}
	return out
}

func parentMandateError(err error) error {
	var fields validation.Errors
	switch {
	case err == nil:
		return nil
	case errors.As(err, &fields):
		return fmt.Errorf("%w: %w", parentportal.ErrSEPAMandateInvalid, err)
	case errors.Is(err, parentbilling.ErrOnlineMandateDisabled):
		return fmt.Errorf("%w: %w", parentportal.ErrOnlineMandateDisabled, err)
	case errors.Is(err, parentbilling.ErrSEPAMandateInForce):
		return fmt.Errorf("%w: %w", parentportal.ErrSEPAMandateInForce, err)
	case errors.Is(err, parentbilling.ErrMandateTextChanged), errors.Is(err, parentbilling.ErrSEPAMandateReferenceTaken):
		return fmt.Errorf("%w: %w", parentportal.ErrSEPAMandateTextChanged, err)
	case errors.Is(err, parentbilling.ErrSEPACreditorMissing):
		return fmt.Errorf("%w: %w", parentportal.ErrSEPACreditorMissing, err)
	case errors.Is(err, parentbilling.ErrSEPAMandateNotFound), errors.Is(err, parentbilling.ErrSEPAMandateRevoked):
		return fmt.Errorf("%w: %w", parentportal.ErrSEPAMandateNotFound, err)
	}
	return err
}
