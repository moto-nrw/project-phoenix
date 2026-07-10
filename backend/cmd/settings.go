package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
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

func runSettingsOverrides(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContext()
	if err != nil {
		return err
	}
	defer ctx.Close()
	keys := settingsOverrideKeys
	if len(keys) == 0 {
		keys = activationSettingKeys
	}
	schools, err := ctx.RepoFactory.School.List(context.Background())
	if err != nil {
		return fmt.Errorf("list schools: %w", err)
	}
	rows := []settingOverrideRow{}
	for _, school := range schools {
		for _, key := range keys {
			value, findErr := ctx.RepoFactory.SettingValue.FindByTenantAndKey(context.Background(), school.ID, key)
			if findErr != nil {
				return fmt.Errorf("read override for tenant %d key %s: %w", school.ID, key, findErr)
			}
			if value == nil {
				continue
			}
			rows = append(rows, settingOverrideRow{TenantID: school.ID, TenantSlug: school.Slug, TenantName: school.Name, Key: key, Value: value.Value})
		}
	}
	if settingsOverridesJSON {
		encoded, marshalErr := json.MarshalIndent(rows, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := fmt.Fprintln(os.Stdout, string(encoded)); writeErr != nil {
			return fmt.Errorf("write settings overrides: %w", writeErr)
		}
		return nil
	}
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
