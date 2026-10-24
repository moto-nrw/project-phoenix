package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionCompose "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/spf13/cobra"
)

func init() { RootCmd.AddCommand(newTenantCommand()) }

func newTenantCommand() *cobra.Command {
	parent := tenantCommand(runTenantExport)
	var request organizationtenancy.ErasureRequest
	var root string
	erase := &cobra.Command{Use: "erase", Short: "Erase one school, or process an organization's schools (root remains open)", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, err := transactionCompose.OpenDatabase()
			if err != nil {
				return errors.New("erasure: database connection unavailable")
			}
			defer func() { _ = db.Close() }()
			service, err := organizationCompose.NewErasure(db.Pool(), objects.NewLocal(root))
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
			service, err := organizationCompose.NewErasure(db.Pool(), objects.NewLocal(""))
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
	parent.AddCommand(erase, receipt)
	return parent
}

func tenantCommand(run func(*cobra.Command, organizationtenancy.TenantExportOptions) error) *cobra.Command {
	parent := &cobra.Command{Use: "tenant", Short: "Internal tenant data lifecycle procedures"}
	options := organizationtenancy.TenantExportOptions{}
	export := &cobra.Command{
		Use: "export", Short: "Return one school's data to a new private local directory", Args: cobra.NoArgs,
		SilenceErrors: true, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("person") && options.Person == "" {
				return errors.New("tenant export: --person must not be empty")
			}
			return run(cmd, options)
		},
	}
	export.Flags().Int64Var(&options.SchoolID, "school", 0, "Authorized school ID")
	export.Flags().StringVar(&options.Output, "out", "", "New private output directory")
	export.Flags().StringVar(&options.Uploads, "uploads", "", "Complete stable uploads root")
	export.Flags().StringVar(&options.Person, "person", "", "Internal review only: kind:<profile ID>, guardian:<profile ID>, staff:<school membership ID>")
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
	message := "Tenant export complete; verify COMPLETE and manifest before handover."
	if options.Person != "" {
		message = "Person review workspace created; completeness, third-party review and school approval REQUIRED before handover."
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), message)
	return err
}
