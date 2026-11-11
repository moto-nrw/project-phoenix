package postgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const batchHolidayClosureNoticesSQL = `SELECT ` + holidayClosureNoticeColumns + `
 FROM calendar.holiday_closure_proposals AS proposal
 LEFT JOIN calendar.holiday_closure_events AS event ON event.proposal_id = proposal.id AND event.tenant_id = proposal.tenant_id
 WHERE proposal.tenant_id = ? AND proposal.id = ANY(?::bigint[]) ORDER BY proposal.id`

const confirmHolidayClosureBatchSQL = `WITH requested AS (
 SELECT * FROM unnest(?1::bigint[], ?2::text[], ?3::boolean[]) AS item(proposal_id, token, preserve_span)
), created AS (
 INSERT INTO calendar.holiday_closure_events (tenant_id, organization_id, proposal_id, operation_id, source_kind, source_id,
  revision, span_from, span_to, confirmed_channel, impact_token)
 SELECT proposal.tenant_id, proposal.organization_id, proposal.id, proposal.operation_id, proposal.source_kind, proposal.source_id,
  COALESCE((SELECT revision FROM calendar.holiday_closure_revisions WHERE tenant_id = ?0), 0),
  CASE WHEN requested.preserve_span THEN proposal.span_from ELSE GREATEST(proposal.span_from, (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date) END,
  proposal.span_to, ?4, requested.token
 FROM calendar.holiday_closure_proposals AS proposal JOIN requested ON requested.proposal_id = proposal.id
 WHERE proposal.tenant_id = ?0 AND proposal.state = 'open' ON CONFLICT (proposal_id) DO NOTHING RETURNING id, proposal_id, operation_id, revision
), confirmed AS (
 UPDATE calendar.holiday_closure_proposals SET state = 'confirmed', confirmed_at = CURRENT_TIMESTAMP
 WHERE tenant_id = ?0 AND id IN (SELECT proposal_id FROM created)
), retried AS (
 UPDATE calendar.holiday_closure_events AS event
 SET state = 'pending', next_attempt_at = CURRENT_TIMESTAMP, error_code = NULL, requested_channel = ?4, requested_at = CURRENT_TIMESTAMP
 FROM requested WHERE event.tenant_id = ?0 AND event.proposal_id = requested.proposal_id AND event.state IN ('failed', 'blocked')
 RETURNING event.id, event.operation_id, event.revision
), authorized AS (
 INSERT INTO calendar.holiday_closure_authorizations (tenant_id, event_id, operation_id, revision, account_id, channel)
 SELECT ?0, id, operation_id, revision, NULLIF(?5, 0), ?4 FROM (SELECT id, operation_id, revision FROM created UNION ALL SELECT id, operation_id, revision FROM retried) AS commands
) SELECT count(*) FROM created`

const renewHolidayClosureBatchSQL = `WITH source AS MATERIALIZED (
 SELECT proposal.*, gen_random_uuid() AS replacement_operation, event.span_from AS confirmed_from, event.span_to AS confirmed_to
 FROM calendar.holiday_closure_proposals AS proposal
 JOIN calendar.holiday_closure_events AS event ON event.proposal_id = proposal.id AND event.tenant_id = proposal.tenant_id
 WHERE proposal.tenant_id = ? AND proposal.id = ANY(?::bigint[]) AND event.state = 'needs_confirmation'
  AND NOT EXISTS (SELECT 1 FROM calendar.holiday_closure_proposals AS newer
   WHERE newer.tenant_id = proposal.tenant_id AND newer.source_kind = proposal.source_kind
    AND newer.source_id = proposal.source_id AND (newer.state = 'open' OR newer.id > proposal.id))
), inserted AS (
 INSERT INTO calendar.holiday_closure_proposals (tenant_id, organization_id, source_kind, source_id, operation_id, reason,
  previous_start, previous_end, current_start, current_end, span_from, span_to, revision)
 SELECT tenant_id, organization_id, source_kind, source_id, replacement_operation, reason,
  previous_start, previous_end, current_start, current_end, confirmed_from, confirmed_to, revision FROM source
 RETURNING id, operation_id
) SELECT source.id AS previous_id, inserted.id AS replacement_id FROM source JOIN inserted ON inserted.operation_id = source.replacement_operation`

// ConfirmHolidayClosureNotices holds the calendar and all requested proposals
// once. Authorization and revisions are checked before any batch write.
func (s *HolidayClosureEventStore) ConfirmHolidayClosureNotices(ctx context.Context, confirmations []schoolcalendar.HolidayClosureConfirmation, actor schoolcalendar.HolidayClosureActor) ([]schoolcalendar.HolidayClosureNotice, error) {
	if len(confirmations) == 0 {
		return nil, nil
	}
	if actor.Channel != schoolcalendar.HolidayClosureChannelSchool && actor.Channel != schoolcalendar.HolidayClosureChannelTraeger {
		return nil, errors.New("school calendar postgres: the channel of the click is required")
	}
	db, schoolID, err := s.schoolDB(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := db.NewRaw(lockHolidayClosureCalendarSQL, schoolID).Exec(ctx); err != nil {
		return nil, err
	}
	ids, tokens := make([]int64, len(confirmations)), make([]string, len(confirmations))
	for i, confirmation := range confirmations {
		ids[i], tokens[i] = confirmation.ProposalID, confirmation.ImpactToken
	}
	rows, err := batchHolidayClosureRows(ctx, db, schoolID, ids, true)
	if err != nil {
		return nil, err
	}
	preserve, renew, err := checkHolidayClosureConfirmations(confirmations, rows, actor)
	if err != nil {
		return nil, err
	}
	replacements, err := renewHolidayClosureConfirmations(ctx, db, schoolID, ids, renew, actor)
	if err != nil {
		return nil, err
	}
	if _, err := db.NewRaw(confirmHolidayClosureBatchSQL, schoolID, pgdialect.Array(ids), pgdialect.Array(tokens), pgdialect.Array(preserve), actor.Channel, actor.AccountID).Exec(ctx); err != nil {
		return nil, err
	}
	if err := finishHolidayClosureRenewals(ctx, db, schoolID, replacements); err != nil {
		return nil, err
	}
	rows, err = batchHolidayClosureRows(ctx, db, schoolID, ids, false)
	if err != nil {
		return nil, err
	}
	result := make([]schoolcalendar.HolidayClosureNotice, len(ids))
	for i, id := range ids {
		result[i] = rows[id].notice()
	}
	return result, nil
}

func batchHolidayClosureRows(ctx context.Context, db bun.IDB, schoolID int64, ids []int64, lock bool) (map[int64]holidayClosureNoticeRow, error) {
	var rows []holidayClosureNoticeRow
	var err error
	if lock {
		err = db.NewRaw(batchHolidayClosureNoticesSQL+" FOR UPDATE OF proposal", schoolID, pgdialect.Array(ids)).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(batchHolidayClosureNoticesSQL, schoolID, pgdialect.Array(ids)).Scan(ctx, &rows)
	}
	if err != nil {
		return nil, err
	}
	result := make(map[int64]holidayClosureNoticeRow, len(rows))
	for _, row := range rows {
		result[row.ProposalID] = row
	}
	return result, nil
}

func checkHolidayClosureConfirmations(confirmations []schoolcalendar.HolidayClosureConfirmation, rows map[int64]holidayClosureNoticeRow, actor schoolcalendar.HolidayClosureActor) ([]bool, []int64, error) {
	preserve := make([]bool, len(confirmations))
	var renew []int64
	seen := map[int64]bool{}
	for i, confirmation := range confirmations {
		row, exists := rows[confirmation.ProposalID]
		if !exists || seen[confirmation.ProposalID] || !holidayClosureActorMatchesOrganization(actor, row.OrganizationID) {
			return nil, nil, schoolcalendar.ErrHolidayClosureNoticeNotFound
		}
		seen[confirmation.ProposalID] = true
		needsApproval := row.EventState != nil && *row.EventState == schoolcalendar.HolidayClosureEventNeedsConfirmation
		if (row.ProposalState == "open" || needsApproval) && row.Revision != confirmation.Revision {
			return nil, nil, schoolcalendar.ErrHolidayClosureNoticeChanged
		}
		preserve[i] = row.ConfirmedAt != nil
		if needsApproval {
			renew = append(renew, row.ProposalID)
		}
	}
	return preserve, renew, nil
}

func renewHolidayClosureConfirmations(ctx context.Context, db bun.IDB, schoolID int64, ids, renew []int64, actor schoolcalendar.HolidayClosureActor) ([]holidayClosureReplacement, error) {
	if len(renew) == 0 {
		return nil, nil
	}
	var replacements []holidayClosureReplacement
	if err := db.NewRaw(renewHolidayClosureBatchSQL, schoolID, pgdialect.Array(renew)).Scan(ctx, &replacements); err != nil {
		return nil, err
	}
	if len(replacements) != len(renew) {
		return nil, schoolcalendar.ErrHolidayClosureNoticeChanged
	}
	mapped := map[int64]int64{}
	for _, replacement := range replacements {
		mapped[replacement.PreviousID] = replacement.ReplacementID
	}
	var replacementIDs []int64
	for i, id := range ids {
		if next, ok := mapped[id]; ok {
			ids[i] = next
			replacementIDs = append(replacementIDs, next)
		}
	}
	rows, err := batchHolidayClosureRows(ctx, db, schoolID, replacementIDs, false)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if !holidayClosureActorMatchesOrganization(actor, row.OrganizationID) {
			return nil, schoolcalendar.ErrHolidayClosureNoticeNotFound
		}
	}
	return replacements, nil
}

type holidayClosureReplacement struct {
	PreviousID    int64 `bun:"previous_id"`
	ReplacementID int64 `bun:"replacement_id"`
}

const finishHolidayClosureRenewalsSQL = `WITH replacements AS (
 SELECT * FROM unnest(?1::bigint[], ?2::bigint[]) AS item(previous_id, replacement_id)
), moved AS (
 UPDATE calendar.holiday_closure_authorizations AS entry SET event_id = next_event.id
 FROM replacements
 JOIN calendar.holiday_closure_events AS prior_event ON prior_event.tenant_id = ?0 AND prior_event.proposal_id = replacements.previous_id
 JOIN calendar.holiday_closure_events AS next_event ON next_event.tenant_id = ?0 AND next_event.proposal_id = replacements.replacement_id
 WHERE entry.tenant_id = ?0 AND entry.event_id = prior_event.id
 RETURNING entry.id
)
DELETE FROM calendar.holiday_closure_proposals WHERE tenant_id = ?0 AND id = ANY(?1::bigint[])
 AND (SELECT count(*) FROM moved) >= 0`

// Move only the retention anchor, preserving the original authorization facts.
func finishHolidayClosureRenewals(ctx context.Context, db bun.IDB, schoolID int64, replacements []holidayClosureReplacement) error {
	if len(replacements) == 0 {
		return nil
	}
	previous, current := make([]int64, len(replacements)), make([]int64, len(replacements))
	for i, replacement := range replacements {
		previous[i], current[i] = replacement.PreviousID, replacement.ReplacementID
	}
	_, err := db.NewRaw(finishHolidayClosureRenewalsSQL, schoolID, pgdialect.Array(previous), pgdialect.Array(current)).Exec(ctx)
	return err
}
