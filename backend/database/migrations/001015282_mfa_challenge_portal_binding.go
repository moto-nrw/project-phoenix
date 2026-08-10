package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mfaChallengePortalBindingVersion     = "1.15.282"
	mfaChallengePortalBindingDescription = "Record portal scope and school on auth.mfa_email_challenges so an email code can only be redeemed by the portal that issued it"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mfaChallengePortalBindingVersion,
		Description: mfaChallengePortalBindingDescription,
		DependsOn: []string{
			authMFAEmailChallengesVersion, // 1.15.81
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return mfaChallengePortalBindingUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return mfaChallengePortalBindingDown(ctx, db)
		},
	)
}

func mfaChallengePortalBindingUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.282: Binding auth.mfa_email_challenges rows to their portal scope and school...")

	// Until now a challenge row recorded only WHO it was issued to, never
	// WHICH portal asked for it. The verify path compensates by naming the
	// exact row inside the challenge JWT, but the enrollment-confirm path has
	// no challenge JWT to name it with: it resolves "the account's newest
	// active code" instead. With the school portal (#2207) that is no longer
	// unambiguous — the same Lehrkraft can have a tenant-portal enrollment
	// challenge and a school-portal login challenge in flight at the same
	// time, and the tenant confirm endpoint would happily consume the school
	// code and mint a tenant session from it.
	//
	// Recording scope + school on the row makes the account-wide lookup
	// portal-specific, so each surface can only ever see its own codes.
	// The default backfills in-flight rows as tenant-scope, which is what
	// every pre-existing row is: the school scope ships with this change.
	// tenant_id stays nullable — the operator portal keeps its own separate
	// challenge table, and older rows have no school recorded.
	_, err := db.NewRaw(`
		ALTER TABLE auth.mfa_email_challenges
			ADD COLUMN IF NOT EXISTS scope     TEXT NOT NULL DEFAULT 'tenant',
			ADD COLUMN IF NOT EXISTS tenant_id BIGINT;

		CREATE INDEX IF NOT EXISTS idx_mfa_email_challenges_account_scope_active
			ON auth.mfa_email_challenges (account_id, scope, tenant_id, expires_at DESC)
			WHERE consumed_at IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding portal binding columns to auth.mfa_email_challenges: %w", err)
	}

	// The DEFAULT answers the scope half for existing rows; tenant_id lands
	// NULL on every one of them. That matters for the codes that are in flight
	// while this migration runs: the enrollment-confirm and passkey-
	// registration paths have no challenge id to look a row up by, so they ask
	// for (account, scope, school) — and a production caller always knows its
	// school. A NULL there stops matching, and the code the user was just
	// emailed reads back as invalid with nothing explaining why.
	//
	// Fill the school in wherever it is unambiguous: an account with exactly
	// one active mapping has exactly one school its tenant-portal code can
	// have been issued for. Only unconsumed, unexpired rows are touched —
	// history stays as it was recorded.
	_, err = db.NewRaw(`
		UPDATE auth.mfa_email_challenges AS c
		SET tenant_id = m.tenant_id
		FROM (
			SELECT account_id, MIN(tenant_id) AS tenant_id
			FROM auth.account_tenants
			WHERE status = 'active'
			GROUP BY account_id
			HAVING COUNT(*) = 1
		) AS m
		WHERE m.account_id = c.account_id
		  AND c.tenant_id IS NULL
		  AND c.consumed_at IS NULL
		  AND c.expires_at > NOW();
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed backfilling tenant_id on in-flight auth.mfa_email_challenges rows: %w", err)
	}

	// What is left is genuinely ambiguous — an account mapped to several
	// active schools, where nothing on the row says which one asked. Guessing
	// would make the code redeemable at the wrong school, so retire those rows
	// instead of leaving them active-but-unmatchable. The user requests a new
	// code; the alternative is a row that looks alive in the table and answers
	// no lookup.
	_, err = db.NewRaw(`
		UPDATE auth.mfa_email_challenges
		SET expires_at = NOW()
		WHERE tenant_id IS NULL
		  AND consumed_at IS NULL
		  AND expires_at > NOW();
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed retiring ambiguous in-flight auth.mfa_email_challenges rows: %w", err)
	}

	return nil
}

func mfaChallengePortalBindingDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.282: Dropping portal binding from auth.mfa_email_challenges...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS auth.idx_mfa_email_challenges_account_scope_active;

		ALTER TABLE auth.mfa_email_challenges
			DROP COLUMN IF EXISTS scope,
			DROP COLUMN IF EXISTS tenant_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping portal binding columns from auth.mfa_email_challenges: %w", err)
	}

	return nil
}
