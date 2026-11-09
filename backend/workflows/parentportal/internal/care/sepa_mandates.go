package care

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The SEPA mandate of the family in the parents portal (#4696, ADR 0171
// addendum 2026-11-09). On a care contract of their inbox the parents give
// the mandate of their family, read it and revoke it. The checks are those
// of the signature in the inbox; none alone is enough:
//
//   - The relationship: parent_portal.enrollments.view on the contract's
//     child reads; parent_portal.enrollment.submit gives and revokes, the
//     right that binds the family with a contract.
//   - The payer: the contract was sent to the account's own address, and
//     the family of the mandate (the billing recipient) is that address.
//   - The contract is sent or active.
//
// The Elternabrechnung checks the Träger's switch and creditor, renders the
// mandate text, compares the confirmed checksum, seals the account and keeps
// the proof, in the Träger transaction this flow opens.

var (
	// ErrOnlineMandateDisabled reports a Träger that does not take mandates
	// in the parents portal.
	ErrOnlineMandateDisabled = errors.New("parent: the Träger takes no SEPA mandate in the parents portal")
	// ErrSEPAMandateInvalid reports an account or address that does not hold.
	ErrSEPAMandateInvalid = errors.New("parent: the SEPA mandate is invalid")
	// ErrSEPAMandateInForce reports a family that already has a mandate in
	// force; it is revoked first.
	ErrSEPAMandateInForce = errors.New("parent: the family already has a SEPA mandate in force")
	// ErrSEPAMandateTextChanged reports a confirmation of a text that is not
	// the one moto renders now.
	ErrSEPAMandateTextChanged = errors.New("parent: the confirmed mandate text is not the current one")
	// ErrSEPACreditorMissing reports a Träger without creditor identifier or
	// address: it cannot take a mandate yet.
	ErrSEPACreditorMissing = errors.New("parent: the Träger cannot take a SEPA mandate yet")
	// ErrSEPAMandateNotFound reports a family without a mandate in force.
	ErrSEPAMandateNotFound = errors.New("parent: the family has no SEPA mandate in force")
)

// SEPAMandateReader is the account on a contract the flow checked, with
// the billing recipient of the contract and the Träger's name.
type SEPAMandateReader struct {
	AccountID    int64
	Email        string
	ContractID   int64
	CreditorName string
}

// SEPAMandateInput is what the parents enter and confirm.
type SEPAMandateInput struct {
	AccountHolder   string `json:"account_holder"`
	IBAN            string `json:"iban"`
	BIC             string `json:"bic"`
	Street          string `json:"street"`
	PostalCode      string `json:"postal_code"`
	City            string `json:"city"`
	Reference       string `json:"reference"`
	TextSHA256      string `json:"text_sha256"`
	HolderConfirmed bool   `json:"holder_confirmed"`
}

// SEPAMandateText is the mandate text to confirm.
type SEPAMandateText struct {
	Version    string `json:"version"`
	Reference  string `json:"reference"`
	Text       string `json:"text"`
	TextSHA256 string `json:"text_sha256"`
}

// SEPAMandateCreditor is the creditor the text names.
type SEPAMandateCreditor struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	CreditorID string `json:"creditor_id"`
}

// SEPAMandatePrefill proposes holder and address of the contract party.
type SEPAMandatePrefill struct {
	AccountHolder string `json:"account_holder"`
	Street        string `json:"street"`
	PostalCode    string `json:"postal_code"`
	City          string `json:"city"`
}

// SEPAMandateProof is the proof of a mandate given online.
type SEPAMandateProof struct {
	Channel     string    `json:"channel"`
	Version     string    `json:"version"`
	TextSHA256  string    `json:"text_sha256"`
	Text        string    `json:"text"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

// SEPAMandateSummary is the family's mandate with the mask of its IBAN.
type SEPAMandateSummary struct {
	CollectionEligible bool              `json:"collection_eligible"`
	ID                 int64             `json:"id,string"`
	Reference          string            `json:"reference"`
	SignedOn           calendar.Date     `json:"signed_on"`
	Kind               string            `json:"kind"`
	AccountHolder      string            `json:"account_holder"`
	IBANMasked         string            `json:"iban_masked"`
	BIC                string            `json:"bic"`
	Source             string            `json:"source"`
	CreditorID         string            `json:"creditor_id"`
	RevokedOn          *calendar.Date    `json:"revoked_on"`
	Proof              *SEPAMandateProof `json:"proof"`
}

// SEPAMandateView is the mandate of the family on a contract. CanManage
// tells that the account may give and revoke it here.
type SEPAMandateView struct {
	Enabled         bool                 `json:"enabled"`
	CanManage       bool                 `json:"can_manage"`
	CreditorMissing bool                 `json:"creditor_missing"`
	Creditor        *SEPAMandateCreditor `json:"creditor"`
	Mandate         *SEPAMandateSummary  `json:"mandate"`
	Prefill         *SEPAMandatePrefill  `json:"prefill"`
}

// SEPAMandateStatus is the lightweight inbox hint for a payer's contract.
type SEPAMandateStatus struct {
	Enabled            bool `json:"enabled"`
	CollectionEligible bool `json:"collection_eligible"`
}

// SEPAMandates is the port to the Elternabrechnung's mandates; every
// operation joins the caller's Träger transaction of organizationID.
type SEPAMandates interface {
	SEPAMandateStatuses(ctx context.Context, organizationID int64, reader SEPAMandateReader, contractIDs []int64) (map[int64]SEPAMandateStatus, error)
	SEPAMandate(ctx context.Context, organizationID int64, reader SEPAMandateReader) (SEPAMandateView, error)
	PreviewSEPAMandate(ctx context.Context, organizationID int64, reader SEPAMandateReader, input SEPAMandateInput) (SEPAMandateText, error)
	GrantSEPAMandate(ctx context.Context, organizationID int64, reader SEPAMandateReader, input SEPAMandateInput) (SEPAMandateView, error)
	RevokeSEPAMandate(ctx context.Context, organizationID int64, reader SEPAMandateReader) (SEPAMandateView, error)
}

// SEPAMandate reads the mandate of the account's family on a contract of
// its inbox.
func (s *Service) SEPAMandate(ctx context.Context, accountID, organizationID, contractID int64) (SEPAMandateView, error) {
	var view SEPAMandateView
	err := s.sepaMandate(ctx, accountID, organizationID, contractID, GuardianPermissionEnrollmentsView,
		func(txCtx context.Context, reader SEPAMandateReader, canManage bool) error {
			var err error
			view, err = s.SEPAMandates.SEPAMandate(txCtx, organizationID, reader)
			view.CanManage = canManage && view.Enabled
			return err
		})
	return view, err
}

// PreviewSEPAMandate renders the mandate text the account confirms next.
func (s *Service) PreviewSEPAMandate(ctx context.Context, accountID, organizationID, contractID int64, input SEPAMandateInput) (SEPAMandateText, error) {
	var text SEPAMandateText
	err := s.sepaMandate(ctx, accountID, organizationID, contractID, GuardianPermissionEnrollmentSubmit,
		func(txCtx context.Context, reader SEPAMandateReader, _ bool) error {
			var err error
			text, err = s.SEPAMandates.PreviewSEPAMandate(txCtx, organizationID, reader, input)
			return err
		})
	return text, err
}

// GrantSEPAMandate stores the mandate the account confirmed for its family.
func (s *Service) GrantSEPAMandate(ctx context.Context, accountID, organizationID, contractID int64, input SEPAMandateInput) (SEPAMandateView, error) {
	var view SEPAMandateView
	err := s.sepaMandate(ctx, accountID, organizationID, contractID, GuardianPermissionEnrollmentSubmit,
		func(txCtx context.Context, reader SEPAMandateReader, _ bool) error {
			var err error
			view, err = s.SEPAMandates.GrantSEPAMandate(txCtx, organizationID, reader, input)
			view.CanManage = view.Enabled
			return err
		})
	return view, err
}

// RevokeSEPAMandate ends the mandate of the account's family today.
func (s *Service) RevokeSEPAMandate(ctx context.Context, accountID, organizationID, contractID int64) (SEPAMandateView, error) {
	var view SEPAMandateView
	err := s.sepaMandate(ctx, accountID, organizationID, contractID, GuardianPermissionEnrollmentSubmit,
		func(txCtx context.Context, reader SEPAMandateReader, _ bool) error {
			var err error
			view, err = s.SEPAMandates.RevokeSEPAMandate(txCtx, organizationID, reader)
			view.CanManage = view.Enabled
			return err
		})
	return view, err
}

type sepaMandateStep func(txCtx context.Context, reader SEPAMandateReader, canManage bool) error

// sepaMandate resolves the children the account holds permission for at
// the Träger, checks in that Träger's transaction that the contract was
// sent to the account for one of them and that its family is the account's
// address, and runs step. A write needs a contract the account may sign or
// end (parent_portal.enrollment.submit).
func (s *Service) sepaMandate(ctx context.Context, accountID, organizationID, contractID int64, permission string, step sepaMandateStep) error {
	if s.SEPAMandates == nil || s.ContractDocuments == nil || s.TraegerRuntime == nil || s.SchoolTraeger == nil || s.ParentAccounts == nil {
		return errors.New("parent: SEPA mandates are not configured")
	}
	inbox, traeger, err := s.sepaMandateReader(ctx, accountID, organizationID, contractID, permission)
	if err != nil {
		return err
	}
	return s.TraegerRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		canManage, err := s.familyContract(txCtx, organizationID, contractID, inbox)
		if err != nil {
			return err
		}
		if permission == GuardianPermissionEnrollmentSubmit && !canManage {
			return ErrGuardianPermissionDenied
		}
		family, err := s.ContractDocuments.FamilyEmail(txCtx, organizationID, contractID)
		if err != nil {
			return err
		}
		// The mandate belongs to the payer: the account's address must be the
		// family's, not only the address the contract was mailed to.
		if !strings.EqualFold(strings.TrimSpace(family), strings.TrimSpace(inbox.Email)) {
			return ErrBillingDocumentNotFound
		}
		return step(txCtx, SEPAMandateReader{AccountID: accountID, Email: family, ContractID: contractID, CreditorName: traeger.Name}, canManage)
	})
}

// sepaMandateReader resolves the inbox reader of the Träger like the
// children of the family do, and the Träger with its name.
func (s *Service) sepaMandateReader(ctx context.Context, accountID, organizationID, contractID int64, permission string) (BillingDocumentReader, Traeger, error) {
	var access documentAccess
	var email string
	err := s.accountDocumentAccess(ctx, accountID, func(adminCtx context.Context, children []*ChildSummary) error {
		var err error
		if access, err = s.documentAccessFor(adminCtx, children, permission); err != nil || len(access.traeger) == 0 {
			return err
		}
		email, err = s.ParentAccounts.AccountEmail(adminCtx, accountID)
		return err
	})
	if err != nil {
		return BillingDocumentReader{}, Traeger{}, err
	}
	if len(access.traeger) == 0 {
		return BillingDocumentReader{}, Traeger{}, ErrGuardianPermissionDenied
	}
	var traeger Traeger
	for _, candidate := range access.traeger {
		if candidate.ID == organizationID {
			traeger = candidate
		}
	}
	if traeger.ID == 0 || contractID <= 0 {
		return BillingDocumentReader{}, Traeger{}, ErrBillingDocumentNotFound
	}
	return BillingDocumentReader{AccountID: accountID, Email: strings.TrimSpace(email), Children: access.permittedChildren(),
		SigningChildren: access.signingChildren()}, traeger, nil
}
