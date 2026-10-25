package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// TraegerMasterDataStore persists the master data of a Träger (#4653, ADR
// 0112). Every method runs in the caller's Träger transaction of
// organizationID and names the Träger in its own filter again.
type TraegerMasterDataStore interface {
	// FindProfile reads the contact person and the logo description; a
	// Träger without a row has neither.
	FindProfile(ctx context.Context, organizationID int64) (organizationtenancy.TraegerContact, *organizationtenancy.TraegerLogo, error)
	// LockProfile locks the row of the Träger until the end of the
	// transaction; create makes it first where it is missing. Every change
	// of the master data takes this lock first, so changes of one Träger run
	// one after the other. A change that may be refused as not found passes
	// create false and writes nothing before it decides.
	LockProfile(ctx context.Context, organizationID int64, create bool) (organizationtenancy.TraegerContact, *organizationtenancy.TraegerLogo, error)
	SaveContact(ctx context.Context, organizationID, accountID int64, contact organizationtenancy.TraegerContact) error
	SaveLogo(ctx context.Context, organizationID, accountID int64, logo domain.TraegerLogoWrite) (organizationtenancy.TraegerLogo, error)
	ClearLogo(ctx context.Context, organizationID, accountID int64) error
	// LogoImage reads the logo with its bytes; found is false without one.
	LogoImage(ctx context.Context, organizationID int64) (organizationtenancy.TraegerLogoImage, bool, error)

	// ListBankAccounts returns the bank accounts, the default first, then
	// by creation.
	ListBankAccounts(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerBankAccount, error)
	InsertBankAccount(ctx context.Context, organizationID, accountID int64, input organizationtenancy.TraegerBankAccountInput) (organizationtenancy.TraegerBankAccount, error)
	UpdateBankAccount(ctx context.Context, organizationID, accountID, bankAccountID int64, input organizationtenancy.TraegerBankAccountInput) (organizationtenancy.TraegerBankAccount, error)
	// ClearDefaultBankAccount takes the default flag from every bank
	// account of the Träger.
	ClearDefaultBankAccount(ctx context.Context, organizationID int64) error
	DeleteBankAccount(ctx context.Context, organizationID, bankAccountID int64) error

	// AppendAudit records a read or change and deletes the Träger's entries
	// older than the retention in the same transaction.
	AppendAudit(ctx context.Context, entry domain.TraegerMasterDataAuditEntry) error
}
