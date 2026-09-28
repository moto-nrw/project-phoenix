package parentaudience

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

// Erklärungen (#3430). Who a declaration reaches and who may declare for each
// reached child. A signer needs a linked account, an active membership at the
// school, parent_portal.access AND parent_portal.declarations.submit on the
// relationship to exactly that child: membership or a guardian link alone
// never makes a signer.

// declarationSignerPermissions is the jsonb a signer's relationship holds.
const declarationSignerPermissions = `'{"parent_portal.access": true, "parent_portal.declarations.submit": true}'::jsonb`

// declarationChildrenSQL lists every child the announcement reaches, with no
// guardian requirement: a child nobody may declare for must stay visible.
//
// Bind order: school, school, today, today, today, announcement, school,
// historical submission links.
const declarationChildrenSQL = `WITH reached AS (
			` + letterReachedStudentsBound + `
			UNION
			SELECT DISTINCT history.student_id
			FROM jsonb_to_recordset(?::jsonb) AS history(student_id bigint)
		)
			SELECT s.id AS student_id,
				COALESCE(p.first_name, '') AS first_name,
				COALESCE(p.last_name, '')  AS last_name,
				COALESCE(sm.school_class, '') AS school_class
			FROM reached
			JOIN users.student_profiles s ON s.id = reached.student_id` + studentMembershipJoins + `
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			ORDER BY last_name ASC, first_name ASC, student_id ASC`

type declarationChildRow struct {
	AnnouncementID int64  `bun:"announcement_id"`
	StudentID      int64  `bun:"student_id"`
	FirstName      string `bun:"first_name"`
	LastName       string `bun:"last_name"`
	SchoolClass    string `bun:"school_class"`
}

func declarationChildren(rows []declarationChildRow) []*domain.DeclarationChild {
	out := make([]*domain.DeclarationChild, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.DeclarationChild{
			AnnouncementID: row.AnnouncementID, StudentID: row.StudentID, FirstName: row.FirstName,
			LastName: row.LastName, SchoolClass: row.SchoolClass,
		})
	}
	return out
}

// DeclarationChildren returns every child the declaration reaches.
func (p *Projection) DeclarationChildren(ctx context.Context, schoolID, announcementID int64, history []domain.DeclarationHistoryLink) ([]*domain.DeclarationChild, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	historyJSON, err := declarationHistoryJSON(history)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []declarationChildRow
	if err := db.NewRaw(declarationChildrenSQL,
		schoolID, schoolID, today, today, today, announcementID, schoolID, historyJSON,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent declaration children: %w", err)
	}
	for i := range rows {
		rows[i].AnnouncementID = announcementID
	}
	return declarationChildren(rows), nil
}

// declarationSignersSQL lists the guardians who may declare for the given
// children. A zero school leaves the school to the relationship row (the
// cross-school feed); a positive one binds it.
//
// Bind order: guardian links, students.
const declarationSignersSQL = `
			SELECT sg.student_id, gp.id AS guardian_profile_id, gp.account_id,
				COALESCE(gp.first_name, '') AS first_name,
				COALESCE(gp.last_name, '') AS last_name,
				COALESCE(lower(gp.email), '') AS email,
				COALESCE(gp.portal_locale, 'de') AS portal_locale,
				COALESCE(sg.guardian_role, '') AS guardian_role
			FROM (?) sg
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = sg.tenant_id
				AND gp.account_id IS NOT NULL
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE sg.student_id IN (?)
				AND sg.permissions @> ` + declarationSignerPermissions + `
			ORDER BY sg.student_id, gp.last_name, gp.first_name, gp.id`

type declarationSignerRow struct {
	StudentID         int64  `bun:"student_id"`
	GuardianProfileID int64  `bun:"guardian_profile_id"`
	AccountID         int64  `bun:"account_id"`
	FirstName         string `bun:"first_name"`
	LastName          string `bun:"last_name"`
	Email             string `bun:"email"`
	PortalLocale      string `bun:"portal_locale"`
	GuardianRole      string `bun:"guardian_role"`
}

// DeclarationSigners returns the guardians who may declare for the children
// of one school.
func (p *Projection) DeclarationSigners(ctx context.Context, schoolID int64, studentIDs []int64) ([]*domain.DeclarationSigner, error) {
	return p.declarationSigners(ctx, schoolID, studentIDs)
}

// DeclarationSignersForStudents is DeclarationSigners for children of several
// schools; the ids come from an already authorized feed.
func (p *Projection) DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*domain.DeclarationSigner, error) {
	return p.declarationSigners(ctx, 0, studentIDs)
}

func (p *Projection) declarationSigners(ctx context.Context, schoolID int64, studentIDs []int64) ([]*domain.DeclarationSigner, error) {
	if len(studentIDs) == 0 {
		return []*domain.DeclarationSigner{}, nil
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []declarationSignerRow
	if err := db.NewRaw(declarationSignersSQL, guardianLinks(db, schoolID), bun.List(studentIDs)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent declaration signers: %w", err)
	}
	out := make([]*domain.DeclarationSigner, 0, len(rows))
	for _, row := range rows {
		signer := domain.DeclarationSigner(row)
		out = append(out, &signer)
	}
	return out, nil
}

// declarationChildrenForAccountSQL is, per declaration, the account's
// children the declaration reaches, whether or not the account may declare
// for them: a pickup-only guardian still sees the Erklärung.
//
// Bind order: today, today, today, guardian links, account, announcements,
// historical submission links, guardian links, account, announcements.
const declarationChildrenForAccountSQL = `
			SELECT DISTINCT a.id AS announcement_id, s.id AS student_id,
				COALESCE(p.first_name, '') AS first_name,
				COALESCE(p.last_name, '') AS last_name,
				COALESCE(sm.school_class, '') AS school_class
			FROM users.parent_announcements a
			JOIN users.parent_announcement_targets pt
				ON pt.announcement_id = a.id AND pt.tenant_id = a.tenant_id
			JOIN users.student_profiles s ON s.tenant_id = a.tenant_id` + studentMembershipJoins + ` AND (` + studentTargetMatchFeed + `
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
				AND sm.status <> 'alumnus'
			JOIN (?) sg ON sg.student_id = s.id AND sg.tenant_id = a.tenant_id
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = a.tenant_id
				AND gp.account_id = ?
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE a.id IN (?)

			UNION

			SELECT DISTINCT a.id AS announcement_id, s.id AS student_id,
				COALESCE(p.first_name, '') AS first_name,
				COALESCE(p.last_name, '') AS last_name,
				COALESCE(sm.school_class, '') AS school_class
			FROM users.parent_announcements a
			JOIN jsonb_to_recordset(?::jsonb) AS history(tenant_id bigint, announcement_id bigint, student_id bigint)
				ON history.announcement_id = a.id AND history.tenant_id = a.tenant_id
			JOIN users.student_profiles s ON s.id = history.student_id AND s.tenant_id = a.tenant_id` + studentMembershipJoins + `
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
				AND sm.status <> 'alumnus'
			JOIN (?) sg ON sg.student_id = s.id AND sg.tenant_id = a.tenant_id
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = a.tenant_id
				AND gp.account_id = ?
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE a.id IN (?) AND a.delivery_mode = 'declaration'
			ORDER BY last_name ASC, first_name ASC, student_id ASC`

// DeclarationChildrenForAccount returns the account's reached children per
// declaration announcement.
func (p *Projection) DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64, history []domain.DeclarationHistoryLink) ([]*domain.DeclarationChild, error) {
	if len(announcementIDs) == 0 {
		return []*domain.DeclarationChild{}, nil
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	historyJSON, err := declarationHistoryJSON(history)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []declarationChildRow
	if err := db.NewRaw(declarationChildrenForAccountSQL,
		today, today, today, guardianLinks(db, 0), accountID, bun.List(announcementIDs),
		historyJSON, guardianLinks(db, 0), accountID, bun.List(announcementIDs),
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent declaration children for account: %w", err)
	}
	return declarationChildren(rows), nil
}

// holdDeclarationSignerSQL returns the relationship that authorizes the
// account to declare for the child right now: the child is reached, the link
// carries both permissions, the account is linked and active. FOR SHARE keeps
// a concurrent revocation of the permission, the link or the membership
// waiting behind the submission (or makes this read see it).
//
// Bind order: school, school, today, today, today, announcement, school,
// guardian links of the school, school, account, student.
const holdDeclarationSignerSQL = `
			SELECT gp.id AS guardian_profile_id,
				COALESCE(gp.first_name, '') AS first_name,
				COALESCE(gp.last_name, '') AS last_name,
				COALESCE(sg.guardian_role, '') AS guardian_role` + reachedStudentsBound + `
			JOIN (?) sg ON sg.student_id = s.id
				AND sg.permissions @> ` + declarationSignerPermissions + `
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id = ?
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE s.id = ?
			ORDER BY gp.id
			LIMIT 1
			FOR SHARE OF sg, gp, act`

type declarationSignerContextRow struct {
	GuardianProfileID int64  `bun:"guardian_profile_id"`
	FirstName         string `bun:"first_name"`
	LastName          string `bun:"last_name"`
	GuardianRole      string `bun:"guardian_role"`
}

// HoldDeclarationSigner reads and share-locks the authorizing relationship,
// or returns nil when the account may not declare for the child.
func (p *Projection) HoldDeclarationSigner(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (*domain.DeclarationSignerContext, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var row declarationSignerContextRow
	err = db.NewRaw(holdDeclarationSignerSQL,
		schoolID, schoolID, today, today, today, announcementID, schoolID, guardianLinks(db, schoolID), schoolID, accountID,
		studentID,
	).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("hold parent declaration signer: %w", err)
	}
	signer := domain.DeclarationSignerContext(row)
	return &signer, nil
}

// holdHistoricalDeclarationSignerSQL keeps a consent withdrawable after an
// activity enrollment ended. The owner store already verified the account's
// submission; this query keeps current portal access to the relationship locked.
//
// Bind order: guardian links of the school, school, account, student.
const holdHistoricalDeclarationSignerSQL = `
			SELECT gp.id AS guardian_profile_id,
				COALESCE(gp.first_name, '') AS first_name,
				COALESCE(gp.last_name, '') AS last_name,
				COALESCE(sg.guardian_role, '') AS guardian_role
			FROM (?) sg
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id = ?
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
		WHERE sg.student_id = ?
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			ORDER BY gp.id
			LIMIT 1
			FOR SHARE OF sg, gp, act`

// HoldHistoricalDeclarationSigner reads and share-locks the current
// relationship for an account's own historical declaration. It deliberately
// does not re-evaluate the activity enrollment: that check would make a
// recorded consent impossible to revoke after the activity has ended.
func (p *Projection) HoldHistoricalDeclarationSigner(ctx context.Context, schoolID, accountID, studentID int64) (*domain.DeclarationSignerContext, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var row declarationSignerContextRow
	err = db.NewRaw(holdHistoricalDeclarationSignerSQL,
		guardianLinks(db, schoolID), schoolID, accountID, studentID,
	).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("hold historical parent declaration signer: %w", err)
	}
	signer := domain.DeclarationSignerContext(row)
	return &signer, nil
}

func declarationHistoryJSON(history []domain.DeclarationHistoryLink) (string, error) {
	encoded, err := json.Marshal(history)
	if err != nil {
		return "", fmt.Errorf("encode declaration history: %w", err)
	}
	return string(encoded), nil
}

const historicalDeclarationAccessSQL = `
		WITH history AS (
			SELECT * FROM jsonb_to_recordset(?::jsonb) AS history(tenant_id bigint, announcement_id bigint, student_id bigint)
		)
		SELECT EXISTS (
			SELECT 1
			FROM history
			JOIN users.parent_announcements a ON a.id = history.announcement_id AND a.tenant_id = history.tenant_id
			JOIN (?) sg ON sg.student_id = history.student_id AND sg.tenant_id = history.tenant_id
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = history.tenant_id
				AND gp.account_id = ?
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE history.tenant_id = ? AND a.delivery_mode = 'declaration'
		)`

// HistoricalDeclarationAccess reports whether the account still has portal
// access to a child from its own preserved declaration history.
func (p *Projection) HistoricalDeclarationAccess(ctx context.Context, schoolID, accountID int64, history []domain.DeclarationHistoryLink) (bool, error) {
	if len(history) == 0 {
		return false, nil
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return false, err
	}
	historyJSON, err := declarationHistoryJSON(history)
	if err != nil {
		return false, err
	}
	var allowed bool
	if err := db.NewRaw(historicalDeclarationAccessSQL, historyJSON, guardianLinks(db, schoolID), accountID, schoolID).Scan(ctx, &allowed); err != nil {
		return false, fmt.Errorf("check historical parent declaration access: %w", err)
	}
	return allowed, nil
}

const historicalDeclarationFeedSQL = `
		WITH history AS (
			SELECT * FROM jsonb_to_recordset(?::jsonb) AS history(tenant_id bigint, announcement_id bigint, student_id bigint)
		)
		SELECT DISTINCT a.id, a.tenant_id, a.title, a.body, a.priority, a.link_url,
			a.requires_acknowledgement, a.published_at, a.expires_at,
			a.response_type, a.response_deadline, a.delivery_mode, a.system_kind,
			a.reminder_sent_at,
			CASE WHEN a.reminder_sent_at IS NOT NULL THEN a.reminder_text END AS reminder_text,
			par.read_at AS read_at,
			par.acknowledged_at AS acknowledged_at
		FROM history
		JOIN users.parent_announcements a ON a.id = history.announcement_id AND a.tenant_id = history.tenant_id
		LEFT JOIN users.parent_announcement_reads par
			ON par.announcement_id = a.id AND par.account_id = ?
		JOIN (?) sg ON sg.student_id = history.student_id AND sg.tenant_id = a.tenant_id
			AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
		JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = a.tenant_id
			AND gp.account_id = ?
		JOIN auth.account_tenants act ON act.account_id = gp.account_id
			AND act.tenant_id = gp.tenant_id AND act.status = 'active'
		WHERE ` + feedScopePredicate + liveAnnouncementPredicate + `
			AND a.delivery_mode = 'declaration'
		ORDER BY a.published_at DESC, a.id DESC`

// HistoricalDeclarationFeed returns the current, portal-accessible declaration
// announcements the account has previously submitted for but no longer reaches
// through an activity enrollment.
func (p *Projection) HistoricalDeclarationFeed(ctx context.Context, accountID int64, scope domain.ParentAnnouncementFeedScope, history []domain.DeclarationHistoryLink) ([]*domain.ParentAnnouncementFeedItem, error) {
	if scope.IsEmpty() || len(history) == 0 {
		return []*domain.ParentAnnouncementFeedItem{}, nil
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	historyJSON, err := declarationHistoryJSON(history)
	if err != nil {
		return nil, err
	}
	var rows []feedRow
	if err := db.NewRaw(historicalDeclarationFeedSQL,
		historyJSON, accountID, guardianLinks(db, 0), accountID,
		feedScopeList(scope.TenantIDs), feedScopeList(scope.SystemOnlyTenantIDs),
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list historical parent declaration feed: %w", err)
	}
	items := make([]*domain.ParentAnnouncementFeedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, &domain.ParentAnnouncementFeedItem{
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
