package parentaudience

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"
)

type feedRow struct {
	ID                      int64      `bun:"id"`
	TenantID                int64      `bun:"tenant_id"`
	Title                   string     `bun:"title"`
	Body                    string     `bun:"body"`
	Priority                string     `bun:"priority"`
	LinkURL                 *string    `bun:"link_url"`
	RequiresAcknowledgement bool       `bun:"requires_acknowledgement"`
	PublishedAt             *time.Time `bun:"published_at"`
	ExpiresAt               *time.Time `bun:"expires_at"`
	ResponseType            string     `bun:"response_type"`
	ResponseDeadline        *time.Time `bun:"response_deadline"`
	DeliveryMode            string     `bun:"delivery_mode"`
	SystemKind              *string    `bun:"system_kind"`
	ReminderSentAt          *time.Time `bun:"reminder_sent_at"`
	ReminderText            *string    `bun:"reminder_text"`
	ReadAt                  *time.Time `bun:"read_at"`
	AcknowledgedAt          *time.Time `bun:"acknowledged_at"`
}

// ListFeedForAccount returns the guardian's cross-school feed. Run it under
// the administrative transaction with the explicit school scope.
func (p *Projection) ListFeedForAccount(ctx context.Context, accountID int64, scope contract.ParentAnnouncementFeedScope) ([]*contract.ParentAnnouncementFeedItem, error) {
	if scope.IsEmpty() {
		return []*contract.ParentAnnouncementFeedItem{}, nil
	}
	applicants, err := p.pendingApplicantsForSchools(ctx, scopeSchools(scope))
	if err != nil {
		return nil, err
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []feedRow
	if err := db.NewRaw(feedSQL,
		applicants,
		accountID, feedScopeList(scope.TenantIDs), feedScopeList(scope.SystemOnlyTenantIDs),
		today, today, today, p.links.AllSchoolsGuardianLinkRows(db), accountID, accountID, accountID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list parent announcement feed: %w", err)
	}
	items := make([]*contract.ParentAnnouncementFeedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, &contract.ParentAnnouncementFeedItem{
			ID: row.ID, TenantID: row.TenantID, Title: row.Title, Body: row.Body, Priority: row.Priority,
			LinkURL: row.LinkURL, RequiresAcknowledgement: row.RequiresAcknowledgement,
			PublishedAt: row.PublishedAt, ExpiresAt: row.ExpiresAt, ResponseType: row.ResponseType,
			ResponseDeadline: row.ResponseDeadline, DeliveryMode: row.DeliveryMode, SystemKind: row.SystemKind,
			ReminderSentAt: row.ReminderSentAt, ReminderText: row.ReminderText,
			ReadAt: row.ReadAt, AcknowledgedAt: row.AcknowledgedAt,
		})
	}
	return items, nil
}

func scopeSchools(scope contract.ParentAnnouncementFeedScope) []int64 {
	ids := make([]int64, 0, len(scope.TenantIDs)+len(scope.SystemOnlyTenantIDs))
	ids = append(ids, scope.TenantIDs...)
	return append(ids, scope.SystemOnlyTenantIDs...)
}

// countOutstandingSQL counts live announcements that still need attention:
// unread, pending confirmation, an open poll with an unanswered child, or an
// open survey the account has not completed.
// Reading an actionable item must not clear its reminder.
const countOutstandingSQL = pendingApplicantsFeedCTE + `
			SELECT COUNT(*)
			FROM users.parent_announcements a
			LEFT JOIN users.parent_announcement_reads par
				ON par.announcement_id = a.id AND par.account_id = ?
			WHERE ` + feedScopePredicate + liveAnnouncementPredicate + `
				AND (
					par.read_at IS NULL
					OR (a.requires_acknowledgement AND par.acknowledged_at IS NULL)
					OR ` + openPollForAccountFeed + `
					OR ` + openSurveyForAccountFeed + `
				)
				AND ` + reachedAccountFeed

// CountOutstandingForAccount counts the account's live announcements that
// still need attention across the given schools.
func (p *Projection) CountOutstandingForAccount(ctx context.Context, accountID int64, scope contract.ParentAnnouncementFeedScope) (int, error) {
	if scope.IsEmpty() {
		return 0, nil
	}
	applicants, err := p.pendingApplicantsForSchools(ctx, scopeSchools(scope))
	if err != nil {
		return 0, err
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return 0, err
	}
	today := p.day()
	var count int
	if err := db.NewRaw(countOutstandingSQL,
		applicants,
		accountID, feedScopeList(scope.TenantIDs), feedScopeList(scope.SystemOnlyTenantIDs),
		today, today, today, p.links.AllSchoolsGuardianLinkRows(db), accountID,
		accountID,
		today, today, today, p.links.AllSchoolsGuardianLinkRows(db), accountID, accountID, accountID,
	).Scan(ctx, &count); err != nil {
		return 0, fmt.Errorf("count outstanding parent announcements: %w", err)
	}
	return count, nil
}

// statsSQL intersects read/ack counts with the CURRENT audience so the staff
// "X von Y" never shows more readers than the live target count.
const statsSQL = pendingApplicantsCTE + `, audience AS (` + audienceAccountsBound + `
			)
			SELECT
				(SELECT COUNT(*) FROM audience) AS target_count,
				(SELECT COUNT(*) FROM users.parent_announcement_reads par
					WHERE par.announcement_id = ? AND par.tenant_id = ?
						AND par.account_id IN (SELECT account_id FROM audience)) AS read_count,
				(SELECT COUNT(*) FROM users.parent_announcement_reads par
					WHERE par.announcement_id = ? AND par.tenant_id = ? AND par.acknowledged_at IS NOT NULL
						AND par.account_id IN (SELECT account_id FROM audience)) AS acknowledged_count`

// Stats returns the reach/read/ack counts for an announcement.
func (p *Projection) Stats(ctx context.Context, schoolID, announcementID int64) (*contract.ParentAnnouncementStats, error) {
	applicants, err := p.pendingApplicants(ctx)
	if err != nil {
		return nil, err
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	stats := &contract.ParentAnnouncementStats{}
	if err := db.NewRaw(statsSQL,
		applicants,
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
		announcementID, schoolID,
		announcementID, schoolID,
		announcementID, schoolID,
	).Scan(ctx, &stats.TargetCount, &stats.ReadCount, &stats.AcknowledgedCount); err != nil {
		return nil, fmt.Errorf("parent announcement stats: %w", err)
	}
	return stats, nil
}

// answerableChildrenSQL lists, per announcement, the children of the bound
// account the poll reaches with the option ids already selected. It is
// cross-school (the feed spans schools), so scoping comes from each
// announcement's own tenant_id.
const answerableChildrenSQL = `
			SELECT DISTINCT a.id AS announcement_id, s.id AS student_id,
				COALESCE(p.first_name, '') AS first_name,
				COALESCE(p.last_name, '') AS last_name,
				COALESCE(sel.options, ARRAY[]::bigint[]) AS selected_options
			FROM users.parent_announcements a
			JOIN users.parent_announcement_targets pt
				ON pt.announcement_id = a.id AND pt.tenant_id = a.tenant_id
			JOIN users.student_profiles s ON s.tenant_id = a.tenant_id` + studentMembershipJoins + ` AND (` + studentTargetMatchFeed + `
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
				AND sm.status <> 'alumnus'
			JOIN (?) sg ON sg.student_id = s.id AND sg.tenant_id = a.tenant_id
				AND sg.permissions @> '{"parent_portal.access": true, "parent_portal.poll.response": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = a.tenant_id
				AND gp.account_id = ?
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			LEFT JOIN LATERAL (
				SELECT array_agg(resp.option_id ORDER BY resp.option_id) AS options
				FROM users.parent_announcement_responses resp
				WHERE resp.announcement_id = a.id AND resp.student_id = s.id
			) sel ON TRUE
			WHERE a.id IN (?)
			ORDER BY last_name ASC, first_name ASC, student_id ASC`
