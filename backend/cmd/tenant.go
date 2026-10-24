package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/erasureledger"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionCompose "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/spf13/cobra"
)

func init() { RootCmd.AddCommand(newTenantCommand()) }

func newTenantCommand() *cobra.Command {
	parent := tenantCommand(runTenantExport)
	var request organizationtenancy.ErasureRequest
	var root, ledgerPath string
	erase := &cobra.Command{Use: "erase", Short: "Erase one school, or process an organization's schools (root remains open)", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, err := transactionCompose.OpenDatabase()
			if err != nil {
				return errors.New("erasure: database connection unavailable")
			}
			defer func() { _ = db.Close() }()
			service, err := organizationCompose.NewErasure(db.Pool(), objects.NewLocal(root), erasureledger.New(ledgerPath, securityruntime.Fingerprint))
			if err != nil {
				return err
			}
			results, runErr := service.Run(cmd.Context(), request)
			for _, result := range results {
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
					return errors.New("erasure: evidence output failed; inspect persisted receipt")
				}
			}
			return runErr
		}}
	f := erase.Flags()
	f.Int64Var(&request.SchoolID, "school", 0, "School ID (exactly one target)")
	f.Int64Var(&request.OrganizationID, "organization", 0, "Organization ID; school phases only, organization root remains open")
	f.Int64Var(&request.OperatorID, "operator", 0, "Active internal operator ID, not a name")
	f.StringVar(&request.OrderReference, "order", "", "Non-personal ticket reference: 3-40 uppercase letters/digits/hyphens")
	f.StringVar(&request.Confirmation, "confirm", "", "Exact target: school:<id> or organization:<id>")
	f.StringVar(&root, "uploads-root", "", "Mounted uploads directory used by this installation (required)")
	f.StringVar(&ledgerPath, "erasure-ledger", "", "Absolute path to existing independent host ledger; required except for dry-run")
	f.BoolVar(&request.DryRun, "dry-run", false, "Count rows and files without changing either")
	f.BoolVar(&request.PrerequisitesChecked, "prerequisites-checked", false, "Order, authorization, return/deadlines and retention reviewed in internal ticket")
	f.BoolVar(&request.WritersStopped, "writers-stopped", false, "All API, worker, upload, import and scheduled writers stopped for maintenance")
	f.DurationVar(&request.LockTimeout, "lock-timeout", 5*time.Second, "Database lock wait limit (maximum 1m)")
	f.DurationVar(&request.StatementTimeout, "statement-timeout", 5*time.Minute, "Per-statement database deadline (maximum 1h)")
	_ = erase.MarkFlagRequired("uploads-root")
	var receiptID int64
	receipt := &cobra.Command{Use: "receipt", Short: "Export non-personal database/local-file phase evidence; external steps remain separate", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, err := transactionCompose.OpenDatabase()
			if err != nil {
				return errors.New("erasure: database connection unavailable")
			}
			defer func() { _ = db.Close() }()
			service, err := organizationCompose.NewErasure(db.Pool(), objects.NewLocal(""), erasureledger.New("", securityruntime.Fingerprint))
			if err != nil {
				return err
			}
			results, err := service.Run(cmd.Context(), organizationtenancy.ErasureRequest{ReceiptID: receiptID})
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(results[0])
		}}
	receipt.Flags().Int64Var(&receiptID, "id", 0, "Erasure evidence ID")
	parent.AddCommand(erase, receipt, newErasureLedgerInitCommand())
	return parent
}

func newErasureLedgerInitCommand() *cobra.Command {
	var initPath string
	initialize := &cobra.Command{Use: "ledger-init", Short: "Initialize a NEW installation's independent erasure ledger; never use after restore", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error { return erasureledger.Initialize(initPath) }}
	initialize.Flags().StringVar(&initPath, "erasure-ledger", "", "Absolute ledger path in a protected existing host directory")
	_ = initialize.MarkFlagRequired("erasure-ledger")
	return initialize
}

func tenantCommand(run func(*cobra.Command, organizationtenancy.TenantExportOptions) error) *cobra.Command {
	parent := &cobra.Command{Use: "tenant", Short: "Internal tenant data lifecycle procedures"}
	options := organizationtenancy.TenantExportOptions{}
	export := &cobra.Command{
		Use: "export", Short: "Return one school's data to a new private local directory", Args: cobra.NoArgs,
		SilenceErrors: true, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, options)
		},
	}
	export.Flags().Int64Var(&options.SchoolID, "school", 0, "Authorized school ID")
	export.Flags().StringVar(&options.Output, "out", "", "New private output directory")
	export.Flags().StringVar(&options.Uploads, "uploads", "", "Complete stable uploads root")
	parent.AddCommand(export)
	return parent
}

func runTenantExport(cmd *cobra.Command, options organizationtenancy.TenantExportOptions) error {
	if options.SchoolID <= 0 || options.Output == "" || options.Uploads == "" {
		return errors.New("tenant export: --school, --out and --uploads are required")
	}
	db, err := transactionCompose.OpenDatabase()
	if err != nil {
		return errors.New("tenant export: database unavailable")
	}
	defer func() { _ = db.Close() }()
	exporter, err := organizationCompose.NewTenantExporter(db.Pool(), organizationtenancy.ExportFingerprints{
		Bytes: securityruntime.Fingerprint, Reader: securityruntime.ReaderFingerprint,
	})
	if err != nil {
		return err
	}
	if err := exporter.Export(cmd.Context(), options); err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Tenant export complete; verify COMPLETE and manifest before handover.")
	return err
}
