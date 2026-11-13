package parentaudience

// The recipients a publication recorded (#6988) in
// users.parent_announcements.recipient_account_ids. A published announcement
// with a recorded array counts and lists these accounts instead of the
// audience of today: a child who changes class after the publication must not
// move the e-mail recipients of the old class out of the statistics and the
// parents of the new class in. NULL means not recorded (a draft, or an
// announcement published before recording); those keep the audience of today.

// recordedMarkBound is the one-row CTE body telling whether the bound
// announcement recorded its recipients.
//
// Bind order: announcement, school.
const recordedMarkBound = `SELECT ra.recipient_account_ids IS NOT NULL AS yes
			FROM users.parent_announcements ra
			WHERE ra.id = ? AND ra.tenant_id = ?`

// recordedAccountsBound is every recorded recipient that still belongs to
// the school: an active membership, or, for a pending_enrollment recipient,
// a request that is still pending. A person who left the school drops out
// (an account erasure removes the membership too); nobody is added. It needs
// the pending_applicants CTE of pendingApplicantsCTE in scope.
//
// Bind order: announcement, school.
const recordedAccountsBound = `
			SELECT DISTINCT rr.account_id
			FROM users.parent_announcements ra
			CROSS JOIN LATERAL unnest(ra.recipient_account_ids) AS rr(account_id)
			WHERE ra.id = ? AND ra.tenant_id = ?
				AND (
					EXISTS (
						SELECT 1 FROM auth.account_tenants act
						WHERE act.account_id = rr.account_id AND act.tenant_id = ra.tenant_id
							AND act.status = 'active'
					)
					OR EXISTS (
						SELECT 1 FROM pending_applicants req
						WHERE req.guardian_account_id = rr.account_id
							OR (
								req.guardian_account_id IS NULL
								AND EXISTS (
									SELECT 1 FROM auth.accounts ea
									WHERE ea.id = rr.account_id AND ea.email IS NOT NULL
										AND LOWER(TRIM(ea.email)) = LOWER(TRIM(req.guardian_email))
								)
							)
					)
				)`

// recipientAccountsBound is the account set Stats counts: the recorded
// recipients once the publication recorded them, the audience of today
// before. Postgres evaluates the uncorrelated mark once and skips the branch
// it rules out. It needs a CTE named recorded built from recordedMarkBound.
//
// Bind order: recordedAccountsBound, then audienceAccountsBound.
const recipientAccountsBound = recordedAccountsBound + `
				AND COALESCE((SELECT yes FROM recorded), FALSE)
			UNION
			SELECT live.account_id FROM (` + audienceAccountsBound + `
			) live
			WHERE NOT COALESCE((SELECT yes FROM recorded), FALSE)`

// recordedPortalAccessFrom proves that a guardian account may still open the
// parent portal at a school: an active membership and parent_portal.access on
// at least one live, non-graduated child. Membership alone authorizes no
// parent view, so a recorded guardian who lost portal access loses the
// recorded announcement with it. The caller adds the WHERE on act.
//
// Bind order: guardian links.
const recordedPortalAccessFrom = `
				SELECT 1 FROM auth.account_tenants act
				JOIN users.guardian_profiles gp ON gp.account_id = act.account_id AND gp.tenant_id = act.tenant_id
				JOIN (?) sg ON sg.guardian_profile_id = gp.id AND sg.tenant_id = act.tenant_id
					AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
				JOIN users.student_profiles s ON s.id = sg.student_id AND s.tenant_id = act.tenant_id` + studentMembershipJoins + `
				JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
					AND sm.status <> 'alumnus'`

// recordedRecipientFeed is the SQL boolean "the bound account was a recorded
// recipient of announcement a and may still open the portal there". The feed
// shows an announcement to everyone it reaches today and, in addition, to
// the parents it reached at publication: a parent whose child changed class
// can still read and confirm the letter that was e-mailed to them.
//
// Bind order: account, guardian links, account.
const recordedRecipientFeed = `(
			? = ANY(a.recipient_account_ids)
			AND EXISTS (` + recordedPortalAccessFrom + `
				WHERE act.account_id = ? AND act.tenant_id = a.tenant_id AND act.status = 'active'
			)
		)`

// recordedRecipientBound is recordedRecipientFeed for one bound announcement.
//
// Bind order: announcement, school, account, guardian links of the school,
// account, school.
const recordedRecipientBound = `(
		EXISTS (
			SELECT 1 FROM users.parent_announcements ra
			WHERE ra.id = ? AND ra.tenant_id = ? AND ? = ANY(ra.recipient_account_ids)
		)
		AND EXISTS (` + recordedPortalAccessFrom + `
			WHERE act.account_id = ? AND act.tenant_id = ? AND act.status = 'active'
		)
	)`
