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
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// Database resolves the caller's ambient transaction plus the tenant the
// statement must be scoped to. A zero tenant means administrative or
// cross-tenant work, where the query relies on its explicit school filter.
type Database func(context.Context) (bun.IDB, int64, error)

// Projection answers every parent-announcement read that spans owners.
type Projection struct {
	database   Database
	applicants PendingApplicants
	links      GuardianLinks
	today      func() calendar.Date
}

// New builds the projection over the ambient transaction runtime and the
// guardian-link row source. The optional clock fixes the calendar day the
// activity-group targets are evaluated on; production uses Berlin's current
// day.
func New(database Database, applicants PendingApplicants, links GuardianLinks, clocks ...func() time.Time) *Projection {
	if database == nil {
		panic("communication parent audience: database runtime is required")
	}
	if applicants == nil {
		panic("communication parent audience: pending applicants are required")
	}
	if links == nil {
		panic("communication parent audience: guardian links are required")
	}
	return &Projection{database: database, applicants: applicants, links: links, today: calendar.CalendarDateClock(clocks...)}
}

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
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
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
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID, accountID,
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
			JOIN (?) sg ON sg.student_id = s.id
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id IS NOT NULL
				AND gp.email IS NOT NULL AND length(btrim(gp.email)) > 0
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'

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
func (p *Projection) ResolveAudienceEmails(ctx context.Context, schoolID, announcementID int64) ([]*contract.ParentAnnouncementRecipient, error) {
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
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
		announcementID, schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("resolve parent announcement audience emails: %w", err)
	}
	recipients := make([]*contract.ParentAnnouncementRecipient, 0, len(rows))
	for _, row := range rows {
		recipients = append(recipients, &contract.ParentAnnouncementRecipient{
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
				COALESCE(sm.school_class, '') AS school_class,
				EXISTS (SELECT 1 FROM confirmable c WHERE c.student_id = s.id) AS can_confirm,
				ack.acknowledged_at AS acknowledged_at,
				COALESCE(ack.first_name, '') AS ack_first_name,
				COALESCE(ack.last_name, '')  AS ack_last_name
			FROM reached
			JOIN users.student_profiles s ON s.id = reached.student_id` + studentMembershipJoins + `
			JOIN users.persons p ON p.id = s.person_id
			LEFT JOIN LATERAL (
				SELECT par.acknowledged_at, gp.first_name, gp.last_name
				FROM (?) sg
				JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id
					AND gp.tenant_id = ? AND gp.account_id IS NOT NULL
				JOIN users.parent_announcement_reads par ON par.announcement_id = ?
					AND par.tenant_id = ? AND par.account_id = gp.account_id
				WHERE sg.student_id = s.id
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
func (p *Projection) LetterChildStatuses(ctx context.Context, schoolID, announcementID int64) ([]*contract.ParentAnnouncementLetterChildStatus, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []letterChildRow
	if err := db.NewRaw(letterChildStatusesSQL,
		schoolID, schoolID, today, today, today, announcementID, schoolID, // reached
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID, // confirmable
		p.guardianLinks(db, schoolID), schoolID, announcementID, schoolID, // acknowledgement lateral
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent announcement letter child statuses: %w", err)
	}
	statuses := make([]*contract.ParentAnnouncementLetterChildStatus, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, &contract.ParentAnnouncementLetterChildStatus{
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
			JOIN (?) sg ON sg.student_id = s.id
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
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
func (p *Projection) ResolveDeliveryRecipients(ctx context.Context, schoolID, announcementID int64) ([]*contract.ParentAnnouncementDeliveryRecipient, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []deliveryRecipientRow
	if err := db.NewRaw(deliveryRecipientsSQL,
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("resolve parent announcement delivery recipients: %w", err)
	}
	recipients := make([]*contract.ParentAnnouncementDeliveryRecipient, 0, len(rows))
	for _, row := range rows {
		recipients = append(recipients, &contract.ParentAnnouncementDeliveryRecipient{
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
					COALESCE(gp.first_name, '') AS first_name, COALESCE(gp.last_name, '') AS last_name` + reachedStudentsBound + portalGuardiansBound + `

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
func (p *Projection) AudienceRecipients(ctx context.Context, schoolID, announcementID int64) ([]*contract.ParentAnnouncementRecipientStatus, error) {
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
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
		announcementID, schoolID,
		schoolID,       // guardian locale
		announcementID, // reads join
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("resolve parent announcement audience recipients: %w", err)
	}
	statuses := make([]*contract.ParentAnnouncementRecipientStatus, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, &contract.ParentAnnouncementRecipientStatus{
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
			FROM users.student_profiles s` + studentMembershipJoins + `
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			JOIN (?) sg ON sg.student_id = s.id
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id IS NOT NULL
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE s.tenant_id = ? AND s.id IN (?) AND sm.status <> 'alumnus'`

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
		p.guardianLinks(db, schoolID), schoolID, schoolID, bun.List(studentIDs),
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
				a.sender_name, a.sender_school_class,
				a.reminder_sent_at,
				CASE WHEN a.reminder_sent_at IS NOT NULL THEN a.reminder_text END AS reminder_text,
				par.read_at AS read_at,
				par.acknowledged_at AS acknowledged_at
			FROM users.parent_announcements a
			LEFT JOIN users.parent_announcement_reads par
				ON par.announcement_id = a.id AND par.account_id = ?
			WHERE ` + feedScopePredicate + liveAnnouncementPredicate + `
			AND ` + reachedAccountFeed + `
			ORDER BY GREATEST(a.published_at, a.reminder_sent_at) DESC, a.id DESC`
