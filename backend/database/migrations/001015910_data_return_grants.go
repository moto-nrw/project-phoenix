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
		Description: "Abruf des Rückgabepakets nach Art. 30 Abs. 2 Data Act: Berechtigungen, Abrufe und die Rolle der Freigabe (#3838, ADR 0203)",
		DependsOn:   []string{offboardingTraegerMembershipVersion, holidayCriteriaAllocationVersion},
	})
	Migrations.MustRegister(dataReturnGrantsUp, dataReturnGrantsDown)
}

// dataReturnGrantsUpSQL stores the retrieval of a released return package
// (ADR 0203).
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
//     like platform.school_erasures: the evidence outlives an erasure, and an
//     erasure refuses while an archive of its subject is not purged.
//   - A download row is one retrieval: started, then completed or failed.
//
// Two roles touch the tables. The HTTP role (phoenix_auth) reads a grant by
// its lookup, counts a download and writes download rows. The release CLI
// runs as the NOLOGIN group phoenix_data_return, like phoenix_school_erasure:
// it holds no table right, only EXECUTE on the functions below. Each
// function checks the active operator, writes and appends the operator audit
// in one transaction, and serializes per archive with an advisory lock, so a
// purge and a grant of the same archive cannot interleave.
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
	CONSTRAINT chk_data_return_grants_expiry CHECK (expires_at <= keep_until),
	CONSTRAINT chk_data_return_grants_key CHECK (wrapped_key IS NOT NULL OR revoked_at IS NOT NULL OR purged_at IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_data_return_grants_archive ON platform.data_return_grants (archive_name);
CREATE INDEX IF NOT EXISTS idx_data_return_grants_subject ON platform.data_return_grants (package, subject_id) WHERE purged_at IS NULL;

CREATE TABLE IF NOT EXISTS platform.data_return_downloads (
	id BIGSERIAL PRIMARY KEY,
	grant_id BIGINT NOT NULL REFERENCES platform.data_return_grants(id) ON DELETE CASCADE,
	started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	finished_at TIMESTAMPTZ,
	outcome TEXT NOT NULL DEFAULT 'started' CHECK (outcome IN ('started', 'completed', 'failed')),
	bytes BIGINT NOT NULL DEFAULT 0 CHECK (bytes >= 0)
);
CREATE INDEX IF NOT EXISTS idx_data_return_downloads_grant ON platform.data_return_downloads (grant_id, started_at);

REVOKE ALL ON platform.data_return_grants, platform.data_return_downloads FROM PUBLIC;
GRANT SELECT, UPDATE (downloads) ON platform.data_return_grants TO phoenix_auth;
GRANT SELECT, INSERT, UPDATE (finished_at, outcome, bytes) ON platform.data_return_downloads TO phoenix_auth;
GRANT USAGE ON SEQUENCE platform.data_return_downloads_id_seq TO phoenix_auth;
GRANT ALL ON platform.data_return_grants, platform.data_return_downloads TO phoenix_admin;
GRANT ALL ON SEQUENCE platform.data_return_grants_id_seq, platform.data_return_downloads_id_seq TO phoenix_admin;

DO $$
BEGIN
	IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_data_return') THEN
		CREATE ROLE phoenix_data_return NOLOGIN NOINHERIT NOBYPASSRLS;
	END IF;
END $$;
GRANT USAGE ON SCHEMA platform TO phoenix_data_return;

-- The operator of every release command: existing and active.
CREATE OR REPLACE FUNCTION platform.data_return_require_operator(p_operator BIGINT)
RETURNS VOID LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
BEGIN
	PERFORM 1 FROM platform.operators WHERE id = p_operator AND active FOR KEY SHARE;
	IF NOT FOUND THEN
		RAISE EXCEPTION 'data return: operator unknown or inactive' USING ERRCODE = '42501';
	END IF;
END $function$;

CREATE OR REPLACE FUNCTION platform.data_return_audit(p_operator BIGINT, p_action TEXT, p_package TEXT, p_subject BIGINT, p_changes JSONB)
RETURNS VOID LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
	INSERT INTO platform.operator_audit_log (operator_id, action, resource_type, resource_id, changes)
	VALUES (p_operator, 'data_return_' || p_action, COALESCE(p_package, 'data_return'), p_subject, p_changes);
$function$;

CREATE OR REPLACE FUNCTION platform.data_return_lock_archive(p_archive TEXT)
RETURNS VOID LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
	SELECT pg_advisory_xact_lock(hashtextextended('data-return-archive:' || p_archive, 0));
$function$;

-- A release stores a new archive; a grant adds a token to an archive that
-- is neither purged nor past its retrieval period, under the same key.
CREATE OR REPLACE FUNCTION platform.data_return_issue(
	p_operator BIGINT, p_action TEXT, p_package TEXT, p_subject BIGINT, p_archive_name TEXT,
	p_archive_bytes BIGINT, p_archive_sha256 TEXT, p_manifest_sha256 TEXT, p_format_version INTEGER,
	p_key_check TEXT, p_recipient TEXT, p_recipient_name TEXT, p_token_lookup TEXT, p_wrapped_key BYTEA,
	p_expires_at TIMESTAMPTZ, p_keep_until TIMESTAMPTZ, p_max_downloads INTEGER)
RETURNS BIGINT LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
DECLARE
	v_id BIGINT;
	v_live BOOLEAN;
BEGIN
	PERFORM platform.data_return_require_operator(p_operator);
	PERFORM platform.data_return_lock_archive(p_archive_name);
	SELECT EXISTS (SELECT 1 FROM platform.data_return_grants WHERE archive_name = p_archive_name
		AND purged_at IS NULL AND keep_until > now() AND key_check = p_key_check
		AND package = p_package AND subject_id = p_subject) INTO v_live;
	IF p_action = 'release' THEN
		IF EXISTS (SELECT 1 FROM platform.data_return_grants WHERE archive_name = p_archive_name) THEN
			RAISE EXCEPTION 'data return: archive already released' USING ERRCODE = '23505';
		END IF;
	ELSIF p_action = 'grant' THEN
		IF NOT v_live THEN
			RAISE EXCEPTION 'data return: archive purged, expired or unknown' USING ERRCODE = '55000';
		END IF;
	ELSE
		RAISE EXCEPTION 'data return: unknown action' USING ERRCODE = '22023';
	END IF;
	INSERT INTO platform.data_return_grants (package, subject_id, archive_name, archive_bytes, archive_sha256,
		manifest_sha256, format_version, key_check, recipient, recipient_name, token_lookup, wrapped_key,
		issued_by, expires_at, keep_until, max_downloads)
	VALUES (p_package, p_subject, p_archive_name, p_archive_bytes, p_archive_sha256, p_manifest_sha256,
		p_format_version, p_key_check, p_recipient, p_recipient_name, p_token_lookup, p_wrapped_key,
		p_operator, p_expires_at, p_keep_until, p_max_downloads)
	RETURNING id INTO v_id;
	PERFORM platform.data_return_audit(p_operator, p_action, p_package, p_subject, jsonb_build_object(
		'grant_id', v_id, 'archive_name', p_archive_name, 'manifest_sha256', p_manifest_sha256,
		'recipient', p_recipient, 'expires_at', p_expires_at, 'keep_until', p_keep_until));
	RETURN v_id;
END $function$;

-- What a further grant needs of a live archive; no token, no wrapped key.
CREATE OR REPLACE FUNCTION platform.data_return_archive(p_operator BIGINT, p_archive_name TEXT)
RETURNS JSONB LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
BEGIN
	PERFORM platform.data_return_require_operator(p_operator);
	RETURN (SELECT jsonb_build_object('package', package, 'subject_id', subject_id, 'archive_name', archive_name,
		'archive_bytes', archive_bytes, 'archive_sha256', archive_sha256, 'manifest_sha256', manifest_sha256,
		'format_version', format_version, 'key_check', key_check, 'keep_until', keep_until)
		FROM platform.data_return_grants WHERE archive_name = p_archive_name AND purged_at IS NULL
		ORDER BY id LIMIT 1);
END $function$;

-- Ends one grant, every grant of an archive, or every unpurged grant (after
-- a database restore revived revoked tokens). One audit entry per grant.
CREATE OR REPLACE FUNCTION platform.data_return_revoke(p_operator BIGINT, p_grant BIGINT, p_archive_name TEXT, p_all BOOLEAN)
RETURNS INTEGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
DECLARE
	r RECORD;
	v_count INTEGER := 0;
BEGIN
	PERFORM platform.data_return_require_operator(p_operator);
	IF num_nonnulls(p_grant, p_archive_name, NULLIF(p_all, false)) <> 1 THEN
		RAISE EXCEPTION 'data return: name exactly one grant, one archive or all' USING ERRCODE = '22023';
	END IF;
	FOR r IN UPDATE platform.data_return_grants SET revoked_at = now(), wrapped_key = NULL
		WHERE revoked_at IS NULL AND purged_at IS NULL
			AND (p_all OR id = p_grant OR archive_name = p_archive_name)
		RETURNING id, package, subject_id, archive_name
	LOOP
		PERFORM platform.data_return_audit(p_operator, 'revoke', r.package, r.subject_id,
			jsonb_build_object('grant_id', r.id, 'archive_name', r.archive_name, 'all', COALESCE(p_all, false)));
		v_count := v_count + 1;
	END LOOP;
	RETURN v_count;
END $function$;

-- The archives whose retrieval period ended or whose grants are all revoked.
CREATE OR REPLACE FUNCTION platform.data_return_purgeable(p_operator BIGINT)
RETURNS SETOF TEXT LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
BEGIN
	PERFORM platform.data_return_require_operator(p_operator);
	RETURN QUERY SELECT archive_name FROM platform.data_return_grants WHERE purged_at IS NULL
		GROUP BY archive_name HAVING max(keep_until) <= now() OR bool_and(revoked_at IS NOT NULL)
		ORDER BY archive_name;
END $function$;

-- Marks one archive purged under its lock, after checking again that it may
-- go: a grant issued since the listing keeps it. The CLI removes the file
-- after the commit; the token stops working with this statement.
CREATE OR REPLACE FUNCTION platform.data_return_purge_archive(p_operator BIGINT, p_archive_name TEXT)
RETURNS BOOLEAN LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
DECLARE
	v_package TEXT;
	v_subject BIGINT;
	v_grants INTEGER;
BEGIN
	PERFORM platform.data_return_require_operator(p_operator);
	PERFORM platform.data_return_lock_archive(p_archive_name);
	SELECT min(package), min(subject_id), count(*) INTO v_package, v_subject, v_grants
		FROM platform.data_return_grants WHERE archive_name = p_archive_name AND purged_at IS NULL
		HAVING max(keep_until) <= now() OR bool_and(revoked_at IS NOT NULL);
	IF v_grants IS NULL OR v_grants = 0 THEN
		RETURN false;
	END IF;
	UPDATE platform.data_return_grants SET purged_at = now(), wrapped_key = NULL
		WHERE archive_name = p_archive_name AND purged_at IS NULL;
	PERFORM platform.data_return_audit(p_operator, 'purge', v_package, v_subject,
		jsonb_build_object('archive_name', p_archive_name, 'grants', v_grants));
	RETURN true;
END $function$;

-- The archives that may still lie in the store: any grant not purged.
CREATE OR REPLACE FUNCTION platform.data_return_live_archives(p_operator BIGINT)
RETURNS SETOF TEXT LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
BEGIN
	PERFORM platform.data_return_require_operator(p_operator);
	RETURN QUERY SELECT DISTINCT archive_name FROM platform.data_return_grants WHERE purged_at IS NULL;
END $function$;

-- Evidence of returns whose archive is gone and whose retrieval period
-- ended three years ago.
CREATE OR REPLACE FUNCTION platform.data_return_drop_evidence(p_operator BIGINT)
RETURNS INTEGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
DECLARE
	v_count INTEGER;
BEGIN
	PERFORM platform.data_return_require_operator(p_operator);
	DELETE FROM platform.data_return_grants WHERE purged_at IS NOT NULL AND keep_until < now() - interval '3 years';
	GET DIAGNOSTICS v_count = ROW_COUNT;
	IF v_count > 0 THEN
		PERFORM platform.data_return_audit(p_operator, 'evidence_expired', NULL, NULL, jsonb_build_object('grants', v_count));
	END IF;
	RETURN v_count;
END $function$;

-- Every grant without its token lookup or wrapped key.
CREATE OR REPLACE FUNCTION platform.data_return_list(p_operator BIGINT)
RETURNS JSONB LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
BEGIN
	PERFORM platform.data_return_require_operator(p_operator);
	RETURN COALESCE((SELECT jsonb_agg(jsonb_build_object('grant_id', id, 'package', package, 'subject_id', subject_id,
		'archive_name', archive_name, 'recipient', recipient, 'recipient_name', recipient_name, 'issued_at', issued_at,
		'expires_at', expires_at, 'keep_until', keep_until, 'revoked_at', revoked_at, 'purged_at', purged_at,
		'downloads', downloads, 'max_downloads', max_downloads) ORDER BY id) FROM platform.data_return_grants), '[]'::jsonb);
END $function$;

-- How many grants of a school or Träger still have an archive: an erasure
-- of that subject refuses until they are revoked and purged.
CREATE OR REPLACE FUNCTION platform.data_return_open_for(p_package TEXT, p_subject BIGINT)
RETURNS INTEGER LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, platform AS $function$
	SELECT count(*)::INTEGER FROM platform.data_return_grants
	WHERE package = p_package AND subject_id = p_subject AND purged_at IS NULL;
$function$;

REVOKE ALL ON FUNCTION platform.data_return_require_operator(BIGINT), platform.data_return_audit(BIGINT, TEXT, TEXT, BIGINT, JSONB),
	platform.data_return_lock_archive(TEXT),
	platform.data_return_issue(BIGINT, TEXT, TEXT, BIGINT, TEXT, BIGINT, TEXT, TEXT, INTEGER, TEXT, TEXT, TEXT, TEXT, BYTEA, TIMESTAMPTZ, TIMESTAMPTZ, INTEGER),
	platform.data_return_archive(BIGINT, TEXT), platform.data_return_revoke(BIGINT, BIGINT, TEXT, BOOLEAN),
	platform.data_return_purgeable(BIGINT), platform.data_return_purge_archive(BIGINT, TEXT),
	platform.data_return_live_archives(BIGINT), platform.data_return_drop_evidence(BIGINT),
	platform.data_return_list(BIGINT), platform.data_return_open_for(TEXT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION
	platform.data_return_issue(BIGINT, TEXT, TEXT, BIGINT, TEXT, BIGINT, TEXT, TEXT, INTEGER, TEXT, TEXT, TEXT, TEXT, BYTEA, TIMESTAMPTZ, TIMESTAMPTZ, INTEGER),
	platform.data_return_archive(BIGINT, TEXT), platform.data_return_revoke(BIGINT, BIGINT, TEXT, BOOLEAN),
	platform.data_return_purgeable(BIGINT), platform.data_return_purge_archive(BIGINT, TEXT),
	platform.data_return_live_archives(BIGINT), platform.data_return_drop_evidence(BIGINT),
	platform.data_return_list(BIGINT), platform.data_return_open_for(TEXT, BIGINT) TO phoenix_data_return;
GRANT EXECUTE ON FUNCTION platform.data_return_open_for(TEXT, BIGINT) TO phoenix_school_erasure;

COMMENT ON TABLE platform.data_return_grants IS
	'Retrieval token of a released return package (Art. 30 Abs. 2 Data Act, ADR 0203): HMAC of the token, archive key wrapped under it, expiry and download budget.';
COMMENT ON TABLE platform.data_return_downloads IS
	'One retrieval of a released return package with its outcome (ADR 0203).';
`

const dataReturnGrantsDownSQL = `
DROP FUNCTION IF EXISTS platform.data_return_open_for(TEXT, BIGINT);
DROP FUNCTION IF EXISTS platform.data_return_list(BIGINT);
DROP FUNCTION IF EXISTS platform.data_return_drop_evidence(BIGINT);
DROP FUNCTION IF EXISTS platform.data_return_live_archives(BIGINT);
DROP FUNCTION IF EXISTS platform.data_return_purge_archive(BIGINT, TEXT);
DROP FUNCTION IF EXISTS platform.data_return_purgeable(BIGINT);
DROP FUNCTION IF EXISTS platform.data_return_revoke(BIGINT, BIGINT, TEXT, BOOLEAN);
DROP FUNCTION IF EXISTS platform.data_return_archive(BIGINT, TEXT);
DROP FUNCTION IF EXISTS platform.data_return_issue(BIGINT, TEXT, TEXT, BIGINT, TEXT, BIGINT, TEXT, TEXT, INTEGER, TEXT, TEXT, TEXT, TEXT, BYTEA, TIMESTAMPTZ, TIMESTAMPTZ, INTEGER);
DROP FUNCTION IF EXISTS platform.data_return_lock_archive(TEXT);
DROP FUNCTION IF EXISTS platform.data_return_audit(BIGINT, TEXT, TEXT, BIGINT, JSONB);
DROP FUNCTION IF EXISTS platform.data_return_require_operator(BIGINT);
DROP TABLE IF EXISTS platform.data_return_downloads;
DROP TABLE IF EXISTS platform.data_return_grants;
REVOKE USAGE ON SCHEMA platform FROM phoenix_data_return;
DROP ROLE IF EXISTS phoenix_data_return;
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
