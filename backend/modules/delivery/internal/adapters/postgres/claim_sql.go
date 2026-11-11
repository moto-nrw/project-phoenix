package postgres

import deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

// emailClaimSQL claims system mail, of no school and no Träger, before the
// rest: a demo link, a password reset or a sign-in code is awaited by one
// person, while a school's queue may hold hundreds of invitations. On the
// demo a new school's seed queued some 340 mails per order ahead of the next
// visitor's link (2026-10-27). Security mail of a school ranks with it: an
// invitation must not wait behind that school's letter to all parents (#5932).
// UPDATE RETURNING has no defined order; the outer SELECT preserves priority
// for the worker, which dispatches the claimed batch sequentially.
const emailClaimSQL = `
	WITH claimed AS (
	UPDATE platform.email_outbox
	SET status = 'claimed', lease_token = uuid_generate_v4()::text,
		lease_expires_at = ?, updated_at = ?
	WHERE id IN (
		SELECT candidate.id FROM platform.email_outbox AS candidate
		WHERE ((status = 'pending' AND next_retry_at <= ?)
			OR (status = 'claimed' AND lease_expires_at <= ? AND lease_token NOT LIKE 'dispatch:%'))
			AND ` + holidayClosurePredecessorReadySQL + `
		ORDER BY (security OR (tenant_id IS NULL AND organization_id IS NULL)) DESC, next_retry_at, id
		FOR UPDATE SKIP LOCKED LIMIT ?
	) RETURNING *
	)
	SELECT * FROM claimed
	ORDER BY (security OR (tenant_id IS NULL AND organization_id IS NULL)) DESC, next_retry_at, id`

const pushClaimSQL = `
	UPDATE platform.push_outbox
	SET status = 'claimed', lease_token = uuid_generate_v4()::text,
		lease_expires_at = ?, updated_at = ?
	WHERE id IN (
		SELECT id FROM platform.push_outbox
		WHERE (status = 'pending' AND next_retry_at <= ?)
			OR (status = 'claimed' AND lease_expires_at <= ? AND lease_token NOT LIKE 'dispatch:%')
		ORDER BY next_retry_at, id FOR UPDATE SKIP LOCKED LIMIT ?
	) RETURNING *`

// A failed or unknown predecessor must remain ahead of its correction,
// including when another worker skips its lock or it awaits a retry click.
const holidayClosurePredecessorReadySQL = `(candidate.kind <> 'holiday_closure_notice' OR NOT EXISTS (
 SELECT 1 FROM platform.email_outbox AS predecessor
 WHERE (predecessor.tenant_id = candidate.tenant_id OR (predecessor.tenant_id IS NULL AND candidate.tenant_id IS NULL))
  AND (predecessor.organization_id = candidate.organization_id OR (predecessor.organization_id IS NULL AND candidate.organization_id IS NULL))
  AND predecessor.kind = 'holiday_closure_notice'
  AND lower(trim(predecessor.recipient->>'address')) = lower(trim(candidate.recipient->>'address'))
  AND predecessor.id < candidate.id
  AND (predecessor.status NOT IN ('sent', 'cancelled') OR ` + holidayClosureRecoverableCancellationSQL + `)
))`

// Suppression and a delivery stop can be lifted; retrying that same mail
// must still precede every correction to it.
const holidayClosureRecoverableCancellationSQL = `(predecessor.status = 'cancelled'
 AND predecessor.last_error IN ('` + deliveryContract.MailHeldByList + `', '` + deliveryContract.MailHeldByStop + `'))`
