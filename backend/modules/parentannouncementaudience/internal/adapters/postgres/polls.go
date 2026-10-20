package parentaudience

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"
	"github.com/uptrace/bun"
)

type pollChildRow struct {
	AnnouncementID  int64   `bun:"announcement_id"`
	StudentID       int64   `bun:"student_id"`
	FirstName       string  `bun:"first_name"`
	LastName        string  `bun:"last_name"`
	SelectedOptions []int64 `bun:"selected_options,array"`
}

// AnswerableChildren returns, for each announcement, the children of the
// account the poll reaches with the option ids selected for each child.
func (p *Projection) AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*contract.ParentAnnouncementPollChild, error) {
	if len(announcementIDs) == 0 {
		return []*contract.ParentAnnouncementPollChild{}, nil
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []pollChildRow
	if err := db.NewRaw(answerableChildrenSQL,
		today, today, today, p.links.AllSchoolsGuardianLinkRows(db), accountID, bun.List(announcementIDs),
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list answerable children for parent announcements: %w", err)
	}
	children := make([]*contract.ParentAnnouncementPollChild, 0, len(rows))
	for _, row := range rows {
		children = append(children, &contract.ParentAnnouncementPollChild{
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
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID, accountID,
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
// Bind order: school, school, today, today, today, announcement, school,
// guardian links of the school, school, account, student.
const holdAnswerPermissionSQL = `
			SELECT sg.id` + reachedStudentsBound + pollGuardiansForAccountBound + `
			WHERE s.id = ?
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
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID, accountID,
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
func (p *Projection) PollResults(ctx context.Context, schoolID, announcementID int64) (*contract.ParentAnnouncementPollResults, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	results := &contract.ParentAnnouncementPollResults{}
	if err := db.NewRaw(pollCountsSQL,
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
		announcementID, schoolID,
	).Scan(ctx, &results.TargetChildCount, &results.ChildCount, &results.AnsweredCount); err != nil {
		return nil, fmt.Errorf("parent announcement poll counts: %w", err)
	}
	var rows []pollOptionResultRow
	if err := db.NewRaw(pollOptionResultsSQL,
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
		announcementID, schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent announcement poll option results: %w", err)
	}
	results.Options = make([]*contract.ParentAnnouncementPollOptionResult, 0, len(rows))
	for _, row := range rows {
		results.Options = append(results.Options, &contract.ParentAnnouncementPollOptionResult{
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
				COALESCE(sm.school_class, '') AS school_class,
				COALESCE(ans.labels, ARRAY[]::text[]) AS answer_labels,
				ans.responded_at AS responded_at,
				EXISTS (SELECT 1 FROM answerable_audience aa WHERE aa.student_id = s.id) AS can_answer
			FROM audience
			JOIN users.student_profiles s ON s.id = audience.student_id` + studentMembershipJoins + `
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
func (p *Projection) PollChildren(ctx context.Context, schoolID, announcementID int64) ([]*contract.ParentAnnouncementPollChildStatus, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []pollChildStatusRow
	if err := db.NewRaw(pollChildrenSQL,
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
		announcementID, schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent announcement poll children: %w", err)
	}
	children := make([]*contract.ParentAnnouncementPollChildStatus, 0, len(rows))
	for _, row := range rows {
		children = append(children, &contract.ParentAnnouncementPollChildStatus{
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
const unansweredRecipientsSQL = reminderRecipientColumns + reachedStudentsBound + pollGuardiansBound + `
			WHERE NOT EXISTS (
					SELECT 1 FROM users.parent_announcement_responses resp
					WHERE resp.announcement_id = s.announcement_id
						AND resp.tenant_id = s.tenant_id
						AND resp.student_id = s.id
				)
			GROUP BY gp.account_id`

// unacknowledgedRecipientsSQL lists the guardians of reached children for
// which NOBODY has confirmed the Elternbrief yet. It is keyed on the CHILD:
// once any guardian confirms, every guardian of that child drops out.
const unacknowledgedRecipientsSQL = reminderRecipientColumns + reachedStudentsBound + portalGuardiansBound + `
			WHERE NOT EXISTS (
					SELECT 1
					FROM (?) osg
					JOIN users.guardian_profiles ogp ON ogp.id = osg.guardian_profile_id
						AND ogp.tenant_id = s.tenant_id AND ogp.account_id IS NOT NULL
					JOIN users.parent_announcement_reads par ON par.announcement_id = s.announcement_id
						AND par.tenant_id = s.tenant_id AND par.account_id = ogp.account_id
					WHERE osg.student_id = s.id AND osg.tenant_id = s.tenant_id
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
func (p *Projection) UnansweredReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*contract.ParentAnnouncementReminderRecipient, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []reminderRecipientRow
	if err := db.NewRaw(unansweredRecipientsSQL,
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent announcement poll reminder recipients: %w", err)
	}
	return reminderRecipients(rows), nil
}

// UnacknowledgedReminderRecipients returns the guardians of reached children
// for which nobody has confirmed the Elternbrief yet.
func (p *Projection) UnacknowledgedReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*contract.ParentAnnouncementReminderRecipient, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []reminderRecipientRow
	if err := db.NewRaw(unacknowledgedRecipientsSQL,
		schoolID, schoolID, today, today, today, announcementID, schoolID, p.guardianLinks(db, schoolID), schoolID, p.guardianLinks(db, schoolID),
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent announcement letter reminder recipients: %w", err)
	}
	return reminderRecipients(rows), nil
}

func reminderRecipients(rows []reminderRecipientRow) []*contract.ParentAnnouncementReminderRecipient {
	recipients := make([]*contract.ParentAnnouncementReminderRecipient, 0, len(rows))
	for _, row := range rows {
		recipients = append(recipients, &contract.ParentAnnouncementReminderRecipient{
			AccountID: row.AccountID, Email: row.Email, FirstName: row.FirstName,
			LastName: row.LastName, PortalLocale: row.PortalLocale,
		})
	}
	return recipients
}
