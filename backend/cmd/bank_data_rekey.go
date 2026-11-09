package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
	transactionRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() { RootCmd.AddCommand(newBankDataRekeyCommand()) }

func newBankDataRekeyCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "bank-data-rekey", Short: "Re-encrypt bank data in bounded, restartable batches", SilenceUsage: true, RunE: runBankDataRekey}
	cmd.Flags().Int64("organization", 0, "Organization ID: SEPA mandates and debit copies of the Träger")
	cmd.Flags().Int64("school", 0, "School ID: bank data of guardians and bank and tax data of staff (#6332)")
	cmd.Flags().Int("batch-size", 100, "Rows per transaction (1..1000)")
	return cmd
}

func runBankDataRekey(cmd *cobra.Command, _ []string) error {
	organizationID, _ := cmd.Flags().GetInt64("organization")
	schoolID, _ := cmd.Flags().GetInt64("school")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	if (organizationID > 0) == (schoolID > 0) || organizationID < 0 || schoolID < 0 || batchSize < 1 || batchSize > 1000 {
		return fmt.Errorf("exactly one of organization or school and batch-size 1..1000 are required")
	}
	cipher, err := securityRuntimeCompose.NewBankDataCipher(viper.GetString("bank_data_keys"), viper.GetString("bank_data_legacy_keys"))
	if err != nil {
		return fmt.Errorf("BANK_DATA_KEYS / BANK_DATA_LEGACY_KEYS configuration invalid")
	}
	db, err := transactionRuntimeCompose.OpenDatabase()
	if err != nil {
		return fmt.Errorf("bank data rekey: maintenance database unavailable")
	}
	defer func() { _ = db.Close() }()
	ctx := tenant.WithUnitOfWork(cmd.Context(), db.UnitOfWork())
	if schoolID > 0 {
		return runSchoolBankDataRekeyJob(ctx, peopleCompose.NewGuardianBankDataRekey(cipher), workforceCompose.NewStaffBankDataRekey(cipher), schoolID, batchSize, cmd.OutOrStdout())
	}
	return runBankDataRekeyJob(ctx, parentBillingCompose.NewBankDataRekey(cipher), organizationID, batchSize, cmd.OutOrStdout())
}

// bankDataBatch runs one bounded batch from cursor and reports its counters.
type bankDataBatch func(ctx context.Context, cursor int64) (scanned, changed int, lastID int64, err error)

// The production CLI and its integration test use the same complete source traversal.
func runBankDataRekeyJob(ctx context.Context, job parentbilling.BankDataRekey, organizationID int64, batchSize int, output io.Writer) error {
	for _, source := range []string{"mandates", "debits"} {
		batch := func(ctx context.Context, cursor int64) (int, int, int64, error) {
			result, err := bankDataRekeyBatch(ctx, job, organizationID, source, cursor, batchSize)
			return result.Scanned, result.Changed, result.LastID, err
		}
		if err := runBankDataSource(ctx, source, batch, output); err != nil {
			return err
		}
	}
	return nil
}

// runSchoolBankDataRekeyJob backfills and rekeys the bank data of one
// school: the guardians' (People Directory) and the staff's (Workforce).
// Each batch runs in its own school transaction of that school.
func runSchoolBankDataRekeyJob(ctx context.Context, guardians peopledirectory.GuardianBankDataRekey, staff workforce.StaffBankDataRekey, schoolID int64, batchSize int, output io.Writer) error {
	school, err := tenant.NewTenantID(schoolID)
	if err != nil {
		return fmt.Errorf("bank data rekey: school is required")
	}
	inSchool := func(ctx context.Context, fn func(context.Context) error) error {
		return tenant.WithinTenant(ctx, school, fn)
	}
	sources := []struct {
		name  string
		batch bankDataBatch
	}{
		{"guardian_financial", func(ctx context.Context, cursor int64) (scanned, changed int, lastID int64, err error) {
			err = inSchool(ctx, func(ctx context.Context) error {
				result, err := guardians.Batch(ctx, schoolID, cursor, batchSize)
				scanned, changed, lastID = result.Scanned, result.Changed, result.LastID
				return err
			})
			return
		}},
		{"staff_financial", func(ctx context.Context, cursor int64) (scanned, changed int, lastID int64, err error) {
			err = inSchool(ctx, func(ctx context.Context) error {
				result, err := staff.Batch(ctx, schoolID, cursor, batchSize)
				scanned, changed, lastID = result.Scanned, result.Changed, result.LastID
				return err
			})
			return
		}},
	}
	for _, source := range sources {
		if err := runBankDataSource(ctx, source.name, source.batch, output); err != nil {
			return err
		}
	}
	return nil
}

func runBankDataSource(ctx context.Context, source string, batch bankDataBatch, output io.Writer) error {
	var cursor int64
	var scannedTotal, changedTotal int
	for {
		scanned, changed, lastID, err := batch(ctx, cursor)
		if err != nil {
			return err
		}
		if scanned == 0 {
			break
		}
		if _, err := fmt.Fprintf(output, "source=%s scanned=%d changed=%d\n", source, scanned, changed); err != nil {
			return err
		}
		scannedTotal += scanned
		changedTotal += changed
		cursor = lastID
	}
	_, err := fmt.Fprintf(output, "source=%s scanned_total=%d changed_total=%d\n", source, scannedTotal, changedTotal)
	return err
}

// The CLI root opens one organization-scoped transaction per bounded batch.
func bankDataRekeyBatch(ctx context.Context, job parentbilling.BankDataRekey, organizationID int64, source string, cursor int64, batchSize int) (parentbilling.BankDataRekeyResult, error) {
	var batch parentbilling.BankDataRekeyResult
	err := tenant.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
		var err error
		batch, err = job.Batch(ctx, organizationID, source, cursor, batchSize)
		return err
	})
	if err != nil {
		return parentbilling.BankDataRekeyResult{}, err
	}
	return batch, nil
}
