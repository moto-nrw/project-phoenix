package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const dataReturnGrantsVersion = "1.15.910"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dataReturnGrantsVersion,
		Description: "Abruf des Rückgabepakets nach Art. 30 Abs. 2 Data Act: Berechtigungen und Abrufe (#3838, ADR 0201)",
		DependsOn:   []string{offboardingTraegerMembershipVersion},
	})
	Migrations.MustRegister(dataReturnGrantsUp, dataReturnGrantsDown)
}

// dataReturnGrantsUpSQL stores the retrieval of a released return package
// (ADR 0201).
//
//   - One grant is one token for one recipient: the customer or a target
//     provider the customer names. Only an HMAC of the token is stored
//     (token_lookup); the archive key is stored only wrapped under the token
//     (wrapped_key), so the database and the store together do not open an
//     archive without a live token. key_check lets a later grant prove the
//     archive key the operator supplies.
//   - The archive itself lies encrypted in the store under archive_name.
//     keep_until is the end of the retrieval period: the purge removes the
//     archive then (or once every grant is revoked), clears the wrapped keys
//     and sets purged_at. The rows stay as evidence of the return and go
//     three years after keep_until.
//   - package and subject_id name the school or Träger without a foreign key,
//     like platform.school_erasures: the evidence outlives an erasure.
//   - A download row is one retrieval: started, then completed or failed.
//
// The HTTP role reads grants by their lookup and counts downloads; it never
// writes a grant otherwise. The CLI writes with the maintenance connection.
const dataReturnGrantsUpSQL = `
CREATE TABLE IF NOT EXISTS platform.data_return_grants (
	id BIGSERIAL PRIMARY KEY,
	package TEXT NOT NULL CHECK (package IN ('school', 'organization')),
	subject_id BIGINT NOT NULL CHECK (subject_id > 0),
	archive_name TEXT NOT NULL CHECK (archive_name ~ '^[0-9a-f]{32}$'),
	archive_bytes BIGINT NOT NULL CHECK (archive_bytes > 0),
	archive_sha256 TEXT NOT NULL CHECK (archive_sha256 ~ '^[0-9a-f]{64}$'),
	manifest_sha256 TEXT NOT NULL CHECK (manifest_sha256 ~ '^[0-9a-f]{64}$'),
	format_version INTEGER NOT NULL CHECK (format_version > 0),
	key_check TEXT NOT NULL CHECK (key_check ~ '^[0-9a-f]{64}$'),
	recipient TEXT NOT NULL CHECK (recipient IN ('customer', 'target_provider')),
	recipient_name TEXT NOT NULL CHECK (length(recipient_name) BETWEEN 1 AND 200),
	token_lookup TEXT NOT NULL CHECK (token_lookup ~ '^[0-9a-f]{64}$'),
	wrapped_key BYTEA,
	issued_by BIGINT NOT NULL CHECK (issued_by > 0),
	issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	expires_at TIMESTAMPTZ NOT NULL,
	keep_until TIMESTAMPTZ NOT NULL,
	revoked_at TIMESTAMPTZ,
	purged_at TIMESTAMPTZ,
	max_downloads INTEGER NOT NULL CHECK (max_downloads BETWEEN 1 AND 20),
	downloads INTEGER NOT NULL DEFAULT 0 CHECK (downloads >= 0 AND downloads <= max_downloads),
	CONSTRAINT uq_data_return_grants_token UNIQUE (token_lookup),
	CONSTRAINT chk_data_return_grants_expiry CHECK (expires_at > issued_at AND expires_at <= keep_until),
	CONSTRAINT chk_data_return_grants_key CHECK (wrapped_key IS NOT NULL OR revoked_at IS NOT NULL OR purged_at IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_data_return_grants_archive ON platform.data_return_grants (archive_name);

CREATE TABLE IF NOT EXISTS platform.data_return_downloads (
	id BIGSERIAL PRIMARY KEY,
	grant_id BIGINT NOT NULL REFERENCES platform.data_return_grants(id) ON DELETE CASCADE,
	started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	finished_at TIMESTAMPTZ,
	outcome TEXT NOT NULL DEFAULT 'started' CHECK (outcome IN ('started', 'completed', 'failed')),
	bytes BIGINT NOT NULL DEFAULT 0 CHECK (bytes >= 0)
);
CREATE INDEX IF NOT EXISTS idx_data_return_downloads_grant ON platform.data_return_downloads (grant_id, started_at);

GRANT SELECT, UPDATE (downloads) ON platform.data_return_grants TO phoenix_auth;
GRANT SELECT, INSERT, UPDATE (finished_at, outcome, bytes) ON platform.data_return_downloads TO phoenix_auth;
GRANT USAGE ON SEQUENCE platform.data_return_downloads_id_seq TO phoenix_auth;
GRANT ALL ON platform.data_return_grants, platform.data_return_downloads TO phoenix_admin;
GRANT ALL ON SEQUENCE platform.data_return_grants_id_seq, platform.data_return_downloads_id_seq TO phoenix_admin;

COMMENT ON TABLE platform.data_return_grants IS
	'Retrieval token of a released return package (Art. 30 Abs. 2 Data Act, ADR 0201): HMAC of the token, archive key wrapped under it, expiry and download budget.';
COMMENT ON TABLE platform.data_return_downloads IS
	'One retrieval of a released return package with its outcome (ADR 0201).';
`

const dataReturnGrantsDownSQL = `
DROP TABLE IF EXISTS platform.data_return_downloads;
DROP TABLE IF EXISTS platform.data_return_grants;
`

func dataReturnGrantsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, dataReturnGrantsUpSQL); err != nil {
			return fmt.Errorf("create data return grants: %w", err)
		}
		return nil
	})
}

func dataReturnGrantsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, dataReturnGrantsDownSQL); err != nil {
			return fmt.Errorf("drop data return grants: %w", err)
		}
		return nil
	})
}
