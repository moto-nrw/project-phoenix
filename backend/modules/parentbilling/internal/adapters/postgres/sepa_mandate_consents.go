package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The proof of a SEPA mandate the parents gave online (#4696). A proof is
// written once in the Träger transaction of its mandate; the database
// refuses any change of it (refuse_sepa_mandate_consent_rewrite), and the
// request role may neither update nor delete it.

type sepaMandateConsentRow struct {
	bun.BaseModel `bun:"table:parent_billing.sepa_mandate_consents"`

	ID               int64     `bun:"id,pk,autoincrement"`
	OrganizationID   int64     `bun:"organization_id"`
	MandateID        int64     `bun:"mandate_id"`
	CareContractID   int64     `bun:"care_contract_id"`
	AccountID        *int64    `bun:"account_id"`
	Channel          string    `bun:"channel"`
	TextVersion      string    `bun:"text_version"`
	TextSHA256       string    `bun:"text_sha256"`
	TextSealed       []byte    `bun:"text_sealed"`
	HolderConfirmed  bool      `bun:"holder_confirmed"`
	CreditorName     string    `bun:"creditor_name"`
	CreditorAddress  string    `bun:"creditor_address"`
	DebtorStreet     string    `bun:"debtor_street"`
	DebtorPostalCode string    `bun:"debtor_postal_code"`
	DebtorCity       string    `bun:"debtor_city"`
	ConfirmedAt      time.Time `bun:"confirmed_at,nullzero,default:current_timestamp"`
}

// InsertSEPAMandateConsent stores the proof of an online mandate.
func (s *Store) InsertSEPAMandateConsent(ctx context.Context, organizationID int64, consent parentbilling.SEPAMandateConsent) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	row := &sepaMandateConsentRow{OrganizationID: organizationID, MandateID: consent.MandateID, CareContractID: consent.ContractID,
		AccountID: accountRef(consent.AccountID), Channel: consent.Channel, TextVersion: consent.TextVersion,
		TextSHA256: consent.TextSHA256, TextSealed: consent.TextSealed, HolderConfirmed: consent.HolderConfirmed, CreditorName: consent.CreditorName,
		CreditorAddress: consent.CreditorAddress, DebtorStreet: consent.DebtorStreet, DebtorPostalCode: consent.DebtorPostalCode,
		DebtorCity: consent.DebtorCity}
	if _, err := db.NewInsert().Model(row).ExcludeColumn("confirmed_at").Exec(ctx); err != nil {
		return fmt.Errorf("insert SEPA mandate consent: %w", err)
	}
	return nil
}

// SEPAMandateConsent reads the proof of a mandate of the Träger, nil for a
// mandate not given online.
func (s *Store) SEPAMandateConsent(ctx context.Context, organizationID, mandateID int64) (*parentbilling.SEPAMandateConsent, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []sepaMandateConsentRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("mandate_id = ?", mandateID).
		Limit(1).Scan(ctx); err != nil {
		return nil, fmt.Errorf("read SEPA mandate consent: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	consent := &parentbilling.SEPAMandateConsent{MandateID: row.MandateID, ContractID: row.CareContractID, Channel: row.Channel,
		TextVersion: row.TextVersion, TextSHA256: row.TextSHA256, TextSealed: row.TextSealed, HolderConfirmed: row.HolderConfirmed,
		CreditorName: row.CreditorName, CreditorAddress: row.CreditorAddress, DebtorStreet: row.DebtorStreet,
		DebtorPostalCode: row.DebtorPostalCode, DebtorCity: row.DebtorCity, ConfirmedAt: row.ConfirmedAt}
	if row.AccountID != nil {
		consent.AccountID = *row.AccountID
	}
	return consent, nil
}
