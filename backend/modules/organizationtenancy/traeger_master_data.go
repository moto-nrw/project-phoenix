package organizationtenancy

import (
	"context"
	"errors"
	"time"
)

// Master data of a Träger (#4653, spec #4611 user story 9, ADR 0112): its
// logo, the contact person for documents and its bank accounts. They belong
// to the Träger, not to one of its schools, and are read and written in the
// Träger transaction of the session's Träger (ADR 0086 section 5, ADR 0105).
// Colours and the texts of letterhead and footer are Träger settings of the
// Settings Platform (ADR 0086 section 1). Every read and change is logged in
// the same transaction.

// Limits of the Träger logo. Documents print it; PNG and JPEG are the
// formats every renderer reads.
const (
	TraegerLogoMaxBytes     = 1 << 20
	TraegerLogoMaxDimension = 2000
)

var (
	// ErrTraegerLogoNotFound reports a Träger without a logo.
	ErrTraegerLogoNotFound = errors.New("the Träger has no logo")
	// ErrTraegerLogoInvalid reports an upload that is no PNG or JPEG image
	// within the size and dimension limits.
	ErrTraegerLogoInvalid = errors.New("the logo must be a PNG or JPEG image of at most 1 MB and 2000 pixels per side")
	// ErrTraegerBankAccountNotFound reports a bank account that does not
	// exist for the Träger.
	ErrTraegerBankAccountNotFound = errors.New("bank account not found")
	// ErrTraegerBankAccountLabelTaken reports a label the Träger already
	// uses for another bank account.
	ErrTraegerBankAccountLabelTaken = errors.New("the Träger already has a bank account with this label")
)

// TraegerLogo describes the stored logo without its bytes.
type TraegerLogo struct {
	ContentType string    `json:"content_type"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	SizeBytes   int       `json:"size_bytes"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TraegerLogoImage is the stored logo with its bytes.
type TraegerLogoImage struct {
	ContentType string
	Data        []byte
	UpdatedAt   time.Time
}

// TraegerContact is the contact person documents of the Träger name.
type TraegerContact struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// TraegerBankAccount is a bank account of the Träger. At most one is the
// default; the first one is.
type TraegerBankAccount struct {
	ID            int64     `json:"id"`
	Label         string    `json:"label"`
	AccountHolder string    `json:"account_holder"`
	IBAN          string    `json:"iban"`
	BIC           string    `json:"bic"`
	BankName      string    `json:"bank_name"`
	IsDefault     bool      `json:"is_default"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TraegerBankAccountInput is a bank account as the Träger-Büro writes it.
// IsDefault makes it the default; clearing the flag of the default does
// nothing, because the Träger always keeps one.
type TraegerBankAccountInput struct {
	Label         string
	AccountHolder string
	IBAN          string
	BIC           string
	BankName      string
	IsDefault     bool
}

// TraegerProfile is the master data of a Träger. Logo is nil without one.
type TraegerProfile struct {
	Contact      TraegerContact       `json:"contact"`
	Logo         *TraegerLogo         `json:"logo"`
	BankAccounts []TraegerBankAccount `json:"bank_accounts"`
}

// TraegerMasterData lets the Träger-Büro keep the master data of its
// Träger. Every operation joins the caller's Träger transaction of
// organizationID; accountID is the acting account the log records.
type TraegerMasterData interface {
	// Profile reads the contact person, the logo description and the bank
	// accounts and logs the read.
	Profile(ctx context.Context, organizationID, accountID int64) (TraegerProfile, error)
	// LogoImage reads the stored logo.
	LogoImage(ctx context.Context, organizationID int64) (TraegerLogoImage, error)
	// ReplaceLogo checks the image and stores it in place of the old one.
	ReplaceLogo(ctx context.Context, organizationID, accountID int64, data []byte) (TraegerLogo, error)
	// RemoveLogo deletes the logo.
	RemoveLogo(ctx context.Context, organizationID, accountID int64) error
	// UpdateContact stores the contact person; empty fields clear it.
	UpdateContact(ctx context.Context, organizationID, accountID int64, contact TraegerContact) (TraegerContact, error)
	CreateBankAccount(ctx context.Context, organizationID, accountID int64, input TraegerBankAccountInput) (TraegerBankAccount, error)
	UpdateBankAccount(ctx context.Context, organizationID, accountID, bankAccountID int64, input TraegerBankAccountInput) (TraegerBankAccount, error)
	// DeleteBankAccount deletes a bank account. Deleting the default makes
	// the oldest remaining one the default.
	DeleteBankAccount(ctx context.Context, organizationID, accountID, bankAccountID int64) error
}
