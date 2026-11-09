package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
	transactionCompose "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/viper"
)

// What the CLI root binds for a Träger package beyond the catalog export of
// ADR 0184/0185 (ADR 0187): the PAIN.008 files Parent Billing opens with the
// bank key in a Träger transaction of its own, the creditor account from the
// Träger's master data for reconstructed files, and the operator audit of
// each run.

// newTenantExporter builds the export. A Träger package also gets its
// PAIN.008 files and its operator audit; a school export uses neither.
func newTenantExporter(db transactionCompose.Database, sealed organizationtenancy.TenantExportSealedFields,
	projected organizationtenancy.TenantExportProjectedFields, traeger bool,
) (organizationtenancy.TenantExporter, error) {
	fingerprints := organizationtenancy.ExportFingerprints{Bytes: securityruntime.Fingerprint, Reader: securityruntime.ReaderFingerprint}
	if !traeger {
		return organizationCompose.NewTenantExporter(db.Pool(), fingerprints, sealed, projected)
	}
	cipher, err := securityRuntimeCompose.NewBankDataCipher(viper.GetString("bank_data_keys"), viper.GetString("bank_data_legacy_keys"))
	if err != nil {
		return nil, errors.New("tenant export: BANK_DATA_KEYS / BANK_DATA_LEGACY_KEYS configuration invalid")
	}
	return newTraegerTenantExporter(db, sealed, projected, cipher)
}

func newTraegerTenantExporter(db transactionCompose.Database, sealed organizationtenancy.TenantExportSealedFields,
	projected organizationtenancy.TenantExportProjectedFields, cipher owners.BankDataCipher,
) (organizationtenancy.TenantExporter, error) {
	billing := parentBillingCompose.NewSEPAReturn(traegerCreditorAccounts{masterData: organizationCompose.NewTraegerMasterData()}, cipher, securityruntime.Fingerprint)
	operators, err := identityaccessCompose.NewOperatorDirectory(db.Pool())
	if err != nil {
		return nil, err
	}
	return organizationCompose.NewTraegerTenantExporter(db.Pool(), organizationtenancy.TenantExportDependencies{
		Fingerprints: organizationtenancy.ExportFingerprints{Bytes: securityruntime.Fingerprint, Reader: securityruntime.ReaderFingerprint},
		Sealed:       sealed, Projected: projected,
		SEPADebitFiles: traegerSEPADebitFiles{billing: billing},
		Audit:          tenantExportAudit{log: owners.NewOperatorAuditLog(db), operators: operators},
	})
}

// traegerSEPADebitFiles reads the PAIN.008 files in a Träger transaction of
// the Träger and hands them to the export.
type traegerSEPADebitFiles struct{ billing parentbilling.SEPAReturn }

func (b traegerSEPADebitFiles) SEPADebitFiles(ctx context.Context, organizationID int64) ([]organizationtenancy.ExportSEPADebitFile, error) {
	var data parentbilling.SEPAReturnData
	if err := tenant.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
		var err error
		data, err = b.billing.SEPAReturn(ctx, organizationID)
		return err
	}); err != nil {
		return nil, err
	}
	files := make([]organizationtenancy.ExportSEPADebitFile, len(data.Files))
	for i, file := range data.Files {
		files[i] = organizationtenancy.ExportSEPADebitFile{Name: file.Path, Data: file.Data, Origin: file.Origin, Reason: file.Reason}
	}
	return files, nil
}

// traegerCreditorAccounts reads the default bank account of the Träger from
// its master data; the read is logged without an account (the CLI).
type traegerCreditorAccounts struct {
	masterData organizationtenancy.TraegerMasterData
}

func (c traegerCreditorAccounts) CreditorAccount(ctx context.Context, organizationID int64) (*parentbilling.LetterheadBankAccount, error) {
	profile, err := c.masterData.Profile(ctx, organizationID, 0)
	if err != nil {
		return nil, err
	}
	for _, account := range profile.BankAccounts {
		if account.IsDefault {
			return &parentbilling.LetterheadBankAccount{AccountHolder: account.AccountHolder, IBAN: account.IBAN, BIC: account.BIC,
				BankName: account.BankName}, nil
		}
	}
	return nil, nil
}

// tenantExportAudit writes each Träger package run into the operator audit:
// action export on the organization, first the attempt of an existing,
// active operator (like the erasure), then the outcome with the counts and
// the manifest checksum or the error class.
type tenantExportAudit struct {
	log       owners.OperatorAuditEntries
	operators identityaccess.OperatorDirectory
}

func (a tenantExportAudit) BeginTenantExport(ctx context.Context, attempt organizationtenancy.TenantExportAttempt) error {
	if attempt.OperatorID <= 0 || attempt.OrganizationID <= 0 {
		return errors.New("tenant export audit: operator and organization are required")
	}
	operator, err := a.operators.FindOperator(ctx, attempt.OperatorID)
	if errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return errors.New("tenant export audit: unknown operator")
	}
	if err != nil {
		return fmt.Errorf("tenant export audit: operator unavailable: %w", err)
	}
	if !operator.Active {
		return errors.New("tenant export audit: operator is not active")
	}
	return a.write(ctx, attempt.OperatorID, attempt.OrganizationID, struct {
		organizationtenancy.TenantExportAttempt
		Outcome string `json:"outcome"`
	}{attempt, "started"})
}

func (a tenantExportAudit) RecordTenantExport(ctx context.Context, record organizationtenancy.TenantExportRecord) error {
	if record.OperatorID <= 0 || record.OrganizationID <= 0 {
		return errors.New("tenant export audit: operator and organization are required")
	}
	return a.write(ctx, record.OperatorID, record.OrganizationID, record)
}

func (a tenantExportAudit) write(ctx context.Context, operatorID, organizationID int64, record any) error {
	changes, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return a.log.Create(ctx, &auditModels.OperatorAuditEntry{OperatorID: operatorID, Action: "export",
		ResourceType: "organization", ResourceID: &organizationID, Changes: changes})
}
