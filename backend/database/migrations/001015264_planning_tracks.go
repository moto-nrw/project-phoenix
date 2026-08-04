package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	planningTracksVersion     = "1.15.264"
	planningTracksDescription = "Add tenant planning tracks and optional recurring-template assignments (issue #2138)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     planningTracksVersion,
		Description: planningTracksDescription,
		DependsOn:   []string{timetableConflictAcksVersion},
	})
	Migrations.MustRegister(planningTracksUp, planningTracksDown)
}

func planningTracksUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.264: Creating schedule.planning_tracks...")
	if _, err := db.NewRaw(`
		CREATE TABLE schedule.planning_tracks (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
			name TEXT NOT NULL,
			color TEXT NOT NULL,
			sort_order INTEGER NOT NULL,
			archived_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uniq_planning_tracks_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT chk_planning_tracks_name CHECK (LENGTH(BTRIM(name)) BETWEEN 1 AND 100),
			CONSTRAINT chk_planning_tracks_color CHECK (color ~ '^#[0-9A-Fa-f]{6}$'),
			CONSTRAINT chk_planning_tracks_sort_order CHECK (sort_order >= 0)
		);

		CREATE UNIQUE INDEX uniq_planning_tracks_tenant_active_name
			ON schedule.planning_tracks (tenant_id, LOWER(BTRIM(name)))
			WHERE archived_at IS NULL;
		CREATE INDEX idx_planning_tracks_tenant_order
			ON schedule.planning_tracks (tenant_id, sort_order, id)
			WHERE archived_at IS NULL;

		CREATE TRIGGER update_planning_tracks_updated_at
		BEFORE UPDATE ON schedule.planning_tracks
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		ALTER TABLE schedule.planning_tracks ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.planning_tracks FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_schedule_planning_tracks ON schedule.planning_tracks
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT);

		GRANT SELECT, INSERT, UPDATE ON schedule.planning_tracks TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.planning_tracks_id_seq TO phoenix_tenant;

		ALTER TABLE activities.groups ADD COLUMN planning_track_id BIGINT;
		ALTER TABLE activities.groups
			ADD CONSTRAINT fk_groups_planning_track
			FOREIGN KEY (tenant_id, planning_track_id)
			REFERENCES schedule.planning_tracks (tenant_id, id)
			ON DELETE RESTRICT;
		CREATE INDEX idx_groups_planning_track
			ON activities.groups (tenant_id, planning_track_id)
			WHERE planning_track_id IS NOT NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("create planning tracks: %w", err)
	}
	return nil
}

func planningTracksDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.264: Dropping schedule.planning_tracks...")
	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_groups_planning_track;
		ALTER TABLE activities.groups DROP CONSTRAINT IF EXISTS fk_groups_planning_track;
		ALTER TABLE activities.groups DROP COLUMN IF EXISTS planning_track_id;
		DROP TRIGGER IF EXISTS update_planning_tracks_updated_at ON schedule.planning_tracks;
		DROP TABLE IF EXISTS schedule.planning_tracks;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop planning tracks: %w", err)
	}
	return nil
}
