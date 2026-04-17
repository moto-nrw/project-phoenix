package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createActivityInstancesVersion     = "1.15.35"
	createActivityInstancesDescription = "Create activity instances, instance staff/students, and activity exceptions tables for timetable system"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createActivityInstancesVersion,
		Description: createActivityInstancesDescription,
		DependsOn: []string{
			FacilitiesRoomsVersion,       // 1.1.1 — instance.room_id, instance_staff.room_id, instance_students.room_id, activity_exceptions.room_id
			UsersStaffVersion,            // 1.2.3 — instance_staff.staff_id, *.created_by, started_by
			ActivitiesGroupsVersion,      // 1.3.2 — instance.activity_group_id, activity_exceptions.activity_group_id
			UsersStudentsVersion,         // 1.3.5 — instance_students.student_id
			ActiveGroupsVersion,          // 1.4.1 — instance.active_group_id (bridge to live layer)
			enableRLSVersion,             // 1.15.1 — RLS infrastructure
			createCalendarPeriodsVersion, // 1.15.33 — instance.calendar_period_id
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActivityInstancesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createActivityInstancesDown(ctx, db)
		},
	)
}

func createActivityInstancesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.35: Creating activity instances and related tables...")

	// 1. schedule.activity_instances — the concrete materialized instance of a template on a given date,
	//    or a spontaneous instance (activity_group_id IS NULL).
	//    - activity_group_id:  nullable FK — NULL = spontaneous (no template)
	//    - calendar_period_id: nullable FK — NULL = applies regardless of period scope
	//    - room_id:            NOT NULL — primary room (per E3, overrides live on instance_staff / instance_students)
	//    - active_group_id:    nullable FK — bridge to active.groups set on instance start (E4)
	//    - status:             TEXT + CHECK (no DB enum, per plan)
	//    - is_spontaneous:     flag preserved per E1/Iteration 1
	//
	// ON DELETE semantics:
	//    activity_group_id → SET NULL (deleting a template should leave historical instances intact and
	//                        convert them into orphan/spontaneous rows rather than CASCADE-deleting history)
	//    calendar_period_id → SET NULL (matches pattern from migration 1.15.34 on activities.*)
	//    room_id            → RESTRICT (rooms are referenced by live sessions too — same as active.groups)
	//    active_group_id    → SET NULL (deleting an active group should not delete the instance, just clear the bridge)
	_, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.activity_instances (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
			date DATE NOT NULL,
			activity_group_id BIGINT REFERENCES activities.groups(id) ON DELETE SET NULL,
			calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id) ON DELETE SET NULL,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			start_time TIME NOT NULL,
			end_time TIME NOT NULL,
			room_id BIGINT NOT NULL REFERENCES facilities.rooms(id) ON DELETE RESTRICT,
			status TEXT NOT NULL DEFAULT 'planned',
			active_group_id BIGINT REFERENCES active.groups(id) ON DELETE SET NULL,
			is_spontaneous BOOLEAN NOT NULL DEFAULT false,
			notes TEXT,
			created_by BIGINT REFERENCES users.staff(id) ON DELETE SET NULL,
			started_by BIGINT REFERENCES users.staff(id) ON DELETE SET NULL,
			started_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT check_activity_instance_times CHECK (end_time > start_time),
			CONSTRAINT check_activity_instance_status CHECK (status IN ('planned', 'active', 'completed', 'cancelled'))
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating activity_instances table: %w", err)
	}

	// Partial UNIQUE (E10): only template-backed instances must be unique per
	// (tenant_id, date, activity_group_id, start_time). Spontaneous rows
	// (activity_group_id IS NULL) are intentionally excluded — in Postgres
	// NULL != NULL, so without WHERE the constraint would still allow dupes
	// of spontaneous rows but would also block legitimate parallel templates.
	_, err = db.NewRaw(`
		CREATE UNIQUE INDEX idx_activity_instances_template_unique
		ON schedule.activity_instances (tenant_id, date, activity_group_id, start_time)
		WHERE activity_group_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating partial unique index on activity_instances: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activity_instances_tenant_date
		ON schedule.activity_instances (tenant_id, date);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating tenant/date index: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activity_instances_activity_group
		ON schedule.activity_instances (activity_group_id)
		WHERE activity_group_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating activity_group index: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activity_instances_active_group
		ON schedule.activity_instances (active_group_id)
		WHERE active_group_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating active_group index: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activity_instances_status
		ON schedule.activity_instances (tenant_id, status);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating status index: %w", err)
	}

	_, err = db.NewRaw(`ALTER TABLE schedule.activity_instances ENABLE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enable RLS on activity_instances: %w", err)
	}
	_, err = db.NewRaw(`ALTER TABLE schedule.activity_instances FORCE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to force RLS on activity_instances: %w", err)
	}
	_, err = db.NewRaw(`
		CREATE POLICY tenant_isolation_schedule_activity_instances
		ON schedule.activity_instances FOR ALL
		USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
		WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating RLS policy on activity_instances: %w", err)
	}

	// 2. schedule.instance_staff — per-instance staff assignment with multi-room override (E3).
	//    instance_id → CASCADE (staff assignments are owned by the instance lifecycle)
	//    staff_id    → RESTRICT (staff referenced by history must not vanish)
	//    room_id     → SET NULL (NULL = use the instance's primary room)
	_, err = db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.instance_staff (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
			instance_id BIGINT NOT NULL REFERENCES schedule.activity_instances(id) ON DELETE CASCADE,
			staff_id BIGINT NOT NULL REFERENCES users.staff(id) ON DELETE RESTRICT,
			room_id BIGINT REFERENCES facilities.rooms(id) ON DELETE SET NULL,
			is_primary BOOLEAN NOT NULL DEFAULT false,
			is_substitute BOOLEAN NOT NULL DEFAULT false,
			is_absent BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT unique_instance_staff UNIQUE (instance_id, staff_id)
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating instance_staff table: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_instance_staff_instance
		ON schedule.instance_staff (instance_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating instance_staff.instance_id index: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_instance_staff_staff
		ON schedule.instance_staff (staff_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating instance_staff.staff_id index: %w", err)
	}

	_, err = db.NewRaw(`ALTER TABLE schedule.instance_staff ENABLE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed enabling RLS on instance_staff: %w", err)
	}
	_, err = db.NewRaw(`ALTER TABLE schedule.instance_staff FORCE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed forcing RLS on instance_staff: %w", err)
	}
	_, err = db.NewRaw(`
		CREATE POLICY tenant_isolation_schedule_instance_staff
		ON schedule.instance_staff FOR ALL
		USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
		WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating RLS policy on instance_staff: %w", err)
	}

	// 3. schedule.instance_students — three-field attendance model (E18).
	//    status:    system-controlled (expected/present/absent) — TEXT + CHECK
	//    substatus: nullable, human/auto-set (late/excused/sick/field_trip/other) — TEXT + CHECK
	//    note:      nullable freetext up to 500 chars
	//    instance_id → CASCADE (attendance rows belong to the instance lifecycle)
	//    student_id  → RESTRICT (historical attendance must survive; students with history shouldn't be hard-deleted)
	//    room_id     → SET NULL (NULL = use the instance's primary room)
	_, err = db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.instance_students (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
			instance_id BIGINT NOT NULL REFERENCES schedule.activity_instances(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE RESTRICT,
			room_id BIGINT REFERENCES facilities.rooms(id) ON DELETE SET NULL,
			status TEXT NOT NULL DEFAULT 'expected',
			substatus TEXT,
			note TEXT,
			checked_in_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT unique_instance_student UNIQUE (instance_id, student_id),
			CONSTRAINT check_instance_student_status CHECK (status IN ('expected', 'present', 'absent')),
			CONSTRAINT check_instance_student_substatus CHECK (
				substatus IS NULL
				OR substatus IN ('late', 'excused', 'sick', 'field_trip', 'other')
			),
			CONSTRAINT check_instance_student_note_length CHECK (note IS NULL OR char_length(note) <= 500)
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating instance_students table: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_instance_students_instance
		ON schedule.instance_students (instance_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating instance_students.instance_id index: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_instance_students_student
		ON schedule.instance_students (student_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating instance_students.student_id index: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_instance_students_status
		ON schedule.instance_students (instance_id, status);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating instance_students status index: %w", err)
	}

	_, err = db.NewRaw(`ALTER TABLE schedule.instance_students ENABLE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed enabling RLS on instance_students: %w", err)
	}
	_, err = db.NewRaw(`ALTER TABLE schedule.instance_students FORCE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed forcing RLS on instance_students: %w", err)
	}
	_, err = db.NewRaw(`
		CREATE POLICY tenant_isolation_schedule_instance_students
		ON schedule.instance_students FOR ALL
		USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
		WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating RLS policy on instance_students: %w", err)
	}

	// 4. schedule.activity_exceptions — cancel or modify a template's occurrence on a specific date.
	//    Consumed during materialization (E8). `modified` rows may override start/end/room.
	//    activity_group_id → CASCADE (exceptions are owned by their template; template delete cleans them up)
	//    room_id           → SET NULL (if an override room gets deleted, drop the override rather than
	//                        failing the template delete chain)
	_, err = db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.activity_exceptions (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
			activity_group_id BIGINT NOT NULL REFERENCES activities.groups(id) ON DELETE CASCADE,
			exception_date DATE NOT NULL,
			exception_type TEXT NOT NULL,
			start_time TIME,
			end_time TIME,
			room_id BIGINT REFERENCES facilities.rooms(id) ON DELETE SET NULL,
			reason VARCHAR(500),
			created_by BIGINT REFERENCES users.staff(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT unique_activity_exception UNIQUE (tenant_id, activity_group_id, exception_date),
			CONSTRAINT check_activity_exception_type CHECK (exception_type IN ('cancelled', 'modified')),
			CONSTRAINT check_activity_exception_times CHECK (
				start_time IS NULL
				OR end_time IS NULL
				OR end_time > start_time
			)
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating activity_exceptions table: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activity_exceptions_activity_group
		ON schedule.activity_exceptions (activity_group_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating activity_exceptions.activity_group index: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activity_exceptions_date
		ON schedule.activity_exceptions (tenant_id, exception_date);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating activity_exceptions date index: %w", err)
	}

	_, err = db.NewRaw(`ALTER TABLE schedule.activity_exceptions ENABLE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed enabling RLS on activity_exceptions: %w", err)
	}
	_, err = db.NewRaw(`ALTER TABLE schedule.activity_exceptions FORCE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed forcing RLS on activity_exceptions: %w", err)
	}
	_, err = db.NewRaw(`
		CREATE POLICY tenant_isolation_schedule_activity_exceptions
		ON schedule.activity_exceptions FOR ALL
		USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
		WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating RLS policy on activity_exceptions: %w", err)
	}

	// Grants on the schedule schema are handled via ALTER DEFAULT PRIVILEGES in
	// migration 1.14.1 — phoenix_tenant automatically gets SELECT/INSERT/UPDATE/DELETE.

	return nil
}

func createActivityInstancesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.35: Dropping activity instance tables...")

	// Drop in reverse dependency order: children first, then parent.
	_, err := db.NewRaw(`DROP TABLE IF EXISTS schedule.activity_exceptions CASCADE;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping activity_exceptions: %w", err)
	}

	_, err = db.NewRaw(`DROP TABLE IF EXISTS schedule.instance_students CASCADE;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping instance_students: %w", err)
	}

	_, err = db.NewRaw(`DROP TABLE IF EXISTS schedule.instance_staff CASCADE;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping instance_staff: %w", err)
	}

	_, err = db.NewRaw(`DROP TABLE IF EXISTS schedule.activity_instances CASCADE;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping activity_instances: %w", err)
	}

	return nil
}
