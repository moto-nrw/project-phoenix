package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayClosureEventsVersion = "1.15.920"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayClosureEventsVersion,
		Description: "Ferien: Schul-Schließtage schlagen einen Elternhinweis vor, ein Klick beauftragt ihn (#6442, #6457, ADR 0196)",
		DependsOn: []string{
			dataReturnErasureOrderLockVersion,           // the newest migration on development
			parentBillingDiscardEmptyHolidayRunsVersion, // the migrations below it
			dataReturnGrantsVersion,
			holidayHostSchoolCostCenterVersion, // the other migration above the base of this one
			holidayClosureNoticesVersion,       // the notices this one versions
			traegerClosingDaysVersion,          // the Träger closing days the read channel names
			mealBillingMonthsVersion,           // the school erasure this one extends
		},
	})
	Migrations.MustRegister(holidayClosureEventsUp, holidayClosureEventsDown)
}

// holidayClosureCalendarUpSQL creates School Calendar's proposals and
// commands (ADR 0196): a change of a school closing day proposes a notice
// to the parents of booked holiday weeks; only a click of the school admin
// or the Träger-Büro turns the proposal into a durable command, which the
// Träger's worker processes. Neither table holds family, child or contact
// data. Deleting the closing day keeps both: source_id has no foreign key.
const holidayClosureCalendarUpSQL = `
CREATE TABLE calendar.holiday_closure_revisions (
	tenant_id BIGINT PRIMARY KEY REFERENCES platform.schools(id) ON DELETE CASCADE,
	revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE calendar.holiday_closure_revisions IS
	'Revision of the closing days of a school (ADR 0196); every change of a school closing day raises it under the row lock.';

CREATE TABLE calendar.holiday_closure_proposals (
	id BIGSERIAL PRIMARY KEY,
	tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	source_kind TEXT NOT NULL CHECK (source_kind IN ('school_closing_day')),
	source_id BIGINT NOT NULL CHECK (source_id > 0),
	operation_id UUID NOT NULL UNIQUE,
	reason VARCHAR(255) NOT NULL,
	previous_start DATE,
	previous_end DATE,
	current_start DATE,
	current_end DATE,
	span_from DATE NOT NULL,
	span_to DATE NOT NULL,
	revision BIGINT NOT NULL CHECK (revision >= 0),
	state TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'confirmed')),
	supersedes_event_id BIGINT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	confirmed_at TIMESTAMPTZ,
	CONSTRAINT chk_holiday_closure_proposals_previous CHECK ((previous_start IS NULL) = (previous_end IS NULL) AND previous_start <= previous_end),
	CONSTRAINT chk_holiday_closure_proposals_current CHECK ((current_start IS NULL) = (current_end IS NULL) AND current_start <= current_end),
	CONSTRAINT chk_holiday_closure_proposals_range CHECK (previous_start IS NOT NULL OR current_start IS NOT NULL),
	CONSTRAINT chk_holiday_closure_proposals_span CHECK (span_from <= span_to),
	CONSTRAINT chk_holiday_closure_proposals_confirmed CHECK ((state = 'confirmed') = (confirmed_at IS NOT NULL))
);
CREATE UNIQUE INDEX uq_holiday_closure_proposals_open_source
	ON calendar.holiday_closure_proposals (tenant_id, source_kind, source_id) WHERE state = 'open';
CREATE INDEX idx_holiday_closure_proposals_organization ON calendar.holiday_closure_proposals (organization_id, state);
COMMENT ON TABLE calendar.holiday_closure_proposals IS
	'Proposed notice to the parents of booked holiday weeks after a school closing day changed (ADR 0196); not a send order.';

CREATE TRIGGER update_holiday_closure_proposals_updated_at
	BEFORE UPDATE ON calendar.holiday_closure_proposals
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

-- The proposal belongs to the Träger of its school at the time of the
-- change. A school without a Träger offers no holiday care: no proposal.
CREATE FUNCTION calendar.holiday_closure_proposal_traeger()
	RETURNS trigger
	LANGUAGE plpgsql
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	BEGIN
		SELECT school.organization_id INTO NEW.organization_id
		FROM platform.schools AS school
		WHERE school.id = NEW.tenant_id AND school.deleted_at IS NULL;
		IF NEW.organization_id IS NULL THEN
			RETURN NULL;
		END IF;
  IF TG_OP = 'UPDATE' AND NEW.organization_id IS DISTINCT FROM OLD.organization_id THEN
   NEW.operation_id := gen_random_uuid();
  END IF;
		RETURN NEW;
	END;
	$$;
REVOKE ALL ON FUNCTION calendar.holiday_closure_proposal_traeger() FROM PUBLIC;
CREATE TRIGGER holiday_closure_proposal_traeger
	BEFORE INSERT OR UPDATE OF revision ON calendar.holiday_closure_proposals
	FOR EACH ROW EXECUTE FUNCTION calendar.holiday_closure_proposal_traeger();

CREATE TABLE calendar.holiday_closure_events (
	id BIGSERIAL PRIMARY KEY,
	tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	proposal_id BIGINT NOT NULL UNIQUE REFERENCES calendar.holiday_closure_proposals(id) ON DELETE CASCADE,
	operation_id UUID NOT NULL UNIQUE,
	source_kind TEXT NOT NULL,
	source_id BIGINT NOT NULL,
	revision BIGINT NOT NULL,
 impact_token TEXT NOT NULL DEFAULT '',
	span_from DATE NOT NULL,
	span_to DATE NOT NULL,
	state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN
		('pending', 'leased', 'prepared', 'completed', 'no_mail', 'failed', 'unknown', 'blocked', 'needs_confirmation')),
	lease_token UUID,
	lease_until TIMESTAMPTZ,
	attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
	error_code TEXT CHECK (error_code IS NULL OR char_length(error_code) <= 80),
	next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	confirmed_channel TEXT NOT NULL CHECK (confirmed_channel IN ('school', 'traeger')),
	confirmed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	requested_channel TEXT CHECK (requested_channel IN ('school', 'traeger')),
	requested_at TIMESTAMPTZ,
	processed_at TIMESTAMPTZ,
	completed_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_holiday_closure_events_span CHECK (span_from <= span_to),
	CONSTRAINT chk_holiday_closure_events_lease CHECK ((state = 'leased') = (lease_token IS NOT NULL AND lease_until IS NOT NULL))
);
CREATE INDEX idx_holiday_closure_events_due ON calendar.holiday_closure_events (organization_id, state, next_attempt_at);
CREATE INDEX idx_holiday_closure_events_school ON calendar.holiday_closure_events (tenant_id, source_kind, source_id);
COMMENT ON TABLE calendar.holiday_closure_events IS
	'Durable command to tell the parents of booked holiday weeks about a confirmed closing day change (ADR 0196).';

CREATE TRIGGER update_holiday_closure_events_updated_at
	BEFORE UPDATE ON calendar.holiday_closure_events
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

-- Field guard (ADR 0196 section 3): the command fields come only from the
-- school unit that confirms the proposal and never change. A Träger unit
-- moves the processing state, a school unit only takes up a failed command
-- again. Row security decides whose row it is; this guard decides which
-- fields each of them writes, below and in addition to it.
CREATE FUNCTION calendar.guard_holiday_closure_event()
	RETURNS trigger
	LANGUAGE plpgsql
	SET search_path = pg_catalog, pg_temp
	AS $$
	DECLARE
		v_school BIGINT := NULLIF(current_setting('app.current_tenant_id', true), '')::bigint;
		v_traeger BIGINT := NULLIF(current_setting('app.current_org_id', true), '')::bigint;
	BEGIN
		IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.organization_id IS DISTINCT FROM OLD.organization_id
			OR NEW.proposal_id IS DISTINCT FROM OLD.proposal_id OR NEW.operation_id IS DISTINCT FROM OLD.operation_id
			OR NEW.source_kind IS DISTINCT FROM OLD.source_kind OR NEW.source_id IS DISTINCT FROM OLD.source_id
			OR NEW.impact_token IS DISTINCT FROM OLD.impact_token
   OR NEW.revision IS DISTINCT FROM OLD.revision OR NEW.span_from IS DISTINCT FROM OLD.span_from
			OR NEW.span_to IS DISTINCT FROM OLD.span_to OR NEW.confirmed_channel IS DISTINCT FROM OLD.confirmed_channel
			OR NEW.confirmed_at IS DISTINCT FROM OLD.confirmed_at OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
			RAISE EXCEPTION 'the command fields of a holiday closure event never change'
				USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_holiday_closure_events_immutable';
		END IF;
		IF v_school IS NOT NULL AND v_traeger IS NULL THEN
			IF OLD.state NOT IN ('failed', 'blocked') OR NEW.state <> 'pending'
				OR NEW.lease_token IS NOT NULL OR NEW.attempts IS DISTINCT FROM OLD.attempts
				OR NEW.processed_at IS DISTINCT FROM OLD.processed_at OR NEW.completed_at IS DISTINCT FROM OLD.completed_at THEN
				RAISE EXCEPTION 'a school only takes up a failed holiday closure event again'
					USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_holiday_closure_events_school_fields';
			END IF;
		ELSIF v_traeger IS NOT NULL AND v_school IS NULL THEN
			IF NEW.requested_channel IS DISTINCT FROM OLD.requested_channel OR NEW.requested_at IS DISTINCT FROM OLD.requested_at THEN
				RAISE EXCEPTION 'the Träger unit processes a holiday closure event, it does not request it'
					USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_holiday_closure_events_traeger_fields';
			END IF;
		END IF;
		RETURN NEW;
	END;
	$$;
CREATE TRIGGER guard_holiday_closure_event
	BEFORE UPDATE ON calendar.holiday_closure_events
	FOR EACH ROW EXECUTE FUNCTION calendar.guard_holiday_closure_event();

ALTER TABLE calendar.holiday_closure_revisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE calendar.holiday_closure_revisions FORCE ROW LEVEL SECURITY;
ALTER TABLE calendar.holiday_closure_proposals ENABLE ROW LEVEL SECURITY;
ALTER TABLE calendar.holiday_closure_proposals FORCE ROW LEVEL SECURITY;
ALTER TABLE calendar.holiday_closure_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE calendar.holiday_closure_events FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_holiday_closure_revisions ON calendar.holiday_closure_revisions FOR ALL
	USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
CREATE POLICY tenant_isolation_holiday_closure_proposals ON calendar.holiday_closure_proposals FOR ALL
	USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
CREATE POLICY tenant_read_holiday_closure_events ON calendar.holiday_closure_events FOR SELECT
	USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
CREATE POLICY tenant_insert_holiday_closure_events ON calendar.holiday_closure_events FOR INSERT
	WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
		AND EXISTS (SELECT 1 FROM platform.schools AS school
			WHERE school.id = holiday_closure_events.tenant_id
				AND school.organization_id = holiday_closure_events.organization_id AND school.deleted_at IS NULL));
CREATE POLICY tenant_update_holiday_closure_events ON calendar.holiday_closure_events FOR UPDATE
	USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

-- The Träger reads the proposals and processes the commands of its current
-- schools only: a school that moved to another Träger or was deleted takes
-- its rows out of the Träger's reach (ADR 0196 section 3.2).
CREATE POLICY traeger_read_holiday_closure_proposals ON calendar.holiday_closure_proposals FOR SELECT
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint
		AND EXISTS (SELECT 1 FROM platform.schools AS school
			WHERE school.id = holiday_closure_proposals.tenant_id
				AND school.organization_id = holiday_closure_proposals.organization_id AND school.deleted_at IS NULL));
CREATE POLICY traeger_read_holiday_closure_events ON calendar.holiday_closure_events FOR SELECT
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint
		AND EXISTS (SELECT 1 FROM platform.schools AS school
			WHERE school.id = holiday_closure_events.tenant_id
				AND school.organization_id = holiday_closure_events.organization_id AND school.deleted_at IS NULL));
CREATE POLICY traeger_process_holiday_closure_events ON calendar.holiday_closure_events FOR UPDATE
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint
		AND EXISTS (SELECT 1 FROM platform.schools AS school
			WHERE school.id = holiday_closure_events.tenant_id
				AND school.organization_id = holiday_closure_events.organization_id AND school.deleted_at IS NULL))
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
CREATE POLICY traeger_retire_holiday_closure_proposals ON calendar.holiday_closure_proposals FOR DELETE
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE ON calendar.holiday_closure_revisions TO phoenix_tenant;
GRANT SELECT, INSERT, UPDATE, DELETE ON calendar.holiday_closure_proposals TO phoenix_tenant;
GRANT SELECT, INSERT, UPDATE ON calendar.holiday_closure_events TO phoenix_tenant;
-- The independent administrative watchdog reads command metadata only.
GRANT SELECT ON calendar.holiday_closure_events TO phoenix_admin;
GRANT USAGE ON SEQUENCE calendar.holiday_closure_proposals_id_seq, calendar.holiday_closure_events_id_seq TO phoenix_tenant;

-- The worker learns which Träger have work, nothing else (ADR 0196 section
-- 6): due commands, expired leases and prepared mails whose acceptance it
-- still waits for.
CREATE FUNCTION calendar.holiday_closure_due_traeger_ids()
	RETURNS SETOF BIGINT
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
		SELECT DISTINCT event.organization_id
		FROM calendar.holiday_closure_events AS event
		WHERE (event.state = 'pending' AND event.next_attempt_at <= clock_timestamp())
			OR (event.state = 'leased' AND event.lease_until < clock_timestamp())
			OR event.state IN ('prepared', 'unknown')
			OR (event.state IN ('completed', 'no_mail')
				AND COALESCE(event.completed_at, event.processed_at) < clock_timestamp() - interval '30 days')
		UNION
		SELECT proposal.organization_id FROM calendar.holiday_closure_proposals AS proposal
		WHERE proposal.state = 'open' AND proposal.span_to < (clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date - 30
		ORDER BY 1
	$$;
REVOKE ALL ON FUNCTION calendar.holiday_closure_due_traeger_ids() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION calendar.holiday_closure_due_traeger_ids() TO phoenix_tenant, phoenix_admin;
`

// holidayClosureReaderUpSQL is the narrow read channel of the Träger's
// worker (ADR 0196 section 3.4): a fixed, read-only SECURITY DEFINER
// function owned by a role without BYPASSRLS and without write rights. Its
// policies reach only the current, undeleted schools of the calling Träger;
// the function names the closing days of one such school, its revision, its
// name and the stored federal state, and nothing else.
const holidayClosureReaderUpSQL = `
DO $role$ BEGIN
	IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_holiday_closure_reader') THEN
		CREATE ROLE phoenix_holiday_closure_reader NOLOGIN NOINHERIT NOBYPASSRLS;
	END IF;
END $role$;
GRANT USAGE ON SCHEMA calendar, schedule, platform, config TO phoenix_holiday_closure_reader;
GRANT SELECT ON platform.schools, schedule.closing_days, schedule.traeger_closing_days, config.setting_values,
	calendar.holiday_closure_revisions TO phoenix_holiday_closure_reader;

CREATE POLICY holiday_closure_reader_closing_days ON schedule.closing_days FOR SELECT TO phoenix_holiday_closure_reader
	USING (EXISTS (SELECT 1 FROM platform.schools AS school
		WHERE school.id = closing_days.tenant_id AND school.deleted_at IS NULL
			AND school.organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint));
CREATE POLICY holiday_closure_reader_traeger_closing_days ON schedule.traeger_closing_days FOR SELECT TO phoenix_holiday_closure_reader
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
CREATE POLICY holiday_closure_reader_federal_state ON config.setting_values FOR SELECT TO phoenix_holiday_closure_reader
	USING (setting_key = 'operations.federal_state' AND EXISTS (SELECT 1 FROM platform.schools AS school
		WHERE school.id = setting_values.tenant_id AND school.deleted_at IS NULL
			AND school.organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint));
CREATE POLICY holiday_closure_reader_revisions ON calendar.holiday_closure_revisions FOR SELECT TO phoenix_holiday_closure_reader
	USING (EXISTS (SELECT 1 FROM platform.schools AS school
		WHERE school.id = holiday_closure_revisions.tenant_id AND school.deleted_at IS NULL
			AND school.organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint));

-- One header row ('school') per current school of the calling Träger in
-- p_school_ids, then their closing days ('closing') and the Träger's closing
-- days that name them ('traeger_closing') overlapping [p_from, p_to]; every
-- row names its school. Every row repeats the membership check in the same
-- statement, so a foreign or deleted school yields no row.
-- Lock only current memberships before calendar and command locks. The
-- caller gains no UPDATE grant on platform.schools; the definer performs no
-- writes and returns only the authorized school IDs.
CREATE FUNCTION calendar.lock_holiday_closure_memberships(p_school_ids BIGINT[])
 RETURNS SETOF BIGINT LANGUAGE sql VOLATILE SECURITY DEFINER
 SET search_path = pg_catalog, pg_temp
 AS $$
  SELECT school.id FROM platform.schools AS school
  WHERE school.id = ANY(p_school_ids) AND school.deleted_at IS NULL
   AND school.organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint
  ORDER BY school.id FOR SHARE
 $$;
REVOKE ALL ON FUNCTION calendar.lock_holiday_closure_memberships(BIGINT[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION calendar.lock_holiday_closure_memberships(BIGINT[]) TO phoenix_tenant;

CREATE FUNCTION calendar.traeger_holiday_closure_calendar(p_school_ids BIGINT[], p_from DATE, p_to DATE)
	RETURNS TABLE (kind TEXT, school_id BIGINT, source_id BIGINT, start_date DATE, end_date DATE, reason TEXT,
		school_name TEXT, federal_state JSONB, revision BIGINT)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
		WITH school AS (
			SELECT school.id, school.name
			FROM platform.schools AS school
			WHERE school.id = ANY (p_school_ids) AND school.deleted_at IS NULL
				AND school.organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint
		)
		SELECT 'school'::text, school.id, school.id, NULL::date, NULL::date, NULL::text, school.name::text,
			(SELECT value FROM config.setting_values AS setting
				WHERE setting.tenant_id = school.id AND setting.setting_key = 'operations.federal_state'),
			COALESCE((SELECT revision.revision FROM calendar.holiday_closure_revisions AS revision
				WHERE revision.tenant_id = school.id), 0)
		FROM school
		UNION ALL
		SELECT 'closing', school.id, closing.id, closing.start_date, closing.end_date, closing.reason::text, NULL, NULL, NULL
		FROM schedule.closing_days AS closing
		JOIN school ON school.id = closing.tenant_id
		WHERE closing.start_date <= p_to AND closing.end_date >= p_from
		UNION ALL
		SELECT 'traeger_closing', school.id, traeger_day.id, traeger_day.start_date, traeger_day.end_date, traeger_day.reason, NULL, NULL, NULL
		FROM schedule.traeger_closing_days AS traeger_day
		JOIN school ON school.id = ANY (traeger_day.school_ids)
		WHERE traeger_day.organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint
			AND traeger_day.start_date <= p_to AND traeger_day.end_date >= p_from
	$$;
ALTER FUNCTION calendar.traeger_holiday_closure_calendar(BIGINT[], DATE, DATE) OWNER TO phoenix_holiday_closure_reader;
REVOKE ALL ON FUNCTION calendar.traeger_holiday_closure_calendar(BIGINT[], DATE, DATE) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION calendar.traeger_holiday_closure_calendar(BIGINT[], DATE, DATE) TO phoenix_tenant;
`

// holidayClosureNoticeStatesUpSQL versions Enrollment's notices (ADR 0196
// section 4): each told day of a registration and week is now 'closed' or
// 'open' again, with the operation that told it. Existing notices told a
// closed day, so they stay 'closed' without a new mail. The sends record
// one logical mail per Träger, operation and family (a keyed fingerprint of
// the recipient address, never the address) and go with every
// registration the mail names.
const holidayClosureNoticeStatesUpSQL = `
ALTER TABLE enrollment.holiday_closure_notices
	ADD COLUMN state TEXT NOT NULL DEFAULT 'closed' CHECK (state IN ('closed', 'open')),
	ADD COLUMN operation_id UUID,
	ADD COLUMN changed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;
GRANT UPDATE (state, operation_id, changed_at) ON enrollment.holiday_closure_notices TO phoenix_tenant;
COMMENT ON TABLE enrollment.holiday_closure_notices IS
	'What the parents of a registration were last told about a day of a booked holiday week (#6434, ADR 0196): closed or open again.';

CREATE TABLE enrollment.holiday_closure_family_keys (
	organization_id BIGINT PRIMARY KEY REFERENCES platform.organizations(id) ON DELETE CASCADE,
	secret BYTEA NOT NULL CHECK (octet_length(secret) = 32),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE enrollment.holiday_closure_family_keys IS
	'Key of the Träger that derives the family fingerprint of a closing day mail (ADR 0196); holds no personal data.';
ALTER TABLE enrollment.holiday_closure_family_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_closure_family_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_closure_family_keys ON enrollment.holiday_closure_family_keys FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
GRANT SELECT, INSERT ON enrollment.holiday_closure_family_keys TO phoenix_tenant;

CREATE TABLE enrollment.holiday_closure_sends (
	organization_id BIGINT NOT NULL,
	operation_id UUID NOT NULL,
	family_key TEXT NOT NULL CHECK (family_key ~ '^[0-9a-f]{64}$'),
	holiday_registration_id BIGINT NOT NULL,
	idempotency_key TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, operation_id, family_key, holiday_registration_id),
	CONSTRAINT fk_holiday_closure_sends_registration FOREIGN KEY (organization_id, holiday_registration_id)
		REFERENCES enrollment.holiday_registrations (organization_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_holiday_closure_sends_registration ON enrollment.holiday_closure_sends (organization_id, holiday_registration_id);
COMMENT ON TABLE enrollment.holiday_closure_sends IS
	'One logical closing day mail per Träger, operation and family (ADR 0196); a row per registration the mail names.';
ALTER TABLE enrollment.holiday_closure_sends ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_closure_sends FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_closure_sends ON enrollment.holiday_closure_sends FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
GRANT SELECT, INSERT ON enrollment.holiday_closure_sends TO phoenix_tenant;
`

// holidayClosureCarryOverSQL takes the open cases of #6443 over without a
// mail (ADR 0196 section 7): a school closing day from today on whose days
// in a booked week nobody told yet becomes a proposal, never a command. Its
// operation ID derives from Träger, school, closing day and this migration,
// so running it again adds nothing. Told days stay told; the worker tells
// only the missing ones after a click.
const holidayClosureCarryOverSQL = `
WITH today AS (SELECT (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date AS day),
pending AS (
	SELECT DISTINCT closing.id, closing.tenant_id, school.organization_id, closing.start_date, closing.end_date, closing.reason
	FROM schedule.closing_days AS closing
	JOIN platform.schools AS school ON school.id = closing.tenant_id AND school.deleted_at IS NULL
	CROSS JOIN today
	JOIN enrollment.holiday_period_weeks AS week
		ON week.organization_id = school.organization_id AND week.host_school_id = closing.tenant_id
		AND week.starts_on <= closing.end_date AND week.ends_on >= closing.start_date
	JOIN enrollment.holiday_registration_bookings AS booking
		ON booking.organization_id = week.organization_id AND booking.holiday_period_week_id = week.id
	JOIN enrollment.holiday_registration_children AS child
		ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	WHERE closing.end_date >= today.day AND registration.status IN ('received', 'confirmed')
		AND EXISTS (
			SELECT 1 FROM generate_series(GREATEST(closing.start_date, week.starts_on, today.day),
				LEAST(closing.end_date, week.ends_on), interval '1 day') AS series(day)
			WHERE extract(isodow FROM series.day) < 6
				AND (booking.days IS NULL OR series.day::date = ANY (booking.days))
				AND NOT EXISTS (SELECT 1 FROM enrollment.holiday_closure_notices AS notice
					WHERE notice.organization_id = booking.organization_id
						AND notice.holiday_registration_id = registration.id
						AND notice.holiday_period_week_id = week.id AND notice.closed_on = series.day::date))
)
INSERT INTO calendar.holiday_closure_proposals
	(tenant_id, organization_id, source_kind, source_id, operation_id, reason, current_start, current_end, span_from, span_to, revision)
SELECT pending.tenant_id, pending.organization_id, 'school_closing_day', pending.id,
	substr(encode(sha256(convert_to('holiday-closure-carry-over:1.15.920:' || pending.organization_id || ':'
		|| pending.tenant_id || ':' || pending.id, 'UTF8')), 'hex'), 1, 32)::uuid,
	left(pending.reason, 255), pending.start_date, pending.end_date, pending.start_date, pending.end_date, 0
FROM pending
ON CONFLICT DO NOTHING;
`

const holidayClosureEventsDownSQL = `
DROP FUNCTION IF EXISTS calendar.traeger_holiday_closure_calendar(BIGINT[], DATE, DATE);
DROP POLICY IF EXISTS holiday_closure_reader_closing_days ON schedule.closing_days;
DROP POLICY IF EXISTS holiday_closure_reader_traeger_closing_days ON schedule.traeger_closing_days;
DROP POLICY IF EXISTS holiday_closure_reader_federal_state ON config.setting_values;
DROP FUNCTION IF EXISTS calendar.holiday_closure_due_traeger_ids();
DROP FUNCTION IF EXISTS calendar.lock_holiday_closure_memberships(BIGINT[]);
DROP TABLE IF EXISTS calendar.holiday_closure_events;
DROP TABLE IF EXISTS calendar.holiday_closure_proposals;
DROP TABLE IF EXISTS calendar.holiday_closure_revisions;
DROP FUNCTION IF EXISTS calendar.guard_holiday_closure_event();
DROP FUNCTION IF EXISTS calendar.holiday_closure_proposal_traeger();
REVOKE ALL ON platform.schools, schedule.closing_days, schedule.traeger_closing_days, config.setting_values
	FROM phoenix_holiday_closure_reader;
REVOKE USAGE ON SCHEMA calendar, schedule, platform, config FROM phoenix_holiday_closure_reader;
DROP TABLE IF EXISTS enrollment.holiday_closure_sends;
DROP TABLE IF EXISTS enrollment.holiday_closure_family_keys;
DO $notices$ BEGIN
	IF EXISTS (SELECT FROM information_schema.columns
		WHERE table_schema = 'enrollment' AND table_name = 'holiday_closure_notices' AND column_name = 'state') THEN
		DELETE FROM enrollment.holiday_closure_notices WHERE state = 'open';
	END IF;
END $notices$;
ALTER TABLE enrollment.holiday_closure_notices DROP COLUMN IF EXISTS state, DROP COLUMN IF EXISTS operation_id,
	DROP COLUMN IF EXISTS changed_at;
`

func holidayClosureEventsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayClosureCalendarUpSQL); err != nil {
		return fmt.Errorf("create the holiday closure proposals and events: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayClosureReaderUpSQL); err != nil {
		return fmt.Errorf("create the read channel of the holiday closure worker: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayClosureNoticeStatesUpSQL); err != nil {
		return fmt.Errorf("version the holiday closure notices: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayClosureCarryOverSQL); err != nil {
		return fmt.Errorf("carry the open holiday closure cases over as proposals: %w", err)
	}
	return installSchoolErasureSteps(ctx, db, "add the holiday closure notices to the school erasure",
		installHolidayClosureErasurePlan, installHolidayClosureErasurePreflight, installHolidayClosureErasureResidual,
		installHolidayClosureErasureDelete, installHolidayClosureErasurePreview)
}

func holidayClosureEventsDown(ctx context.Context, db *bun.DB) error {
	if err := installSchoolErasureSteps(ctx, db, "remove the holiday closure notices from the school erasure",
		installAttendanceRetentionMarkErasurePlan, installEmailDeliveryEventErasurePreflight, installAttendanceRetentionMarkErasureResidual,
		installAttendanceRetentionMarkErasureDelete, installAttendanceRetentionMarkErasurePreview); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, holidayClosureEventsDownSQL); err != nil {
		return fmt.Errorf("drop the holiday closure proposals and events: %w", err)
	}
	return nil
}
