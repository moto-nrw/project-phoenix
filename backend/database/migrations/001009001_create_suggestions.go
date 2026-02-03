package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	createSuggestionsVersion     = "1.9.1"
	createSuggestionsDescription = "Create suggestions schema with posts and votes tables"
)

func init() {
	MigrationRegistry[createSuggestionsVersion] = &Migration{
		Version:     createSuggestionsVersion,
		Description: createSuggestionsDescription,
		DependsOn:   []string{"1.0.1"}, // Depends on auth.accounts
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createSuggestionsTables(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropSuggestionsTables(ctx, db)
		},
	)
}

func createSuggestionsTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.9.1: Creating suggestions schema and tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create schema
	_, err = tx.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS suggestions;`)
	if err != nil {
		return fmt.Errorf("error creating suggestions schema: %w", err)
	}

	// Create posts table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS suggestions.posts (
			id            BIGSERIAL PRIMARY KEY,
			title         VARCHAR(200) NOT NULL,
			description   TEXT NOT NULL,
			author_id     BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			status        VARCHAR(20) NOT NULL DEFAULT 'open'
			              CHECK (status IN ('open', 'planned', 'done', 'rejected')),
			score         INT NOT NULL DEFAULT 0,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating suggestions.posts table: %w", err)
	}

	// Create votes table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS suggestions.votes (
			id            BIGSERIAL PRIMARY KEY,
			post_id       BIGINT NOT NULL REFERENCES suggestions.posts(id) ON DELETE CASCADE,
			voter_id      BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			direction     VARCHAR(4) NOT NULL CHECK (direction IN ('up', 'down')),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(post_id, voter_id)
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating suggestions.votes table: %w", err)
	}

	// Create indexes
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_suggestions_posts_score ON suggestions.posts(score DESC);
		CREATE INDEX IF NOT EXISTS idx_suggestions_posts_status ON suggestions.posts(status);
		CREATE INDEX IF NOT EXISTS idx_suggestions_posts_author ON suggestions.posts(author_id);
		CREATE INDEX IF NOT EXISTS idx_suggestions_votes_post ON suggestions.votes(post_id);
		CREATE INDEX IF NOT EXISTS idx_suggestions_votes_voter ON suggestions.votes(voter_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for suggestions tables: %w", err)
	}

	// Create triggers for updated_at
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_suggestions_posts_updated_at ON suggestions.posts;
		CREATE TRIGGER update_suggestions_posts_updated_at
		BEFORE UPDATE ON suggestions.posts
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_suggestions_votes_updated_at ON suggestions.votes;
		CREATE TRIGGER update_suggestions_votes_updated_at
		BEFORE UPDATE ON suggestions.votes
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers for suggestions tables: %w", err)
	}

	fmt.Println("Migration 1.9.1: Successfully created suggestions schema and tables")
	return tx.Commit()
}

func dropSuggestionsTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.9.1: Removing suggestions schema...")

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
		DROP TRIGGER IF EXISTS update_suggestions_posts_updated_at ON suggestions.posts;
		DROP TRIGGER IF EXISTS update_suggestions_votes_updated_at ON suggestions.votes;
	`)
	if err != nil {
		return fmt.Errorf("error dropping triggers for suggestions tables: %w", err)
	}

	// Drop tables
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS suggestions.votes CASCADE;
		DROP TABLE IF EXISTS suggestions.posts CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping suggestions tables: %w", err)
	}

	// Drop schema
	_, err = tx.ExecContext(ctx, `DROP SCHEMA IF EXISTS suggestions CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping suggestions schema: %w", err)
	}

	fmt.Println("Migration 1.9.1: Successfully removed suggestions schema")
	return tx.Commit()
}
