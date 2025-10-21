# Add Email Service and User Invitation System

## Why

Currently, user creation requires manual database intervention where admins must:
- Directly access the database
- Manually create user records
- Set initial passwords themselves
- No automated notification to new users

This creates security risks (admins know passwords), poor UX, and doesn't scale for production use. Additionally, password reset functionality exists at the database/service layer but has no email integration, rendering it unusable in production.

The project already has complete email infrastructure (wneessen/go-mail library, template system with HTML/CSS inlining) from a previous version, but it's not integrated into services or configured.

## What Changes

- **Email Service Integration** (existing infrastructure, just needs wiring):
  - Add SMTP configuration to repo root `.env`/`.env.example` (docker compose) and `backend/dev.env.example` (direct Go) so every environment shares the same EMAIL_* values
  - Inject mailer into service factory
  - Centralize token expiry durations: read `INVITATION_TOKEN_EXPIRY_HOURS` and `PASSWORD_RESET_TOKEN_EXPIRY_MINUTES` once in the factory and pass them into auth/invitation services
  - Create email templates (invitation.html, password-reset.html) with moto branding
  - Update password reset service to send emails (currently creates tokens but doesn't notify users)

- **User Invitation System** (new capability):
  - Database: New `auth.invitation_tokens` table (similar to existing `password_reset_tokens`)
  - Service: New invitation service with create/validate/accept/resend/revoke operations
  - API: 6 new endpoints (admin: create/list/revoke/resend, public: validate/accept)
  - Frontend: Public invitation acceptance page + password reset page
  - Admin UI: Send invitations, view pending, resend/revoke

- **Security Best Practices**:
  - Cryptographically secure UUID v4 tokens
  - Email enumeration prevention (already implemented for password reset)
  - Rate limiting for password reset (3 requests/hour per email string, even if no account exists) using atomic upserts that return both attempt counts and retry deadlines, allowing the API to emit a precise `Retry-After` header
  - Token expiry: invitations 48h, password reset 30min
  - Audit logging (using `log.Printf`) for all invitation/password reset lifecycle events (create, resend, revoke, accept, reset)
  - HTTPS-only email links in production
  - Mock mail logging redacts token payloads while SMTP is misconfigured

- **Provider-Agnostic Design**:
  - SMTP config uses standard host/port/user/password (no vendor lock-in)
  - Easy migration from Strato to SendGrid/Mailgun/AWS SES/Postmark
  - Mock mailer for testing

## Impact

- **Affected specs**:
  - New: `email-service` (SMTP integration, template system)
  - New: `user-invitations` (invitation workflow, token management)
  - Modified: `password-reset` (add email notification)

- **Affected code**:
  - Backend (modified):
    - `services/factory.go` (inject mailer, configure email From address, add HTTPS validation, compute shared token expiry durations)
    - `services/auth/auth_service.go` (add async email sending to InitiatePasswordReset, swap hard-coded expiry for configured duration, add rate limit checks with retry metadata, normalize frontend URL, ensure `WithTx` retains mailer/defaultFrom/frontendURL/durations, add audit logging)
    - `services/auth/interface.go` (add CleanupExpiredRateLimits method)
    - `services/auth/errors.go` (add ErrInvitationExpired, ErrInvitationUsed, ErrInvitationNotFound, ErrRateLimitExceeded)
    - `services/scheduler/scheduler.go` (add RunCleanupJobs method, extend executeTokenCleanup)
    - `api/auth/api.go` (fix reset password error mapping 400/500 including weak-password → 400, add Retry-After header on rate limit responses using service-provided timestamps, add audit logging, mount invitation routes in correct public/admin groups)
    - `cmd/cleanup.go` (add invitations and rate-limits subcommands)
    - `.env.example` & root `.env` (add EMAIL_* config for docker compose)
    - `backend/dev.env.example` (add EMAIL_* config)
    - `email/smtp.go` (ensure From field defaulting and add audit logging in Send())
    - `email/mockMailer.go` (log only recipient/subject/template to avoid leaking tokens)
    - `templates/email/footer.html` (change "GoBase" to "moto")
  - Backend (new):
    - `models/auth/invitation_token.go` (model + validation)
    - `models/auth/password_reset_rate_limit.go` (rate limit model, email as PK)
    - `database/repositories/auth/invitation_token.go` (repository)
    - `database/repositories/auth/password_reset_rate_limit.go` (per-email rate tracking repository with atomic upsert logic)
    - `database/migrations/*_auth_invitation_tokens.go` (migration)
    - `database/migrations/*_auth_password_reset_rate_limits.go` (migration for rate limiting)
    - `services/auth/invitation_service.go` (business logic with accountRoleRepo, duplicates guard for existing accounts, transaction handling, URL normalization helper, `WithTx` propagation)
    - `services/auth/password_helpers.go` (shared password validation and hashing)
    - `api/auth/invitation_handlers.go` (HTTP handlers)
    - `templates/email/invitation.html` (email template)
    - `templates/email/password-reset.html` (email template)
  - Frontend (modified):
    - `app/reset-password/page.tsx` (update existing password reset page)
    - `components/ui/password-reset-modal.tsx` (update existing modal)
    - `lib/auth-api.ts` (update existing password reset functions)
    - `app/api/auth/password-reset/route.ts` (ensure email sending)
    - `app/api/auth/password-reset/confirm/route.ts` (keep existing)
  - Frontend (new):
    - `app/(public)/invite/page.tsx` (invitation acceptance with query param ?token=)
    - `components/auth/invitation-accept-form.tsx` (invitation UI)
    - `app/invitations/page.tsx` (admin invitation management)
    - `app/api/invitations/validate/route.ts` (proxy validation endpoint)
    - `app/api/invitations/accept/route.ts` (proxy acceptance endpoint)
    - `lib/invitation-api.ts` (API client)
    - `lib/invitation-helpers.ts` (type mapping)

- **Breaking changes**: None - pure addition, existing manual user creation still works

- **GDPR compliance**: Minimal data in email templates (no sensitive student data), audit logging required

- **Performance**: Negligible overhead (email sending async, doesn't block API responses)

- **Ops visibility**: New CLI cleanup subcommands (invitations, rate-limits) and the extended scheduler job should be documented so operations can schedule manual runs if automation is disabled
