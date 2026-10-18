package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/cobra"
)

var (
	settingsOverrideKeys  []string
	settingsOverridesJSON bool
)

var settingsCmd = &cobra.Command{Use: "settings", Short: "Inspect tenant settings"}

var settingsOverridesCmd = &cobra.Command{
	Use:   "overrides",
	Short: "List stored tenant overrides without changing them",
	RunE:  runSettingsOverrides,
}

var activationSettingKeys = []string{
	settings.KeyAttendanceWebEnabled,
	settings.KeyGroupMode,
	settings.KeyOperationalOverviewScope,
	settings.KeyTimetableShowExpectedChildrenCount,
	settings.KeyEnrollmentCollectGradeLevel,
	settings.KeyEnrollmentCareOfferingsEnabled,
	settings.KeyEnrollmentNotifyPerDecision,
	settings.KeyEnrollmentDuplicateHandling,
	settings.KeyEnrollmentRejectedRetentionDays,
	settings.KeyEnrollmentWaitlistEnabled,
	settings.KeyEnrollmentAutoInviteGuardianOnApprove,
}

type settingOverrideRow struct {
	TenantID   int64           `json:"tenant_id"`
	TenantSlug string          `json:"tenant_slug"`
	TenantName string          `json:"tenant_name"`
	Key        string          `json:"key"`
	Value      json.RawMessage `json:"value"`
}

// storedOverrides reads the overrides a school stored, without defaults.
type storedOverrides interface {
	StoredOverride(ctx context.Context, tenantID int64, key string) (json.RawMessage, bool, error)
}

// openSettingsOverrides opens the database with the schools and the stored
// overrides the command reads. The caller must close the database.
func openSettingsOverrides() (transactionruntime.Database, organizationtenancy.Query, storedOverrides, error) {
	db, err := defaultCleanupRoot.open()
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	schools, err := newCleanupSchools(db)
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	return db, schools, settingsCompose.NewStoredOverrideReader(db.Pool()), nil
}

func runSettingsOverrides(_ *cobra.Command, _ []string) error {
	db, schools, values, err := openSettingsOverrides()
	if err != nil {
		return err
	}
	defer closeCleanupDatabase(db, log.Default())
	return listSettingOverrides(db, schools, values)
}

func listSettingOverrides(db transactionruntime.Database, schools organizationtenancy.Query, values storedOverrides) error {
	ctx := tenant.WithUnitOfWork(context.Background(), db.UnitOfWork())
	listed, err := schools.ListSchools(ctx)
	if err != nil {
		return fmt.Errorf("list schools: %w", err)
	}
	rows, err := collectSettingOverrideRows(ctx, listed, selectedSettingOverrideKeys(), values)
	if err != nil {
		return err
	}
	return writeSettingOverrideRows(rows)
}

func selectedSettingOverrideKeys() []string {
	if len(settingsOverrideKeys) > 0 {
		return settingsOverrideKeys
	}
	return activationSettingKeys
}

func collectSettingOverrideRows(ctx context.Context, schools []organizationtenancy.School, keys []string, values storedOverrides) ([]settingOverrideRow, error) {
	rows := []settingOverrideRow{}
	for _, school := range schools {
		for _, key := range keys {
			value, found, findErr := values.StoredOverride(ctx, school.ID, key)
			if findErr != nil {
				return nil, fmt.Errorf("read override for tenant %d key %s: %w", school.ID, key, findErr)
			}
			if !found {
				continue
			}
			rows = append(rows, settingOverrideRow{TenantID: school.ID, TenantSlug: school.Slug, TenantName: school.Name, Key: key, Value: value})
		}
	}
	return rows, nil
}

func writeSettingOverrideRows(rows []settingOverrideRow) error {
	if settingsOverridesJSON {
		return writeSettingOverrideJSON(rows)
	}
	return writeSettingOverrideTable(rows)
}

func writeSettingOverrideJSON(rows []settingOverrideRow) error {
	encoded, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, string(encoded)); err != nil {
		return fmt.Errorf("write settings overrides: %w", err)
	}
	return nil
}

func writeSettingOverrideTable(rows []settingOverrideRow) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "TENANT_ID\tSLUG\tSCHOOL\tKEY\tVALUE"); err != nil {
		return fmt.Errorf("write settings header: %w", err)
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", row.TenantID, row.TenantSlug, row.TenantName, row.Key, string(row.Value)); err != nil {
			return fmt.Errorf("write settings override: %w", err)
		}
	}
	return w.Flush()
}

func init() {
	RootCmd.AddCommand(settingsCmd)
	settingsCmd.AddCommand(settingsOverridesCmd)
	settingsOverridesCmd.Flags().StringSliceVar(&settingsOverrideKeys, "key", nil, "setting key to inspect (repeatable; defaults to issue #1856 keys)")
	settingsOverridesCmd.Flags().BoolVar(&settingsOverridesJSON, "json", false, "write JSON output")
}
