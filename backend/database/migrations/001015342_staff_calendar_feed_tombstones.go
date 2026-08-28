package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffCalendarFeedTombstonesVersion     = "1.15.342"
	staffCalendarFeedTombstonesDescription = "Retain cancelled staff timetable and shift feed events"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffCalendarFeedTombstonesVersion,
		Description: staffCalendarFeedTombstonesDescription,
		DependsOn:   []string{staffCalendarFeedTokenVersion},
	})

	Migrations.MustRegister(staffCalendarFeedTombstonesUp, staffCalendarFeedTombstonesDown)
}

func staffCalendarFeedTombstonesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.342: Adding staff calendar feed tombstones...")
	if _, err := db.NewRaw(`
		CREATE TABLE calendar.staff_feed_tombstones (
			id           BIGSERIAL PRIMARY KEY,
			tenant_id    BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id     BIGINT NOT NULL REFERENCES users.staff(id) ON DELETE CASCADE,
			source       TEXT NOT NULL,
			source_id    BIGINT NOT NULL,
			title        TEXT NOT NULL,
			event_date   DATE NOT NULL,
			start_time   TIME WITHOUT TIME ZONE NOT NULL,
			end_time     TIME WITHOUT TIME ZONE NOT NULL,
			cancelled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_calendar_staff_feed_tombstone_source
				CHECK (source IN ('timetable', 'shift')),
			CONSTRAINT uq_calendar_staff_feed_tombstone
				UNIQUE (tenant_id, staff_id, source, source_id)
		);

		CREATE INDEX idx_calendar_staff_feed_tombstones_lookup
			ON calendar.staff_feed_tombstones (tenant_id, staff_id, cancelled_at);

		ALTER TABLE calendar.staff_feed_tombstones ENABLE ROW LEVEL SECURITY;
		ALTER TABLE calendar.staff_feed_tombstones FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_calendar_staff_feed_tombstones
			ON calendar.staff_feed_tombstones FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON calendar.staff_feed_tombstones TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE calendar.staff_feed_tombstones_id_seq TO phoenix_tenant;

		CREATE OR REPLACE FUNCTION calendar.upsert_staff_feed_tombstone(
			p_tenant_id BIGINT,
			p_staff_id BIGINT,
			p_source TEXT,
			p_source_id BIGINT,
			p_title TEXT,
			p_event_date calendar.staff_feed_tombstones.event_date%TYPE,
			p_start_time TIME WITHOUT TIME ZONE,
			p_end_time TIME WITHOUT TIME ZONE
		) RETURNS VOID
		LANGUAGE plpgsql
		AS $$
		BEGIN
			INSERT INTO calendar.staff_feed_tombstones (
				tenant_id, staff_id, source, source_id, title,
				event_date, start_time, end_time, cancelled_at
			) VALUES (
				p_tenant_id, p_staff_id, p_source, p_source_id, p_title,
				p_event_date, p_start_time, p_end_time, NOW()
			)
			ON CONFLICT (tenant_id, staff_id, source, source_id) DO UPDATE SET
				title = EXCLUDED.title,
				event_date = EXCLUDED.event_date,
				start_time = EXCLUDED.start_time,
				end_time = EXCLUDED.end_time,
				cancelled_at = EXCLUDED.cancelled_at,
				updated_at = NOW();
		END;
		$$;

		CREATE OR REPLACE FUNCTION calendar.capture_instance_staff_feed_tombstone()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		AS $$
		DECLARE
			instance_row schedule.activity_instances%ROWTYPE;
		BEGIN
			IF TG_OP = 'UPDATE'
				AND OLD.staff_id = NEW.staff_id
				AND OLD.instance_id = NEW.instance_id THEN
				RETURN NEW;
			END IF;

			SELECT * INTO instance_row
			FROM schedule.activity_instances
			WHERE tenant_id = OLD.tenant_id AND id = OLD.instance_id;

			IF FOUND THEN
				PERFORM calendar.upsert_staff_feed_tombstone(
					OLD.tenant_id, OLD.staff_id, 'timetable', instance_row.id,
					instance_row.title, instance_row.date,
					instance_row.start_time, instance_row.end_time
				);
			END IF;
			IF TG_OP = 'DELETE' THEN
				RETURN OLD;
			END IF;
			RETURN NEW;
		END;
		$$;

		CREATE TRIGGER capture_instance_staff_feed_tombstone
			BEFORE DELETE OR UPDATE OF staff_id, instance_id ON schedule.instance_staff
			FOR EACH ROW EXECUTE FUNCTION calendar.capture_instance_staff_feed_tombstone();

		CREATE OR REPLACE FUNCTION calendar.capture_activity_instance_feed_tombstones()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		AS $$
		DECLARE
			assignment RECORD;
		BEGIN
			FOR assignment IN
				SELECT staff_id
				FROM schedule.instance_staff
				WHERE tenant_id = OLD.tenant_id AND instance_id = OLD.id
			LOOP
				PERFORM calendar.upsert_staff_feed_tombstone(
					OLD.tenant_id, assignment.staff_id, 'timetable', OLD.id,
					OLD.title, OLD.date, OLD.start_time, OLD.end_time
				);
			END LOOP;
			RETURN OLD;
		END;
		$$;

		CREATE TRIGGER capture_activity_instance_feed_tombstones
			BEFORE DELETE ON schedule.activity_instances
			FOR EACH ROW EXECUTE FUNCTION calendar.capture_activity_instance_feed_tombstones();

		CREATE OR REPLACE FUNCTION calendar.capture_staff_shift_feed_tombstone()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		AS $$
		BEGIN
			IF TG_OP = 'UPDATE' AND OLD.staff_id = NEW.staff_id THEN
				RETURN NEW;
			END IF;
			PERFORM calendar.upsert_staff_feed_tombstone(
				OLD.tenant_id, OLD.staff_id, 'shift', OLD.id,
				'Dienst', OLD.date, OLD.start_time, OLD.end_time
			);
			IF TG_OP = 'DELETE' THEN
				RETURN OLD;
			END IF;
			RETURN NEW;
		END;
		$$;

		CREATE TRIGGER capture_staff_shift_feed_tombstone
			BEFORE DELETE OR UPDATE OF staff_id ON schedule.staff_shifts
			FOR EACH ROW EXECUTE FUNCTION calendar.capture_staff_shift_feed_tombstone();
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding staff calendar feed tombstones: %w", err)
	}
	return nil
}

func staffCalendarFeedTombstonesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.342: Dropping staff calendar feed tombstones...")
	if _, err := db.NewRaw(`
		DROP TRIGGER IF EXISTS capture_staff_shift_feed_tombstone ON schedule.staff_shifts;
		DROP TRIGGER IF EXISTS capture_activity_instance_feed_tombstones ON schedule.activity_instances;
		DROP TRIGGER IF EXISTS capture_instance_staff_feed_tombstone ON schedule.instance_staff;
		DROP FUNCTION IF EXISTS calendar.capture_staff_shift_feed_tombstone();
		DROP FUNCTION IF EXISTS calendar.capture_activity_instance_feed_tombstones();
		DROP FUNCTION IF EXISTS calendar.capture_instance_staff_feed_tombstone();
		DROP FUNCTION IF EXISTS calendar.upsert_staff_feed_tombstone;
		DROP TABLE IF EXISTS calendar.staff_feed_tombstones;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping staff calendar feed tombstones: %w", err)
	}
	return nil
}
