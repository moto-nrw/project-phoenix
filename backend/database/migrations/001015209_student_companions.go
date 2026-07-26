package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	studentCompanionsVersion     = "1.15.209"
	studentCompanionsDescription = "Create users.student_companions - per-weekday child-to-child departure links (Laufgemeinschaften)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentCompanionsVersion,
		Description: studentCompanionsDescription,
		DependsOn: []string{
			UsersStudentsVersion,                // both FK targets
			compositePKIndexesVersion,           // UNIQUE(tenant_id, id) backing the composite FKs
			refreshTokenRotationRecoveryVersion, // latest development migration before this branch
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentCompanionsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentCompanionsDown(ctx, db)
		},
	)
}

// studentCompanionsUp creates the store for "läuft mit" links between two
// children on a given weekday — the Laufgemeinschaft an OGS wants to group the
// Kindersuche by.
//
// The link is an undirected EDGE, not a group: there is deliberately no named
// group entity. A Laufgemeinschaft is the connected component of these edges on
// one weekday, resolved at read time. That keeps the whole feature living on
// the child (one field in the detail view) instead of introducing a second
// thing to manage, and it makes membership symmetric for free — entering Tom on
// Lina's card puts Lina on Tom's card.
//
// Undirectedness is enforced by storage, not by convention: the smaller student
// id always goes into student_low_id (CHECK student_low_id < student_high_id),
// so a pair can exist exactly once per weekday and no query has to look in two
// directions. Callers normalize via users.NewStudentCompanion.
//
// The weekday column is what keeps distinct constellations apart. Without it,
// "Lina walks with Tom" and "Lina walks with Mia" would collapse into one
// three-child component even when they happen on different days; with it, each
// weekday resolves its own components.
//
// Weekdays are 1..5 (Mon..Fri), matching schedule.student_pickup_schedules —
// OGS care does not run on weekends.
func studentCompanionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.209: Creating users.student_companions...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.student_companions (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_low_id  BIGINT NOT NULL,
			student_high_id BIGINT NOT NULL,
			weekday         SMALLINT NOT NULL CHECK (weekday BETWEEN 1 AND 5),
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			-- Composite FKs, not single-column ones: RLS only vets the row's own
			-- tenant_id, so a plain REFERENCES users.students(id) would still
			-- accept a child owned by another school on any write path that
			-- bypasses the service (inherited CRUD, import, repair). Referencing
			-- (tenant_id, id) makes the database itself reject a cross-tenant
			-- edge, so recursive grouping can never surface a foreign member.
			CONSTRAINT fk_student_companions_low
				FOREIGN KEY (tenant_id, student_low_id)
				REFERENCES users.students(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_student_companions_high
				FOREIGN KEY (tenant_id, student_high_id)
				REFERENCES users.students(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT student_companions_ordered CHECK (student_low_id < student_high_id),
			CONSTRAINT student_companions_unique_pair_weekday
				UNIQUE (tenant_id, student_low_id, student_high_id, weekday)
		);

		CREATE INDEX IF NOT EXISTS idx_student_companions_low
			ON users.student_companions (tenant_id, student_low_id, weekday);
		CREATE INDEX IF NOT EXISTS idx_student_companions_high
			ON users.student_companions (tenant_id, student_high_id, weekday);

		DROP TRIGGER IF EXISTS update_student_companions_updated_at ON users.student_companions;
		CREATE TRIGGER update_student_companions_updated_at
		BEFORE UPDATE ON users.student_companions
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE users.student_companions ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.student_companions FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_users_student_companions ON users.student_companions;
		CREATE POLICY tenant_isolation_users_student_companions ON users.student_companions
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.student_companions TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.student_companions_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating users.student_companions: %w", err)
	}

	return tx.Commit()
}

func studentCompanionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.209: Dropping users.student_companions...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_student_companions_updated_at ON users.student_companions;
		DROP TABLE IF EXISTS users.student_companions CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.student_companions: %w", err)
	}
	return tx.Commit()
}
