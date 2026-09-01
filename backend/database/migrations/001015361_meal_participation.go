package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	mealParticipationVersion     = "1.15.361"
	mealParticipationDescription = "Add recurring and date-specific lunch participation (#2638)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealParticipationVersion,
		Description: mealParticipationDescription,
		DependsOn:   []string{leasedDeliveryOutboxVersion},
	})
	Migrations.MustRegister(mealParticipationUp, mealParticipationDown)
}

func mealParticipationUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting", "migration", mealParticipationVersion)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			slog.Warn("migration rollback failed", "migration", mealParticipationVersion, "error", rollbackErr)
		}
	}()

	_, err = tx.NewRaw(`
		CREATE TABLE schedule.meal_participation_schedules (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL,
			effective_from DATE NOT NULL,
			monday BOOLEAN NOT NULL DEFAULT FALSE,
			tuesday BOOLEAN NOT NULL DEFAULT FALSE,
			wednesday BOOLEAN NOT NULL DEFAULT FALSE,
			thursday BOOLEAN NOT NULL DEFAULT FALSE,
			friday BOOLEAN NOT NULL DEFAULT FALSE,
			changed_by_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_meal_participation_schedule UNIQUE (tenant_id, student_id, effective_from),
			CONSTRAINT fk_meal_participation_schedule_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.students(tenant_id, id) ON DELETE CASCADE
		);

		CREATE INDEX idx_meal_participation_schedule_lookup
			ON schedule.meal_participation_schedules (tenant_id, student_id, effective_from DESC);

		CREATE TABLE schedule.meal_participation_overrides (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL,
			date DATE NOT NULL,
			participating BOOLEAN NOT NULL,
			changed_by_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_meal_participation_override UNIQUE (tenant_id, student_id, date),
			CONSTRAINT fk_meal_participation_override_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.students(tenant_id, id) ON DELETE CASCADE
		);

		CREATE INDEX idx_meal_participation_override_date
			ON schedule.meal_participation_overrides (tenant_id, date, student_id);

		CREATE TABLE schedule.meal_sickness_status_history (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL,
			date DATE NOT NULL,
			changed_at TIMESTAMPTZ NOT NULL,
			reported_at TIMESTAMPTZ NOT NULL,
			cleared_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_meal_sickness_history_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.students(tenant_id, id) ON DELETE CASCADE
		);

		CREATE INDEX idx_meal_sickness_history_cutoff
			ON schedule.meal_sickness_status_history (tenant_id, date, student_id, changed_at DESC, id DESC);

		CREATE FUNCTION schedule.record_meal_sickness_status()
		RETURNS TRIGGER AS $$
		DECLARE
			effective_change TIMESTAMPTZ;
		BEGIN
			IF TG_OP = 'DELETE' THEN
				IF OLD.status <> 'sick' THEN
					RETURN OLD;
				END IF;
				effective_change := clock_timestamp();
				INSERT INTO schedule.meal_sickness_status_history
					(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
				VALUES
					(OLD.tenant_id, OLD.student_id, OLD.date, effective_change, OLD.reported_at, effective_change);
				RETURN OLD;
			END IF;

			IF TG_OP = 'UPDATE' AND OLD.status = 'sick' AND NEW.status <> 'sick' THEN
				effective_change := clock_timestamp();
				INSERT INTO schedule.meal_sickness_status_history
					(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
				VALUES
					(OLD.tenant_id, OLD.student_id, OLD.date, effective_change, OLD.reported_at, effective_change);
				RETURN NEW;
			END IF;

			IF NEW.status <> 'sick' THEN
				RETURN NEW;
			END IF;
			IF TG_OP = 'INSERT' THEN
				effective_change := NEW.reported_at;
			ELSIF OLD.status <> 'sick' OR NEW.reported_at IS DISTINCT FROM OLD.reported_at THEN
				effective_change := NEW.reported_at;
			ELSIF NEW.cleared_at IS DISTINCT FROM OLD.cleared_at THEN
				effective_change := COALESCE(NEW.cleared_at, NEW.reported_at);
			ELSE
				RETURN NEW;
			END IF;

			INSERT INTO schedule.meal_sickness_status_history
				(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
			VALUES
				(NEW.tenant_id, NEW.student_id, NEW.date, effective_change, NEW.reported_at, NEW.cleared_at);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		CREATE TRIGGER record_meal_sickness_status
			AFTER INSERT OR UPDATE OF status, reported_at, cleared_at ON active.student_status_days
			FOR EACH ROW EXECUTE FUNCTION schedule.record_meal_sickness_status();
		CREATE TRIGGER record_deleted_meal_sickness_status
			AFTER DELETE ON active.student_status_days
			FOR EACH ROW EXECUTE FUNCTION schedule.record_meal_sickness_status();

		INSERT INTO schedule.meal_sickness_status_history
			(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
		SELECT tenant_id, student_id, date, reported_at, reported_at, NULL
		FROM active.student_status_days
		WHERE status = 'sick';

		INSERT INTO schedule.meal_sickness_status_history
			(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
		SELECT tenant_id, student_id, date, cleared_at, reported_at, cleared_at
		FROM active.student_status_days
		WHERE status = 'sick' AND cleared_at IS NOT NULL;

		CREATE TRIGGER update_meal_participation_schedules_updated_at
			BEFORE UPDATE ON schedule.meal_participation_schedules
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();
		CREATE TRIGGER update_meal_participation_overrides_updated_at
			BEFORE UPDATE ON schedule.meal_participation_overrides
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		ALTER TABLE schedule.meal_participation_schedules ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.meal_participation_schedules FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_schedule_meal_participation_schedules
			ON schedule.meal_participation_schedules FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT);

		ALTER TABLE schedule.meal_participation_overrides ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.meal_participation_overrides FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_schedule_meal_participation_overrides
			ON schedule.meal_participation_overrides FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT);

		ALTER TABLE schedule.meal_sickness_status_history ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.meal_sickness_status_history FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_schedule_meal_sickness_status_history
			ON schedule.meal_sickness_status_history FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.meal_participation_schedules TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.meal_participation_overrides TO phoenix_tenant;
		GRANT SELECT, INSERT ON schedule.meal_sickness_status_history TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.meal_participation_schedules_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.meal_participation_overrides_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.meal_sickness_status_history_id_seq TO phoenix_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create meal participation tables: %w", err)
	}
	return tx.Commit()
}

func mealParticipationDown(ctx context.Context, db *bun.DB) error {
	slog.Info("migration rollback starting", "migration", mealParticipationVersion)
	_, err := db.NewRaw(`
		DROP TRIGGER IF EXISTS record_meal_sickness_status ON active.student_status_days;
		DROP TRIGGER IF EXISTS record_deleted_meal_sickness_status ON active.student_status_days;
		DROP FUNCTION IF EXISTS schedule.record_meal_sickness_status();
		DROP TABLE IF EXISTS schedule.meal_sickness_status_history CASCADE;
		DROP TABLE IF EXISTS schedule.meal_participation_overrides CASCADE;
		DROP TABLE IF EXISTS schedule.meal_participation_schedules CASCADE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("drop meal participation tables: %w", err)
	}
	return nil
}
