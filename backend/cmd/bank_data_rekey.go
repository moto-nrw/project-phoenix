package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
	transactionRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() { RootCmd.AddCommand(newBankDataRekeyCommand()) }

func newBankDataRekeyCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "bank-data-rekey", Short: "Re-encrypt bank data in bounded, restartable batches", SilenceUsage: true, RunE: runBankDataRekey}
	cmd.Flags().Int64("organization", 0, "Required organization ID")
	cmd.Flags().Int("batch-size", 100, "Rows per transaction (1..1000)")
	return cmd
}

func runBankDataRekey(cmd *cobra.Command, _ []string) error {
	organizationID, _ := cmd.Flags().GetInt64("organization")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	if organizationID <= 0 || batchSize < 1 || batchSize > 1000 {
		return fmt.Errorf("organization and batch-size 1..1000 are required")
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
	job := parentBillingCompose.NewBankDataRekey(cipher)
	ctx := tenant.WithUnitOfWork(cmd.Context(), db.UnitOfWork())
	return runBankDataRekeyJob(ctx, job, organizationID, batchSize, cmd.OutOrStdout())
}

// The production CLI and its integration test use the same complete source traversal.
func runBankDataRekeyJob(ctx context.Context, job parentbilling.BankDataRekey, organizationID int64, batchSize int, output io.Writer) error {
	for _, source := range []string{"mandates", "debits", "debit_files"} {
		if err := runBankDataSource(ctx, job, organizationID, source, batchSize, output); err != nil {
			return err
		}
	}
	return nil
}

func runBankDataSource(ctx context.Context, job parentbilling.BankDataRekey, organizationID int64, source string, batchSize int, output io.Writer) error {
	var cursor int64
	var scannedTotal, changedTotal int
	for {
		batch, err := bankDataRekeyBatch(ctx, job, organizationID, source, cursor, batchSize)
		if err != nil {
			return err
		}
		if batch.Scanned == 0 {
			break
		}
		if _, err := fmt.Fprintf(output, "source=%s scanned=%d changed=%d\n", source, batch.Scanned, batch.Changed); err != nil {
			return err
		}
		scannedTotal += batch.Scanned
		changedTotal += batch.Changed
		cursor = batch.LastID
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
