package cmd

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"

	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
	transactionRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() { RootCmd.AddCommand(newBankDataRekeyCommand()) }

func newBankDataRekeyCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "bank-data-rekey", Short: "Re-encrypt bank data in bounded, restartable batches", SilenceUsage: true}
	cmd.Flags().Int64("organization", 0, "Required organization ID")
	cmd.Flags().Int("batch-size", 100, "Rows per transaction (1..1000)")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
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
		for _, source := range []string{"mandates", "debits"} {
			var cursor int64
			for {
				batch, err := job.Batch(tenant.WithUnitOfWork(cmd.Context(), db.UnitOfWork()), organizationID, source, cursor, batchSize)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "source=%s scanned=%d changed=%d\n", source, batch.Scanned, batch.Changed); err != nil {
					return err
				}
				if batch.Scanned == 0 {
					break
				}
				cursor = batch.LastID
			}
		}
		return nil
	}
	return cmd
}
