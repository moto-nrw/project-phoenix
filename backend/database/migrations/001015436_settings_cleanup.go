package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	settingsCleanupVersion     = "1.15.436"
	settingsCleanupDescription = "Settings cleanup (#3729-#3738): carry the care concept into the spontaneous activities switch, pin the old on-duty and indicator behavior, delete removed settings"

	settingsCleanupCareConceptKey      = "operations.care_concept"
	settingsCleanupSpontaneousKey      = "attendance.web_spontaneous_activities_enabled"
	settingsCleanupOnDutyOnlyKey       = "notifications.on_duty_only"
	settingsCleanupIndicatorsKey       = "tracking.indicators_enabled"
	settingsCleanupIndicator1Key       = "tracking.indicator_1"
	settingsCleanupIndicator2Key       = "tracking.indicator_2"
	settingsCleanupFixedScheduleOption = "fixed_schedule"
)

// settingsCleanupRemovedKeys are the settings whose registry definitions are
// gone. Their stored overrides and audit rows would be orphans the settings
// UI can no longer show.
var settingsCleanupRemovedKeys = []string{
	"operations.status_flag_clear_time",              // #3729: end of day is 18:00 for every school
	settingsCleanupCareConceptKey,                    // #3730: folded into the spontaneous activities switch
	"notifications.care_cancelled_enabled",           // #3731: decided in the cancel dialog
	"notifications.care_cancelled_default_on",        // #3731
	"notifications.care_cancelled_email",             // #3731
	"operations.emergency_list_health_info",          // #3732: always printed
	"operations.student_activation_interval_minutes", // #3733: code constant
}

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     settingsCleanupVersion,
		Description: settingsCleanupDescription,
		DependsOn: []string{
			configSettingValuesVersion,    // config.setting_values + config.setting_audit
			attendanceCheckOutNoteVersion, // latest migration at authoring time
		},
	})
	Migrations.MustRegister(settingsCleanupUp, settingsCleanupDown)
}

func settingsCleanupUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []struct {
			name string
			run  func(context.Context, bun.IDB) error
		}{
			{"carry care concept into spontaneous activities", settingsCleanupSpontaneous},
			{"pin on-duty-only notifications", settingsCleanupPinOnDutyOnly},
			{"pin empty indicator labels", settingsCleanupPinIndicators},
			{"delete removed settings", settingsCleanupDeleteRemoved},
		} {
			if err := step.run(ctx, tx); err != nil {
				return fmt.Errorf("settings cleanup: %s: %w", step.name, err)
			}
		}
		return nil
	})
}

// settingsCleanupSpontaneous (#3730): "Fester Betriebsplan" used to switch
// spontaneous activities off regardless of their own switch. Those schools
// get the switch stored as off; every other school keeps its stored value or
// the default (on).
func settingsCleanupSpontaneous(ctx context.Context, db bun.IDB) error {
	_, err := db.NewRaw(`
		WITH fixed AS (
			SELECT concept.tenant_id, stored.value AS old_value
			FROM config.setting_values AS concept
			LEFT JOIN config.setting_values AS stored
				ON stored.tenant_id = concept.tenant_id
				AND stored.setting_key = ?
			WHERE concept.setting_key = ?
				AND concept.value #>> '{}' = ?
		), changed AS (
			INSERT INTO config.setting_values (tenant_id, setting_key, value)
			SELECT tenant_id, ?, 'false'::jsonb
			FROM fixed
			ON CONFLICT (tenant_id, setting_key) DO UPDATE
			SET value = EXCLUDED.value
			WHERE config.setting_values.value IS DISTINCT FROM EXCLUDED.value
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (
			tenant_id, setting_key, old_value, new_value, action, changed_by
		)
		SELECT changed.tenant_id, changed.setting_key, fixed.old_value, changed.value, 'set', NULL
		FROM changed
		JOIN fixed ON fixed.tenant_id = changed.tenant_id;
	`, settingsCleanupSpontaneousKey, settingsCleanupCareConceptKey, settingsCleanupFixedScheduleOption,
		settingsCleanupSpontaneousKey).Exec(ctx)
	return err
}

// settingsCleanupPinOnDutyOnly (#3736): the default flips to off. Schools
// that ran on the old default keep "on", so their team does not start
// getting notices outside their shift without anyone deciding that.
func settingsCleanupPinOnDutyOnly(ctx context.Context, db bun.IDB) error {
	_, err := db.NewRaw(`
		WITH inserted AS (
			INSERT INTO config.setting_values (tenant_id, setting_key, value)
			SELECT id, ?, 'true'::jsonb
			FROM platform.schools
			ON CONFLICT (tenant_id, setting_key) DO NOTHING
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (
			tenant_id, setting_key, old_value, new_value, action, changed_by
		)
		SELECT tenant_id, setting_key, NULL, value, 'set', NULL
		FROM inserted;
	`, settingsCleanupOnDutyOnlyKey).Exec(ctx)
	return err
}

// settingsCleanupPinIndicators (#3738): the first two indicator labels get
// defaults ("Mensa", "Hausaufgaben"). A school that already shows the
// indicators must keep exactly its labels, so an empty slot it never filled
// is stored as empty. Schools with the indicators off get the defaults when
// they switch them on.
func settingsCleanupPinIndicators(ctx context.Context, db bun.IDB) error {
	_, err := db.NewRaw(`
		WITH inserted AS (
			INSERT INTO config.setting_values (tenant_id, setting_key, value)
			SELECT enabled.tenant_id, slot.key, '""'::jsonb
			FROM config.setting_values AS enabled
			CROSS JOIN (VALUES (?), (?)) AS slot(key)
			WHERE enabled.setting_key = ?
				AND enabled.value = 'true'::jsonb
			ON CONFLICT (tenant_id, setting_key) DO NOTHING
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (
			tenant_id, setting_key, old_value, new_value, action, changed_by
		)
		SELECT tenant_id, setting_key, NULL, value, 'set', NULL
		FROM inserted;
	`, settingsCleanupIndicator1Key, settingsCleanupIndicator2Key, settingsCleanupIndicatorsKey).Exec(ctx)
	return err
}

func settingsCleanupDeleteRemoved(ctx context.Context, db bun.IDB) error {
	if _, err := db.NewRaw(
		`DELETE FROM config.setting_values WHERE setting_key IN (?);`,
		bun.List(settingsCleanupRemovedKeys),
	).Exec(ctx); err != nil {
		return fmt.Errorf("delete setting values: %w", err)
	}
	if _, err := db.NewRaw(
		`DELETE FROM config.setting_audit WHERE setting_key IN (?);`,
		bun.List(settingsCleanupRemovedKeys),
	).Exec(ctx); err != nil {
		return fmt.Errorf("delete setting audit rows: %w", err)
	}
	return nil
}

// settingsCleanupDown is a no-op. The deleted overrides are gone, and the
// pinned values carry no provenance once a school edits them. Old binaries
// read the pinned values as ordinary overrides and ignore the removed keys'
// absence by falling back to their registry defaults.
func settingsCleanupDown(_ context.Context, _ *bun.DB) error {
	return nil
}
