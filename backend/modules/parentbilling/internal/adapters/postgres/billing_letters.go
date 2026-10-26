package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The confirmation letters of the released runs (#4669, ADR 0121). The
// request role may insert and read them, never change or delete them.

// runLettersKey is the primary key that keeps a run to one set of letters.
const runLettersKey = "run_letters_pkey"

type runLettersRow struct {
	bun.BaseModel `bun:"table:parent_billing.run_letters"`

	OrganizationID     int64                                   `bun:"organization_id,pk"`
	RunID              int64                                   `bun:"run_id,pk"`
	PaymentDueOn       calendar.Date                           `bun:"payment_due_on,type:date"`
	Notes              string                                  `bun:"notes"`
	EmergencyContacts  []parentbilling.BillingEmergencyContact `bun:"emergency_contacts,type:jsonb"`
	CreatedByAccountID *int64                                  `bun:"created_by_account_id"`
	CreatedAt          time.Time                               `bun:"created_at,nullzero,default:current_timestamp"`
}

func (r runLettersRow) letters() parentbilling.BillingLetters {
	contacts := r.EmergencyContacts
	if contacts == nil {
		contacts = []parentbilling.BillingEmergencyContact{}
	}
	return parentbilling.BillingLetters{
		PaymentDueOn: r.PaymentDueOn, Notes: r.Notes, EmergencyContacts: contacts, CreatedAt: r.CreatedAt,
		CreatedByAccountID: r.CreatedByAccountID,
	}
}

type documentLetterRow struct {
	bun.BaseModel `bun:"table:parent_billing.document_letters"`

	OrganizationID int64     `bun:"organization_id,pk"`
	DocumentID     int64     `bun:"document_id,pk"`
	RunID          int64     `bun:"run_id"`
	Filename       string    `bun:"filename"`
	PDF            []byte    `bun:"pdf"`
	SHA256         string    `bun:"sha256,scanonly"`
	CreatedAt      time.Time `bun:"created_at,nullzero,default:current_timestamp"`
}

// RunLetters reads the letters of a run; nil before they are created.
func (s *Store) RunLetters(ctx context.Context, organizationID, runID int64) (*parentbilling.BillingLetters, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	row := &runLettersRow{}
	if err := db.NewSelect().Model(row).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).
		Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	letters := row.letters()
	return &letters, nil
}

// InsertRunLetters stores what the Abrechnung wrote for the letters of a
// run.
func (s *Store) InsertRunLetters(ctx context.Context, organizationID, runID, accountID int64, input parentbilling.BillingLetterInput) (parentbilling.BillingLetters, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.BillingLetters{}, err
	}
	row := &runLettersRow{
		OrganizationID: organizationID, RunID: runID, PaymentDueOn: input.PaymentDueOn, Notes: input.Notes,
		EmergencyContacts: input.EmergencyContacts, CreatedByAccountID: &accountID,
	}
	if row.EmergencyContacts == nil {
		row.EmergencyContacts = []parentbilling.BillingEmergencyContact{}
	}
	if _, err := db.NewInsert().Model(row).Returning("created_at").Exec(ctx); err != nil {
		if constraintOf(err) == runLettersKey {
			return parentbilling.BillingLetters{}, parentbilling.ErrBillingLettersExist
		}
		return parentbilling.BillingLetters{}, err
	}
	return row.letters(), nil
}

// InsertDocumentLetter stores the PDF of the letter of an issued invoice.
func (s *Store) InsertDocumentLetter(ctx context.Context, organizationID, runID, documentID int64, file parentbilling.BillingLetterFile) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	_, err = db.NewInsert().Model(&documentLetterRow{
		OrganizationID: organizationID, DocumentID: documentID, RunID: runID, Filename: file.Filename, PDF: file.Data,
	}).Exec(ctx)
	return err
}

// DocumentLetter reads the stored letter of an invoice of a run.
func (s *Store) DocumentLetter(ctx context.Context, organizationID, runID, documentID int64) (parentbilling.BillingLetterFile, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	row := &documentLetterRow{}
	if err := db.NewSelect().Model(row).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).
		Where("document_id = ?", documentID).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingLetterNotFound
		}
		return parentbilling.BillingLetterFile{}, err
	}
	return parentbilling.BillingLetterFile{Filename: row.Filename, ContentType: "application/pdf", Data: row.PDF}, nil
}
