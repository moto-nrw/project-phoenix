package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	unifiedSuggestionCommentsVersion     = "1.12.1"
	unifiedSuggestionCommentsDescription = "Replace operator_comments with unified suggestions.comments table"
)

func init() {
	MigrationRegistry[unifiedSuggestionCommentsVersion] = &Migration{
		Version:     unifiedSuggestionCommentsVersion,
		Description: unifiedSuggestionCommentsDescription,
		DependsOn:   []string{"1.11.1"}, // Depends on platform schema (operator_comments)
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createUnifiedComments(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackUnifiedComments(ctx, db)
		},
	)
}

func createUnifiedComments(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.12.1: Creating unified suggestions.comments table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create unified comments table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS suggestions.comments (
			id            BIGSERIAL PRIMARY KEY,
			post_id       BIGINT NOT NULL REFERENCES suggestions.posts(id) ON DELETE CASCADE,
			author_id     BIGINT NOT NULL,
			author_type   VARCHAR(20) NOT NULL CHECK (author_type IN ('operator', 'user')),
			content       TEXT NOT NULL CHECK (length(content) <= 5000),
			is_internal   BOOLEAN NOT NULL DEFAULT false,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at    TIMESTAMPTZ
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating suggestions.comments table: %w", err)
	}

	// Create indexes
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_suggestions_comments_post ON suggestions.comments(post_id);
		CREATE INDEX IF NOT EXISTS idx_suggestions_comments_author ON suggestions.comments(author_id, author_type);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for suggestions.comments: %w", err)
	}

	// Create updated_at trigger
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_suggestions_comments_updated_at ON suggestions.comments;
		CREATE TRIGGER update_suggestions_comments_updated_at
		BEFORE UPDATE ON suggestions.comments
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for suggestions.comments: %w", err)
	}

	// Migrate existing operator_comments data
	_, err = tx.ExecContext(ctx, `
		INSERT INTO suggestions.comments (id, post_id, author_id, author_type, content, is_internal, created_at, updated_at)
		SELECT id, post_id, operator_id, 'operator', content, is_internal, created_at, updated_at
		FROM suggestions.operator_comments;
	`)
	if err != nil {
		return fmt.Errorf("error migrating operator_comments data: %w", err)
	}

	// Fix sequence after data migration
	_, err = tx.ExecContext(ctx, `
		SELECT setval('suggestions.comments_id_seq', COALESCE((SELECT MAX(id) FROM suggestions.comments), 0) + 1, false);
	`)
	if err != nil {
		return fmt.Errorf("error fixing comments sequence: %w", err)
	}

	// Drop old trigger
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_suggestions_operator_comments_updated_at ON suggestions.operator_comments;
	`)
	if err != nil {
		return fmt.Errorf("error dropping old trigger: %w", err)
	}

	// Drop old table
	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS suggestions.operator_comments CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping suggestions.operator_comments table: %w", err)
	}

	fmt.Println("Migration 1.12.1: Successfully created unified suggestions.comments table")
	return tx.Commit()
}

func rollbackUnifiedComments(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.12.1: Restoring operator_comments table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Recreate operator_comments table
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
		return fmt.Errorf("error recreating suggestions.operator_comments table: %w", err)
	}

	// Copy operator comments back
	_, err = tx.ExecContext(ctx, `
		INSERT INTO suggestions.operator_comments (id, post_id, operator_id, content, is_internal, created_at, updated_at)
		SELECT id, post_id, author_id, content, is_internal, created_at, updated_at
		FROM suggestions.comments
		WHERE author_type = 'operator';
	`)
	if err != nil {
		return fmt.Errorf("error copying operator comments back: %w", err)
	}

	// Fix sequence
	_, err = tx.ExecContext(ctx, `
		SELECT setval('suggestions.operator_comments_id_seq', COALESCE((SELECT MAX(id) FROM suggestions.operator_comments), 0) + 1, false);
	`)
	if err != nil {
		return fmt.Errorf("error fixing operator_comments sequence: %w", err)
	}

	// Recreate index and trigger
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_operator_comments_post ON suggestions.operator_comments(post_id);

		DROP TRIGGER IF EXISTS update_suggestions_operator_comments_updated_at ON suggestions.operator_comments;
		CREATE TRIGGER update_suggestions_operator_comments_updated_at
		BEFORE UPDATE ON suggestions.operator_comments
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error recreating indexes/triggers for operator_comments: %w", err)
	}

	// Drop unified comments table
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_suggestions_comments_updated_at ON suggestions.comments;
		DROP TABLE IF EXISTS suggestions.comments CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping suggestions.comments table: %w", err)
	}

	fmt.Println("Migration 1.12.1: Successfully restored operator_comments table")
	return tx.Commit()
}
