package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	pinSessionEndTimeVersion = "1.15.469"
	pinSessionEndTimeKey     = "operations.session_end_time"
	// pinSessionEndTimeValue is SESSION_END_TIME of every deployed environment
	// before the scheduler stopped reading it (#4437). The registry default
	// is 18:00.
	pinSessionEndTimeValue = `"17:00"`
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     pinSessionEndTimeVersion,
		Description: "Pin the session end time schools ran on before the scheduler stopped reading SESSION_END_TIME (#4437)",
		DependsOn: []string{
			configSettingValuesVersion, // config.setting_values + config.setting_audit
			"1.15.468",                 // latest migration at authoring time
		},
	})
	Migrations.MustRegister(pinSessionEndTimeUp, pinSessionEndTimeDown)
}

// pinSessionEndTimeUp stores 17:00 for every school without its own session
// end time, so the switch from the environment to the registry default does
// not move their session end by an hour. A school's own value stays.
func pinSessionEndTimeUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		WITH inserted AS (
			INSERT INTO config.setting_values (tenant_id, setting_key, value)
			SELECT id, ?, ?::jsonb
			FROM platform.schools
			ON CONFLICT (tenant_id, setting_key) DO NOTHING
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (
			tenant_id, setting_key, old_value, new_value, action, changed_by
		)
		SELECT tenant_id, setting_key, NULL, value, 'set', NULL
		FROM inserted;
	`, pinSessionEndTimeKey, pinSessionEndTimeValue).Exec(ctx)
	if err != nil {
		return fmt.Errorf("pin session end time: %w", err)
	}
	return nil
}

// pinSessionEndTimeDown keeps the pinned values: a pinned row cannot be told
// apart from one an operator set to 17:00 afterwards.
func pinSessionEndTimeDown(_ context.Context, _ *bun.DB) error {
	return nil
}
