package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	teamChatOptInOffsVersion     = "1.15.482"
	teamChatOptInOffsDescription = "Team-Chat: Abschaltungen aus der Opt-in-Zeit zurücksetzen (#3262, ADR 0096)"

	// teamChatOptOutSince is when the opt-out default (#3254, PR #3266)
	// reached production with release PR #3307. Before it, "off" was the
	// default; a stored off from then is not a decision against the chat.
	teamChatOptOutSince = "2026-09-17T03:33:54Z"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     teamChatOptInOffsVersion,
		Description: teamChatOptInOffsDescription,
		DependsOn:   []string{configSettingValuesVersion},
	})

	Migrations.MustRegister(teamChatOptInOffsUp, teamChatOptInOffsDown)
}

// teamChatOptInOffsUp removes every stored "off" of the Team-Chat written
// while the chat was opt-in, so those schools get the registry default (on)
// like every school without a value (ADR 0096). An off stored or written
// again since the opt-out default stays: that school switched the chat off
// on purpose. Each reset is recorded in the settings audit like a manual
// reset by the system.
func teamChatOptInOffsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		WITH deleted AS (
			DELETE FROM config.setting_values
			WHERE setting_key = 'operations.staff_messaging_enabled'
			  AND value = 'false'::jsonb
			  AND updated_at < ?::timestamptz
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (tenant_id, setting_key, old_value, new_value, action, changed_by)
		SELECT tenant_id, setting_key, value, NULL, 'reset', NULL
		FROM deleted;
	`, teamChatOptOutSince).Exec(ctx); err != nil {
		return fmt.Errorf("reset operations.staff_messaging_enabled=false overrides: %w", err)
	}
	return nil
}

// teamChatOptInOffsDown keeps the reset: the old offs were the opt-in default,
// not a school's choice. The settings audit names every school whose value
// went, so the moto team can restore one deliberately.
func teamChatOptInOffsDown(context.Context, *bun.DB) error {
	return nil
}
