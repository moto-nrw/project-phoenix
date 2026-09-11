package parentaudience

// Every statement in this projection is a compile-time constant so the
// architecture evaluator can see which tables it reads. The fragments below
// are the shared building blocks; each exported query assembles them once and
// documents its bind order, because a `?` binds in textual order no matter
// which fragment it came from.

// activeEnrollmentBound renders the activity_group branch of a target match:
// the student has an enrollment in pt.target_ref_id that is active on the
// bound calendar day. It mirrors the timetable's active-enrollment rule:
// valid_until is EXCLUSIVE and weekday is either unscoped or matches the ISO
// weekday. The school is bound because the surrounding query binds it too.
//
// Bind order: school, today, today, today.
const activeEnrollmentBound = `(pt.target_type = 'activity_group' AND EXISTS (
						SELECT 1 FROM activities.student_enrollments se
						WHERE se.student_id = s.id AND se.tenant_id = ?
							AND se.activity_group_id = pt.target_ref_id
							AND se.valid_from <= ?::date
							AND (se.valid_until IS NULL OR se.valid_until > ?::date)
							AND (se.weekday IS NULL OR se.weekday = date_part('isodow', ?::date)::INT)
					))`

// activeEnrollmentFeed is activeEnrollmentBound for the cross-school feed,
// where the school comes from the announcement row.
//
// Bind order: today, today, today.
const activeEnrollmentFeed = `(pt.target_type = 'activity_group' AND EXISTS (
						SELECT 1 FROM activities.student_enrollments se
						WHERE se.student_id = s.id AND se.tenant_id = a.tenant_id
							AND se.activity_group_id = pt.target_ref_id
							AND se.valid_from <= ?::date
							AND (se.valid_until IS NULL OR se.valid_until > ?::date)
							AND (se.weekday IS NULL OR se.weekday = date_part('isodow', ?::date)::INT)
					))`

// studentTargetMatchPrefix is the OR-union of the student-based selectors;
// the activity_group branch is appended by the variants below.
const studentTargetMatchPrefix = `
					pt.target_type = 'school_all'
					OR (pt.target_type = 'class' AND LOWER(TRIM(s.school_class)) = LOWER(TRIM(pt.target_ref_text)))
					OR (pt.target_type = 'group' AND s.group_id = pt.target_ref_id)
					OR (pt.target_type = 'student' AND s.id = pt.target_ref_id)
					OR `

// Bind order: school, today, today, today.
const studentTargetMatchBound = studentTargetMatchPrefix + activeEnrollmentBound

// Bind order: today, today, today.
const studentTargetMatchFeed = studentTargetMatchPrefix + activeEnrollmentFeed

// reachedStudentsBound joins the announcement's targets to every live,
// non-graduated child they reach. Graduated (alumnus) students are
// soft-deleted: their guardians drop out of every audience once the child
// has left the OGS.
//
// Bind order: school, school, today, today, today.
const reachedStudentsBound = `
			FROM users.parent_announcement_targets pt
			JOIN users.students s ON s.tenant_id = ? AND (` + studentTargetMatchBound + `
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			AND s.status <> 'alumnus'`

// portalGuardiansBound continues reachedStudentsBound with the guardians who
// hold parent_portal.access on the child, a linked account, and an ACTIVE
// membership at the school: a guardian whose mapping went pending or inactive
// keeps the relationship rows but has lost portal access.
//
// Bind order: school, school.
const portalGuardiansBound = `
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id IS NOT NULL
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'`

// pollGuardiansBound is portalGuardiansBound restricted to guardians who may
// also answer polls for the child.
//
// Bind order: school, school.
const pollGuardiansBound = `
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
				 AND sg.permissions @> '{"parent_portal.poll.response": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id IS NOT NULL
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'`

// pollGuardiansForAccountBound is pollGuardiansBound narrowed to one account.
//
// Bind order: school, school, account.
const pollGuardiansForAccountBound = `
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
				 AND sg.permissions @> '{"parent_portal.poll.response": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id IS NOT NULL AND gp.account_id = ?
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'`

// announcementTargetsBound closes a reached-students query on one
// announcement.
//
// Bind order: announcement, school.
const announcementTargetsBound = `
			WHERE pt.announcement_id = ? AND pt.tenant_id = ?`

// pendingApplicantsCTE binds the Enrollment-owned applicant rows as a record
// set, so the audience can be joined without reading Enrollment's tables.
//
// Bind order: applicants (jsonb).
const pendingApplicantsCTE = `WITH pending_applicants AS (
			 SELECT * FROM jsonb_to_recordset(?::jsonb) AS applicant(guardian_account_id bigint, guardian_email text, guardian_first_name text, guardian_last_name text)
			)`

// pendingApplicantsFeedCTE is the cross-school variant with the school on
// every row.
//
// Bind order: applicants (jsonb).
const pendingApplicantsFeedCTE = `WITH pending_applicants AS (
			 SELECT * FROM jsonb_to_recordset(?::jsonb) AS applicant(
			 tenant_id bigint, guardian_account_id bigint, guardian_email text
			 )
			) `

// pendingApplicantAccountsBound resolves the pending_enrollment target to
// accounts: a stamped guardian_account_id, or an unstamped request whose
// guardian_email resolves to an account (the ListByAccount e-mail fallback).
//
// Bind order: announcement, school.
const pendingApplicantAccountsBound = `
			FROM users.parent_announcement_targets pt
			CROSS JOIN pending_applicants req
			LEFT JOIN auth.accounts ea ON req.guardian_account_id IS NULL
				AND ea.email IS NOT NULL
				AND LOWER(TRIM(ea.email)) = LOWER(TRIM(req.guardian_email))
			WHERE pt.announcement_id = ? AND pt.tenant_id = ? AND pt.target_type = 'pending_enrollment'`

// audienceAccountsBound is the distinct guardian ACCOUNT set an announcement
// reaches right now: the student-based targets plus the pending_enrollment
// target.
//
// Bind order: school, school, today, today, today, school, school,
// announcement, school, announcement, school.
const audienceAccountsBound = `
			SELECT DISTINCT gp.account_id AS account_id` + reachedStudentsBound + portalGuardiansBound + announcementTargetsBound + `
			UNION
			SELECT DISTINCT COALESCE(req.guardian_account_id, ea.id) AS account_id` + pendingApplicantAccountsBound + `
				AND COALESCE(req.guardian_account_id, ea.id) IS NOT NULL`

// reachedAccountFeed is the SQL boolean "the bound account is reached by
// announcement a right now" for the cross-school feed, where the announcement
// and school are column references.
//
// Bind order: today, today, today, account, account, account.
const reachedAccountFeed = `(
		EXISTS (
			SELECT 1
			FROM users.parent_announcement_targets pt
			JOIN users.students s ON s.tenant_id = a.tenant_id AND (` + studentTargetMatchFeed + `
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			AND s.status <> 'alumnus'
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = a.tenant_id
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = a.tenant_id
				AND gp.account_id = ?
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE pt.announcement_id = a.id AND pt.tenant_id = a.tenant_id
		)
		OR EXISTS (
			SELECT 1
			FROM users.parent_announcement_targets pt
			JOIN pending_applicants req ON req.tenant_id = a.tenant_id
				AND (
					req.guardian_account_id = ?
					OR (
						req.guardian_account_id IS NULL
						AND EXISTS (
							SELECT 1 FROM auth.accounts ea
							WHERE ea.id = ? AND ea.email IS NOT NULL
								AND LOWER(TRIM(req.guardian_email)) = LOWER(TRIM(ea.email))
						)
					)
				)
			WHERE pt.announcement_id = a.id AND pt.tenant_id = a.tenant_id
				AND pt.target_type = 'pending_enrollment'
		)
	)`

// reachedAccountBound is reachedAccountFeed for one bound announcement.
//
// Bind order: school, school, today, today, today, school, school, account,
// announcement, school, school, account, account, announcement, school.
const reachedAccountBound = `(
		EXISTS (
			SELECT 1` + reachedStudentsBound + `
			JOIN users.students_guardians sg ON sg.student_id = s.id AND sg.tenant_id = ?
				AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
				AND gp.account_id = ?
			JOIN auth.account_tenants act ON act.account_id = gp.account_id
				AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			WHERE pt.announcement_id = ? AND pt.tenant_id = ?
		)
		OR EXISTS (
			SELECT 1
			FROM users.parent_announcement_targets pt
			JOIN pending_applicants req ON req.tenant_id = ?
				AND (
					req.guardian_account_id = ?
					OR (
						req.guardian_account_id IS NULL
						AND EXISTS (
							SELECT 1 FROM auth.accounts ea
							WHERE ea.id = ? AND ea.email IS NOT NULL
								AND LOWER(TRIM(req.guardian_email)) = LOWER(TRIM(ea.email))
						)
					)
				)
			WHERE pt.announcement_id = ? AND pt.tenant_id = ?
				AND pt.target_type = 'pending_enrollment'
		)
	)`

// openPollForAccountFeed is the SQL boolean "announcement a is an open poll
// and at least one of the bound account's reached children has no answer
// yet". A guardian who read a poll but never answered still counts as
// outstanding, because a poll that quietly stops nagging is a poll nobody
// answers.
//
// Bind order: today, today, today, account.
const openPollForAccountFeed = `(
		a.response_type <> 'none'
		AND (a.response_deadline IS NULL OR a.response_deadline > NOW())
		AND EXISTS (
			SELECT 1
			FROM users.parent_announcement_targets pt
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
			WHERE pt.announcement_id = a.id AND pt.tenant_id = a.tenant_id
				AND NOT EXISTS (
					SELECT 1 FROM users.parent_announcement_responses resp
					WHERE resp.announcement_id = a.id AND resp.student_id = s.id
				)
		)
	)`

// feedScopePredicate selects hand-written rows from schools with news on and
// system-authored rows from schools with the cancellation-notice gate on.
//
// Bind order: news schools, notice-only schools.
const feedScopePredicate = `((a.tenant_id IN (?) AND a.system_kind IS NULL) OR (a.tenant_id IN (?) AND a.system_kind IS NOT NULL))`

// liveAnnouncementPredicate is "published, active, and not expired".
const liveAnnouncementPredicate = `
				AND a.active
				AND a.published_at IS NOT NULL
				AND a.published_at <= NOW()
				AND (a.expires_at IS NULL OR a.expires_at > NOW())`

// pollAudienceStudentsBound is every portal-visible child a poll reaches.
// Staff results must include every child the announcement reaches, even if
// no guardian may answer the poll.
//
// Bind order: school, school, today, today, today, school, school,
// announcement, school.
const pollAudienceStudentsBound = `
			SELECT DISTINCT s.id AS student_id` + reachedStudentsBound + portalGuardiansBound + announcementTargetsBound

// pollAnswerableStudentsBound is the subset of the audience for which at least
// one guardian currently has poll.response: the completion denominator and
// reminder source.
//
// Bind order: school, school, today, today, today, school, school,
// announcement, school.
const pollAnswerableStudentsBound = `
			SELECT DISTINCT s.id AS student_id` + reachedStudentsBound + pollGuardiansBound + announcementTargetsBound

// pollAnswerableStudentsForAccountBound narrows pollAnswerableStudentsBound to
// one guardian's children.
//
// Bind order: school, school, today, today, today, school, school, account,
// announcement, school.
const pollAnswerableStudentsForAccountBound = `
			SELECT DISTINCT s.id AS student_id` + reachedStudentsBound + pollGuardiansForAccountBound + announcementTargetsBound

// letterReachedStudentsBound is every child the announcement's targets reach
// with NO guardian requirement at all. A letter to the whole school reaches
// every child in it; whether anyone can confirm for a given child is a
// separate question.
//
// Bind order: school, school, today, today, today, announcement, school.
const letterReachedStudentsBound = `
			SELECT DISTINCT s.id AS student_id` + reachedStudentsBound + announcementTargetsBound
