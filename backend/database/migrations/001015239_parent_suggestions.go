package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentSuggestionsVersion     = "1.15.239"
	parentSuggestionsDescription = "Separate parent feedback from staff suggestions via author_type + actor-scoped RLS"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentSuggestionsVersion,
		Description: parentSuggestionsDescription,
		DependsOn:   []string{"1.15.1"}, // RLS policies this migration rewrites
	})

	Migrations.MustRegister(upParentSuggestions, downParentSuggestions)
}

// upParentSuggestions adds the author_type discriminator to suggestion posts and
// rewrites the tenant-isolation policies so the school can never read parent
// feedback and vice versa.
//
// The policy compares author_type against app.current_actor_type, which
// tenant.WithTenantTx sets to 'staff' and tenant.WithParentTx to 'parent'. The
// COALESCE default keeps every pre-existing caller on the staff board, so no
// current code path changes behaviour. phoenix_admin (BYPASSRLS) still sees
// both boards — that is the operator, who is the intended recipient.
func upParentSuggestions(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE suggestions.posts
		ADD COLUMN IF NOT EXISTS author_type VARCHAR(20) NOT NULL DEFAULT 'staff'
			CHECK (author_type IN ('staff', 'parent'))
	`); err != nil {
		return fmt.Errorf("error adding author_type to suggestions.posts: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_suggestions_posts_tenant_author_type
			ON suggestions.posts(tenant_id, author_type)
	`); err != nil {
		return fmt.Errorf("error creating author_type index: %w", err)
	}

	// Parent comments are a third author type next to the existing operator/user.
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE suggestions.comments
		DROP CONSTRAINT IF EXISTS comments_author_type_check
	`); err != nil {
		return fmt.Errorf("error dropping comments author_type check: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE suggestions.comments
		ADD CONSTRAINT comments_author_type_check
			CHECK (author_type IN ('operator', 'user', 'parent'))
	`); err != nil {
		return fmt.Errorf("error adding comments author_type check: %w", err)
	}

	// Posts: tenant isolation AND actor isolation.
	if _, err := db.ExecContext(ctx, `
		DROP POLICY IF EXISTS tenant_isolation_suggestions_posts ON suggestions.posts
	`); err != nil {
		return fmt.Errorf("error dropping posts policy: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE POLICY tenant_isolation_suggestions_posts ON suggestions.posts FOR ALL
		USING (
			tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND author_type = COALESCE(NULLIF(current_setting('app.current_actor_type', true), ''), 'staff')
		)
		WITH CHECK (
			tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND author_type = COALESCE(NULLIF(current_setting('app.current_actor_type', true), ''), 'staff')
		)
	`); err != nil {
		return fmt.Errorf("error creating posts policy: %w", err)
	}

	// Comments carry free text, so they get the same guarantee transitively:
	// the EXISTS subquery is itself subject to the posts policy above, which
	// means a staff transaction cannot see a comment on a parent post.
	if _, err := db.ExecContext(ctx, `
		DROP POLICY IF EXISTS tenant_isolation_suggestions_comments ON suggestions.comments
	`); err != nil {
		return fmt.Errorf("error dropping comments policy: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE POLICY tenant_isolation_suggestions_comments ON suggestions.comments FOR ALL
		USING (
			tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND EXISTS (SELECT 1 FROM suggestions.posts p WHERE p.id = post_id)
		)
		WITH CHECK (
			tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND EXISTS (SELECT 1 FROM suggestions.posts p WHERE p.id = post_id)
		)
	`); err != nil {
		return fmt.Errorf("error creating comments policy: %w", err)
	}

	return nil
}

// downParentSuggestions restores the plain tenant-only policies and drops the
// discriminator. Parent posts are deleted first: without author_type they would
// become indistinguishable from staff posts and leak to the school.
func downParentSuggestions(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		DELETE FROM suggestions.posts WHERE author_type = 'parent'
	`); err != nil {
		return fmt.Errorf("error deleting parent posts: %w", err)
	}

	for _, table := range []string{"suggestions.posts", "suggestions.comments"} {
		policy := "tenant_isolation_suggestions_" + table[len("suggestions."):]
		if _, err := db.ExecContext(ctx, fmt.Sprintf(
			`DROP POLICY IF EXISTS %s ON %s`, policy, table)); err != nil {
			return fmt.Errorf("error dropping policy on %s: %w", table, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`
			CREATE POLICY %s ON %s FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)`,
			policy, table)); err != nil {
			return fmt.Errorf("error restoring policy on %s: %w", table, err)
		}
	}

	if _, err := db.ExecContext(ctx, `
		ALTER TABLE suggestions.comments DROP CONSTRAINT IF EXISTS comments_author_type_check
	`); err != nil {
		return fmt.Errorf("error dropping comments author_type check: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE suggestions.comments
		ADD CONSTRAINT comments_author_type_check CHECK (author_type IN ('operator', 'user'))
	`); err != nil {
		return fmt.Errorf("error restoring comments author_type check: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		DROP INDEX IF EXISTS suggestions.idx_suggestions_posts_tenant_author_type
	`); err != nil {
		return fmt.Errorf("error dropping author_type index: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE suggestions.posts DROP COLUMN IF EXISTS author_type
	`); err != nil {
		return fmt.Errorf("error dropping author_type column: %w", err)
	}

	return nil
}
