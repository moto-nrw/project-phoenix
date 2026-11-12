package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/delivery/objects"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-ogs/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy/erasureledger"
	parentBillingCompose "github.com/moto-ogs/project-phoenix/modules/parentbilling/compose"
	peopleCompose "github.com/moto-ogs/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	securityRuntimeCompose "github.com/moto-ogs/project-phoenix/modules/securityruntime/compose"
	settingsCompose "github.com/moto-ogs/project-phoenix/modules/settings/compose"
	studentDirectoryViewCompose "github.com/moto-ogs/project-phoenix/modules/studentdirectoryview/compose"
	transactionCompose "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-ogs/project-phoenix/modules/workforce/compose"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() { RootCmd.AddCommand(newTenantCommand()) }

func newTenantCommand() *cobra.Command {
	parent := tenantCommand(runTenantExport)
	var request organizationtenancy.ErasureRequest
	var root, ledgerPath string
	erase := &cobra.Command{Use: "erase", Short: "Erase one school, or an organization with its recorded order: its schools, then the Träger root", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
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
	f.Int64Var(&request.OrganizationID, "organization", 0, "Organization ID; needs the order recorded in the operator portal after its grace period; erases every school, then the root")
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
	parent.AddCommand(erase, receipt, newErasureLedgerInitCommand(), newTenantReturnCommand())
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
		Use: "export", Short: "Return one school's or one Träger's data to a new private local directory", Args: cobra.NoArgs,
		SilenceErrors: true, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("person") && options.Person == "" {
				return errors.New("tenant export: --person must not be empty")
			}
			return run(cmd, options)
		},
	}
	export.Flags().Int64Var(&options.SchoolID, "school", 0, "Authorized school ID")
	export.Flags().Int64Var(&options.OrganizationID, "organization", 0, "Authorized Träger ID: the Träger package, without the data its schools hold")
	export.Flags().Int64Var(&options.OperatorID, "operator", 0, "Active operator ID for the operator audit; required")
	export.Flags().StringVar(&options.Output, "out", "", "New private output directory")
	export.Flags().StringVar(&options.Uploads, "uploads", "", "Complete stable uploads root")
	export.Flags().StringVar(&options.Person, "person", "", "Internal review only: kind:<profile ID>, guardian:<profile ID>, staff:<school membership ID>")
	export.MarkFlagsMutuallyExclusive("school", "organization")
	export.MarkFlagsMutuallyExclusive("organization", "person")
	parent.AddCommand(export)
	return parent
}

func runTenantExport(cmd *cobra.Command, options organizationtenancy.TenantExportOptions) error {
	if (options.SchoolID <= 0 && options.OrganizationID <= 0) || options.Output == "" || options.Uploads == "" {
		return errors.New("tenant export: --school or --organization, --out and --uploads are required")
	}
	if options.OperatorID <= 0 {
		return errors.New("tenant export: --operator is required for the operator audit")
	}
	db, err := transactionCompose.OpenDatabase()
	if err != nil {
		return errors.New("tenant export: database unavailable")
	}
	defer func() { _ = db.Close() }()
	sealed, err := tenantExportSealedFields()
	if err != nil {
		return err
	}
	exporter, err := newTenantExporter(db, sealed, tenantExportProjectedFields(), options.OrganizationID > 0)
	if err != nil {
		return err
	}
	if err := exporter.Export(tenant.WithUnitOfWork(cmd.Context(), db.UnitOfWork()), options); err != nil {
		return err
	}
	message := "Tenant export complete and in the operator audit; verify COMPLETE and manifest before handover."
	if options.OrganizationID > 0 {
		message = "Träger package complete and in the operator audit; it holds FULL BANK DATA. Verify COMPLETE and manifest; hand over encrypted only."
	}
	if options.Person != "" {
		message = "Person review workspace created; completeness, third-party review and school approval REQUIRED before handover."
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), message)
	return err
}

// tenantExportSealedFields binds each field-sealed table to the owner that
// sealed it (#3838, #6412): the return carries IBAN, account holder, tax and
// social security numbers, mandate text and health information as entered,
// not their ciphertext. Without either key ring of
// the source installation the export refuses (ADR 0192, ADR 0193).
func tenantExportSealedFields() (organizationtenancy.TenantExportSealedFieldsByTable, error) {
	bankData, err := securityRuntimeCompose.NewBankDataCipher(viper.GetString("bank_data_keys"), viper.GetString("bank_data_legacy_keys"))
	if err != nil {
		return nil, errors.New("tenant export: BANK_DATA_KEYS / BANK_DATA_LEGACY_KEYS configuration invalid")
	}
	healthKeys := strings.TrimSpace(viper.GetString("health_data_keys"))
	if healthKeys == "" {
		return nil, errors.New("tenant export: HEALTH_DATA_KEYS is required to open the health information")
	}
	healthData, err := securityRuntimeCompose.NewFieldCipher(healthKeys)
	if err != nil {
		return nil, errors.New("tenant export: HEALTH_DATA_KEYS configuration invalid")
	}
	sepa := parentBillingCompose.NewSEPAExportFields(bankData)
	holiday := enrollmentCompose.NewHolidayHealthExportFields(healthData)
	return organizationtenancy.TenantExportSealedFieldsByTable{
		"parent_billing.sepa_mandates":             sepa,
		"parent_billing.sepa_debits":               sepa,
		"parent_billing.sepa_mandate_consents":     sepa,
		"users.guardian_financial_data":            peopleCompose.NewGuardianBankExportFields(bankData),
		"users.staff_financial_data":               workforceCompose.NewStaffBankExportFields(bankData),
		"users.student_care_profiles":              studentDirectoryViewCompose.NewReader(healthData),
		"enrollment.holiday_registrations":         holiday,
		"enrollment.holiday_registration_children": holiday,
		"enrollment.holiday_guests":                holiday,
	}, nil
}

// tenantExportProjectedFields binds each untyped container to the owner that
// knows its secrets (#3838, ADR 0193): settings keep their non-secret values,
// a school keeps its branding images; nothing of a container leaves raw.
func tenantExportProjectedFields() organizationtenancy.TenantExportProjectedFieldsByTable {
	settingValues := settingsCompose.NewExportFields()
	return organizationtenancy.TenantExportProjectedFieldsByTable{
		"config.setting_values":         settingValues,
		"config.setting_audit":          settingValues,
		"config.traeger_setting_values": settingValues,
		"config.traeger_setting_audit":  settingValues,
		"platform.schools":              organizationtenancy.SchoolSettingsExportFields{},
	}
}
