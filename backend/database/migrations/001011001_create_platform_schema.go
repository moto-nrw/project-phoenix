package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	createPlatformSchemaVersion     = "1.11.1"
	createPlatformSchemaDescription = "Create platform schema for operator dashboard with announcements and operator comments"
)

func init() {
	MigrationRegistry[createPlatformSchemaVersion] = &Migration{
		Version:     createPlatformSchemaVersion,
		Description: createPlatformSchemaDescription,
		DependsOn:   []string{"1.0.1"}, // Depends on auth.accounts (suggestions at 1.9.1 runs before by file order)
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createPlatformSchemaTables(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropPlatformSchemaTables(ctx, db)
		},
	)
}

func createPlatformSchemaTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.11.1: Creating platform schema for operator dashboard...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create platform schema
	_, err = tx.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS platform;`)
	if err != nil {
		return fmt.Errorf("error creating platform schema: %w", err)
	}

	// Create operators table (outside tenant boundary)
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.operators (
			id              BIGSERIAL PRIMARY KEY,
			email           VARCHAR(255) NOT NULL UNIQUE,
			display_name    VARCHAR(100) NOT NULL,
			password_hash   TEXT NOT NULL,
			active          BOOLEAN NOT NULL DEFAULT true,
			last_login      TIMESTAMPTZ,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating platform.operators table: %w", err)
	}

	// Create announcements table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.announcements (
			id                  BIGSERIAL PRIMARY KEY,
			title               VARCHAR(200) NOT NULL,
			content             TEXT NOT NULL,
			type                VARCHAR(20) NOT NULL DEFAULT 'announcement'
			                    CHECK (type IN ('announcement', 'release', 'maintenance')),
			severity            VARCHAR(20) NOT NULL DEFAULT 'info'
			                    CHECK (severity IN ('info', 'warning', 'critical')),
			version             VARCHAR(50),
			active              BOOLEAN NOT NULL DEFAULT true,
			published_at        TIMESTAMPTZ,
			expires_at          TIMESTAMPTZ,
			target_school_ids   BIGINT[],
			created_by          BIGINT NOT NULL REFERENCES platform.operators(id),
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating platform.announcements table: %w", err)
	}

	// Create announcement_views table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.announcement_views (
			user_id         BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			announcement_id BIGINT NOT NULL REFERENCES platform.announcements(id) ON DELETE CASCADE,
			seen_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			dismissed       BOOLEAN NOT NULL DEFAULT false,
			PRIMARY KEY (user_id, announcement_id)
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating platform.announcement_views table: %w", err)
	}

	// Create operator_comments table (extends suggestions)
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS suggestions.operator_comments (
			id              BIGSERIAL PRIMARY KEY,
			post_id         BIGINT NOT NULL REFERENCES suggestions.posts(id) ON DELETE CASCADE,
			operator_id     BIGINT NOT NULL REFERENCES platform.operators(id),
			content         TEXT NOT NULL,
			is_internal     BOOLEAN NOT NULL DEFAULT false,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating suggestions.operator_comments table: %w", err)
	}

	// Create operator_audit_log table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.operator_audit_log (
			id              BIGSERIAL PRIMARY KEY,
			operator_id     BIGINT NOT NULL REFERENCES platform.operators(id),
			action          VARCHAR(100) NOT NULL,
			resource_type   VARCHAR(100) NOT NULL,
			resource_id     BIGINT,
			changes         JSONB,
			request_ip      INET,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating platform.operator_audit_log table: %w", err)
	}

	// Create indexes
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_announcements_active ON platform.announcements(active, published_at, expires_at);
		CREATE INDEX IF NOT EXISTS idx_announcement_views_user ON platform.announcement_views(user_id);
		CREATE INDEX IF NOT EXISTS idx_operator_comments_post ON suggestions.operator_comments(post_id);
		CREATE INDEX IF NOT EXISTS idx_operator_audit_log_operator ON platform.operator_audit_log(operator_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_operators_email ON platform.operators(email);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for platform tables: %w", err)
	}

	// Create triggers for updated_at
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_platform_operators_updated_at ON platform.operators;
		CREATE TRIGGER update_platform_operators_updated_at
		BEFORE UPDATE ON platform.operators
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_platform_announcements_updated_at ON platform.announcements;
		CREATE TRIGGER update_platform_announcements_updated_at
		BEFORE UPDATE ON platform.announcements
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_suggestions_operator_comments_updated_at ON suggestions.operator_comments;
		CREATE TRIGGER update_suggestions_operator_comments_updated_at
		BEFORE UPDATE ON suggestions.operator_comments
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers for platform tables: %w", err)
	}

	fmt.Println("Migration 1.11.1: Successfully created platform schema and tables")
	return tx.Commit()
}

func dropPlatformSchemaTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.11.1: Removing platform schema...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop triggers
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_platform_operators_updated_at ON platform.operators;
		DROP TRIGGER IF EXISTS update_platform_announcements_updated_at ON platform.announcements;
		DROP TRIGGER IF EXISTS update_suggestions_operator_comments_updated_at ON suggestions.operator_comments;
	`)
	if err != nil {
		return fmt.Errorf("error dropping triggers for platform tables: %w", err)
	}

	// Drop operator comments table (in suggestions schema)
	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS suggestions.operator_comments CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping suggestions.operator_comments table: %w", err)
	}

	// Drop platform tables
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS platform.operator_audit_log CASCADE;
		DROP TABLE IF EXISTS platform.announcement_views CASCADE;
		DROP TABLE IF EXISTS platform.announcements CASCADE;
		DROP TABLE IF EXISTS platform.operators CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping platform tables: %w", err)
	}

	// Drop schema
	_, err = tx.ExecContext(ctx, `DROP SCHEMA IF EXISTS platform CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping platform schema: %w", err)
	}

	fmt.Println("Migration 1.11.1: Successfully removed platform schema")
	return tx.Commit()
}
