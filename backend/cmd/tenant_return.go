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
// ADR 0192/0193 (ADR 0187): the PAIN.008 files Parent Billing opens with the
// bank key in a Träger transaction of its own, the creditor account from the
// Träger's master data for reconstructed files, and the operator audit of
// each run, which a school package gets as well (#6326).

// newTenantExporter builds the export with the operator audit of each run
// (#6326). A Träger package also gets its PAIN.008 files.
func newTenantExporter(db transactionCompose.Database, sealed organizationtenancy.TenantExportSealedFields,
	projected organizationtenancy.TenantExportProjectedFields, traeger bool,
) (organizationtenancy.TenantExporter, error) {
	if !traeger {
		audit, err := newTenantExportAudit(db)
		if err != nil {
			return nil, err
		}
		fingerprints := organizationtenancy.ExportFingerprints{Bytes: securityruntime.Fingerprint, Reader: securityruntime.ReaderFingerprint}
		return organizationCompose.NewTenantExporter(db.Pool(), fingerprints, sealed, projected, audit)
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
	audit, err := newTenantExportAudit(db)
	if err != nil {
		return nil, err
	}
	return organizationCompose.NewTraegerTenantExporter(db.Pool(), organizationtenancy.TenantExportDependencies{
		Fingerprints: organizationtenancy.ExportFingerprints{Bytes: securityruntime.Fingerprint, Reader: securityruntime.ReaderFingerprint},
		Sealed:       sealed, Projected: projected,
		SEPADebitFiles: traegerSEPADebitFiles{billing: billing},
		Audit:          audit,
	})
}

func newTenantExportAudit(db transactionCompose.Database) (tenantExportAudit, error) {
	operators, err := identityaccessCompose.NewOperatorDirectory(db.Pool())
	if err != nil {
		return tenantExportAudit{}, err
	}
	return tenantExportAudit{log: owners.NewOperatorAuditLog(db), operators: operators}, nil
}

// traegerSEPADebitFiles hands the PAIN.008 files to the export one at a
// time, read in a Träger transaction of the Träger.
type traegerSEPADebitFiles struct{ billing parentbilling.SEPAReturn }

func (b traegerSEPADebitFiles) SEPADebitFiles(ctx context.Context, organizationID int64,
	write func(organizationtenancy.ExportSEPADebitFile) error,
) error {
	return tenant.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
		return b.billing.SEPAReturn(ctx, organizationID, func(file parentbilling.SEPAReturnFile) error {
			return write(organizationtenancy.ExportSEPADebitFile{Name: file.Path, Data: file.Data, Origin: file.Origin, Reason: file.Reason})
		})
	})
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

// tenantExportAudit writes each package run into the operator audit:
// action export on the school or the organization, first the attempt of an
// existing, active operator (like the erasure), then the outcome with the
// counts and the manifest checksum or the error class (#6326).
type tenantExportAudit struct {
	log       owners.OperatorAuditEntries
	operators identityaccess.OperatorDirectory
}

func (a tenantExportAudit) BeginTenantExport(ctx context.Context, attempt organizationtenancy.TenantExportAttempt) error {
	resourceType, resourceID, err := tenantExportResource(attempt.OperatorID, attempt.SchoolID, attempt.OrganizationID)
	if err != nil {
		return err
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
	return a.write(ctx, attempt.OperatorID, resourceType, resourceID, struct {
		organizationtenancy.TenantExportAttempt
		Outcome string `json:"outcome"`
	}{attempt, "started"})
}

func (a tenantExportAudit) RecordTenantExport(ctx context.Context, record organizationtenancy.TenantExportRecord) error {
	resourceType, resourceID, err := tenantExportResource(record.OperatorID, record.SchoolID, record.OrganizationID)
	if err != nil {
		return err
	}
	return a.write(ctx, record.OperatorID, resourceType, resourceID, record)
}

// tenantExportResource names the audited package: exactly one school or
// one organization, by an operator.
func tenantExportResource(operatorID, schoolID, organizationID int64) (string, int64, error) {
	switch {
	case operatorID <= 0 || (schoolID > 0) == (organizationID > 0) || schoolID < 0 || organizationID < 0:
		return "", 0, errors.New("tenant export audit: operator and exactly one school or organization are required")
	case schoolID > 0:
		return "school", schoolID, nil
	default:
		return "organization", organizationID, nil
	}
}

func (a tenantExportAudit) write(ctx context.Context, operatorID int64, resourceType string, resourceID int64, record any) error {
	changes, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return a.log.Create(ctx, &auditModels.OperatorAuditEntry{OperatorID: operatorID, Action: "export",
		ResourceType: resourceType, ResourceID: &resourceID, Changes: changes})
}
