package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS settings (
				id BIGSERIAL PRIMARY KEY,
				key TEXT NOT NULL UNIQUE,
				value TEXT NOT NULL,
				category TEXT NOT NULL,
				description TEXT,
				requires_restart BOOLEAN NOT NULL DEFAULT false,
				requires_db_reset BOOLEAN NOT NULL DEFAULT false,
				created_at TIMESTAMP NOT NULL DEFAULT now(),
				modified_at TIMESTAMP NOT NULL DEFAULT now()
			);
			
			-- Insert default settings
			INSERT INTO settings (key, value, category, description) VALUES
			('max_room_capacity', '30', 'room', 'Maximum capacity of a standard room'),
			('default_room_category', 'Other', 'room', 'Default category for new rooms'),
			('login_token_expiry', '30', 'auth', 'Login token expiry in minutes'),
			('combined_group_default_expiry', '1440', 'group', 'Default expiry time for combined groups in minutes (1440 = 24 hours)'),
			('system_name', 'MOTO System', 'system', 'Name of the system displayed in UI and emails'),
			('enable_email_notifications', 'true', 'notification', 'Enable email notifications'),
			('api_rate_limit', '100', 'system', 'API rate limit per minute'),
			('default_student_visit_duration', '60', 'visit', 'Default duration for student visits in minutes')
			ON CONFLICT (key) DO NOTHING;
		`)

		return err
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS settings;`)
		return err
	})
}
