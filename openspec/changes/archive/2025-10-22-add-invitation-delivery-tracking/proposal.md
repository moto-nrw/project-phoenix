## Why
- Admins currently receive HTTP 201 responses for invitations even when SMTP delivery fails, breaking trust in onboarding workflows.
- The `auth.invitation_tokens` table lacks delivery metadata, so support staff cannot audit or retry failed invitations.
- Password reset emails share the same fire-and-forget sending pattern, leading to silent incidents that block user access.

## What Changes
- Persist delivery status and error metadata for invitation and password reset emails.
- Replace the unmanaged goroutine email sends with a tracked async dispatch that records success/failure.
- Surface delivery status through existing admin APIs and add notifications when retries are exhausted.
- Introduce bounded retry logic for transient SMTP failures.

## Impact
- Schema migration for `auth.invitation_tokens` and password reset token storage.
- Service, repository, and API handler updates in the Go backend.
- Admin UI work to display delivery state (future PR).
- New tests covering status transitions and retry behaviour.
