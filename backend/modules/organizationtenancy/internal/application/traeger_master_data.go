package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

// Master data of a Träger (#4653, ADR 0112). The service opens no
// transaction: the Träger-Portal route runs it in the Träger transaction of
// the session's Träger (ADR 0105), and the store checks that it does. Every
// change locks the Träger's profile row first, so the default bank account
// stays unique and the log keeps the order of the changes. Every refusal is
// decided before the first write, so a refused request leaves nothing behind
// although the route commits its 4xx answers. Every read of the profile and
// every change writes the log in the same transaction; a failed log entry
// fails the operation.

var errTraegerRequired = errors.New("organization tenancy: the Träger is required")

// TraegerMasterData implements organizationtenancy.TraegerMasterData.
type TraegerMasterData struct {
	store ports.TraegerMasterDataStore
}

// NewTraegerMasterData builds the service.
func NewTraegerMasterData(store ports.TraegerMasterDataStore) *TraegerMasterData {
	if store == nil {
		panic("organization tenancy: the master data store is required")
	}
	return &TraegerMasterData{store: store}
}

var _ organizationtenancy.TraegerMasterData = (*TraegerMasterData)(nil)

// Profile reads the master data and logs the read.
func (m *TraegerMasterData) Profile(ctx context.Context, organizationID, accountID int64) (organizationtenancy.TraegerProfile, error) {
	if organizationID <= 0 {
		return organizationtenancy.TraegerProfile{}, errTraegerRequired
	}
	contact, logo, err := m.store.FindProfile(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerProfile{}, err
	}
	accounts, err := m.store.ListBankAccounts(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerProfile{}, err
	}
	read, _ := json.Marshal(map[string]int{"bank_accounts": len(accounts)})
	if err := m.audit(ctx, organizationID, accountID, domain.TraegerAuditSubjectMasterData, 0, domain.TraegerAuditActionRead, nil, read); err != nil {
		return organizationtenancy.TraegerProfile{}, err
	}
	return organizationtenancy.TraegerProfile{Contact: contact, Logo: logo, BankAccounts: accounts}, nil
}

// LogoImage reads the stored logo.
func (m *TraegerMasterData) LogoImage(ctx context.Context, organizationID int64) (organizationtenancy.TraegerLogoImage, error) {
	if organizationID <= 0 {
		return organizationtenancy.TraegerLogoImage{}, errTraegerRequired
	}
	logo, found, err := m.store.LogoImage(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerLogoImage{}, err
	}
	if !found {
		return organizationtenancy.TraegerLogoImage{}, organizationtenancy.ErrTraegerLogoNotFound
	}
	return logo, nil
}

// ReplaceLogo checks the image and stores it in place of the old one.
func (m *TraegerMasterData) ReplaceLogo(ctx context.Context, organizationID, accountID int64, data []byte) (organizationtenancy.TraegerLogo, error) {
	if organizationID <= 0 {
		return organizationtenancy.TraegerLogo{}, errTraegerRequired
	}
	checked, ok := domain.CheckLogo(data, organizationtenancy.TraegerLogoMaxBytes, organizationtenancy.TraegerLogoMaxDimension)
	if !ok {
		return organizationtenancy.TraegerLogo{}, organizationtenancy.ErrTraegerLogoInvalid
	}
	_, old, err := m.store.LockProfile(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerLogo{}, err
	}
	logo, err := m.store.SaveLogo(ctx, organizationID, accountID, domain.TraegerLogoWrite{
		ContentType: checked.ContentType, Data: data, Width: checked.Width, Height: checked.Height,
	})
	if err != nil {
		return organizationtenancy.TraegerLogo{}, err
	}
	if err := m.audit(ctx, organizationID, accountID, domain.TraegerAuditSubjectLogo, 0, domain.TraegerAuditActionSet, logoAuditValue(old), logoAuditValue(&logo)); err != nil {
		return organizationtenancy.TraegerLogo{}, err
	}
	return logo, nil
}

// RemoveLogo deletes the logo.
func (m *TraegerMasterData) RemoveLogo(ctx context.Context, organizationID, accountID int64) error {
	if organizationID <= 0 {
		return errTraegerRequired
	}
	_, old, err := m.store.LockProfile(ctx, organizationID)
	if err != nil {
		return err
	}
	if old == nil {
		return organizationtenancy.ErrTraegerLogoNotFound
	}
	if err := m.store.ClearLogo(ctx, organizationID, accountID); err != nil {
		return err
	}
	return m.audit(ctx, organizationID, accountID, domain.TraegerAuditSubjectLogo, 0, domain.TraegerAuditActionDelete, logoAuditValue(old), nil)
}

// UpdateContact stores the contact person; empty fields clear it.
func (m *TraegerMasterData) UpdateContact(ctx context.Context, organizationID, accountID int64, contact organizationtenancy.TraegerContact) (organizationtenancy.TraegerContact, error) {
	if organizationID <= 0 {
		return organizationtenancy.TraegerContact{}, errTraegerRequired
	}
	contact = organizationtenancy.TraegerContact{
		Name: strings.TrimSpace(contact.Name), Phone: strings.TrimSpace(contact.Phone), Email: strings.ToLower(strings.TrimSpace(contact.Email)),
	}
	if err := validateContact(contact); err != nil {
		return organizationtenancy.TraegerContact{}, err
	}
	old, _, err := m.store.LockProfile(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerContact{}, err
	}
	if err := m.store.SaveContact(ctx, organizationID, accountID, contact); err != nil {
		return organizationtenancy.TraegerContact{}, err
	}
	if err := m.audit(ctx, organizationID, accountID, domain.TraegerAuditSubjectContact, 0, domain.TraegerAuditActionSet, mustJSON(old), mustJSON(contact)); err != nil {
		return organizationtenancy.TraegerContact{}, err
	}
	return contact, nil
}

// CreateBankAccount adds a bank account. The first one of the Träger is the
// default; a new default takes the flag from the old one.
func (m *TraegerMasterData) CreateBankAccount(ctx context.Context, organizationID, accountID int64, input organizationtenancy.TraegerBankAccountInput) (organizationtenancy.TraegerBankAccount, error) {
	if organizationID <= 0 {
		return organizationtenancy.TraegerBankAccount{}, errTraegerRequired
	}
	input, err := normalizeBankAccount(input)
	if err != nil {
		return organizationtenancy.TraegerBankAccount{}, err
	}
	existing, err := m.lockedBankAccounts(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerBankAccount{}, err
	}
	if labelTaken(existing, input.Label, 0) {
		return organizationtenancy.TraegerBankAccount{}, organizationtenancy.ErrTraegerBankAccountLabelTaken
	}
	input.IsDefault = input.IsDefault || len(existing) == 0
	if input.IsDefault && len(existing) > 0 {
		if err := m.store.ClearDefaultBankAccount(ctx, organizationID); err != nil {
			return organizationtenancy.TraegerBankAccount{}, err
		}
	}
	created, err := m.store.InsertBankAccount(ctx, organizationID, accountID, input)
	if err != nil {
		return organizationtenancy.TraegerBankAccount{}, err
	}
	if err := m.audit(ctx, organizationID, accountID, domain.TraegerAuditSubjectBankAccount, created.ID, domain.TraegerAuditActionCreate, nil, bankAccountAuditValue(&created)); err != nil {
		return organizationtenancy.TraegerBankAccount{}, err
	}
	return created, nil
}

// UpdateBankAccount changes a bank account. The default stays the default:
// the Träger chooses another one instead.
func (m *TraegerMasterData) UpdateBankAccount(ctx context.Context, organizationID, accountID, bankAccountID int64, input organizationtenancy.TraegerBankAccountInput) (organizationtenancy.TraegerBankAccount, error) {
	if organizationID <= 0 {
		return organizationtenancy.TraegerBankAccount{}, errTraegerRequired
	}
	input, err := normalizeBankAccount(input)
	if err != nil {
		return organizationtenancy.TraegerBankAccount{}, err
	}
	existing, err := m.lockedBankAccounts(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerBankAccount{}, err
	}
	old := findBankAccount(existing, bankAccountID)
	if old == nil {
		return organizationtenancy.TraegerBankAccount{}, organizationtenancy.ErrTraegerBankAccountNotFound
	}
	if labelTaken(existing, input.Label, bankAccountID) {
		return organizationtenancy.TraegerBankAccount{}, organizationtenancy.ErrTraegerBankAccountLabelTaken
	}
	input.IsDefault = input.IsDefault || old.IsDefault
	if input.IsDefault && !old.IsDefault {
		if err := m.store.ClearDefaultBankAccount(ctx, organizationID); err != nil {
			return organizationtenancy.TraegerBankAccount{}, err
		}
	}
	updated, err := m.store.UpdateBankAccount(ctx, organizationID, accountID, bankAccountID, input)
	if err != nil {
		return organizationtenancy.TraegerBankAccount{}, err
	}
	if err := m.audit(ctx, organizationID, accountID, domain.TraegerAuditSubjectBankAccount, bankAccountID, domain.TraegerAuditActionUpdate,
		bankAccountAuditValue(old), bankAccountAuditValue(&updated)); err != nil {
		return organizationtenancy.TraegerBankAccount{}, err
	}
	return updated, nil
}

// DeleteBankAccount deletes a bank account. Deleting the default makes the
// oldest remaining one the default.
func (m *TraegerMasterData) DeleteBankAccount(ctx context.Context, organizationID, accountID, bankAccountID int64) error {
	if organizationID <= 0 {
		return errTraegerRequired
	}
	existing, err := m.lockedBankAccounts(ctx, organizationID)
	if err != nil {
		return err
	}
	old := findBankAccount(existing, bankAccountID)
	if old == nil {
		return organizationtenancy.ErrTraegerBankAccountNotFound
	}
	if err := m.store.DeleteBankAccount(ctx, organizationID, bankAccountID); err != nil {
		return err
	}
	if err := m.audit(ctx, organizationID, accountID, domain.TraegerAuditSubjectBankAccount, bankAccountID, domain.TraegerAuditActionDelete, bankAccountAuditValue(old), nil); err != nil {
		return err
	}
	if !old.IsDefault {
		return nil
	}
	return m.promoteOldestBankAccount(ctx, organizationID, accountID, existing, bankAccountID)
}

// promoteOldestBankAccount makes the oldest remaining bank account the
// default after the default was deleted.
func (m *TraegerMasterData) promoteOldestBankAccount(ctx context.Context, organizationID, accountID int64, existing []organizationtenancy.TraegerBankAccount, deletedID int64) error {
	var next *organizationtenancy.TraegerBankAccount
	for i := range existing {
		if existing[i].ID != deletedID && (next == nil || existing[i].ID < next.ID) {
			next = &existing[i]
		}
	}
	if next == nil {
		return nil
	}
	promoted, err := m.store.UpdateBankAccount(ctx, organizationID, accountID, next.ID, organizationtenancy.TraegerBankAccountInput{
		Label: next.Label, AccountHolder: next.AccountHolder, IBAN: next.IBAN, BIC: next.BIC, BankName: next.BankName, IsDefault: true,
	})
	if err != nil {
		return err
	}
	return m.audit(ctx, organizationID, accountID, domain.TraegerAuditSubjectBankAccount, next.ID, domain.TraegerAuditActionUpdate,
		bankAccountAuditValue(next), bankAccountAuditValue(&promoted))
}

// lockedBankAccounts locks the Träger's profile row, then reads its bank
// accounts.
func (m *TraegerMasterData) lockedBankAccounts(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerBankAccount, error) {
	if _, _, err := m.store.LockProfile(ctx, organizationID); err != nil {
		return nil, err
	}
	return m.store.ListBankAccounts(ctx, organizationID)
}

func (m *TraegerMasterData) audit(ctx context.Context, organizationID, accountID int64, subject string, subjectID int64, action string, oldValue, newValue json.RawMessage) error {
	err := m.store.AppendAudit(ctx, domain.TraegerMasterDataAuditEntry{
		OrganizationID: organizationID, Subject: subject, SubjectID: subjectID, Action: action,
		OldValue: oldValue, NewValue: newValue, ActorAccountID: accountID,
	})
	if err != nil {
		return fmt.Errorf("organization tenancy: log Träger master data %s %s: %w", subject, action, err)
	}
	return nil
}

// labelTaken reports whether another bank account of the Träger carries the
// label, compared without case like the unique index.
func labelTaken(accounts []organizationtenancy.TraegerBankAccount, label string, exceptID int64) bool {
	for _, account := range accounts {
		if account.ID != exceptID && strings.EqualFold(account.Label, label) {
			return true
		}
	}
	return false
}

func findBankAccount(accounts []organizationtenancy.TraegerBankAccount, id int64) *organizationtenancy.TraegerBankAccount {
	for i := range accounts {
		if accounts[i].ID == id {
			return &accounts[i]
		}
	}
	return nil
}

// Field limits of the master data; the table checks the same.
const (
	maxContactName      = 120
	maxContactPhone     = 40
	maxContactEmail     = 254
	maxBankAccountLabel = 80
	maxAccountHolder    = 140
	maxBankName         = 120
)

var (
	errTooLong       = errors.New("is too long")
	errRequiredField = errors.New("is required")
	errIBANInvalid   = errors.New("is not a valid IBAN")
	errBICInvalid    = errors.New("is not a valid BIC")
	errPhoneInvalid  = errors.New("is not a valid phone number")
)

func validateContact(contact organizationtenancy.TraegerContact) error {
	fields := validation.Errors{}
	if utf8.RuneCountInString(contact.Name) > maxContactName {
		fields["name"] = errTooLong
	}
	if contact.Phone != "" && (utf8.RuneCountInString(contact.Phone) > maxContactPhone || !domain.ValidPhone(contact.Phone)) {
		fields["phone"] = errPhoneInvalid
	}
	if contact.Email != "" {
		if err := validation.Validate(contact.Email, is.Email, validation.Length(0, maxContactEmail)); err != nil {
			fields["email"] = err
		}
	}
	return fields.Filter()
}

func normalizeBankAccount(input organizationtenancy.TraegerBankAccountInput) (organizationtenancy.TraegerBankAccountInput, error) {
	input.Label = strings.TrimSpace(input.Label)
	input.AccountHolder = strings.TrimSpace(input.AccountHolder)
	input.BankName = strings.TrimSpace(input.BankName)
	input.IBAN = domain.CompactCode(input.IBAN)
	input.BIC = domain.CompactCode(input.BIC)
	fields := validation.Errors{}
	requiredText(fields, "label", input.Label, maxBankAccountLabel)
	requiredText(fields, "account_holder", input.AccountHolder, maxAccountHolder)
	if !domain.ValidIBAN(input.IBAN) {
		fields["iban"] = errIBANInvalid
	}
	if input.BIC != "" && !domain.ValidBIC(input.BIC) {
		fields["bic"] = errBICInvalid
	}
	if utf8.RuneCountInString(input.BankName) > maxBankName {
		fields["bank_name"] = errTooLong
	}
	return input, fields.Filter()
}

func requiredText(fields validation.Errors, name, value string, limit int) {
	switch {
	case value == "":
		fields[name] = errRequiredField
	case utf8.RuneCountInString(value) > limit:
		fields[name] = errTooLong
	}
}

func logoAuditValue(logo *organizationtenancy.TraegerLogo) json.RawMessage {
	if logo == nil {
		return nil
	}
	return mustJSON(map[string]any{
		"content_type": logo.ContentType, "width": logo.Width, "height": logo.Height,
		"size_bytes": logo.SizeBytes, "updated_at": logo.UpdatedAt,
	})
}

func bankAccountAuditValue(account *organizationtenancy.TraegerBankAccount) json.RawMessage {
	return mustJSON(map[string]any{
		"label": account.Label, "account_holder": account.AccountHolder, "iban": account.IBAN,
		"bic": account.BIC, "bank_name": account.BankName, "is_default": account.IsDefault,
	})
}

// mustJSON encodes values of plain strings, numbers and booleans, which
// cannot fail.
func mustJSON(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}
