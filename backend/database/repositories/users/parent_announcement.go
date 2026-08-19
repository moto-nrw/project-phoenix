package users

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// ParentAnnouncementRepository is the tenant-scoped data-access layer for
// parent announcements, targets and read/ack state. The CRUD surface uses the
// generic base.Repository; the audience-resolution and feed queries are raw SQL
// because they fan out across users/activities/enrollment to map a target
// selector to the guardian accounts it reaches.
type ParentAnnouncementRepository struct {
	*base.Repository[*users.ParentAnnouncement]
}

// NewParentAnnouncementRepository wires a fresh repository.
func NewParentAnnouncementRepository(db *bun.DB) users.ParentAnnouncementRepository {
	repo := base.NewRepository[*users.ParentAnnouncement](db, "users.parent_announcements", "ParentAnnouncement")
	repo.TenantScoped = true
	return &ParentAnnouncementRepository{Repository: repo}
}

// pendingEnrollmentStatuses are the request-child states that count as an "open
// enrollment" for the pending_enrollment target: submitted or in some stage of
// review/renewal, never a decided (approved/rejected/withdrawn) or waitlisted
// outcome.
var pendingEnrollmentStatuses = []string{
	enrollment.ChildStatusSubmitted,
	enrollment.ChildStatusUnderReview,
	enrollment.ChildStatusPendingRenewal,
	enrollment.ChildStatusAutoRenewed,
	enrollment.ChildStatusPendingAdminReview,
}

// pendingStatusList renders the pending-enrollment statuses as a quoted SQL CSV.
// The values are package constants (never user input), so building the IN list
// by string concat is injection-safe and keeps the only runtime `?` args in the
// audience SQL as the int ids — which makes their positional order easy to keep
// correct.
func pendingStatusList() string {
	quoted := make([]string, len(pendingEnrollmentStatuses))
	for i, s := range pendingEnrollmentStatuses {
		quoted[i] = "'" + s + "'"
	}
	return strings.Join(quoted, ",")
}

// activeActivityGroupExists renders the activity_group branch of a target match:
// the student has an enrollment in pt.target_ref_id that is active TODAY. It is
// the single canonical active-enrollment predicate for the audience/feed/stats
// queries — mirroring activities.StudentEnrollmentRepository.FindActiveByStudentIDs:
// valid_until is EXCLUSIVE (an enrollment ending today is no longer active), and
// weekday is either unscoped or matches today's ISO weekday. Both checks use
// Berlin's calendar date, not the DB session's CURRENT_DATE (which is
// UTC and rolls a day early/late around Berlin midnight). tenantExpr is the SQL
// expression for the tenant (a bound `?` in the raw builders, `a.tenant_id` in
// the feed). The Berlin date is rendered from validated integer fields, so
// inlining it as a literal is injection-safe and — like pendingStatusList — keeps
// the runtime `?` args limited to the ids, preserving their positional order.
func activeActivityGroupExists(tenantExpr string) string {
	today := "'" + timezone.TodayDate().String() + "'"
	return fmt.Sprintf(`(pt.target_type = 'activity_group' AND EXISTS (
						SELECT 1 FROM activities.student_enrollments se
						WHERE se.student_id = s.id AND se.tenant_id = %[1]s
							AND se.activity_group_id = pt.target_ref_id
							AND se.valid_from <= %[2]s
							AND (se.valid_until IS NULL OR se.valid_until > %[2]s)
							AND (se.weekday IS NULL OR se.weekday = EXTRACT(ISODOW FROM DATE %[2]s)::INT)
					))`, tenantExpr, today)
}

// reachedPredicate builds the SQL boolean "guardian account :acc is reached by
// the announcement identified by (annExpr, tenantExpr) right now". annExpr and
// tenantExpr are SQL expressions (a column reference like `a.id`/`a.tenant_id`
// in the feed query, or a bound `?` in the per-announcement check). accPlace is
// the placeholder/expression for the account id. It is the OR-union of the
// student-based targets (school/class/group/AG/student, gated on a
// non-soft-deleted student/person, a live students_guardians link granting
// parent_portal.access AND an ACTIVE auth.account_tenants membership for the
// school — a guardian whose mapping went pending/inactive keeps the
// relationship rows but has lost portal access, so they drop out; mirrors
// parent messaging + parent.ChildRepository) and the guardian-based
// pending_enrollment target.
//
// The pending_enrollment branch matches an open enrollment request to the
// account two ways, mirroring the parent.EnrollmentRequestRepository.ListByAccount
// ownership rule that drives "Meine Anmeldungen" and newsEnabledTenants:
//   - primary: req.guardian_account_id = acc (stamped on a parent-auth submit or
//     backfilled by the invite-accept flow)
//   - fallback: req.guardian_account_id IS NULL AND the request's guardian_email
//     matches the account's e-mail (case/trim-insensitive)
//
// Without the fallback an applicant whose submission was never stamped (e.g. a
// silently-failed invite-accept backfill) sees the school in the feed's tenant
// set yet no announcement, while staff stats/e-mail counted them symmetrically.
func reachedPredicate(annExpr, tenantExpr, accPlace string) string {
	return fmt.Sprintf(`(
		EXISTS (
			SELECT 1
			FROM users.parent_announcement_targets pt
			JOIN users.students s ON s.tenant_id = %[2]s AND (
				pt.target_type = 'school_all'
				OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
				OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
				OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
				OR %[5]s
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			-- Graduated (alumnus) students are soft-deleted: their guardians must
			-- drop out of every announcement audience (reach count, feed
			-- visibility, and e-mails) once the child has left the OGS (#405).
			AND s.status <> 'alumnus'
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = %[2]s
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = %[2]s
				AND gp.account_id = %[3]s
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE pt.announcement_id = %[1]s AND pt.tenant_id = %[2]s
		)
		OR EXISTS (
			SELECT 1
			FROM users.parent_announcement_targets pt
			JOIN enrollment.requests req ON req.tenant_id = %[2]s
				AND req.withdrawn_at IS NULL
				AND (
					req.guardian_account_id = %[3]s
					OR (
						req.guardian_account_id IS NULL
						AND EXISTS (
							SELECT 1 FROM auth.accounts ea
							WHERE ea.id = %[3]s AND ea.email IS NOT NULL
								AND LOWER(TRIM(req.guardian_email)) = LOWER(TRIM(ea.email))
						)
					)
				)
			JOIN enrollment.request_children rc ON rc.request_id = req.id AND rc.tenant_id = %[2]s
				AND rc.status IN (%[4]s)
			WHERE pt.announcement_id = %[1]s AND pt.tenant_id = %[2]s
				AND pt.target_type = 'pending_enrollment'
		)
	)`, annExpr, tenantExpr, accPlace, pendingStatusList(), activeActivityGroupExists(tenantExpr))
}

// FindByID returns the announcement by id (tenant-scoped), or nil when absent.
func (r *ParentAnnouncementRepository) FindByID(ctx context.Context, id int64) (*users.ParentAnnouncement, error) {
	return r.FindByIDOrNil(ctx, id)
}

// Delete removes the announcement by id (targets + reads cascade in the DB).
func (r *ParentAnnouncementRepository) Delete(ctx context.Context, id int64) error {
	return r.Repository.Delete(ctx, id)
}

// ListForTenant returns the tenant's announcements newest-first, each with its
// target rows attached (one batched IN query — the staff list renders the
// audience per row). includeInactive controls whether soft-disabled
// (active=false) rows appear.
func (r *ParentAnnouncementRepository) ListForTenant(ctx context.Context, includeInactive bool) ([]*users.ParentAnnouncement, error) {
	var rows []*users.ParentAnnouncement
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.parent_announcements AS "parent_announcement"`).
		OrderExpr(`"parent_announcement".created_at DESC`)
	if !includeInactive {
		query = query.Where(`"parent_announcement".active`)
	}
	query = base.WithTenantFilter(ctx, query, "parent_announcement")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent announcements for tenant", Err: err}
	}
	if len(rows) == 0 {
		return rows, nil
	}

	ids := make([]int64, 0, len(rows))
	byID := make(map[int64]*users.ParentAnnouncement, len(rows))
	for _, a := range rows {
		ids = append(ids, a.ID)
		byID[a.ID] = a
		a.Targets = []*users.ParentAnnouncementTarget{}
	}
	var targets []*users.ParentAnnouncementTarget
	tq := base.GetDB(ctx, r.DB).NewSelect().
		Model(&targets).
		ModelTableExpr("users.parent_announcement_targets AS pat").
		Where("pat.announcement_id IN (?)", bun.List(ids)).
		OrderExpr("pat.id ASC")
	tq = base.WithTenantFilter(ctx, tq, "pat")
	if err := tq.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent announcement targets for tenant", Err: err}
	}
	for _, t := range targets {
		if a, ok := byID[t.AnnouncementID]; ok {
			a.Targets = append(a.Targets, t)
		}
	}
	return rows, nil
}

// Update writes only the editable content columns of a DRAFT announcement, and
// it is atomic against a concurrent publish. Two guarantees:
//
//  1. It never writes published_at (nor active/created_by), and its WHERE clause
//     requires published_at IS NULL. So if another staff request publishes the
//     row between the service loading it and this write, no row matches and the
//     published wording is left untouched — a stale draft can never silently
//     revert (or un-publish) a live announcement. A non-match returns
//     users.ErrAnnouncementPublished, which the service maps to its
//     published-immutable error.
//
//  2. Editing a draft invalidates any read/ack state left over from a previous
//     publication. The correction path is unpublish -> edit -> republish, and
//     the old read rows belong to the RETRACTED wording; leaving them would show
//     a corrected announcement as already read/acknowledged and inflate the
//     staff stats. It clears the announcement's read rows in the same tenant tx
//     (a no-op for a never-published draft), so a corrected announcement
//     re-appears as unread.
//
// RLS pins both statements to the current tenant (mirrors SetPublished).
func (r *ParentAnnouncementRepository) Update(ctx context.Context, a *users.ParentAnnouncement) error {
	now := time.Now()
	res, err := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*users.ParentAnnouncement)(nil)).
		ModelTableExpr("users.parent_announcements").
		Set("title = ?", a.Title).
		Set("body = ?", a.Body).
		Set("priority = ?", a.Priority).
		Set("link_url = ?", a.LinkURL).
		Set("requires_acknowledgement = ?", a.RequiresAcknowledgement).
		Set("send_email = ?", a.SendEmail).
		Set("expires_at = ?", a.ExpiresAt).
		Set("response_type = ?", a.ResponseType).
		Set("response_deadline = ?", a.ResponseDeadline).
		Set("delivery_mode = ?", a.DeliveryMode).
		Set("email_audience = ?", a.EmailAudience).
		Set("updated_at = ?", now).
		Where("id = ?", a.ID).
		Where("published_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update parent announcement draft", Err: err}
	}
	n, err := res.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{Op: "update parent announcement draft rows affected", Err: err}
	}
	if n == 0 {
		return users.ErrAnnouncementPublished
	}
	a.UpdatedAt = now
	if err := r.clearReads(ctx, a.ID); err != nil {
		return err
	}
	// Same reasoning for poll answers: they belong to the RETRACTED wording and
	// its options. ReplaceOptions deletes the option rows (cascading the answers)
	// whenever the option set changes, but an edit that keeps the options while
	// changing the question must not carry the old answers over either.
	if err := r.clearResponses(ctx, a.ID); err != nil {
		return err
	}
	return nil
}

// clearResponses deletes every poll answer for an announcement (RLS-scoped to
// the current tenant). Used when a draft is (re-)edited, mirroring clearReads.
func (r *ParentAnnouncementRepository) clearResponses(ctx context.Context, announcementID int64) error {
	if _, err := base.GetDB(ctx, r.DB).NewDelete().
		Model((*users.ParentAnnouncementResponse)(nil)).
		ModelTableExpr("users.parent_announcement_responses").
		Where("announcement_id = ?", announcementID).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear parent announcement responses", Err: err}
	}
	return nil
}

// clearReads deletes every read/ack row for an announcement (RLS-scoped to the
// current tenant). Used when a draft is (re-)edited so stale reads from a prior
// publication don't carry over to the corrected wording.
func (r *ParentAnnouncementRepository) clearReads(ctx context.Context, announcementID int64) error {
	if _, err := base.GetDB(ctx, r.DB).NewDelete().
		Model((*users.ParentAnnouncementRead)(nil)).
		ModelTableExpr("users.parent_announcement_reads").
		Where("announcement_id = ?", announcementID).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear parent announcement reads", Err: err}
	}
	return nil
}

// SetPublished sets (or clears, when publishedAt is nil) the publication
// timestamp. RLS pins the update to the current tenant.
func (r *ParentAnnouncementRepository) SetPublished(ctx context.Context, id int64, publishedAt *time.Time) error {
	_, err := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*users.ParentAnnouncement)(nil)).
		ModelTableExpr("users.parent_announcements").
		Set("published_at = ?", publishedAt).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "set parent announcement published_at", Err: err}
	}
	return nil
}

// PublishIfDraft atomically transitions a DRAFT (published_at IS NULL) to
// published, stamping publishedAt, and reports whether THIS call flipped the
// row. The UPDATE is guarded by published_at IS NULL, so a concurrent second
// publish (double-click / two tabs) blocks on the row lock, re-reads a now
// non-null published_at, matches nothing, and returns false — the caller then
// skips the opted-in e-mail enqueue, so a publish race can never double-send the
// parent notification. RLS pins the update to the current tenant.
func (r *ParentAnnouncementRepository) PublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error) {
	res, err := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*users.ParentAnnouncement)(nil)).
		ModelTableExpr("users.parent_announcements").
		Set("published_at = ?", publishedAt).
		Where("id = ?", id).
		Where("published_at IS NULL").
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "publish parent announcement", Err: err}
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "publish parent announcement rows affected", Err: err}
	}
	return n > 0, nil
}

// ReplaceTargets swaps an announcement's target rows wholesale: delete the
// existing ones, insert the supplied set. Runs inside the caller's tenant tx.
//
// It first locks the announcement row and re-checks published_at IS NULL, so a
// target swap can only ever land on a DRAFT. Without this guard a publish that
// slipped in between the content Update and this call could enqueue the opt-in
// e-mails against the OLD targets while this swap moves the live audience to the
// NEW ones — e-mailing the wrong guardians while the feed/stats show a different
// set. On a published row it returns users.ErrAnnouncementPublished (the service
// maps it to the same immutability conflict as a racing Update). The FOR UPDATE
// lock also blocks a concurrent publish until this tenant tx commits, so targets
// and published_at can never interleave.
func (r *ParentAnnouncementRepository) ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*users.ParentAnnouncementTarget) error {
	db := base.GetDB(ctx, r.DB)
	var publishedAt *time.Time
	if err := db.NewSelect().
		ColumnExpr("published_at").
		TableExpr("users.parent_announcements").
		Where("id = ?", announcementID).
		Where("tenant_id = ?", tenantID).
		For("UPDATE").
		Scan(ctx, &publishedAt); err != nil {
		return &modelBase.DatabaseError{Op: "lock parent announcement for target replace", Err: err}
	}
	if publishedAt != nil {
		return users.ErrAnnouncementPublished
	}
	if _, err := db.NewDelete().
		Model((*users.ParentAnnouncementTarget)(nil)).
		ModelTableExpr("users.parent_announcement_targets").
		Where("announcement_id = ?", announcementID).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear parent announcement targets", Err: err}
	}
	if len(targets) == 0 {
		return nil
	}
	for _, t := range targets {
		t.AnnouncementID = announcementID
		t.SetTenantID(tenantID)
	}
	if _, err := db.NewInsert().
		Model(&targets).
		ModelTableExpr("users.parent_announcement_targets").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "insert parent announcement targets", Err: err}
	}
	return nil
}

// ListTargets returns an announcement's target rows.
func (r *ParentAnnouncementRepository) ListTargets(ctx context.Context, announcementID int64) ([]*users.ParentAnnouncementTarget, error) {
	var rows []*users.ParentAnnouncementTarget
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr("users.parent_announcement_targets AS pat").
		Where("pat.announcement_id = ?", announcementID).
		OrderExpr("pat.id ASC")
	query = base.WithTenantFilter(ctx, query, "pat")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent announcement targets", Err: err}
	}
	return rows, nil
}

// CountAudience returns the number of distinct guardian accounts an
// announcement's targets currently reach within its tenant.
func (r *ParentAnnouncementRepository) CountAudience(ctx context.Context, tenantID, announcementID int64) (int, error) {
	var count int
	sqlStr := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT DISTINCT gp.account_id AS account_id
			FROM users.parent_announcement_targets pt
			JOIN users.students s ON s.tenant_id = ? AND (
				pt.target_type = 'school_all'
				OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
				OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
				OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
				OR %s
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			-- Graduated (alumnus) students are soft-deleted: their guardians must
			-- drop out of every announcement audience (reach count, feed
			-- visibility, and e-mails) once the child has left the OGS (#405).
			AND s.status <> 'alumnus'
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id IS NOT NULL
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE pt.announcement_id = ? AND pt.tenant_id = ?

			UNION

			SELECT DISTINCT COALESCE(req.guardian_account_id, ea.id) AS account_id
			FROM users.parent_announcement_targets pt
			JOIN enrollment.requests req ON req.tenant_id = ?
				AND req.withdrawn_at IS NULL
			LEFT JOIN auth.accounts ea ON req.guardian_account_id IS NULL
				AND ea.email IS NOT NULL
				AND LOWER(TRIM(ea.email)) = LOWER(TRIM(req.guardian_email))
			JOIN enrollment.request_children rc ON rc.request_id = req.id AND rc.tenant_id = ?
				AND rc.status IN (%s)
			WHERE pt.announcement_id = ? AND pt.tenant_id = ? AND pt.target_type = 'pending_enrollment'
				AND COALESCE(req.guardian_account_id, ea.id) IS NOT NULL
		) reached`, activeActivityGroupExists("?"), pendingStatusList())
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr,
		tenantID, tenantID, tenantID, tenantID, announcementID, tenantID, // student path
		tenantID, tenantID, announcementID, tenantID, // pending path
	).Scan(ctx, &count); err != nil {
		return 0, &modelBase.DatabaseError{Op: "count parent announcement audience", Err: err}
	}
	return count, nil
}

// AccountMatchesAnnouncement reports whether a guardian account is in an
// announcement's audience right now.
func (r *ParentAnnouncementRepository) AccountMatchesAnnouncement(ctx context.Context, tenantID, announcementID, accountID int64) (bool, error) {
	var matched bool
	predicate := reachedPredicate("?", "?", "?")
	sqlStr := "SELECT " + predicate
	// reachedPredicate references, in order: ann (student), tenant (student x4
	// inside the fragment), acc (student); ann (pending), tenant (pending), acc
	// (pending). The fmt placeholders are positional %[1]s=ann %[2]s=tenant
	// %[3]s=acc, but each `?` in the rendered SQL still binds left-to-right, so
	// the args below follow the textual `?` order of the rendered string.
	args := reachedArgs(announcementID, tenantID, accountID)
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr, args...).Scan(ctx, &matched); err != nil {
		return false, &modelBase.DatabaseError{Op: "check parent announcement audience match", Err: err}
	}
	return matched, nil
}

// reachedArgs returns the bind args for reachedPredicate("?","?","?") in the
// exact textual order the `?` placeholders appear in the rendered SQL. Keeping
// this next to reachedPredicate is deliberate: the two MUST change together.
//
// Student EXISTS, in order of appearance:
//
//	s.tenant_id = ?                 -> tenant
//	se.tenant_id = ?                -> tenant   (inside the activity_group sub-EXISTS)
//	sg.tenant_id = ?                -> tenant
//	gp.tenant_id = ?                -> tenant
//	gp.account_id = ?               -> acc
//	pt.announcement_id = ?          -> ann
//	pt.tenant_id = ?                -> tenant
//
// Pending EXISTS, in order:
//
//	req.tenant_id = ?               -> tenant
//	req.guardian_account_id = ?     -> acc   (primary ownership match)
//	ea.id = ?                       -> acc   (e-mail fallback: ea.id = account)
//	rc.tenant_id = ?                -> tenant
//	pt.announcement_id = ?          -> ann
//	pt.tenant_id = ?                -> tenant
func reachedArgs(announcementID, tenantID, accountID int64) []any {
	return []any{
		tenantID, tenantID, tenantID, tenantID, accountID, announcementID, tenantID, // student EXISTS
		tenantID, accountID, accountID, tenantID, announcementID, tenantID, // pending EXISTS
	}
}

// ResolveAudienceEmails returns the distinct guardian recipients (non-empty
// e-mail) an announcement currently reaches. Student-based targets read the
// guardian profile's e-mail/name; the pending_enrollment target reads the
// e-mail/name captured on the enrollment request (those guardians may not have
// a profile yet). An outer GROUP BY collapses the UNION to one row per
// account and normalized e-mail, so a guardian reached through BOTH a student
// target and pending_enrollment — possibly with different captured vs profile
// names — is enqueued (and e-mailed) only once per address; the retained name
// prefers a non-empty value across the merged rows.
func (r *ParentAnnouncementRepository) ResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*users.AnnouncementRecipient, error) {
	// Only guardians WITH a linked account receive the e-mail: the mail is a
	// pointer into the parent portal (title + link, no body), which is useless
	// without an account — and it keeps recipients consistent with the feed
	// audience and the staff reach stats. A pending_enrollment applicant counts
	// as linked either way the feed audience does: a stamped guardian_account_id,
	// or an unstamped request whose guardian_email resolves to an auth.accounts
	// row (the ListByAccount e-mail fallback) — the ea.id IS NOT NULL guard drops
	// requests with no account at all.
	var rows []*users.AnnouncementRecipient
	sqlStr := fmt.Sprintf(`
		SELECT account_id, email,
			COALESCE(min(NULLIF(first_name, '')), '') AS first_name,
			COALESCE(min(NULLIF(last_name, '')), '') AS last_name
		FROM (
		SELECT DISTINCT gp.account_id, lower(gp.email) AS email,
			COALESCE(gp.first_name, '') AS first_name, COALESCE(gp.last_name, '') AS last_name
		FROM users.parent_announcement_targets pt
		JOIN users.students s ON s.tenant_id = ? AND (
			pt.target_type = 'school_all'
			OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
			OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
			OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
			OR %s
		)
		JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
		-- Graduated (alumnus) students are soft-deleted: their guardians must
		-- drop out of every announcement audience (reach count, feed
		-- visibility, and e-mails) once the child has left the OGS (#405).
		AND s.status <> 'alumnus'
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
			COALESCE(req.guardian_first_name, '') AS first_name, COALESCE(req.guardian_last_name, '') AS last_name
		FROM users.parent_announcement_targets pt
		JOIN enrollment.requests req ON req.tenant_id = ?
			AND req.withdrawn_at IS NULL
			AND length(btrim(req.guardian_email)) > 0
		LEFT JOIN auth.accounts ea ON req.guardian_account_id IS NULL
			AND ea.email IS NOT NULL
			AND LOWER(TRIM(ea.email)) = LOWER(TRIM(req.guardian_email))
		JOIN enrollment.request_children rc ON rc.request_id = req.id AND rc.tenant_id = ?
			AND rc.status IN (%s)
		WHERE pt.announcement_id = ? AND pt.tenant_id = ? AND pt.target_type = 'pending_enrollment'
			AND (req.guardian_account_id IS NOT NULL OR ea.id IS NOT NULL)
		) recips
		GROUP BY account_id, email`,
		activeActivityGroupExists("?"), pendingStatusList())
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr,
		tenantID, tenantID, tenantID, tenantID, announcementID, tenantID, // student path
		tenantID, tenantID, announcementID, tenantID, // pending path
	).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "resolve parent announcement audience emails", Err: err}
	}
	return rows, nil
}

// LetterChildStatuses returns every reached child with the DERIVED fulfilment
// state of an Elternbrief (#2384): who confirmed it for that child and when.
//
// The audience is the same portal-visible student set the poll view uses, so a
// child nobody can currently reach in moto stays visible instead of silently
// disappearing from the count.
//
// The LATERAL picks the FIRST acknowledgement, ordered by time: once the letter
// is fulfilled, "wer hat für dieses Kind bestätigt" means whoever got there
// first. It joins through students_guardians with parent_portal.access, so a
// guardian who acknowledged the announcement for a DIFFERENT child cannot
// accidentally fulfil this one.
//
// Nothing here is stored: because acknowledgement lives on the account, one
// confirmation covers every addressed sibling of that guardian automatically.
func (r *ParentAnnouncementRepository) LetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*users.AnnouncementLetterChildStatus, error) {
	audience := pollAudienceStudentsSQL("?", "?", "")
	args := audienceStudentArgs(announcementID, tenantID, nil)
	sqlStr := `WITH audience AS (` + audience + `)
		SELECT s.id AS student_id,
			COALESCE(p.first_name, '') AS first_name,
			COALESCE(p.last_name, '')  AS last_name,
			COALESCE(s.school_class, '') AS school_class,
			ack.acknowledged_at AS acknowledged_at,
			COALESCE(ack.first_name, '') AS ack_first_name,
			COALESCE(ack.last_name, '')  AS ack_last_name
		FROM audience
		JOIN users.students s ON s.id = audience.student_id
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
	sqlArgs := append(append([]any{}, args...), tenantID, announcementID, tenantID, tenantID)
	var rows []*users.AnnouncementLetterChildStatus
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr, sqlArgs...).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "parent announcement letter child statuses", Err: err}
	}
	return rows, nil
}

// ResolveDeliveryRecipients returns every guardian linked to a child the
// announcement's student-based targets reach, with no portal-access filter —
// the input for the per-recipient delivery rows behind the staff matrix (#2384).
//
// Two things make this different from ResolveAudienceEmails, and both are the
// point: guardians WITHOUT parent_portal.access are included (so the matrix can
// show "kein Portalzugang" instead of silently omitting the person), and
// guardians without an address are included (so the school sees the gap it has
// to fix). Whether such a person is actually mailed is the caller's decision,
// taken from the announcement's email_audience.
//
// has_portal_access is aggregated with bool_or across the reached children: a
// guardian who has portal access for one addressed child but not another can
// see the announcement, so they count as reachable in moto.
func (r *ParentAnnouncementRepository) ResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*users.AnnouncementDeliveryRecipient, error) {
	var rows []*users.AnnouncementDeliveryRecipient
	sqlStr := fmt.Sprintf(`
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
			) AS has_portal_access
		FROM users.parent_announcement_targets pt
		JOIN users.students s ON s.tenant_id = ? AND (
			pt.target_type = 'school_all'
			OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
			OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
			OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
			OR %s
		)
		JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
		-- Same alumnus rule as every other audience query (#405): a graduated
		-- child's guardians drop out entirely.
		AND s.status <> 'alumnus'
		JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
		JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
		WHERE pt.announcement_id = ? AND pt.tenant_id = ?
		GROUP BY gp.id, gp.account_id, gp.first_name, gp.last_name, gp.email, gp.portal_locale
		ORDER BY LOWER(COALESCE(gp.last_name, '')), LOWER(COALESCE(gp.first_name, '')), gp.id`,
		activeActivityGroupExists("?"))
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr,
		tenantID, tenantID, tenantID, tenantID, announcementID, tenantID,
	).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "resolve parent announcement delivery recipients", Err: err}
	}
	return rows, nil
}

// SchoolName returns the tenant's school name (empty when unknown).
func (r *ParentAnnouncementRepository) SchoolName(ctx context.Context, tenantID int64) (string, error) {
	var name string
	if err := base.GetDB(ctx, r.DB).NewRaw(
		`SELECT COALESCE(name, '') FROM platform.schools WHERE id = ?`, tenantID,
	).Scan(ctx, &name); err != nil {
		return "", &modelBase.DatabaseError{Op: "resolve school name", Err: err}
	}
	return name, nil
}

// AudienceRecipients returns the guardian ACCOUNTS an announcement currently
// reaches — same audience as CountAudience/Stats — each with a display name
// and the account's read/ack state (LEFT JOIN, nil = pending). The inner UNION
// mirrors ResolveAudienceEmails (guardians with a linked account via the
// student targets, applicants with an account via pending_enrollment); an
// account reachable through several children/paths collapses to one row.
func (r *ParentAnnouncementRepository) AudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*users.AnnouncementRecipientStatus, error) {
	var rows []*users.AnnouncementRecipientStatus
	sqlStr := fmt.Sprintf(`
		SELECT acc.account_id,
			min(acc.first_name) AS first_name,
			min(acc.last_name) AS last_name,
			COALESCE(min(locale_gp.portal_locale), 'de') AS portal_locale,
			par.read_at AS read_at,
			par.acknowledged_at AS acknowledged_at
		FROM (
			SELECT gp.account_id,
				COALESCE(gp.first_name, '') AS first_name, COALESCE(gp.last_name, '') AS last_name
			FROM users.parent_announcement_targets pt
			JOIN users.students s ON s.tenant_id = ? AND (
				pt.target_type = 'school_all'
				OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
				OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
				OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
				OR %s
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			-- Graduated (alumnus) students are soft-deleted: their guardians must
			-- drop out of every announcement audience (reach count, feed
			-- visibility, and e-mails) once the child has left the OGS (#405).
			AND s.status <> 'alumnus'
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id IS NOT NULL
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE pt.announcement_id = ? AND pt.tenant_id = ?

			UNION

			SELECT COALESCE(req.guardian_account_id, ea.id) AS account_id,
				COALESCE(req.guardian_first_name, '') AS first_name, COALESCE(req.guardian_last_name, '') AS last_name
			FROM users.parent_announcement_targets pt
			JOIN enrollment.requests req ON req.tenant_id = ?
				AND req.withdrawn_at IS NULL
			LEFT JOIN auth.accounts ea ON req.guardian_account_id IS NULL
				AND ea.email IS NOT NULL
				AND LOWER(TRIM(ea.email)) = LOWER(TRIM(req.guardian_email))
			JOIN enrollment.request_children rc ON rc.request_id = req.id AND rc.tenant_id = ?
				AND rc.status IN (%s)
			WHERE pt.announcement_id = ? AND pt.tenant_id = ? AND pt.target_type = 'pending_enrollment'
				AND COALESCE(req.guardian_account_id, ea.id) IS NOT NULL
		) acc
		LEFT JOIN users.guardian_profiles locale_gp
			ON locale_gp.account_id = acc.account_id AND locale_gp.tenant_id = ?
		LEFT JOIN users.parent_announcement_reads par
			ON par.announcement_id = ? AND par.account_id = acc.account_id
		GROUP BY acc.account_id, par.read_at, par.acknowledged_at
		ORDER BY last_name ASC, first_name ASC, acc.account_id ASC`,
		activeActivityGroupExists("?"), pendingStatusList())
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr,
		tenantID, tenantID, tenantID, tenantID, announcementID, tenantID, // student path
		tenantID, tenantID, announcementID, tenantID, // pending path
		tenantID,       // guardian locale
		announcementID, // reads join
	).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "resolve parent announcement audience recipients", Err: err}
	}
	return rows, nil
}

// ListFeedForAccount returns published, active, unexpired announcements the
// account is targeted by across the given tenants, newest-published first, each
// with the account's read/ack state. Cross-tenant: run under WithAdminTx with
// the explicit tenant set.
func (r *ParentAnnouncementRepository) ListFeedForAccount(ctx context.Context, accountID int64, tenantIDs []int64) ([]*users.AnnouncementFeedItem, error) {
	if len(tenantIDs) == 0 {
		return []*users.AnnouncementFeedItem{}, nil
	}
	var rows []*users.AnnouncementFeedItem
	reached := reachedPredicate("a.id", "a.tenant_id", "?")
	sqlStr := `
		SELECT a.id, a.tenant_id, a.title, a.body, a.priority, a.link_url,
			a.requires_acknowledgement, a.published_at, a.expires_at,
			a.response_type, a.response_deadline, a.delivery_mode,
			COALESCE(sch.name, '') AS school_name,
			par.read_at AS read_at,
			par.acknowledged_at AS acknowledged_at
		FROM users.parent_announcements a
		LEFT JOIN platform.schools sch ON sch.id = a.tenant_id
		LEFT JOIN users.parent_announcement_reads par
			ON par.announcement_id = a.id AND par.account_id = ?
		WHERE a.tenant_id IN (?)
			AND a.active
			AND a.published_at IS NOT NULL
			AND a.published_at <= NOW()
			AND (a.expires_at IS NULL OR a.expires_at > NOW())
			AND ` + reached + `
		ORDER BY a.published_at DESC, a.id DESC`
	// Arg order: read-state join (acc), tenant set, then reachedPredicate's acc
	// appears three times (student EXISTS once, pending EXISTS twice: the primary
	// guardian_account_id match plus the e-mail-fallback acc.id) since ann/tenant
	// are column refs.
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr,
		accountID, bun.List(tenantIDs), accountID, accountID, accountID,
	).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent announcement feed", Err: err}
	}
	return rows, nil
}

// CountUnreadForAccount counts live announcements that still need attention:
// unread letters, pending read confirmations, or unanswered open polls.
func (r *ParentAnnouncementRepository) CountUnreadForAccount(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
	if len(tenantIDs) == 0 {
		return 0, nil
	}
	var count int
	reached := reachedPredicate("a.id", "a.tenant_id", "?")
	openPoll := openPollForAccountPredicate("a", "a.tenant_id", "?")
	// Reading an actionable item must not clear its reminder. It stays outstanding
	// until the requested confirmation or every required poll answer is stored.
	sqlStr := `
		SELECT COUNT(*)
		FROM users.parent_announcements a
		LEFT JOIN users.parent_announcement_reads par
			ON par.announcement_id = a.id AND par.account_id = ?
		WHERE a.tenant_id IN (?)
			AND a.active
			AND a.published_at IS NOT NULL
			AND a.published_at <= NOW()
			AND (a.expires_at IS NULL OR a.expires_at > NOW())
			AND (
				par.read_at IS NULL
				OR (a.requires_acknowledgement AND par.acknowledged_at IS NULL)
				OR ` + openPoll + `
			)
			AND ` + reached
	// Arg order: read-state join (acc), tenant set, the open-poll predicate's acc,
	// then reachedPredicate's acc three times (student once, pending twice).
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr,
		accountID, bun.List(tenantIDs), accountID, accountID, accountID, accountID,
	).Scan(ctx, &count); err != nil {
		return 0, &modelBase.DatabaseError{Op: "count outstanding parent announcements", Err: err}
	}
	return count, nil
}

// liveVersionGuardCTE is the leading CTE shared by MarkRead/MarkAcknowledged: it
// yields a single row IFF the announcement is still live (active, published,
// publish time reached, not expired) AND its published_at still equals the
// version the client loaded. published_at = ? already implies NOT NULL. Mirrors
// announcementIsLive in the service, but evaluated in the same statement as the
// write so a concurrent correction cannot slip a stale read/ack onto the
// corrected wording.
//
// Both methods gate the read/ack write on EXISTS(guard) AND return EXISTS(guard)
// as their result, so the caller can tell a genuine no-op (guard missed — a
// correction slipped in between the service's authorize phase and the write)
// apart from an idempotent repeat (guard matched, ON CONFLICT skipped the row).
// Rows-affected alone cannot distinguish those two for MarkRead's DO NOTHING.
// Its three bind args are announcementID, tenantID, expectedPublishedAt.
const liveVersionGuardCTE = `WITH guard AS (
		SELECT 1 FROM users.parent_announcements a
		WHERE a.id = ? AND a.tenant_id = ?
			AND a.active
			AND a.published_at = ?
			AND a.published_at <= NOW()
			AND (a.expires_at IS NULL OR a.expires_at > NOW())
	)`

// MarkRead upserts the account's read row for an announcement (idempotent: a
// repeat read leaves read_at and any acknowledgement untouched). The insert is
// gated on the announcement still being live at expectedPublishedAt, so a
// request that loaded a since-retracted/republished wording records nothing. It
// returns true when the announcement was still live at that version (write
// applied or a prior read already existed) and false when the version guard
// missed — the caller maps false to a stale conflict rather than a silent 200.
func (r *ParentAnnouncementRepository) MarkRead(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	var live bool
	if err := base.GetDB(ctx, r.DB).NewRaw(liveVersionGuardCTE+`,
		ins AS (
			INSERT INTO users.parent_announcement_reads (tenant_id, announcement_id, account_id, read_at)
			SELECT ?, ?, ?, ?
			WHERE EXISTS (SELECT 1 FROM guard)
			ON CONFLICT (announcement_id, account_id) DO NOTHING
		)
		SELECT EXISTS (SELECT 1 FROM guard)`,
		announcementID, tenantID, expectedPublishedAt, // guard CTE
		tenantID, announcementID, accountID, time.Now(), // insert values
	).Scan(ctx, &live); err != nil {
		return false, &modelBase.DatabaseError{Op: "mark parent announcement read", Err: err}
	}
	return live, nil
}

// MarkAcknowledged upserts the account's read row and stamps acknowledged_at
// (clearing nothing on a repeat ack). A read row is created if the guardian
// acknowledges without a prior explicit read. Same version guard and return
// contract as MarkRead: true when the announcement was still live at
// expectedPublishedAt, false when the guard missed (a since-slipped correction).
func (r *ParentAnnouncementRepository) MarkAcknowledged(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	now := time.Now()
	var live bool
	if err := base.GetDB(ctx, r.DB).NewRaw(liveVersionGuardCTE+`,
		ups AS (
			INSERT INTO users.parent_announcement_reads (tenant_id, announcement_id, account_id, read_at, acknowledged_at)
			SELECT ?, ?, ?, ?, ?
			WHERE EXISTS (SELECT 1 FROM guard)
			ON CONFLICT (announcement_id, account_id) DO UPDATE
			SET acknowledged_at = COALESCE(parent_announcement_reads.acknowledged_at, EXCLUDED.acknowledged_at)
		)
		SELECT EXISTS (SELECT 1 FROM guard)`,
		announcementID, tenantID, expectedPublishedAt, // guard CTE
		tenantID, announcementID, accountID, now, now, // upsert values
	).Scan(ctx, &live); err != nil {
		return false, &modelBase.DatabaseError{Op: "mark parent announcement acknowledged", Err: err}
	}
	return live, nil
}

// Stats returns the reach/read/ack counts for an announcement. read/ack are
// intersected with the CURRENT audience so the staff "X von Y" never shows more
// readers than the live target count when the audience has since shrunk.
func (r *ParentAnnouncementRepository) Stats(ctx context.Context, tenantID, announcementID int64) (*users.AnnouncementStats, error) {
	stats := &users.AnnouncementStats{}
	audienceCTE := fmt.Sprintf(`
		WITH audience AS (
			SELECT DISTINCT gp.account_id AS account_id
			FROM users.parent_announcement_targets pt
			JOIN users.students s ON s.tenant_id = ? AND (
				pt.target_type = 'school_all'
				OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
				OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
				OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
				OR %s
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			-- Graduated (alumnus) students are soft-deleted: their guardians must
			-- drop out of every announcement audience (reach count, feed
			-- visibility, and e-mails) once the child has left the OGS (#405).
			AND s.status <> 'alumnus'
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id IS NOT NULL
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE pt.announcement_id = ? AND pt.tenant_id = ?
			UNION
			SELECT DISTINCT COALESCE(req.guardian_account_id, ea.id) AS account_id
			FROM users.parent_announcement_targets pt
			JOIN enrollment.requests req ON req.tenant_id = ?
				AND req.withdrawn_at IS NULL
			LEFT JOIN auth.accounts ea ON req.guardian_account_id IS NULL
				AND ea.email IS NOT NULL
				AND LOWER(TRIM(ea.email)) = LOWER(TRIM(req.guardian_email))
			JOIN enrollment.request_children rc ON rc.request_id = req.id AND rc.tenant_id = ?
				AND rc.status IN (%s)
			WHERE pt.announcement_id = ? AND pt.tenant_id = ? AND pt.target_type = 'pending_enrollment'
				AND COALESCE(req.guardian_account_id, ea.id) IS NOT NULL
		)
		SELECT
			(SELECT COUNT(*) FROM audience) AS target_count,
			(SELECT COUNT(*) FROM users.parent_announcement_reads par
				WHERE par.announcement_id = ? AND par.tenant_id = ?
					AND par.account_id IN (SELECT account_id FROM audience)) AS read_count,
			(SELECT COUNT(*) FROM users.parent_announcement_reads par
				WHERE par.announcement_id = ? AND par.tenant_id = ? AND par.acknowledged_at IS NOT NULL
					AND par.account_id IN (SELECT account_id FROM audience)) AS acknowledged_count`,
		activeActivityGroupExists("?"), pendingStatusList())
	if err := base.GetDB(ctx, r.DB).NewRaw(audienceCTE,
		tenantID, tenantID, tenantID, tenantID, announcementID, tenantID, // student path
		tenantID, tenantID, announcementID, tenantID, // pending path
		announcementID, tenantID, // read_count
		announcementID, tenantID, // acknowledged_count
	).Scan(ctx, &stats.TargetCount, &stats.ReadCount, &stats.AcknowledgedCount); err != nil {
		return nil, &modelBase.DatabaseError{Op: "parent announcement stats", Err: err}
	}
	return stats, nil
}
