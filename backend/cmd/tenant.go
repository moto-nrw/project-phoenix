package cmd

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionCompose "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/spf13/cobra"
)

func init() { RootCmd.AddCommand(newTenantCommand()) }

func newTenantCommand() *cobra.Command { return tenantCommand(runTenantExport) }

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
