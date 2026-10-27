package postgres

// emailClaimSQL claims system mail, of no school and no Träger, before the
// rest: a demo link, a password reset or a sign-in code is awaited by one
// person, while a school's queue may hold hundreds of invitations. On the
// demo a new school's seed queued some 340 mails per order ahead of the next
// visitor's link (2026-10-27).
const emailClaimSQL = `
	UPDATE platform.email_outbox
	SET status = 'claimed', lease_token = uuid_generate_v4()::text,
		lease_expires_at = ?, updated_at = ?
	WHERE id IN (
		SELECT id FROM platform.email_outbox
		WHERE (status = 'pending' AND next_retry_at <= ?)
			OR (status = 'claimed' AND lease_expires_at <= ? AND lease_token NOT LIKE 'dispatch:%')
		ORDER BY (tenant_id IS NULL AND organization_id IS NULL) DESC, next_retry_at, id
		FOR UPDATE SKIP LOCKED LIMIT ?
	) RETURNING *`

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
