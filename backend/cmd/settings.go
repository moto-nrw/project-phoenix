package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/services"
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
	configModel.KeyAttendanceWebEnabled,
	configModel.KeyGroupMode,
	configModel.KeyOperationalOverviewScope,
	configModel.KeyTimetableShowExpectedChildrenCount,
	configModel.KeyEnrollmentCollectGradeLevel,
	configModel.KeyEnrollmentCareOfferingsEnabled,
	configModel.KeyEnrollmentNotifyPerDecision,
	configModel.KeyEnrollmentDuplicateHandling,
	configModel.KeyEnrollmentRejectedRetentionDays,
	configModel.KeyEnrollmentWaitlistEnabled,
	configModel.KeyEnrollmentAutoInviteGuardianOnApprove,
}

type settingOverrideRow struct {
	TenantID   int64           `json:"tenant_id"`
	TenantSlug string          `json:"tenant_slug"`
	TenantName string          `json:"tenant_name"`
	Key        string          `json:"key"`
	Value      json.RawMessage `json:"value"`
}

type settingsCommandContext struct {
	*cleanupContext
	schools organizationtenancy.Query
	values  configModel.SettingValueRepository
}

func newSettingsCommandContext() (*settingsCommandContext, error) {
	base, err := newCleanupContext()
	if err != nil {
		return nil, err
	}
	return &settingsCommandContext{
		cleanupContext: base,
		schools:        base.Schools,
		values:         services.NewSettingsCommandRepository(base.DB),
	}, nil
}

func runSettingsOverrides(_ *cobra.Command, _ []string) error {
	ctx, err := newSettingsCommandContext()
	if err != nil {
		return err
	}
	defer ctx.Close()
	schools, err := ctx.schools.ListSchools(context.Background())
	if err != nil {
		return fmt.Errorf("list schools: %w", err)
	}
	rows, err := collectSettingOverrideRows(context.Background(), schools, selectedSettingOverrideKeys(), ctx.values)
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

func collectSettingOverrideRows(ctx context.Context, schools []organizationtenancy.School, keys []string, values configModel.SettingValueRepository) ([]settingOverrideRow, error) {
	rows := []settingOverrideRow{}
	for _, school := range schools {
		for _, key := range keys {
			value, findErr := values.FindByTenantAndKey(ctx, school.ID, key)
			if findErr != nil {
				return nil, fmt.Errorf("read override for tenant %d key %s: %w", school.ID, key, findErr)
			}
			if value == nil {
				continue
			}
			rows = append(rows, settingOverrideRow{TenantID: school.ID, TenantSlug: school.Slug, TenantName: school.Name, Key: key, Value: value.Value})
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
