// Package parentaudience implements the tenant-safe parent-announcement
// audience projection: who an announcement reaches, the guardian-facing feed
// and badge, the staff reach/read/poll evaluations, and the reminder and
// e-mail recipient lists.
//
// It is a read-only projection because every one of those answers joins
// Communication's parent_announcement* tables with People Directory's
// student, person, and guardian rows, Identity's account membership, and the
// timetable's activity enrollments. It never writes; the announcement,
// target, option, read, and answer writes stay with their owner in
// parentpostgres. Enrollment's pending applicants arrive as values and are
// bound as a record set, so the projection never reads Enrollment's tables.
package parentaudience

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

// Database resolves the caller's ambient transaction plus the tenant the
// statement must be scoped to. A zero tenant means administrative or
// cross-tenant work, where the query relies on its explicit school filter.
type Database func(context.Context) (bun.IDB, int64, error)

// PendingApplicants supplies Enrollment's undecided applications: the
// pending_enrollment target reaches them although no student row exists yet.
type PendingApplicants interface {
	PendingAnnouncementApplicants(context.Context) ([]domain.PendingAnnouncementApplicant, error)
	PendingAnnouncementApplicantsForSchools(context.Context, []int64) ([]domain.PendingAnnouncementApplicant, error)
}

// Projection answers every parent-announcement read that spans owners.
type Projection struct {
	database   Database
	applicants PendingApplicants
	today      func() timezone.Date
}

// New builds the projection over the ambient transaction runtime. The
// optional clock fixes the calendar day the activity-group targets are
// evaluated on; production uses Berlin's current day.
func New(database Database, applicants PendingApplicants, clocks ...func() time.Time) *Projection {
	if database == nil {
		panic("communication parent audience: database runtime is required")
	}
	if applicants == nil {
		panic("communication parent audience: pending applicants are required")
	}
	return &Projection{database: database, applicants: applicants, today: timezone.CalendarDateClock(clocks...)}
}

func (p *Projection) day() string { return p.today().String() }

func (p *Projection) pendingApplicants(ctx context.Context) (string, error) {
	rows, err := p.applicants.PendingAnnouncementApplicants(ctx)
	if err != nil {
		return "", fmt.Errorf("load pending announcement audience: %w", err)
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return "", fmt.Errorf("encode pending announcement audience: %w", err)
	}
	return string(encoded), nil
}

func (p *Projection) pendingApplicantsForSchools(ctx context.Context, schoolIDs []int64) (string, error) {
	rows, err := p.applicants.PendingAnnouncementApplicantsForSchools(ctx, schoolIDs)
	if err != nil {
		return "", fmt.Errorf("load pending announcement feed audience: %w", err)
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return "", fmt.Errorf("encode pending announcement feed audience: %w", err)
	}
	return string(encoded), nil
}

// feedScopeList renders a school list for feedScopePredicate. An empty list
// binds a sentinel no school has, because `IN ()` is not valid SQL.
func feedScopeList(ids []int64) any {
	if len(ids) == 0 {
		return bun.List([]int64{-1})
	}
	return bun.List(ids)
}

// countAudienceSQL counts the distinct guardian accounts an announcement's
// targets currently reach within its school.
const countAudienceSQL = pendingApplicantsCTE + `
			SELECT COUNT(*) FROM (` + audienceAccountsBound + `
			) reached`

// CountAudience returns the number of distinct guardian accounts an
// announcement's targets currently reach within its school.
func (p *Projection) CountAudience(ctx context.Context, schoolID, announcementID int64) (int, error) {
	applicants, err := p.pendingApplicants(ctx)
	if err != nil {
		return 0, err
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return 0, err
	}
	today := p.day()
	var count int
	if err := db.NewRaw(countAudienceSQL,
		applicants,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
		announcementID, schoolID,
	).Scan(ctx, &count); err != nil {
		return 0, fmt.Errorf("count parent announcement audience: %w", err)
	}
	return count, nil
}

const accountMatchesSQL = pendingApplicantsFeedCTE + `SELECT ` + reachedAccountBound

// AccountMatchesAnnouncement reports whether a guardian account is in an
// announcement's audience right now.
func (p *Projection) AccountMatchesAnnouncement(ctx context.Context, schoolID, announcementID, accountID int64) (bool, error) {
	applicants, err := p.pendingApplicantsForSchools(ctx, []int64{schoolID})
	if err != nil {
		return false, err
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return false, err
	}
	today := p.day()
	var matched bool
	if err := db.NewRaw(accountMatchesSQL,
		applicants,
		schoolID, schoolID, today, today, today, schoolID, schoolID, accountID, announcementID, schoolID,
		schoolID, accountID, accountID, announcementID, schoolID,
	).Scan(ctx, &matched); err != nil {
		return false, fmt.Errorf("check parent announcement audience match: %w", err)
	}
	return matched, nil
}

// audienceEmailsSQL returns the distinct guardian recipients (non-empty
// e-mail) an announcement currently reaches. Only guardians WITH a linked
// account receive the e-mail: the mail is a pointer into the parent portal.
// A guardian reached through both a student target and pending_enrollment is
// collapsed to one row per account and normalized address; the retained name
// prefers a non-empty value across the merged rows.
const audienceEmailsSQL = pendingApplicantsCTE + `
			SELECT account_id, email,
				COALESCE(min(NULLIF(first_name, '')), '') AS first_name,
				COALESCE(min(NULLIF(last_name, '')), '') AS last_name
			FROM (
			SELECT DISTINCT gp.account_id, lower(gp.email) AS email,
				COALESCE(gp.first_name, '') AS first_name, COALESCE(gp.last_name, '') AS last_name` + reachedStudentsBound + `
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id IS NOT NULL
				AND gp.email IS NOT NULL AND length(btrim(gp.email)) > 0
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE pt.announcement_id = ? AND pt.tenant_id = ?

			UNION

			SELECT DISTINCT COALESCE(req.guardian_account_id, ea.id) AS account_id,
				lower(req.guardian_email) AS email,
				COALESCE(req.guardian_first_name, '') AS first_name, COALESCE(req.guardian_last_name, '') AS last_name` + pendingApplicantAccountsBound + `
				AND (req.guardian_account_id IS NOT NULL OR ea.id IS NOT NULL)
				AND length(btrim(req.guardian_email)) > 0
			) recips
			GROUP BY account_id, email`

type recipientRow struct {
	AccountID int64  `bun:"account_id"`
	Email     string `bun:"email"`
	FirstName string `bun:"first_name"`
	LastName  string `bun:"last_name"`
}

// ResolveAudienceEmails returns the distinct guardian recipients (with a
// non-empty e-mail) an announcement currently reaches.
func (p *Projection) ResolveAudienceEmails(ctx context.Context, schoolID, announcementID int64) ([]*domain.ParentAnnouncementRecipient, error) {
	applicants, err := p.pendingApplicants(ctx)
	if err != nil {
		return nil, err
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []recipientRow
	if err := db.NewRaw(audienceEmailsSQL,
		applicants,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
		announcementID, schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("resolve parent announcement audience emails: %w", err)
	}
	recipients := make([]*domain.ParentAnnouncementRecipient, 0, len(rows))
	for _, row := range rows {
		recipients = append(recipients, &domain.ParentAnnouncementRecipient{
			AccountID: row.AccountID, Email: row.Email, FirstName: row.FirstName, LastName: row.LastName,
		})
	}
	return recipients, nil
}

// letterChildStatusesSQL lists every reached child of an Elternbrief with its
// DERIVED fulfilment: the LATERAL picks the FIRST acknowledgement by a
// portal-access guardian of that child, so a guardian who acknowledged for a
// DIFFERENT child cannot fulfil this one.
const letterChildStatusesSQL = `WITH reached AS (` + letterReachedStudentsBound + `),
			confirmable AS (` + pollAudienceStudentsBound + `)
			SELECT s.id AS student_id,
				COALESCE(p.first_name, '') AS first_name,
				COALESCE(p.last_name, '')  AS last_name,
				COALESCE(s.school_class, '') AS school_class,
				EXISTS (SELECT 1 FROM confirmable c WHERE c.student_id = s.id) AS can_confirm,
				ack.acknowledged_at AS acknowledged_at,
				COALESCE(ack.first_name, '') AS ack_first_name,
				COALESCE(ack.last_name, '')  AS ack_last_name
			FROM reached
			JOIN users.students s ON s.id = reached.student_id
			JOIN users.persons p ON p.id = s.person_id
			LEFT JOIN LATERAL (
				SELECT par.acknowledged_at, gp.first_name, gp.last_name
				FROM users.students_guardians sg
				JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id
					AND gp.tenant_id = ? AND gp.account_id IS NOT NULL
				JOIN users.parent_announcement_reads par ON par.announcement_id = ?
					AND par.tenant_id = ? AND par.account_id = gp.account_id
				WHERE sg.student_id = s.id AND sg.tenant_id = ?
					AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
					AND par.acknowledged_at IS NOT NULL
				ORDER BY par.acknowledged_at ASC
				LIMIT 1
			) ack ON TRUE
			ORDER BY last_name ASC, first_name ASC, student_id ASC`

type letterChildRow struct {
	StudentID      int64      `bun:"student_id"`
	FirstName      string     `bun:"first_name"`
	LastName       string     `bun:"last_name"`
	SchoolClass    string     `bun:"school_class"`
	CanConfirm     bool       `bun:"can_confirm"`
	AcknowledgedAt *time.Time `bun:"acknowledged_at"`
	AckFirstName   string     `bun:"ack_first_name"`
	AckLastName    string     `bun:"ack_last_name"`
}

// LetterChildStatuses returns every reached child with the derived fulfilment
// state of an Elternbrief: who confirmed it for that child and when.
func (p *Projection) LetterChildStatuses(ctx context.Context, schoolID, announcementID int64) ([]*domain.ParentAnnouncementLetterChildStatus, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []letterChildRow
	if err := db.NewRaw(letterChildStatusesSQL,
		schoolID, schoolID, today, today, today, announcementID, schoolID, // reached
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID, // confirmable
		schoolID, announcementID, schoolID, schoolID, // acknowledgement lateral
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent announcement letter child statuses: %w", err)
	}
	statuses := make([]*domain.ParentAnnouncementLetterChildStatus, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, &domain.ParentAnnouncementLetterChildStatus{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName,
			SchoolClass: row.SchoolClass, CanConfirm: row.CanConfirm, AcknowledgedAt: row.AcknowledgedAt,
			AckFirstName: row.AckFirstName, AckLastName: row.AckLastName,
		})
	}
	return statuses, nil
}

// deliveryRecipientsSQL returns every guardian linked to a reached child with
// NO portal-access filter, so the recipient matrix can show "kein
// Portalzugang" instead of silently omitting the person. has_portal_access is
// aggregated across the reached children: access on any addressed child
// makes the guardian reachable in moto.
const deliveryRecipientsSQL = `
			SELECT gp.id AS guardian_profile_id,
				gp.account_id,
				COALESCE(gp.first_name, '') AS first_name,
				COALESCE(gp.last_name, '')  AS last_name,
				COALESCE(LOWER(BTRIM(gp.email)), '') AS email,
				COALESCE(gp.portal_locale, '') AS portal_locale,
				bool_or(
					sg.permissions @> '{"parent_portal.access": true}'::jsonb
					AND gp.account_id IS NOT NULL
					AND EXISTS (
						SELECT 1 FROM auth.account_tenants act
						WHERE act.account_id = gp.account_id
							AND act.tenant_id = gp.tenant_id
							AND act.status = 'active'
					)
				) AS has_portal_access` + reachedStudentsBound + `
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
			WHERE pt.announcement_id = ? AND pt.tenant_id = ?
			GROUP BY gp.id, gp.account_id, gp.first_name, gp.last_name, gp.email, gp.portal_locale
			ORDER BY LOWER(COALESCE(gp.last_name, '')), LOWER(COALESCE(gp.first_name, '')), gp.id`

type deliveryRecipientRow struct {
	GuardianProfileID int64  `bun:"guardian_profile_id"`
	AccountID         *int64 `bun:"account_id"`
	FirstName         string `bun:"first_name"`
	LastName          string `bun:"last_name"`
	Email             string `bun:"email"`
	HasPortalAccess   bool   `bun:"has_portal_access"`
	PortalLocale      string `bun:"portal_locale"`
}

// ResolveDeliveryRecipients returns every guardian linked to a child the
// announcement's student-based targets reach, including those without an
// address or portal access.
func (p *Projection) ResolveDeliveryRecipients(ctx context.Context, schoolID, announcementID int64) ([]*domain.ParentAnnouncementDeliveryRecipient, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []deliveryRecipientRow
	if err := db.NewRaw(deliveryRecipientsSQL,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("resolve parent announcement delivery recipients: %w", err)
	}
	recipients := make([]*domain.ParentAnnouncementDeliveryRecipient, 0, len(rows))
	for _, row := range rows {
		recipients = append(recipients, &domain.ParentAnnouncementDeliveryRecipient{
			GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID, FirstName: row.FirstName,
			LastName: row.LastName, Email: row.Email, HasPortalAccess: row.HasPortalAccess, PortalLocale: row.PortalLocale,
		})
	}
	return recipients, nil
}

// audienceRecipientsSQL lists the guardian ACCOUNTS an announcement reaches,
// each with a display name and the account's read/ack state; an account
// reachable through several children or paths collapses to one row.
const audienceRecipientsSQL = pendingApplicantsCTE + `
			SELECT acc.account_id,
				min(acc.first_name) AS first_name,
				min(acc.last_name) AS last_name,
				COALESCE(min(locale_gp.portal_locale), 'de') AS portal_locale,
				par.read_at AS read_at,
				par.acknowledged_at AS acknowledged_at
			FROM (
				SELECT gp.account_id,
					COALESCE(gp.first_name, '') AS first_name, COALESCE(gp.last_name, '') AS last_name` + reachedStudentsBound + portalGuardiansBound + announcementTargetsBound + `

				UNION

				SELECT COALESCE(req.guardian_account_id, ea.id) AS account_id,
					COALESCE(req.guardian_first_name, '') AS first_name, COALESCE(req.guardian_last_name, '') AS last_name` + pendingApplicantAccountsBound + `
					AND COALESCE(req.guardian_account_id, ea.id) IS NOT NULL
			) acc
			LEFT JOIN users.guardian_profiles locale_gp
				ON locale_gp.account_id = acc.account_id AND locale_gp.tenant_id = ?
			LEFT JOIN users.parent_announcement_reads par
				ON par.announcement_id = ? AND par.account_id = acc.account_id
			GROUP BY acc.account_id, par.read_at, par.acknowledged_at
			ORDER BY last_name ASC, first_name ASC, acc.account_id ASC`

type recipientStatusRow struct {
	AccountID      int64      `bun:"account_id"`
	FirstName      string     `bun:"first_name"`
	LastName       string     `bun:"last_name"`
	PortalLocale   string     `bun:"portal_locale"`
	ReadAt         *time.Time `bun:"read_at"`
	AcknowledgedAt *time.Time `bun:"acknowledged_at"`
}

// AudienceRecipients returns the guardian accounts an announcement currently
// reaches with name and read/ack state, ordered by name.
func (p *Projection) AudienceRecipients(ctx context.Context, schoolID, announcementID int64) ([]*domain.ParentAnnouncementRecipientStatus, error) {
	applicants, err := p.pendingApplicants(ctx)
	if err != nil {
		return nil, err
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []recipientStatusRow
	if err := db.NewRaw(audienceRecipientsSQL,
		applicants,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
		announcementID, schoolID,
		schoolID,       // guardian locale
		announcementID, // reads join
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("resolve parent announcement audience recipients: %w", err)
	}
	statuses := make([]*domain.ParentAnnouncementRecipientStatus, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, &domain.ParentAnnouncementRecipientStatus{
			AccountID: row.AccountID, FirstName: row.FirstName, LastName: row.LastName,
			PortalLocale: row.PortalLocale, ReadAt: row.ReadAt, AcknowledgedAt: row.AcknowledgedAt,
		})
	}
	return statuses, nil
}

// reachableGuardiansSQL mirrors the student branch of the audience for a set
// of children that no announcement targets yet.
const reachableGuardiansSQL = `
			SELECT COUNT(DISTINCT gp.account_id)
			FROM users.students s
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id IS NOT NULL
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE s.tenant_id = ? AND s.id IN (?) AND s.status <> 'alumnus'`

// CountReachableGuardiansForStudents counts the distinct guardian accounts a
// student-targeted announcement for the given children would reach right now.
func (p *Projection) CountReachableGuardiansForStudents(ctx context.Context, schoolID int64, studentIDs []int64) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	if err := db.NewRaw(reachableGuardiansSQL,
		schoolID, schoolID, schoolID, bun.List(studentIDs),
	).Scan(ctx, &count); err != nil {
		return 0, fmt.Errorf("count reachable guardians for students: %w", err)
	}
	return count, nil
}

// feedSQL lists published, active, unexpired announcements the account is
// targeted by across the given schools, newest-published first, with the
// account's read/ack state.
const feedSQL = pendingApplicantsFeedCTE + `
			SELECT a.id, a.tenant_id, a.title, a.body, a.priority, a.link_url,
				a.requires_acknowledgement, a.published_at, a.expires_at,
				a.response_type, a.response_deadline, a.delivery_mode, a.system_kind,
				par.read_at AS read_at,
				par.acknowledged_at AS acknowledged_at
			FROM users.parent_announcements a
			LEFT JOIN users.parent_announcement_reads par
				ON par.announcement_id = a.id AND par.account_id = ?
			WHERE ` + feedScopePredicate + liveAnnouncementPredicate + `
				AND ` + reachedAccountFeed + `
			ORDER BY a.published_at DESC, a.id DESC`

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
	ReadAt                  *time.Time `bun:"read_at"`
	AcknowledgedAt          *time.Time `bun:"acknowledged_at"`
}

// ListFeedForAccount returns the guardian's cross-school feed. Run it under
// the administrative transaction with the explicit school scope.
func (p *Projection) ListFeedForAccount(ctx context.Context, accountID int64, scope domain.ParentAnnouncementFeedScope) ([]*domain.ParentAnnouncementFeedItem, error) {
	if scope.IsEmpty() {
		return []*domain.ParentAnnouncementFeedItem{}, nil
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
		today, today, today, accountID, accountID, accountID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list parent announcement feed: %w", err)
	}
	items := make([]*domain.ParentAnnouncementFeedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, &domain.ParentAnnouncementFeedItem{
			ID: row.ID, TenantID: row.TenantID, Title: row.Title, Body: row.Body, Priority: row.Priority,
			LinkURL: row.LinkURL, RequiresAcknowledgement: row.RequiresAcknowledgement,
			PublishedAt: row.PublishedAt, ExpiresAt: row.ExpiresAt, ResponseType: row.ResponseType,
			ResponseDeadline: row.ResponseDeadline, DeliveryMode: row.DeliveryMode, SystemKind: row.SystemKind,
			ReadAt: row.ReadAt, AcknowledgedAt: row.AcknowledgedAt,
		})
	}
	return items, nil
}

func scopeSchools(scope domain.ParentAnnouncementFeedScope) []int64 {
	ids := make([]int64, 0, len(scope.TenantIDs)+len(scope.SystemOnlyTenantIDs))
	ids = append(ids, scope.TenantIDs...)
	return append(ids, scope.SystemOnlyTenantIDs...)
}

// countOutstandingSQL counts live announcements that still need attention:
// unread, pending confirmation, or an open poll with an unanswered child.
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
				)
				AND ` + reachedAccountFeed

// CountOutstandingForAccount counts the account's live announcements that
// still need attention across the given schools.
func (p *Projection) CountOutstandingForAccount(ctx context.Context, accountID int64, scope domain.ParentAnnouncementFeedScope) (int, error) {
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
		today, today, today, accountID,
		today, today, today, accountID, accountID, accountID,
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
func (p *Projection) Stats(ctx context.Context, schoolID, announcementID int64) (*domain.ParentAnnouncementStats, error) {
	applicants, err := p.pendingApplicants(ctx)
	if err != nil {
		return nil, err
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	stats := &domain.ParentAnnouncementStats{}
	if err := db.NewRaw(statsSQL,
		applicants,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
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
			JOIN users.students s ON s.tenant_id = a.tenant_id AND (` + studentTargetMatchFeed + `
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
				AND s.status <> 'alumnus'
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = a.tenant_id
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

type pollChildRow struct {
	AnnouncementID  int64   `bun:"announcement_id"`
	StudentID       int64   `bun:"student_id"`
	FirstName       string  `bun:"first_name"`
	LastName        string  `bun:"last_name"`
	SelectedOptions []int64 `bun:"selected_options,array"`
}

// AnswerableChildren returns, for each announcement, the children of the
// account the poll reaches with the option ids selected for each child.
func (p *Projection) AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*domain.ParentAnnouncementPollChild, error) {
	if len(announcementIDs) == 0 {
		return []*domain.ParentAnnouncementPollChild{}, nil
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []pollChildRow
	if err := db.NewRaw(answerableChildrenSQL,
		today, today, today, accountID, bun.List(announcementIDs),
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list answerable children for parent announcements: %w", err)
	}
	children := make([]*domain.ParentAnnouncementPollChild, 0, len(rows))
	for _, row := range rows {
		children = append(children, &domain.ParentAnnouncementPollChild{
			AnnouncementID: row.AnnouncementID, StudentID: row.StudentID, FirstName: row.FirstName,
			LastName: row.LastName, SelectedOptions: row.SelectedOptions,
		})
	}
	return children, nil
}

// accountMayAnswerSQL is "the child is in the poll's live audience AND the
// account is a poll-enabled guardian of exactly that child". Both halves come
// from one audience query, so a guardian can never answer for a child the
// poll does not reach, nor for a reached child that is not theirs.
const accountMayAnswerSQL = `SELECT EXISTS (SELECT 1 FROM (` + pollAnswerableStudentsForAccountBound + `) reached WHERE reached.student_id = ?)`

// AccountMayAnswerForStudent reports whether accountID may answer a poll for
// studentID right now.
func (p *Projection) AccountMayAnswerForStudent(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (bool, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return false, err
	}
	today := p.day()
	var allowed bool
	if err := db.NewRaw(accountMayAnswerSQL,
		schoolID, schoolID, today, today, today, schoolID, schoolID, accountID, announcementID, schoolID,
		studentID,
	).Scan(ctx, &allowed); err != nil {
		return false, fmt.Errorf("check parent announcement answer permission: %w", err)
	}
	return allowed, nil
}

// holdAnswerPermissionSQL is accountMayAnswerSQL as a share lock: it returns
// one relationship row that authorizes the answer and holds the guardian
// link, profile, and membership rows it was derived from until the caller's
// transaction ends. A revocation of parent_portal.poll.response, the child
// link, or the school membership therefore waits behind the answer write
// (or, if it committed first, is seen here), which closes the window between
// this check and the write the old single-statement guard closed by joining
// the rows inside the write itself.
//
// Bind order: school, school, today, today, today, school, school, account,
// announcement, school, student.
const holdAnswerPermissionSQL = `
			SELECT sg.id` + reachedStudentsBound + pollGuardiansForAccountBound + announcementTargetsBound + `
				AND s.id = ?
			LIMIT 1
			FOR SHARE OF sg, gp, act`

// HoldAnswerPermission reports whether accountID may answer the poll for
// studentID right now and, when it may, keeps that permission from changing
// underneath the caller's transaction. Call it inside the transaction that
// writes the answer.
func (p *Projection) HoldAnswerPermission(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (bool, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return false, err
	}
	today := p.day()
	var relationshipID int64
	err = db.NewRaw(holdAnswerPermissionSQL,
		schoolID, schoolID, today, today, today, schoolID, schoolID, accountID, announcementID, schoolID,
		studentID,
	).Scan(ctx, &relationshipID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("hold parent announcement answer permission: %w", err)
	}
	return true, nil
}

const pollCountsSQL = `WITH target_audience AS (` + pollAudienceStudentsBound + `),
			answerable_audience AS (` + pollAnswerableStudentsBound + `)
			SELECT
				(SELECT COUNT(*) FROM target_audience) AS target_child_count,
				(SELECT COUNT(*) FROM answerable_audience) AS child_count,
				(SELECT COUNT(DISTINCT resp.student_id)
					FROM users.parent_announcement_responses resp
					WHERE resp.announcement_id = ? AND resp.tenant_id = ?
						AND resp.student_id IN (SELECT student_id FROM answerable_audience)) AS answered_count`

const pollOptionResultsSQL = `WITH audience AS (` + pollAnswerableStudentsBound + `)
			SELECT o.id AS option_id, o.label, o.position,
				COUNT(DISTINCT resp.student_id) AS count
			FROM users.parent_announcement_options o
			LEFT JOIN users.parent_announcement_responses resp
				ON resp.option_id = o.id AND resp.announcement_id = o.announcement_id
				AND resp.student_id IN (SELECT student_id FROM audience)
			WHERE o.announcement_id = ? AND o.tenant_id = ?
			GROUP BY o.id, o.label, o.position
			ORDER BY o.position ASC, o.id ASC`

type pollOptionResultRow struct {
	OptionID int64  `bun:"option_id"`
	Label    string `bun:"label"`
	Position int    `bun:"position"`
	Count    int    `bun:"count"`
}

// PollResults returns the per-option tally plus how many answerable children
// the poll reaches and how many of them have answered. Answers are
// intersected with the CURRENT answerable audience, so a child who left the
// school or lost response permission stops counting toward completion.
func (p *Projection) PollResults(ctx context.Context, schoolID, announcementID int64) (*domain.ParentAnnouncementPollResults, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	results := &domain.ParentAnnouncementPollResults{}
	if err := db.NewRaw(pollCountsSQL,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
		announcementID, schoolID,
	).Scan(ctx, &results.TargetChildCount, &results.ChildCount, &results.AnsweredCount); err != nil {
		return nil, fmt.Errorf("parent announcement poll counts: %w", err)
	}
	var rows []pollOptionResultRow
	if err := db.NewRaw(pollOptionResultsSQL,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
		announcementID, schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent announcement poll option results: %w", err)
	}
	results.Options = make([]*domain.ParentAnnouncementPollOptionResult, 0, len(rows))
	for _, row := range rows {
		results.Options = append(results.Options, &domain.ParentAnnouncementPollOptionResult{
			OptionID: row.OptionID, Label: row.Label, Position: row.Position, Count: row.Count,
		})
	}
	return results, nil
}

const pollChildrenSQL = `WITH audience AS (` + pollAudienceStudentsBound + `),
			answerable_audience AS (` + pollAnswerableStudentsBound + `)
			SELECT s.id AS student_id,
				COALESCE(p.first_name, '') AS first_name,
				COALESCE(p.last_name, '') AS last_name,
				COALESCE(s.school_class, '') AS school_class,
				COALESCE(ans.labels, ARRAY[]::text[]) AS answer_labels,
				ans.responded_at AS responded_at,
				EXISTS (SELECT 1 FROM answerable_audience aa WHERE aa.student_id = s.id) AS can_answer
			FROM audience
			JOIN users.students s ON s.id = audience.student_id
			JOIN users.persons p ON p.id = s.person_id
			LEFT JOIN LATERAL (
				SELECT array_agg(o.label ORDER BY o.position) AS labels,
					max(resp.responded_at) AS responded_at
				FROM users.parent_announcement_responses resp
				JOIN users.parent_announcement_options o ON o.id = resp.option_id
				WHERE resp.announcement_id = ? AND resp.tenant_id = ? AND resp.student_id = s.id
			) ans ON TRUE
			ORDER BY last_name ASC, first_name ASC, student_id ASC`

type pollChildStatusRow struct {
	StudentID    int64      `bun:"student_id"`
	FirstName    string     `bun:"first_name"`
	LastName     string     `bun:"last_name"`
	SchoolClass  string     `bun:"school_class"`
	AnswerLabels []string   `bun:"answer_labels,array"`
	RespondedAt  *time.Time `bun:"responded_at"`
	CanAnswer    bool       `bun:"can_answer"`
}

// PollChildren returns every portal-visible child the poll reaches with the
// option labels answered for them and whether someone may currently answer.
func (p *Projection) PollChildren(ctx context.Context, schoolID, announcementID int64) ([]*domain.ParentAnnouncementPollChildStatus, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []pollChildStatusRow
	if err := db.NewRaw(pollChildrenSQL,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
		announcementID, schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent announcement poll children: %w", err)
	}
	children := make([]*domain.ParentAnnouncementPollChildStatus, 0, len(rows))
	for _, row := range rows {
		children = append(children, &domain.ParentAnnouncementPollChildStatus{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName,
			SchoolClass: row.SchoolClass, AnswerLabels: row.AnswerLabels, RespondedAt: row.RespondedAt,
			CanAnswer: row.CanAnswer,
		})
	}
	return children, nil
}

// reminderRecipientColumns is the per-person shape of both reminder lists: one
// row per account, because the reminder is one e-mail per person, not per
// child.
const reminderRecipientColumns = `
			SELECT gp.account_id, COALESCE(min(lower(gp.email)), '') AS email,
				COALESCE(min(gp.first_name), '') AS first_name,
				COALESCE(min(gp.last_name), '') AS last_name,
				COALESCE(min(gp.portal_locale), 'de') AS portal_locale`

// unansweredRecipientsSQL lists the guardians of reached children that have
// NO answer yet.
const unansweredRecipientsSQL = reminderRecipientColumns + reachedStudentsBound + pollGuardiansBound + announcementTargetsBound + `
				AND NOT EXISTS (
					SELECT 1 FROM users.parent_announcement_responses resp
					WHERE resp.announcement_id = pt.announcement_id
						AND resp.tenant_id = pt.tenant_id
						AND resp.student_id = s.id
				)
			GROUP BY gp.account_id`

// unacknowledgedRecipientsSQL lists the guardians of reached children for
// which NOBODY has confirmed the Elternbrief yet. It is keyed on the CHILD:
// once any guardian confirms, every guardian of that child drops out.
const unacknowledgedRecipientsSQL = reminderRecipientColumns + reachedStudentsBound + portalGuardiansBound + announcementTargetsBound + `
				AND NOT EXISTS (
					SELECT 1
					FROM users.students_guardians osg
					JOIN users.guardian_profiles ogp ON ogp.id = osg.guardian_profile_id
						AND ogp.tenant_id = pt.tenant_id AND ogp.account_id IS NOT NULL
					JOIN users.parent_announcement_reads par ON par.announcement_id = pt.announcement_id
						AND par.tenant_id = pt.tenant_id AND par.account_id = ogp.account_id
					WHERE osg.student_id = s.id AND osg.tenant_id = pt.tenant_id
						AND osg.permissions @> '{"parent_portal.access": true}'::jsonb
						AND par.acknowledged_at IS NOT NULL
				)
			GROUP BY gp.account_id`

type reminderRecipientRow struct {
	AccountID    int64  `bun:"account_id"`
	Email        string `bun:"email"`
	FirstName    string `bun:"first_name"`
	LastName     string `bun:"last_name"`
	PortalLocale string `bun:"portal_locale"`
}

// UnansweredReminderRecipients returns the distinct guardians of reached
// children that have no poll answer yet.
func (p *Projection) UnansweredReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*domain.ParentAnnouncementReminderRecipient, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []reminderRecipientRow
	if err := db.NewRaw(unansweredRecipientsSQL,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent announcement poll reminder recipients: %w", err)
	}
	return reminderRecipients(rows), nil
}

// UnacknowledgedReminderRecipients returns the guardians of reached children
// for which nobody has confirmed the Elternbrief yet.
func (p *Projection) UnacknowledgedReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*domain.ParentAnnouncementReminderRecipient, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []reminderRecipientRow
	if err := db.NewRaw(unacknowledgedRecipientsSQL,
		schoolID, schoolID, today, today, today, schoolID, schoolID, announcementID, schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent announcement letter reminder recipients: %w", err)
	}
	return reminderRecipients(rows), nil
}

func reminderRecipients(rows []reminderRecipientRow) []*domain.ParentAnnouncementReminderRecipient {
	recipients := make([]*domain.ParentAnnouncementReminderRecipient, 0, len(rows))
	for _, row := range rows {
		recipients = append(recipients, &domain.ParentAnnouncementReminderRecipient{
			AccountID: row.AccountID, Email: row.Email, FirstName: row.FirstName,
			LastName: row.LastName, PortalLocale: row.PortalLocale,
		})
	}
	return recipients
}
