package application

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// fakeParentMandateStore keeps the mandates, proofs and trail of one Träger
// in memory; methods the parents' flow does not use panic.
type fakeParentMandateStore struct {
	MandateStore
	creditor     *parentbilling.SEPACreditor
	mandates     []parentbilling.SEPAMandate
	sealed       map[int64][]byte
	consents     map[int64]parentbilling.SEPAMandateConsent
	events       []string
	listCalls    int
	consentReads int
}

func (s *fakeParentMandateStore) SEPACreditor(context.Context, int64) (*parentbilling.SEPACreditor, error) {
	return s.creditor, nil
}

func (s *fakeParentMandateStore) SEPAMandates(_ context.Context, _ int64, family string) ([]parentbilling.SEPAMandate, error) {
	s.listCalls++
	var out []parentbilling.SEPAMandate
	for _, mandate := range s.mandates {
		if mandate.FamilyEmail == family {
			out = append(out, mandate)
		}
	}
	return out, nil
}

func (s *fakeParentMandateStore) SEPAMandate(_ context.Context, _ int64, id int64, _ bool) (parentbilling.SEPAMandate, error) {
	for _, mandate := range s.mandates {
		if mandate.ID == id {
			return mandate, nil
		}
	}
	return parentbilling.SEPAMandate{}, parentbilling.ErrSEPAMandateNotFound
}

func (s *fakeParentMandateStore) InsertSEPAMandate(_ context.Context, _ int64, _ int64, mandate parentbilling.SealedSEPAMandate) (parentbilling.SEPAMandate, error) {
	for _, existing := range s.mandates {
		if existing.FamilyEmail == mandate.Mandate.FamilyEmail && existing.InForce() {
			return parentbilling.SEPAMandate{}, parentbilling.ErrSEPAMandateInForce
		}
	}
	stored := mandate.Mandate
	stored.ID = int64(len(s.mandates) + 1)
	stored.HolderSealed = mandate.Holder
	s.mandates = append(s.mandates, stored)
	if s.sealed == nil {
		s.sealed = map[int64][]byte{}
	}
	s.sealed[stored.ID] = mandate.IBAN
	return stored, nil
}

func (s *fakeParentMandateStore) RevokeSEPAMandate(_ context.Context, _, _, id int64, revokedOn calendar.Date) (parentbilling.SEPAMandate, error) {
	for i := range s.mandates {
		if s.mandates[i].ID == id && s.mandates[i].InForce() {
			s.mandates[i].RevokedOn = &revokedOn
			return s.mandates[i], nil
		}
	}
	return parentbilling.SEPAMandate{}, parentbilling.ErrSEPAMandateNotFound
}

func (s *fakeParentMandateStore) InsertSEPAMandateEvents(_ context.Context, _, _ int64, action, source string, _ []string,
	mandates []parentbilling.SEPAMandate,
) error {
	for range mandates {
		s.events = append(s.events, action+"/"+source)
	}
	return nil
}

func (s *fakeParentMandateStore) InsertSEPAMandateConsent(_ context.Context, _ int64, consent parentbilling.SEPAMandateConsent) error {
	if s.consents == nil {
		s.consents = map[int64]parentbilling.SEPAMandateConsent{}
	}
	s.consents[consent.MandateID] = consent
	return nil
}

func (s *fakeParentMandateStore) SEPAMandateConsent(_ context.Context, _, mandateID int64) (*parentbilling.SEPAMandateConsent, error) {
	s.consentReads++
	consent, ok := s.consents[mandateID]
	if !ok {
		return nil, nil
	}
	return &consent, nil
}

// reversingCipher seals by reversing the bytes after the context: enough to
// prove that the store never sees the plain value.
type reversingCipher struct{}

func (reversingCipher) BankDataRing() {}

func (reversingCipher) Seal(plaintext, context []byte) ([]byte, error) {
	sealed := slices.Clone(plaintext)
	slices.Reverse(sealed)
	return append(slices.Clone(context), sealed...), nil
}

func (reversingCipher) Open(sealed, context []byte) ([]byte, error) {
	plain := slices.Clone(sealed[len(context):])
	slices.Reverse(plain)
	return plain, nil
}

type fakeParties struct{}

func (fakeParties) Recipients(_ context.Context, _ int64, ids []int64) (map[int64]parentbilling.CareRecipient, error) {
	return map[int64]parentbilling.CareRecipient{ids[0]: {FirstName: "Eva", LastName: "Muster", Email: "eva@example.org"}}, nil
}

type fakeTerms struct {
	terms parentbilling.OnlineMandateTerms
}

func (f *fakeTerms) OnlineMandateTerms(context.Context, int64) (parentbilling.OnlineMandateTerms, error) {
	return f.terms, nil
}

// fnvFingerprints stands in for Security Runtime's SHA-256: 64 hex digits
// that differ for different texts.
type fnvFingerprints struct{}

func (fnvFingerprints) Content(content []byte) string {
	hash := fnv.New64a()
	_, _ = hash.Write(content)
	return fmt.Sprintf("%048x%016x", len(content), hash.Sum64())
}

func textChecksum(text string) string { return fnvFingerprints{}.Content([]byte(text)) }

const parentTestIBAN = "DE89370400440532013000"

var mandateDay = calendar.NewDate(2026, 11, 9)

func newParentMandatesForTest() (*ParentMandates, *fakeParentMandateStore, *fakeTerms) {
	store := &fakeParentMandateStore{creditor: &parentbilling.SEPACreditor{CreditorID: "DE98ZZZ09999999999"}}
	terms := &fakeTerms{terms: parentbilling.OnlineMandateTerms{Allowed: true, CreditorAddress: "Jugendring e.V.\nHauptstraße 1\n75175 Pforzheim"}}
	return NewParentMandates(store, fakeParties{}, reversingCipher{}, terms, fnvFingerprints{}, func() calendar.Date { return mandateDay }), store, terms
}

var mandateReader = parentbilling.ParentMandateReader{AccountID: 41, Email: " Eva@Example.org ", ContractID: 7, CreditorName: "Jugendring Muster"}

func parentInput() parentbilling.ParentMandateInput {
	return parentbilling.ParentMandateInput{AccountHolder: "Eva Muster", IBAN: "de89 3704 0044 0532 0130 00", Street: "Gartenweg 2",
		PostalCode: "75175", City: "Pforzheim", HolderConfirmed: true}
}

// The text names everything the EPC rulebook asks of a mandate and binds it
// by its checksum; the confirmation of exactly that text stores the
// recurring mandate with its sealed IBAN, the proof and the trail.
func TestParentMandateGrantStoresTheConfirmedText(t *testing.T) {
	t.Parallel()
	mandates, store, _ := newParentMandatesForTest()
	ctx := context.Background()
	input := parentInput()
	text, err := mandates.PreviewParentSEPAMandate(ctx, 3, mandateReader, input)
	require.NoError(t, err)
	assert.Equal(t, parentbilling.MandateTextVersion, text.Version)
	assert.True(t, strings.HasPrefix(text.Reference, "MOTO-7-20261109-"), text.Reference)
	for _, part := range []string{"SEPA-Lastschriftmandat", "Zahlungsempfänger: Jugendring Muster",
		"Anschrift: Jugendring e.V., Hauptstraße 1, 75175 Pforzheim", "Gläubiger-Identifikationsnummer: DE98ZZZ09999999999",
		"Mandatsreferenz: " + text.Reference, "Wiederkehrende Zahlung", "acht Wochen", "Kontoinhaber: Eva Muster",
		"Anschrift: Gartenweg 2, 75175 Pforzheim", "IBAN: DE•• •••• •••• •••• ••30 00", "Ort, Datum: Pforzheim, 09.11.2026"} {
		assert.Contains(t, text.Text, part)
	}
	assert.NotContains(t, text.Text, parentTestIBAN, "the text shows the mask")
	assert.Equal(t, textChecksum(text.Text), text.TextSHA256)

	input.Reference, input.TextSHA256 = text.Reference, strings.ToUpper(text.TextSHA256)
	view, err := mandates.GrantParentSEPAMandate(ctx, 3, mandateReader, input)
	require.NoError(t, err)
	require.Len(t, store.mandates, 1)
	stored := store.mandates[0]
	assert.Equal(t, "eva@example.org", stored.FamilyEmail, "the family of the billing recipient")
	assert.Equal(t, parentbilling.MandateFromParent, stored.Source)
	assert.Equal(t, parentbilling.MandateRecurring, stored.Kind)
	assert.Equal(t, mandateDay, stored.SignedOn)
	assert.Equal(t, "DE98ZZZ09999999999", stored.CreditorID)
	assert.NotContains(t, string(store.sealed[stored.ID]), parentTestIBAN, "the store never sees the plain IBAN")
	consent := store.consents[stored.ID]
	assert.Equal(t, text.TextSHA256, consent.TextSHA256)
	assert.Equal(t, parentbilling.ChannelParentAccount, consent.Channel)
	assert.True(t, consent.HolderConfirmed)
	assert.Equal(t, int64(41), consent.AccountID)
	assert.Equal(t, []string{"create/parent"}, store.events)

	require.NotNil(t, view.Mandate)
	require.NotNil(t, view.Mandate.Proof)
	assert.Equal(t, text.Text, view.Mandate.Proof.Text, "the copy for the parents is the confirmed text")
	wire, err := json.Marshal(view)
	require.NoError(t, err)
	assert.NotContains(t, string(wire), parentTestIBAN, "no answer carries the IBAN")
}

// Another text than the one moto renders now is never stored: a changed
// checksum, an account changed after the preview, a reference of another
// contract or day, a missing holder confirmation or a missing creditor.
func TestParentMandateRefusesAnotherText(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mandates, _, _ := newParentMandatesForTest()
	text, err := mandates.PreviewParentSEPAMandate(ctx, 3, mandateReader, parentInput())
	require.NoError(t, err)
	confirmed := func(change func(*parentbilling.ParentMandateInput)) parentbilling.ParentMandateInput {
		input := parentInput()
		input.Reference, input.TextSHA256 = text.Reference, text.TextSHA256
		change(&input)
		return input
	}
	for name, input := range map[string]parentbilling.ParentMandateInput{
		"checksum":      confirmed(func(i *parentbilling.ParentMandateInput) { i.TextSHA256 = strings.Repeat("0", 64) }),
		"other account": confirmed(func(i *parentbilling.ParentMandateInput) { i.IBAN = "DE02120300000000202051" }),
		"other holder":  confirmed(func(i *parentbilling.ParentMandateInput) { i.AccountHolder = "Jan Muster" }),
		"other contract": confirmed(func(i *parentbilling.ParentMandateInput) {
			i.Reference = strings.Replace(i.Reference, "MOTO-7-", "MOTO-8-", 1)
		}),
		"own reference": confirmed(func(i *parentbilling.ParentMandateInput) { i.Reference = "EIGENE-REFERENZ" }),
	} {
		store := &fakeParentMandateStore{creditor: &parentbilling.SEPACreditor{CreditorID: "DE98ZZZ09999999999"}}
		check := NewParentMandates(store, fakeParties{}, reversingCipher{}, &fakeTerms{terms: parentbilling.OnlineMandateTerms{
			Allowed: true, CreditorAddress: "Jugendring e.V.\nHauptstraße 1\n75175 Pforzheim"}}, fnvFingerprints{}, func() calendar.Date { return mandateDay })
		_, err := check.GrantParentSEPAMandate(ctx, 3, mandateReader, input)
		require.ErrorIs(t, err, parentbilling.ErrMandateTextChanged, name)
		assert.Empty(t, store.mandates, name)
	}

	unconfirmed := confirmed(func(i *parentbilling.ParentMandateInput) { i.HolderConfirmed = false })
	_, err = mandates.GrantParentSEPAMandate(ctx, 3, mandateReader, unconfirmed)
	var fields validation.Errors
	require.ErrorAs(t, err, &fields)
	assert.Contains(t, fields, "holder_confirmed")

	invalid := parentInput()
	invalid.IBAN, invalid.PostalCode = "DE00123", "7517"
	_, err = mandates.PreviewParentSEPAMandate(ctx, 3, mandateReader, invalid)
	require.ErrorAs(t, err, &fields)
	assert.Contains(t, fields, "iban")
	assert.Contains(t, fields, "postal_code")
	assert.NotContains(t, err.Error(), "DE00123", "an error never repeats the account")

	missing, store, _ := newParentMandatesForTest()
	store.creditor = nil
	_, err = missing.PreviewParentSEPAMandate(ctx, 3, mandateReader, parentInput())
	require.ErrorIs(t, err, parentbilling.ErrSEPACreditorMissing)
	view, err := missing.ParentSEPAMandate(ctx, 3, mandateReader)
	require.NoError(t, err)
	assert.True(t, view.CreditorMissing)
}

// Switched off, the read answers nothing but that and records nothing; the
// writes are refused. Switched on, a read of a mandate lands in the trail
// and the revocation ends it today.
func TestParentMandateSwitchAndRevocation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mandates, store, terms := newParentMandatesForTest()
	text, err := mandates.PreviewParentSEPAMandate(ctx, 3, mandateReader, parentInput())
	require.NoError(t, err)
	input := parentInput()
	input.Reference, input.TextSHA256 = text.Reference, text.TextSHA256
	_, err = mandates.GrantParentSEPAMandate(ctx, 3, mandateReader, input)
	require.NoError(t, err)

	terms.terms = parentbilling.OnlineMandateTerms{}
	view, err := mandates.ParentSEPAMandate(ctx, 3, mandateReader)
	require.NoError(t, err)
	assert.Equal(t, parentbilling.ParentMandate{}, view)
	_, err = mandates.RevokeParentSEPAMandate(ctx, 3, mandateReader)
	require.ErrorIs(t, err, parentbilling.ErrOnlineMandateDisabled)
	_, err = mandates.PreviewParentSEPAMandate(ctx, 3, mandateReader, input)
	require.ErrorIs(t, err, parentbilling.ErrOnlineMandateDisabled)
	assert.Equal(t, []string{"create/parent"}, store.events, "nothing recorded while off")

	terms.terms = parentbilling.OnlineMandateTerms{Allowed: true, CreditorAddress: "Hauptstraße 1"}
	view, err = mandates.ParentSEPAMandate(ctx, 3, mandateReader)
	require.NoError(t, err)
	require.NotNil(t, view.Mandate)
	assert.Equal(t, "Eva Muster", view.Mandate.AccountHolder)
	revoked, err := mandates.RevokeParentSEPAMandate(ctx, 3, mandateReader)
	require.NoError(t, err)
	require.NotNil(t, revoked.Mandate.RevokedOn)
	assert.Equal(t, mandateDay, *revoked.Mandate.RevokedOn)
	assert.Equal(t, []string{"create/parent", "read/parent", "revoke/parent"}, store.events)

	other := mandateReader
	other.Email = "andere@example.org"
	_, err = mandates.RevokeParentSEPAMandate(ctx, 3, other)
	require.ErrorIs(t, err, parentbilling.ErrSEPAMandateNotFound, "another family has no mandate here")
}

func TestParentMandateProofSurvivesAccountChanges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mandates, store, _ := newParentMandatesForTest()
	input := parentInput()
	preview, err := mandates.PreviewParentSEPAMandate(ctx, 3, mandateReader, input)
	require.NoError(t, err)
	input.Reference, input.TextSHA256 = preview.Reference, preview.TextSHA256
	_, err = mandates.GrantParentSEPAMandate(ctx, 3, mandateReader, input)
	require.NoError(t, err)

	// The office may correct the live account; the proof must still name
	// exactly what the parent confirmed before that correction.
	store.mandates[0].HolderSealed, err = reversingCipher{}.Seal([]byte("Jan Muster"), holderContext(3, preview.Reference))
	require.NoError(t, err)
	store.mandates[0].IBANMasked = "DE•• •••• •••• •••• ••20 51"
	store.mandates[0].BIC = "COBADEFFXXX"
	view, err := mandates.ParentSEPAMandate(ctx, 3, mandateReader)
	require.NoError(t, err)
	require.NotNil(t, view.Mandate.Proof)
	assert.Equal(t, "Jan Muster", view.Mandate.AccountHolder)
	assert.Equal(t, preview.Text, view.Mandate.Proof.Text)
	assert.Equal(t, preview.TextSHA256, textChecksum(view.Mandate.Proof.Text))
}

func TestParentMandateShowsCollectionEligibility(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(*parentbilling.SEPAMandate){
		"eligible": func(*parentbilling.SEPAMandate) {},
		"expired":  func(m *parentbilling.SEPAMandate) { m.SignedOn = calendar.NewDate(2020, 1, 1) },
		"one-off collected": func(m *parentbilling.SEPAMandate) {
			m.Kind = parentbilling.MandateOneOff
			m.LastCollectedOn = &mandateDay
		},
		"creditor changed": func(m *parentbilling.SEPAMandate) { m.CreditorID = "DE98ABC09999999999" },
		"future signature": func(m *parentbilling.SEPAMandate) { m.SignedOn = mandateDay.AddDays(1) },
	} {
		t.Run(name, func(t *testing.T) {
			mandates, store, _ := newParentMandatesForTest()
			ctx := context.Background()
			input := parentInput()
			text, err := mandates.PreviewParentSEPAMandate(ctx, 3, mandateReader, input)
			require.NoError(t, err)
			input.Reference, input.TextSHA256 = text.Reference, text.TextSHA256
			_, err = mandates.GrantParentSEPAMandate(ctx, 3, mandateReader, input)
			require.NoError(t, err)
			change(&store.mandates[0])
			view, err := mandates.ParentSEPAMandate(ctx, 3, mandateReader)
			require.NoError(t, err)
			require.NotNil(t, view.Mandate)
			assert.Equal(t, name == "eligible", view.Mandate.CollectionEligible)
			assert.Equal(t, text.Text, view.Mandate.Proof.Text)
			_, err = mandates.RevokeParentSEPAMandate(ctx, 3, mandateReader)
			require.NoError(t, err, "ineligible mandates can still be revoked")
		})
	}
}

type statusParties struct{ calls int }

func (f *statusParties) Recipients(_ context.Context, _ int64, ids []int64) (map[int64]parentbilling.CareRecipient, error) {
	f.calls++
	out := make(map[int64]parentbilling.CareRecipient, len(ids))
	for _, id := range ids {
		out[id] = parentbilling.CareRecipient{Email: "eva@example.org"}
	}
	out[ids[len(ids)-1]] = parentbilling.CareRecipient{Email: "other@example.org"}
	return out, nil
}

func TestParentMandateInboxStatusBatchesOnlyThePayersContracts(t *testing.T) {
	t.Parallel()
	mandates, store, terms := newParentMandatesForTest()
	parties := &statusParties{}
	mandates.contracts = parties
	// Deliberately unopened ciphertext: status must not decrypt the holder
	// or read a proof, unlike the mandate detail endpoint.
	store.mandates = []parentbilling.SEPAMandate{{FamilyEmail: "eva@example.org", SignedOn: mandateDay,
		Kind: parentbilling.MandateRecurring, CreditorID: store.creditor.CreditorID}}
	ids := make([]int64, 100)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	ctx := context.Background()
	statuses, err := mandates.ParentSEPAMandateStatuses(ctx, 3, mandateReader, ids)
	require.NoError(t, err)
	assert.Len(t, statuses, 99)
	assert.Equal(t, parentbilling.ParentMandateStatus{Enabled: true, CollectionEligible: true}, statuses[ids[0]])
	assert.NotContains(t, statuses, ids[len(ids)-1], "a contract sent to this account can name another payer")
	assert.Equal(t, 1, parties.calls)
	assert.Equal(t, 1, store.listCalls)
	assert.Zero(t, store.consentReads)
	assert.Empty(t, store.events)

	store.mandates[0].SignedOn = calendar.NewDate(2020, 1, 1)
	statuses, err = mandates.ParentSEPAMandateStatuses(ctx, 3, mandateReader, ids)
	require.NoError(t, err)
	assert.Equal(t, parentbilling.ParentMandateStatus{Enabled: true}, statuses[ids[0]])
	terms.terms.Allowed = false
	statuses, err = mandates.ParentSEPAMandateStatuses(ctx, 3, mandateReader, ids)
	require.NoError(t, err)
	assert.Empty(t, statuses)
	assert.Equal(t, 2, parties.calls, "disabled status does not load contract parties")
}
