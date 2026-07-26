package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	personalCalendarVersion     = "1.15.173"
	personalCalendarDescription = "Create tenant-scoped personal calendar appointments, recipients, recurrence, and permissions"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     personalCalendarVersion,
		Description: personalCalendarDescription,
		DependsOn: []string{
			"1.15.1",  // RLS infrastructure
			"1.2.3",   // users.staff
			"1.3.5",   // users.students
			"1.3.5.1", // users.guardian_profiles
			"1.0.5",   // auth.permissions
			"1.0.4",   // auth.roles
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return personalCalendarUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return personalCalendarDown(ctx, db)
		},
	)
}

func personalCalendarUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.173: Creating personal calendar tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	if _, err = tx.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS calendar;`); err != nil {
		return fmt.Errorf("error creating calendar schema: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS calendar.appointments (
			id                 BIGSERIAL PRIMARY KEY,
			tenant_id          BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			-- No ON DELETE action needed: users.staff is soft-deleted
			-- (deleted_at, bun soft_delete tag). Offboarding sets deleted_at and
			-- leaves the row in place (see services/users/staff_offboarding.go —
			-- "before the soft delete"), so this FK is never violated. Staff rows
			-- are not hard-deleted anywhere, so CASCADE/SET NULL would be dead
			-- (and CASCADE is undesirable here — a departed organizer must not
			-- silently delete their appointments).
			organizer_staff_id BIGINT NOT NULL REFERENCES users.staff(id),
			title              TEXT NOT NULL,
			description        TEXT,
			location           TEXT,
			start_date         DATE NOT NULL,
			end_date           DATE NOT NULL,
			start_time         TIME WITHOUT TIME ZONE NOT NULL,
			end_time           TIME WITHOUT TIME ZONE NOT NULL,
			all_day            BOOLEAN NOT NULL DEFAULT FALSE,
			delivery_mode      TEXT NOT NULL,
			cancelled_at       TIMESTAMPTZ,
			created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_calendar_appointments_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT chk_calendar_appointments_title_not_blank CHECK (length(btrim(title)) > 0),
			CONSTRAINT chk_calendar_appointments_date_order CHECK (end_date >= start_date),
			CONSTRAINT chk_calendar_appointments_same_day_time_order CHECK (
				all_day OR end_date > start_date OR end_time > start_time
			),
			CONSTRAINT chk_calendar_appointments_delivery_mode CHECK (
				delivery_mode IN ('rsvp_required', 'informational')
			)
		);

		CREATE INDEX IF NOT EXISTS idx_calendar_appointments_range
			ON calendar.appointments (tenant_id, start_date, end_date);
		CREATE INDEX IF NOT EXISTS idx_calendar_appointments_organizer
			ON calendar.appointments (tenant_id, organizer_staff_id, start_date);

		CREATE TABLE IF NOT EXISTS calendar.recurrence_rules (
			id               BIGSERIAL PRIMARY KEY,
			tenant_id        BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			appointment_id   BIGINT NOT NULL,
			frequency        TEXT NOT NULL,
			interval_count   INT NOT NULL DEFAULT 1,
			weekdays         TEXT[],
			month_days       INT[],
			ends_on          DATE,
			occurrence_count INT,
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_calendar_recurrence_appointment_tenant
				FOREIGN KEY (tenant_id, appointment_id)
				REFERENCES calendar.appointments (tenant_id, id)
				ON DELETE CASCADE,
			CONSTRAINT uq_calendar_recurrence_rules_appointment UNIQUE (appointment_id),
			CONSTRAINT chk_calendar_recurrence_frequency CHECK (
				frequency IN ('daily', 'weekly', 'monthly', 'yearly')
			),
			CONSTRAINT chk_calendar_recurrence_interval CHECK (interval_count > 0),
			CONSTRAINT chk_calendar_recurrence_occurrence_count CHECK (
				occurrence_count IS NULL OR occurrence_count > 0
			),
			CONSTRAINT chk_calendar_recurrence_single_end CHECK (
				ends_on IS NULL OR occurrence_count IS NULL
			)
		);
		CREATE INDEX IF NOT EXISTS idx_calendar_recurrence_lookup
			ON calendar.recurrence_rules (tenant_id, appointment_id, ends_on);

		CREATE TABLE IF NOT EXISTS calendar.appointment_recipients (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			appointment_id      BIGINT NOT NULL,
			recipient_type      TEXT NOT NULL,
			-- No ON DELETE action: users.staff is soft-deleted (see the note on
			-- appointments.organizer_staff_id above), so a staff recipient row is
			-- never orphaned by a hard delete.
			staff_id            BIGINT REFERENCES users.staff(id),
			-- ON DELETE CASCADE: a guardian profile deletion (e.g. GDPR account
			-- removal via DeleteGuardianWithLinks) must not fail on this FK.
			-- Removing the guardian drops their recipient rows (and the
			-- recipient_students rows cascade from appointment_recipients below);
			-- the appointment itself is retained.
			guardian_profile_id BIGINT REFERENCES users.guardian_profiles(id) ON DELETE CASCADE,
			status              TEXT NOT NULL,
			responded_at        TIMESTAMPTZ,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_calendar_appointment_recipients_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT fk_calendar_recipients_appointment_tenant
				FOREIGN KEY (tenant_id, appointment_id)
				REFERENCES calendar.appointments (tenant_id, id)
				ON DELETE CASCADE,
			CONSTRAINT chk_calendar_recipients_type CHECK (
				recipient_type IN ('staff', 'guardian_profile')
			),
			CONSTRAINT chk_calendar_recipients_exact_subject CHECK (
				(recipient_type = 'staff' AND staff_id IS NOT NULL AND guardian_profile_id IS NULL)
				OR (recipient_type = 'guardian_profile' AND guardian_profile_id IS NOT NULL AND staff_id IS NULL)
			),
			CONSTRAINT chk_calendar_recipients_status CHECK (
				status IN ('pending', 'accepted', 'declined', 'info')
			)
		);
		CREATE UNIQUE INDEX IF NOT EXISTS uq_calendar_recipients_staff
			ON calendar.appointment_recipients (tenant_id, appointment_id, staff_id)
			WHERE staff_id IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_calendar_recipients_guardian
			ON calendar.appointment_recipients (tenant_id, appointment_id, guardian_profile_id)
			WHERE guardian_profile_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_calendar_recipients_staff
			ON calendar.appointment_recipients (tenant_id, staff_id, appointment_id)
			WHERE staff_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_calendar_recipients_guardian
			ON calendar.appointment_recipients (tenant_id, guardian_profile_id, appointment_id)
			WHERE guardian_profile_id IS NOT NULL;

		CREATE TABLE IF NOT EXISTS calendar.appointment_recipient_students (
			id           BIGSERIAL PRIMARY KEY,
			tenant_id    BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			recipient_id BIGINT NOT NULL,
			student_id   BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_calendar_recipient_students_recipient_tenant
				FOREIGN KEY (tenant_id, recipient_id)
				REFERENCES calendar.appointment_recipients (tenant_id, id)
				ON DELETE CASCADE,
			CONSTRAINT uq_calendar_recipient_students UNIQUE (tenant_id, recipient_id, student_id)
		);
		CREATE INDEX IF NOT EXISTS idx_calendar_recipient_students_student
			ON calendar.appointment_recipient_students (tenant_id, student_id);

		CREATE TABLE IF NOT EXISTS calendar.appointment_targets (
			id             BIGSERIAL PRIMARY KEY,
			tenant_id      BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			appointment_id BIGINT NOT NULL,
			target_type    TEXT NOT NULL,
			target_id      BIGINT,
			target_value   TEXT,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_calendar_targets_appointment_tenant
				FOREIGN KEY (tenant_id, appointment_id)
				REFERENCES calendar.appointments (tenant_id, id)
				ON DELETE CASCADE,
			CONSTRAINT chk_calendar_targets_type CHECK (
				target_type IN (
					'staff',
					'guardian_profile',
					'all_staff',
					'parents_by_class',
					'parents_by_group',
					'parents_by_student'
				)
			)
		);
		CREATE INDEX IF NOT EXISTS idx_calendar_targets_appointment
			ON calendar.appointment_targets (tenant_id, appointment_id);

		CREATE TABLE IF NOT EXISTS calendar.appointment_occurrence_overrides (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			appointment_id  BIGINT NOT NULL,
			occurrence_date DATE NOT NULL,
			cancelled       BOOLEAN NOT NULL DEFAULT FALSE,
			title           TEXT,
			description     TEXT,
			location        TEXT,
			start_date      DATE,
			end_date        DATE,
			start_time      TIME WITHOUT TIME ZONE,
			end_time        TIME WITHOUT TIME ZONE,
			all_day         BOOLEAN,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_calendar_overrides_appointment_tenant
				FOREIGN KEY (tenant_id, appointment_id)
				REFERENCES calendar.appointments (tenant_id, id)
				ON DELETE CASCADE,
			CONSTRAINT uq_calendar_occurrence_overrides UNIQUE (tenant_id, appointment_id, occurrence_date),
			CONSTRAINT chk_calendar_occurrence_override_date_order CHECK (
				start_date IS NULL OR end_date IS NULL OR end_date >= start_date
			)
		);
		CREATE INDEX IF NOT EXISTS idx_calendar_occurrence_overrides_range
			ON calendar.appointment_occurrence_overrides (tenant_id, appointment_id, occurrence_date);
	`)
	if err != nil {
		return fmt.Errorf("error creating calendar tables: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_calendar_appointments_updated_at ON calendar.appointments;
		CREATE TRIGGER update_calendar_appointments_updated_at
		BEFORE UPDATE ON calendar.appointments
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_calendar_recurrence_rules_updated_at ON calendar.recurrence_rules;
		CREATE TRIGGER update_calendar_recurrence_rules_updated_at
		BEFORE UPDATE ON calendar.recurrence_rules
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_calendar_appointment_recipients_updated_at ON calendar.appointment_recipients;
		CREATE TRIGGER update_calendar_appointment_recipients_updated_at
		BEFORE UPDATE ON calendar.appointment_recipients
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_calendar_appointment_recipient_students_updated_at ON calendar.appointment_recipient_students;
		CREATE TRIGGER update_calendar_appointment_recipient_students_updated_at
		BEFORE UPDATE ON calendar.appointment_recipient_students
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_calendar_appointment_targets_updated_at ON calendar.appointment_targets;
		CREATE TRIGGER update_calendar_appointment_targets_updated_at
		BEFORE UPDATE ON calendar.appointment_targets
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_calendar_appointment_occurrence_overrides_updated_at ON calendar.appointment_occurrence_overrides;
		CREATE TRIGGER update_calendar_appointment_occurrence_overrides_updated_at
		BEFORE UPDATE ON calendar.appointment_occurrence_overrides
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating calendar updated_at triggers: %w", err)
	}

	for _, table := range []string{
		"appointments",
		"recurrence_rules",
		"appointment_recipients",
		"appointment_recipient_students",
		"appointment_targets",
		"appointment_occurrence_overrides",
	} {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE calendar.%[1]s ENABLE ROW LEVEL SECURITY;
			ALTER TABLE calendar.%[1]s FORCE ROW LEVEL SECURITY;

			DO $$
			BEGIN
				IF NOT EXISTS (
					SELECT 1 FROM pg_policies
					WHERE schemaname = 'calendar'
						AND tablename = '%[1]s'
						AND policyname = 'tenant_isolation_calendar_%[1]s'
				) THEN
					CREATE POLICY tenant_isolation_calendar_%[1]s ON calendar.%[1]s
						FOR ALL
						USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
						WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
				END IF;
			END $$;

			GRANT SELECT, INSERT, UPDATE, DELETE ON calendar.%[1]s TO phoenix_tenant;
		`, table))
		if err != nil {
			return fmt.Errorf("error enabling RLS on calendar.%s: %w", table, err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		GRANT USAGE ON SCHEMA calendar TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE calendar.appointments_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE calendar.recurrence_rules_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE calendar.appointment_recipients_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE calendar.appointment_recipient_students_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE calendar.appointment_targets_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE calendar.appointment_occurrence_overrides_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error granting calendar table permissions: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action)
		VALUES
			('calendar:own', 'Use personal calendar and respond to own invitations', 'calendar', 'own'),
			('calendar:manage', 'Create and manage calendar appointments and invitations', 'calendar', 'manage')
		ON CONFLICT (name) DO NOTHING;

		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE p.name = 'calendar:own'
		  AND r.name IN ('admin', 'user', 'teacher', 'staff', 'guardian')
		ON CONFLICT (role_id, permission_id) DO NOTHING;

		-- calendar:manage gates appointment creation and recipient search
		-- (create school-wide staff/parent invitations). This is an
		-- administration capability, so grant it only to the admin role by
		-- default; schools can assign it to additional roles via the
		-- role-permission admin UI. calendar:own (respond to own invitations)
		-- above stays broad.
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE p.name = 'calendar:manage'
		  AND r.name IN ('admin')
		ON CONFLICT (role_id, permission_id) DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("error inserting calendar permissions: %w", err)
	}

	fmt.Println("Migration 1.15.173: Successfully created personal calendar tables")
	return tx.Commit()
}

func personalCalendarDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.173: Dropping personal calendar tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	if _, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE permission_id IN (
			SELECT id FROM auth.permissions WHERE resource = 'calendar'
		);
		DELETE FROM auth.permissions WHERE resource = 'calendar';

		DROP TABLE IF EXISTS calendar.appointment_occurrence_overrides CASCADE;
		DROP TABLE IF EXISTS calendar.appointment_targets CASCADE;
		DROP TABLE IF EXISTS calendar.appointment_recipient_students CASCADE;
		DROP TABLE IF EXISTS calendar.appointment_recipients CASCADE;
		DROP TABLE IF EXISTS calendar.recurrence_rules CASCADE;
		DROP TABLE IF EXISTS calendar.appointments CASCADE;
		DROP SCHEMA IF EXISTS calendar;
	`); err != nil {
		return fmt.Errorf("error dropping personal calendar tables: %w", err)
	}

	return tx.Commit()
}
