package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	groupTargetsVersion     = "1.15.265"
	groupTargetsDescription = "Normalize multiple dynamic target groups for timetable templates (#2130)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     groupTargetsVersion,
		Description: groupTargetsDescription,
		DependsOn:   []string{rosterWeekdayScopeVersion},
	})
	Migrations.MustRegister(groupTargetsUp, groupTargetsDown)
}

func groupTargetsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.265: Creating activities.group_targets...")
	_, err := db.NewRaw(`
		CREATE TABLE activities.group_targets (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL,
			activity_group_id   BIGINT NOT NULL,
			target_group_type   TEXT NOT NULL,
			target_grade_level  SMALLINT,
			target_school_class TEXT,
			education_group_id  BIGINT,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_group_targets_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT fk_group_targets_school FOREIGN KEY (tenant_id)
				REFERENCES platform.schools(id) ON DELETE CASCADE,
			CONSTRAINT fk_group_targets_activity_group FOREIGN KEY (tenant_id, activity_group_id)
				REFERENCES activities.groups(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_group_targets_education_group FOREIGN KEY (tenant_id, education_group_id)
				REFERENCES education.groups(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT chk_group_targets_type CHECK (
				target_group_type IN ('jahrgang', 'klasse', 'gruppe')
			),
			CONSTRAINT chk_group_targets_value CHECK (
				(target_group_type = 'jahrgang'
					AND target_grade_level IS NOT NULL
					AND target_grade_level BETWEEN 1 AND 13
					AND target_school_class IS NULL
					AND education_group_id IS NULL)
				OR (target_group_type = 'klasse'
					AND target_grade_level IS NULL
					AND target_school_class IS NOT NULL
					AND BTRIM(target_school_class) <> ''
					AND education_group_id IS NULL)
				OR (target_group_type = 'gruppe'
					AND target_grade_level IS NULL
					AND target_school_class IS NULL
					AND education_group_id IS NOT NULL)
			)
		);

		CREATE UNIQUE INDEX uq_group_targets_grade
			ON activities.group_targets (tenant_id, activity_group_id, target_grade_level)
			WHERE target_group_type = 'jahrgang';
		CREATE UNIQUE INDEX uq_group_targets_class
			ON activities.group_targets (tenant_id, activity_group_id, LOWER(BTRIM(target_school_class)))
			WHERE target_group_type = 'klasse';
		CREATE UNIQUE INDEX uq_group_targets_group
			ON activities.group_targets (tenant_id, activity_group_id, education_group_id)
			WHERE target_group_type = 'gruppe';
		CREATE INDEX idx_group_targets_activity_group
			ON activities.group_targets (tenant_id, activity_group_id);
		CREATE INDEX idx_students_tenant_school_class_normalized
			ON users.students (tenant_id, LOWER(BTRIM(school_class)));

		INSERT INTO activities.group_targets (
			tenant_id, activity_group_id, target_group_type,
			target_grade_level, target_school_class, education_group_id
		)
		SELECT
			tenant_id, id, target_group_type,
			target_grade_level, target_school_class,
			CASE WHEN target_group_type = 'gruppe' THEN education_group_id END
		FROM activities.groups
		WHERE target_group_type IN ('jahrgang', 'klasse', 'gruppe');

		CREATE TRIGGER update_group_targets_updated_at
		BEFORE UPDATE ON activities.group_targets
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		ALTER TABLE activities.group_targets ENABLE ROW LEVEL SECURITY;
		ALTER TABLE activities.group_targets FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_activities_group_targets
			ON activities.group_targets FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON activities.group_targets TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE activities.group_targets_id_seq TO phoenix_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating activities.group_targets: %w", err)
	}
	return nil
}

func groupTargetsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.265: Dropping activities.group_targets...")
	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS users.idx_students_tenant_school_class_normalized;
		DROP TABLE IF EXISTS activities.group_targets CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping activities.group_targets: %w", err)
	}
	return nil
}
