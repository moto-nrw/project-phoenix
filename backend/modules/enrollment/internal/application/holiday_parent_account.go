package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The holiday registration knows the parent account (ADR 0216); the rules
// are on enrollment.HolidayParentRegistrations. The prefill seals the
// signed-in account with the field cipher that seals the health information
// and the statement of the holiday care wish, under a context of its own
// that names the Träger and the period: a proof opens only for the link it
// was made for, and a parent cannot change the account in it.

// holidayParentAccountProofVersion is the version of the sealed proof.
const holidayParentAccountProofVersion = 1

// errHolidayParentAccountLinksRequired reports a registration that asks for
// an account offer without the setup links bound.
var errHolidayParentAccountLinksRequired = errors.New("enrollment: the holiday registration needs the setup links of a parent account")

// holidayParentAccountProof is the content of the sealed proof.
type holidayParentAccountProof struct {
	Version        int   `json:"v"`
	OrganizationID int64 `json:"o"`
	PeriodID       int64 `json:"p"`
	AccountID      int64 `json:"a"`
	ExpiresAt      int64 `json:"e"`
}

func holidayParentAccountProofContext(organizationID, periodID int64) []byte {
	return fmt.Appendf(nil, "enrollment.holiday_parent_account_proof:v%d:%d:%d", holidayParentAccountProofVersion, organizationID, periodID)
}

// sealHolidayParentAccountProof seals the account for the link of the
// Träger's period.
func sealHolidayParentAccountProof(cipher enrollment.HolidayHealthCipher, organizationID, periodID, accountID int64, now time.Time) (string, error) {
	plaintext, err := json.Marshal(holidayParentAccountProof{
		Version: holidayParentAccountProofVersion, OrganizationID: organizationID, PeriodID: periodID, AccountID: accountID,
		ExpiresAt: now.Add(enrollment.HolidayParentAccountProofLifetime).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("enrollment: encode holiday parent account proof: %w", err)
	}
	sealed, err := cipher.Seal(plaintext, holidayParentAccountProofContext(organizationID, periodID))
	if err != nil {
		return "", fmt.Errorf("enrollment: seal holiday parent account proof: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// openHolidayParentAccountProof opens a proof for the link's period and
// returns its account. Anything that does not open, belongs to another link
// or expired is no proof: the registration then stays without an account,
// and nothing in the answer tells.
func openHolidayParentAccountProof(cipher enrollment.HolidayHealthCipher, organizationID, periodID int64, proof string, now time.Time) (int64, bool) {
	proof = strings.TrimSpace(proof)
	if proof == "" || cipher == nil {
		return 0, false
	}
	sealed, err := base64.RawURLEncoding.DecodeString(proof)
	if err != nil || len(sealed) == 0 {
		return 0, false
	}
	plaintext, err := cipher.Open(sealed, holidayParentAccountProofContext(organizationID, periodID))
	if err != nil {
		return 0, false
	}
	var opened holidayParentAccountProof
	if err := json.Unmarshal(plaintext, &opened); err != nil || opened.Version != holidayParentAccountProofVersion ||
		opened.OrganizationID != organizationID || opened.PeriodID != periodID || opened.AccountID <= 0 || now.Unix() > opened.ExpiresAt {
		return 0, false
	}
	return opened.AccountID, true
}

// HolidayParentAccountProofs serves enrollment.HolidayParentAccountProofs.
type HolidayParentAccountProofs struct {
	cipher enrollment.HolidayHealthCipher
	now    func() time.Time
}

var _ enrollment.HolidayParentAccountProofs = (*HolidayParentAccountProofs)(nil)

// NewHolidayParentAccountProofs builds the proofs of the prefill; a nil now
// uses the wall clock.
func NewHolidayParentAccountProofs(cipher enrollment.HolidayHealthCipher, now func() time.Time) *HolidayParentAccountProofs {
	if cipher == nil {
		panic(errHolidayHealthCipherRequired)
	}
	if now == nil {
		now = time.Now
	}
	return &HolidayParentAccountProofs{cipher: cipher, now: now}
}

// HolidayParentAccountProof seals the account for the link of the Träger's
// period.
func (p *HolidayParentAccountProofs) HolidayParentAccountProof(organizationID, periodID, accountID int64) (string, error) {
	if organizationID <= 0 || periodID <= 0 || accountID <= 0 {
		return "", errors.New("enrollment: a holiday parent account proof names a Träger, a period and an account")
	}
	return sealHolidayParentAccountProof(p.cipher, organizationID, periodID, accountID, p.now())
}

// resolveHolidayParentAccount drops whatever account a caller put in and
// keeps only the account of a proof that opens for the link (ADR 0216
// point 2).
func (h *HolidayRegistrations) resolveHolidayParentAccount(link enrollment.HolidayRegistrationLink, input enrollment.HolidayRegistrationInput) enrollment.HolidayRegistrationInput {
	proof := input.ParentAccountProof
	input.ParentAccountProof, input.ParentAccountID = "", 0
	if accountID, ok := openHolidayParentAccountProof(h.deps.Health, link.OrganizationID, link.PeriodID, proof, h.deps.Now()); ok {
		input.ParentAccountID = accountID
	}
	return input
}

// offerHolidayParentAccount enqueues the offer of a parent account in the
// registration's Träger transaction when the parent asked for one and the
// registration is not linked to an account yet (ADR 0216 point 4). The mail
// is the same for every address: it names the setup link and the sign-in
// for an address that already has an account. It returns the setup link.
func (h *HolidayRegistrations) offerHolidayParentAccount(ctx context.Context, link enrollment.HolidayRegistrationLink, period enrollment.HolidayPeriod,
	input enrollment.HolidayRegistrationInput, registrationID int64) (string, error) {
	if !input.ParentAccountRequested || input.ParentAccountID > 0 {
		return "", nil
	}
	if h.deps.ParentAccountLinks == nil {
		return "", errHolidayParentAccountLinksRequired
	}
	links, err := h.deps.ParentAccountLinks.HolidayParentAccountLink(input.Guardian.Email, link.OrganizationID)
	if err != nil {
		return "", fmt.Errorf("enrollment: issue holiday parent account link: %w", err)
	}
	payload, err := json.Marshal(enrollment.HolidayParentAccountOfferMail{
		Recipient: input.Guardian.Email, GuardianName: strings.TrimSpace(input.Guardian.FirstName + " " + input.Guardian.LastName),
		TraegerName: link.Traeger.Name, PeriodName: period.Name, SetupURL: links.SetupURL, LoginURL: links.LoginURL,
	})
	if err != nil {
		return "", fmt.Errorf("enrollment: encode holiday parent account offer: %w", err)
	}
	if err := h.deps.Mail.EnqueueTraegerMail(ctx, enrollment.TraegerMail{
		OrganizationID: link.OrganizationID, Kind: enrollment.MailKindHolidayParentAccountOffer,
		Recipient: input.Guardian.Email, Payload: payload,
		RelatedType: enrollment.MailRelatedHolidayRegistration, RelatedID: registrationID,
		IdempotencyKey: fmt.Sprintf("holiday-parent-account-offer:%d", registrationID),
	}); err != nil {
		return "", fmt.Errorf("enrollment: enqueue holiday parent account offer: %w", err)
	}
	return links.SetupURL, nil
}
