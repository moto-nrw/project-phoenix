package users

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// This file holds the poll (Umfrage, #1371) half of the parent-announcement
// repository: answer options, per-child responses, and the staff-facing
// evaluation. It shares the audience definition with the announcement half —
// see audienceStudentsSQL below, which is the per-CHILD sibling of the
// per-ACCOUNT reachedPredicate in parent_announcement.go. The two must stay
// consistent: a poll's reach is exactly the set of children behind the
// announcement reach, so "12 von 30 Kindern" can never exceed what the same
// targets deliver as announcement recipients.
//
// The pending_enrollment target has no student behind it, so it contributes to
// an announcement's account reach but never to a poll's child reach. That is
// intentional and documented in the UI: an applicant without an enrolled child
// has no child to answer for.

// audienceStudentsSQL renders "the children this announcement reaches right
// now" as a SELECT of distinct student ids. tenantExpr is the SQL expression for
// the tenant (a bound `?` or a column reference like `a.tenant_id`); annExpr is
// the announcement id expression. accountFilter is either "" (all reached
// children) or "AND gp.account_id = ?" (only one guardian's children).
//
// The join chain mirrors the normal student-backed announcement audience: a
// live, non-graduated student whose person is not soft-deleted, a
// parent_portal.access guardian link, a guardian profile with a linked account,
// and an ACTIVE account_tenants membership. Staff results must include every
// child the announcement reaches, even if no guardian may answer the poll.
func pollAudienceStudentsSQL(annExpr, tenantExpr, accountFilter string) string {
	return audienceStudentsSQLForPermission(annExpr, tenantExpr, accountFilter, "")
}

// pollAnswerableStudentsSQL is the subset of the normal portal-visible
// audience for which at least one guardian currently has poll.response. It is
// the completion denominator and reminder source; the broader audience remains
// available to staff so inaccessible targets are visible rather than appearing
// as permanently unanswered.
func pollAnswerableStudentsSQL(annExpr, tenantExpr, accountFilter string) string {
	return audienceStudentsSQLForPermission(
		annExpr,
		tenantExpr,
		accountFilter,
		`sg.permissions @> '{"parent_portal.poll.response": true}'::jsonb`,
	)
}

// audienceStudentsSQLForPermission is the permission-specific variant used by
// actions that need stronger authority than merely seeing a child in the
// portal. permissionPredicate is a trusted SQL predicate over sg.
func audienceStudentsSQLForPermission(annExpr, tenantExpr, accountFilter, permissionPredicate string) string {
	if permissionPredicate != "" {
		permissionPredicate = " AND " + permissionPredicate
	}
	return fmt.Sprintf(`
		SELECT DISTINCT s.id AS student_id
		FROM users.parent_announcement_targets pt
		JOIN users.students s ON s.tenant_id = %[2]s AND (
			pt.target_type = 'school_all'
			OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
			OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
			OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
			OR %[3]s
		)
		JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
		AND s.status <> 'alumnus'
		JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = %[2]s
			AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			%[5]s
		JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = %[2]s
			AND gp.account_id IS NOT NULL %[4]s
		JOIN auth.account_tenants act ON act.account_id = gp.account_id
			AND act.tenant_id = gp.tenant_id AND act.status = 'active'
		WHERE pt.announcement_id = %[1]s AND pt.tenant_id = %[2]s`,
		annExpr, tenantExpr, activeActivityGroupExists(tenantExpr), accountFilter, permissionPredicate)
}

// audienceStudentArgs returns the bind args for audienceStudentsSQL("?","?",...)
// in the textual order the placeholders appear. Keep in lockstep with the SQL
// above (mirrors reachedArgs):
//
//	s.tenant_id = ?          -> tenant
//	se.tenant_id = ?         -> tenant  (inside the activity_group sub-EXISTS)
//	sg.tenant_id = ?         -> tenant
//	gp.tenant_id = ?         -> tenant
//	[gp.account_id = ?       -> account, only with an account filter]
//	pt.announcement_id = ?   -> ann
//	pt.tenant_id = ?         -> tenant
func audienceStudentArgs(announcementID, tenantID int64, accountID *int64) []any {
	args := []any{tenantID, tenantID, tenantID, tenantID}
	if accountID != nil {
		args = append(args, *accountID)
	}
	return append(args, announcementID, tenantID)
}

// openPollForAccountPredicate builds the SQL boolean "this announcement is an
// open poll and at least one of account :acc's reached children has no answer
// yet". annExpr/tenantExpr are SQL expressions (column refs in the feed query),
// accPlace the account placeholder.
//
// It is what makes an answered-but-unread distinction impossible to game in the
// parent badge: a guardian who read a poll but never answered it still counts as
// outstanding, because a poll that quietly stops nagging is a poll nobody
// answers.
func openPollForAccountPredicate(annExpr, tenantExpr, accPlace string) string {
	return fmt.Sprintf(`(
		%[1]s.response_type <> 'none'
		AND (%[1]s.response_deadline IS NULL OR %[1]s.response_deadline > NOW())
		AND EXISTS (
			SELECT 1
			FROM users.parent_announcement_targets pt
			JOIN users.students s ON s.tenant_id = %[2]s AND (
				pt.target_type = 'school_all'
				OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
				OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
				OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
				OR %[4]s
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			AND s.status <> 'alumnus'
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = %[2]s
				AND sg.permissions @> '{"parent_portal.access": true, "parent_portal.poll.response": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = %[2]s
				AND gp.account_id = %[3]s
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE pt.announcement_id = %[1]s.id AND pt.tenant_id = %[2]s
				AND NOT EXISTS (
					SELECT 1 FROM users.parent_announcement_responses resp
					WHERE resp.announcement_id = %[1]s.id AND resp.student_id = s.id
				)
		)
	)`, annExpr, tenantExpr, accPlace, activeActivityGroupExists(tenantExpr))
}

// ReplaceOptions swaps a DRAFT poll's answer options wholesale. It locks the
// announcement and re-checks published_at IS NULL for the same reason
// ReplaceTargets does: options are referenced by response rows, so changing them
// under a published poll would silently re-label answers guardians already gave.
// A published row returns users.ErrAnnouncementPublished.
func (r *ParentAnnouncementRepository) ReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*users.ParentAnnouncementOption) error {
	db := base.GetDB(ctx, r.DB)
	var publishedAt *time.Time
	if err := db.NewSelect().
		ColumnExpr("published_at").
		TableExpr("users.parent_announcements").
		Where("id = ?", announcementID).
		Where("tenant_id = ?", tenantID).
		For("UPDATE").
		Scan(ctx, &publishedAt); err != nil {
		return &modelBase.DatabaseError{Op: "lock parent announcement for option replace", Err: err}
	}
	if publishedAt != nil {
		return users.ErrAnnouncementPublished
	}
	if _, err := db.NewDelete().
		Model((*users.ParentAnnouncementOption)(nil)).
		ModelTableExpr("users.parent_announcement_options").
		Where("announcement_id = ?", announcementID).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear parent announcement options", Err: err}
	}
	if len(options) == 0 {
		return nil
	}
	for i, o := range options {
		o.AnnouncementID = announcementID
		o.Position = i
		o.SetTenantID(tenantID)
	}
	if _, err := db.NewInsert().
		Model(&options).
		ModelTableExpr("users.parent_announcement_options").
		Returning("*").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "insert parent announcement options", Err: err}
	}
	return nil
}

// ListOptions returns a poll's options in display order.
func (r *ParentAnnouncementRepository) ListOptions(ctx context.Context, announcementID int64) ([]*users.ParentAnnouncementOption, error) {
	var rows []*users.ParentAnnouncementOption
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.parent_announcement_options AS "pao"`).
		Where("pao.announcement_id = ?", announcementID).
		OrderExpr("pao.position ASC, pao.id ASC")
	query = base.WithTenantFilter(ctx, query, "pao")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent announcement options", Err: err}
	}
	return rows, nil
}

// ListOptionsForAnnouncements batches the option lookup for a whole feed page.
// No tenant filter: the announcement id set is already the caller's authorized
// scope (the feed query resolved it), and this runs in the cross-tenant admin tx
// where a per-row tenant filter would exclude everything.
func (r *ParentAnnouncementRepository) ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*users.ParentAnnouncementOption, error) {
	if len(announcementIDs) == 0 {
		return []*users.ParentAnnouncementOption{}, nil
	}
	var rows []*users.ParentAnnouncementOption
	if err := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.parent_announcement_options AS "pao"`).
		Where("pao.announcement_id IN (?)", bun.List(announcementIDs)).
		OrderExpr("pao.announcement_id ASC, pao.position ASC, pao.id ASC").
		Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent announcement options for feed", Err: err}
	}
	return rows, nil
}

// AnswerableChildren returns, for each of the given announcements, the children
// of accountID the poll reaches, with the option ids already selected for that
// child. Cross-tenant (the feed spans schools), so tenant scoping comes from
// each announcement's own tenant_id rather than a bound tenant.
func (r *ParentAnnouncementRepository) AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*users.AnnouncementPollChild, error) {
	if len(announcementIDs) == 0 {
		return []*users.AnnouncementPollChild{}, nil
	}
	var rows []*users.AnnouncementPollChild
	sqlStr := fmt.Sprintf(`
		SELECT DISTINCT a.id AS announcement_id, s.id AS student_id,
			COALESCE(p.first_name, '') AS first_name,
			COALESCE(p.last_name, '') AS last_name,
			COALESCE(sel.options, ARRAY[]::bigint[]) AS selected_options
		FROM users.parent_announcements a
		JOIN users.parent_announcement_targets pt
			ON pt.announcement_id = a.id AND pt.tenant_id = a.tenant_id
		JOIN users.students s ON s.tenant_id = a.tenant_id AND (
			pt.target_type = 'school_all'
			OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
			OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
			OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
			OR %s
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
		ORDER BY last_name ASC, first_name ASC, student_id ASC`,
		activeActivityGroupExists("a.tenant_id"))
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr, accountID, bun.List(announcementIDs)).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list answerable children for parent announcements", Err: err}
	}
	return rows, nil
}

// AccountMayAnswerForStudent reports whether accountID may answer a poll for
// studentID: the child must be in the poll's live audience AND the account must
// be a portal-enabled guardian of exactly that child. Both halves come from the
// same audience query, so a guardian can never answer for a child the poll does
// not reach, nor for a reached child that is not theirs.
func (r *ParentAnnouncementRepository) AccountMayAnswerForStudent(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (bool, error) {
	var allowed bool
	inner := audienceStudentsSQLForPermission("?", "?", "AND gp.account_id = ?", `sg.permissions @> '{"parent_portal.poll.response": true}'::jsonb`)
	sqlStr := "SELECT EXISTS (SELECT 1 FROM (" + inner + ") reached WHERE reached.student_id = ?)"
	args := append(audienceStudentArgs(announcementID, tenantID, &accountID), studentID)
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr, args...).Scan(ctx, &allowed); err != nil {
		return false, &modelBase.DatabaseError{Op: "check parent announcement answer permission", Err: err}
	}
	return allowed, nil
}

// SetResponse replaces a child's answer for a poll: it deletes the child's
// existing rows and inserts one row per selected option, both gated on the
// announcement still being live at expectedPublishedAt AND still accepting
// answers (no deadline, or a deadline in the future). An empty optionIDs
// withdraws the answer.
//
// A transaction-scoped advisory lock serializes replacements for one poll and
// child. Each write has its own live/version/relationship/selection guard, so
// a correction, revocation, or deadline that changes between the delete and
// insert rolls the transaction back instead of storing a stale answer. It
// returns whether the guard and selection matched; false makes the service
// answer 409.
func (r *ParentAnnouncementRepository) SetResponse(ctx context.Context, tenantID, announcementID, studentID, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error) {
	db := base.GetDB(ctx, r.DB)
	// The advisory transaction lock serializes all replacements for this
	// announcement/child pair.  The response table's option-level uniqueness is
	// insufficient: two guardians could otherwise both delete and then insert a
	// different option for the same child.
	//
	// Each guarded write repeats audience/relationship authorization and poll
	// liveness. Revoking poll.response after Phase 1 therefore prevents the
	// write, rather than leaving a TOCTOU window.
	if err := base.AcquireXactLock(ctx, r.DB, parentAnnouncementResponseLockKey(announcementID, studentID)); err != nil {
		return false, &modelBase.DatabaseError{Op: "lock parent announcement response", Err: err}
	}
	guard := `
		WITH guard AS (
			SELECT a.response_type
			FROM users.parent_announcements a
			JOIN users.parent_announcement_targets pt
				ON pt.announcement_id = a.id AND pt.tenant_id = a.tenant_id
			JOIN users.students s ON s.id = ? AND s.tenant_id = a.tenant_id AND (
				pt.target_type = 'school_all'
				OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
				OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
				OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
				OR ` + activeActivityGroupExists("a.tenant_id") + `
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL AND s.status <> 'alumnus'
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = a.tenant_id
				AND sg.permissions @> '{"parent_portal.access": true, "parent_portal.poll.response": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = a.tenant_id
				AND gp.account_id = ?
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE a.id = ? AND a.tenant_id = ?
				AND a.active AND a.published_at = ? AND a.published_at <= NOW()
				AND (a.expires_at IS NULL OR a.expires_at > NOW())
				AND a.response_type <> 'none'
				AND (a.response_deadline IS NULL OR a.response_deadline > NOW())
		)`
	var live bool
	if err := db.NewRaw(guard+`, requested AS (
		SELECT option_id FROM unnest(?::bigint[]) AS selection(option_id)
	), selection_valid AS (
		SELECT EXISTS (SELECT 1 FROM guard)
			AND (SELECT count(*) FROM requested) = (SELECT count(DISTINCT option_id) FROM requested)
			AND (SELECT count(*) FROM requested) = (
				SELECT count(*) FROM requested
				JOIN users.parent_announcement_options o ON o.id = requested.option_id
				WHERE o.announcement_id = ? AND o.tenant_id = ?
			)
			AND ((SELECT response_type FROM guard LIMIT 1) <> 'single_choice'
				OR (SELECT count(*) FROM requested) <= 1) AS valid
	), del AS (
		DELETE FROM users.parent_announcement_responses
		WHERE announcement_id = ? AND student_id = ? AND tenant_id = ?
			AND (SELECT valid FROM selection_valid)
	)
	SELECT valid FROM selection_valid`,
		studentID, accountID, announcementID, tenantID, expectedPublishedAt,
		pgdialect.Array(optionIDs), announcementID, tenantID,
		announcementID, studentID, tenantID,
	).Scan(ctx, &live); err != nil {
		return false, &modelBase.DatabaseError{Op: "replace parent announcement response", Err: err}
	}
	if !live || len(optionIDs) == 0 {
		return live, nil
	}
	// Reapply the complete liveness, relationship, and selection guard to this
	// insert: if the deadline closes or access is revoked after the delete, this
	// transaction returns false and the caller rolls the delete back.
	if err := db.NewRaw(guard+`, requested AS (
		SELECT option_id FROM unnest(?::bigint[]) AS selection(option_id)
	), selection_valid AS (
		SELECT EXISTS (SELECT 1 FROM guard)
			AND (SELECT count(*) FROM requested) = (SELECT count(DISTINCT option_id) FROM requested)
			AND (SELECT count(*) FROM requested) = (
				SELECT count(*) FROM requested
				JOIN users.parent_announcement_options o ON o.id = requested.option_id
				WHERE o.announcement_id = ? AND o.tenant_id = ?
			)
			AND ((SELECT response_type FROM guard LIMIT 1) <> 'single_choice'
				OR (SELECT count(*) FROM requested) <= 1) AS valid
	), ins AS (
		INSERT INTO users.parent_announcement_responses
			(tenant_id, announcement_id, option_id, student_id, account_id, responded_at)
		SELECT ?, ?, o.id, ?, ?, ?
		FROM users.parent_announcement_options o
		WHERE o.announcement_id = ? AND o.tenant_id = ? AND o.id IN (?)
			AND (SELECT valid FROM selection_valid)
		ON CONFLICT (announcement_id, student_id, option_id) DO NOTHING
	)
	SELECT valid FROM selection_valid`,
		studentID, accountID, announcementID, tenantID, expectedPublishedAt,
		pgdialect.Array(optionIDs), announcementID, tenantID,
		tenantID, announcementID, studentID, accountID, time.Now(), announcementID, tenantID, bun.List(optionIDs),
	).Scan(ctx, &live); err != nil {
		return false, &modelBase.DatabaseError{Op: "insert parent announcement response", Err: err}
	}
	return live, nil
}

func parentAnnouncementResponseLockKey(announcementID, studentID int64) string {
	return fmt.Sprintf("parent-announcement-response:%d:%d", announcementID, studentID)
}

// PollResults returns the per-option tally plus how many answerable children
// the poll reaches and how many of them have answered. TargetChildCount retains
// the full portal-visible audience so staff can distinguish unavailable
// respondents from missing answers. Answers are intersected with the CURRENT
// answerable audience, so a child who left the school or lost response
// permission stops counting toward completion.
func (r *ParentAnnouncementRepository) PollResults(ctx context.Context, tenantID, announcementID int64) (*users.AnnouncementPollResults, error) {
	targetAudience := pollAudienceStudentsSQL("?", "?", "")
	answerableAudience := pollAnswerableStudentsSQL("?", "?", "")
	args := audienceStudentArgs(announcementID, tenantID, nil)

	results := &users.AnnouncementPollResults{}
	countSQL := `WITH target_audience AS (` + targetAudience + `),
		answerable_audience AS (` + answerableAudience + `)
		SELECT
			(SELECT COUNT(*) FROM target_audience) AS target_child_count,
			(SELECT COUNT(*) FROM answerable_audience) AS child_count,
			(SELECT COUNT(DISTINCT resp.student_id)
				FROM users.parent_announcement_responses resp
				WHERE resp.announcement_id = ? AND resp.tenant_id = ?
					AND resp.student_id IN (SELECT student_id FROM answerable_audience)) AS answered_count`
	countArgs := append(append(append([]any{}, args...), args...), announcementID, tenantID)
	if err := base.GetDB(ctx, r.DB).NewRaw(countSQL, countArgs...).
		Scan(ctx, &results.TargetChildCount, &results.ChildCount, &results.AnsweredCount); err != nil {
		return nil, &modelBase.DatabaseError{Op: "parent announcement poll counts", Err: err}
	}

	var options []*users.AnnouncementPollOptionResult
	optionSQL := `WITH audience AS (` + answerableAudience + `)
		SELECT o.id AS option_id, o.label, o.position,
			COUNT(DISTINCT resp.student_id) AS count
		FROM users.parent_announcement_options o
		LEFT JOIN users.parent_announcement_responses resp
			ON resp.option_id = o.id AND resp.announcement_id = o.announcement_id
			AND resp.student_id IN (SELECT student_id FROM audience)
		WHERE o.announcement_id = ? AND o.tenant_id = ?
		GROUP BY o.id, o.label, o.position
		ORDER BY o.position ASC, o.id ASC`
	optionArgs := append(append([]any{}, args...), announcementID, tenantID)
	if err := base.GetDB(ctx, r.DB).NewRaw(optionSQL, optionArgs...).Scan(ctx, &options); err != nil {
		return nil, &modelBase.DatabaseError{Op: "parent announcement poll option results", Err: err}
	}
	results.Options = options
	return results, nil
}

// PollChildren returns every portal-visible child the poll reaches with the
// option labels answered for them and whether someone may currently answer.
// This lets staff see inaccessible targets without treating them as overdue.
func (r *ParentAnnouncementRepository) PollChildren(ctx context.Context, tenantID, announcementID int64) ([]*users.AnnouncementPollChildStatus, error) {
	audience := pollAudienceStudentsSQL("?", "?", "")
	answerableAudience := pollAnswerableStudentsSQL("?", "?", "")
	args := audienceStudentArgs(announcementID, tenantID, nil)
	sqlStr := `WITH audience AS (` + audience + `),
		answerable_audience AS (` + answerableAudience + `)
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
	sqlArgs := append(append([]any{}, args...), args...)
	sqlArgs = append(sqlArgs, announcementID, tenantID)
	var rows []*users.AnnouncementPollChildStatus
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr, sqlArgs...).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "parent announcement poll children", Err: err}
	}
	return rows, nil
}

// UnansweredReminderRecipients returns the distinct guardians (account + address)
// of children the poll reaches that have NO answer yet. A guardian with two
// unanswered children appears once — the reminder is one e-mail per person, not
// per child.
func (r *ParentAnnouncementRepository) UnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*users.AnnouncementPollReminderRecipient, error) {
	sqlStr := fmt.Sprintf(`
		SELECT DISTINCT gp.account_id, COALESCE(lower(gp.email), '') AS email,
			COALESCE(gp.first_name, '') AS first_name,
			COALESCE(gp.last_name, '') AS last_name
		FROM users.parent_announcement_targets pt
		JOIN users.students s ON s.tenant_id = ? AND (
			pt.target_type = 'school_all'
			OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
			OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
			OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
			OR %s
		)
		JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
		AND s.status <> 'alumnus'
		JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
			AND sg.permissions @> '{"parent_portal.access": true, "parent_portal.poll.response": true}'::jsonb
		JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
			AND gp.account_id IS NOT NULL
		JOIN auth.account_tenants act ON act.account_id = gp.account_id
			AND act.tenant_id = gp.tenant_id AND act.status = 'active'
		WHERE pt.announcement_id = ? AND pt.tenant_id = ?
			AND NOT EXISTS (
				SELECT 1 FROM users.parent_announcement_responses resp
				WHERE resp.announcement_id = pt.announcement_id
					AND resp.tenant_id = pt.tenant_id
					AND resp.student_id = s.id
			)`, activeActivityGroupExists("?"))
	var rows []*users.AnnouncementPollReminderRecipient
	if err := base.GetDB(ctx, r.DB).NewRaw(sqlStr,
		tenantID, tenantID, tenantID, tenantID, announcementID, tenantID,
	).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "parent announcement poll reminder recipients", Err: err}
	}
	return rows, nil
}
