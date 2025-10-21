## Overview
Silent email failures block onboarding and password recovery. We will track delivery status in the database, move send operations into an observable dispatcher, and notify admins when retries fail.

## Key Decisions
1. **Delivery Metadata**: Add `email_sent_at TIMESTAMPTZ`, `email_error TEXT`, and `email_retry_count INT` columns to `auth.invitation_tokens`. Mirror a lightweight status enum (`pending`, `sent`, `failed`) derived from these fields to simplify API payloads.
2. **Dispatcher Abstraction**: Wrap the existing `email.Mailer` behind a new `email.Dispatcher` that performs the send asynchronously but reports result via callback to persist success/failure. This enables reuse for invitations and password resets.
3. **Retry Strategy**: Implement exponential backoff with a cap (e.g., 3 attempts over 15 minutes) using a worker queue seeded from a new `email_outbox` table or in-memory scheduler. MVP keeps it simple: enqueue tasks in a channel serviced by a background worker in the auth service process.
4. **Notifications**: Emit structured logs + optional admin alert (email or dashboard flag) when attempts exceed the retry budget, surfacing actionable errors.

## Open Questions
- Should we generalize delivery tracking across all outbound emails (new table) or keep fields on invitations/reset tokens for now? Starting with per-feature fields to minimize scope; revisit abstraction later.
- Do we need a UI-ready status enum or will the frontend derive it from timestamps/errors? Proposal leans toward backend-provided enum for consistency.

## Risks & Mitigations
- **Schema Drift**: Ensure migrations include backfill defaults for existing rows and production data size is modest; use transaction-safe migrations.
- **Worker Reliability**: Background worker must start with the API server. Add health logging and ensure graceful shutdown drains the queue.
- **Error Flooding**: Cap stored error text length (truncate) and redact sensitive SMTP info before persisting.
