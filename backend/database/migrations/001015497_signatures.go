package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const signaturesVersion = "1.15.497"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     signaturesVersion,
		Description: "Unterschrift: einfache elektronische Signatur mit gezeichneter Unterschrift als gemeinsamer Baustein, zuerst für Einverständnisse (#4785)",
		DependsOn: []string{
			enrollmentRequestUploadsVersion, // the last migration before this one
			parentDeclarationsVersion,       // the declaration columns and submissions this migration extends
			schoolErasureVersion,            // the erasure plan this migration extends
			demoVisitorErasureVersion,       // the demo visitor erasure this migration extends
		},
	})
	Migrations.MustRegister(signaturesUp, signaturesDown)
}

// signaturesUpSQL adds the shared signature store (#4785).
//
//   - audit.signatures holds one signature per row: the PNG, its SHA-256, the
//     SHA-256 of exactly what was signed, the signer (account nullable: a
//     public enrollment has none, and account deletion clears it while the
//     frozen name stays), the time and a record hash. The request roles may
//     only read and insert. Every row belongs to a child or an enrollment
//     request and goes with it; the school deletion removes the rest.
//   - An Einverständnis can require a signature for agreeing
//     (declaration_requires_signature). A submission made with one names it
//     in signature_id and uses the method simple_electronic_signature. The
//     reference clears instead of restricting: a signature only disappears
//     with its child or school, which remove the submission in the same
//     statement, so the school erasure may delete the signatures with the
//     other audit rows.
//   - The demo visitor erasure (1.15.456) deletes the visitor's signatures
//     with their declarations: both carry the visitor's name.
const signaturesUpSQL = `
		CREATE TABLE IF NOT EXISTS audit.signatures (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			purpose TEXT NOT NULL CHECK (purpose IN ('declaration','enrollment','pickup_permission')),
			student_id BIGINT,
			enrollment_request_id BIGINT REFERENCES enrollment.requests(id) ON DELETE CASCADE,
			signer_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			signer_name TEXT NOT NULL CHECK (btrim(signer_name) <> ''),
			method TEXT NOT NULL CHECK (method IN ('drawn','typed')),
			typed_name TEXT,
			image_png BYTEA NOT NULL CHECK (octet_length(image_png) BETWEEN 1 AND 204800),
			image_sha256 TEXT NOT NULL CHECK (image_sha256 ~ '^[0-9a-f]{64}$'),
			document_sha256 TEXT NOT NULL CHECK (document_sha256 ~ '^[0-9a-f]{64}$'),
			record_hash TEXT NOT NULL CHECK (record_hash ~ '^[0-9a-f]{64}$'),
			signed_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_signatures_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT fk_signatures_student FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT chk_signatures_subject CHECK ((student_id IS NOT NULL) <> (enrollment_request_id IS NOT NULL)),
			CONSTRAINT chk_signatures_typed_name CHECK ((method = 'typed') = (typed_name IS NOT NULL))
		);
		CREATE INDEX IF NOT EXISTS idx_signatures_student
			ON audit.signatures (tenant_id, student_id) WHERE student_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_signatures_enrollment_request
			ON audit.signatures (enrollment_request_id) WHERE enrollment_request_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_signatures_signer_account
			ON audit.signatures (signer_account_id) WHERE signer_account_id IS NOT NULL;

		GRANT SELECT, INSERT ON audit.signatures TO phoenix_tenant, phoenix_admin;
		REVOKE UPDATE, DELETE, TRUNCATE ON audit.signatures FROM phoenix_tenant, phoenix_admin;
		GRANT USAGE ON SEQUENCE audit.signatures_id_seq TO phoenix_tenant, phoenix_admin;

		ALTER TABLE users.parent_announcements
			ADD COLUMN IF NOT EXISTS declaration_requires_signature BOOLEAN NOT NULL DEFAULT false;
		ALTER TABLE users.parent_announcements
			ADD CONSTRAINT chk_parent_announcements_declaration_signature
			CHECK (NOT declaration_requires_signature OR delivery_mode = 'declaration');

		ALTER TABLE users.parent_announcement_declaration_submissions
			ADD COLUMN IF NOT EXISTS signature_id BIGINT;
		ALTER TABLE users.parent_announcement_declaration_submissions
			ADD CONSTRAINT fk_parent_declaration_submissions_signature FOREIGN KEY (tenant_id, signature_id)
				REFERENCES audit.signatures(tenant_id, id) ON DELETE SET NULL (signature_id);
		ALTER TABLE users.parent_announcement_declaration_submissions
			DROP CONSTRAINT IF EXISTS parent_announcement_declaration_submissions_method_check;
		ALTER TABLE users.parent_announcement_declaration_submissions
			ADD CONSTRAINT chk_parent_declaration_submissions_method CHECK (
				method IN ('simple_electronic','simple_electronic_signature')
				AND (signature_id IS NULL OR method = 'simple_electronic_signature')
			);
		CREATE INDEX IF NOT EXISTS idx_parent_declaration_submissions_signature
			ON users.parent_announcement_declaration_submissions (tenant_id, signature_id)
			WHERE signature_id IS NOT NULL;
`

const signaturesDownSQL = `
		DROP INDEX IF EXISTS users.idx_parent_declaration_submissions_signature;
		ALTER TABLE users.parent_announcement_declaration_submissions
			DROP CONSTRAINT IF EXISTS chk_parent_declaration_submissions_method;
		ALTER TABLE users.parent_announcement_declaration_submissions
			DROP CONSTRAINT IF EXISTS fk_parent_declaration_submissions_signature;
		ALTER TABLE users.parent_announcement_declaration_submissions
			DROP COLUMN IF EXISTS signature_id;
		ALTER TABLE users.parent_announcement_declaration_submissions
			ADD CONSTRAINT parent_announcement_declaration_submissions_method_check
			CHECK (method IN ('simple_electronic'));
		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_declaration_signature;
		ALTER TABLE users.parent_announcements
			DROP COLUMN IF EXISTS declaration_requires_signature;
		DROP TABLE IF EXISTS audit.signatures;
`

// demoVisitorErasureFunction cuts the definition of
// platform.erase_closed_demo_visitors out of 1.15.456. withSignatures adds
// the visitor's signatures after their declarations: a signature carries the
// signer's name like the declaration it belongs to.
func demoVisitorErasureFunction(withSignatures bool) (string, error) {
	const start = "CREATE OR REPLACE FUNCTION platform.erase_closed_demo_visitors()"
	const end = "CREATE OR REPLACE FUNCTION platform.retire_expired_demo_schools"
	from, to := strings.Index(demoVisitorErasureUpSQL, start), strings.Index(demoVisitorErasureUpSQL, end)
	if from < 0 || to < from {
		return "", errors.New("demo visitor erasure function not found in 1.15.456")
	}
	function := demoVisitorErasureUpSQL[from:to]
	if !withSignatures {
		return function, nil
	}
	const anchor = `DELETE FROM users.parent_announcement_declaration_submissions
					WHERE tenant_id = ANY (hidden_schools)
						AND account_id = ANY (visitor.visitor_parent_account_ids);`
	if strings.Count(function, anchor) != 1 {
		return "", errors.New("demo visitor erasure: declaration delete not found exactly once")
	}
	return strings.Replace(function, anchor, anchor+`
					DELETE FROM audit.signatures
					WHERE tenant_id = ANY (hidden_schools)
						AND signer_account_id = ANY (visitor.visitor_parent_account_ids);`, 1), nil
}

func installDemoVisitorErasure(ctx context.Context, tx bun.Tx, withSignatures bool) error {
	function, err := demoVisitorErasureFunction(withSignatures)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, function); err != nil {
		return fmt.Errorf("install demo visitor erasure: %w", err)
	}
	return nil
}

func signaturesUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSignatures(ctx, tx)
	if _, err := tx.ExecContext(ctx, signaturesUpSQL); err != nil {
		return fmt.Errorf("create signatures: %w", err)
	}
	if err := provisionTenantRLS(ctx, tx, "audit.signatures"); err != nil {
		return err
	}
	if err := installSchoolErasurePlan(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure plan: %w", err)
	}
	if err := installSchoolErasureDelete(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure delete: %w", err)
	}
	if err := installSchoolErasurePreflight(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure preflight: %w", err)
	}
	if err := installSchoolErasurePreview(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure preview: %w", err)
	}
	if err := installSchoolErasureResidual(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure residual check: %w", err)
	}
	if err := installDemoVisitorErasure(ctx, tx, true); err != nil {
		return err
	}
	return tx.Commit()
}

// signaturesDown refuses while signed declarations exist: dropping the
// signatures would silently turn signed proof into unsigned proof.
func signaturesDown(ctx context.Context, db *bun.DB) error {
	var signed int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.signatures`).Scan(ctx, &signed); err != nil {
		return fmt.Errorf("count signatures: %w", err)
	}
	if signed > 0 {
		return fmt.Errorf("cannot drop audit.signatures while %d signatures exist", signed)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSignatures(ctx, tx)
	if err := installDemoVisitorErasure(ctx, tx, false); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, signaturesDownSQL); err != nil {
		return fmt.Errorf("revert signatures: %w", err)
	}
	return tx.Commit()
}

func rollbackSignatures(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
