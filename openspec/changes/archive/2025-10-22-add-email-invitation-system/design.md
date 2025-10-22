# Email Service and User Invitation System Design

## Context

Project Phoenix needs email-based user onboarding to eliminate manual database user creation. The project already has complete email infrastructure (`wneessen/go-mail`, HTML template system with CSS inlining) from a previous version, but it's not integrated into services.

**Stakeholders**: Admin users (sending invitations), new staff/teachers (receiving invitations), end users (password reset)

**Constraints**:
- Current SMTP provider: Strato (must work immediately)
- Must support future provider migration (SendGrid, AWS SES, etc.)
- GDPR compliance: minimal data transmission, audit logging
- Zero downtime: existing manual user creation must keep working

## Goals / Non-Goals

**Goals**:
- Enable admin-initiated user invitations via email
- Complete password reset email integration (service layer already exists)
- Provider-agnostic SMTP configuration
- Secure token-based workflows (invitation + password reset)
- Admin UI for invitation management (send/view/resend/revoke)

**Non-Goals**:
- Email verification for existing users (out of scope)
- Bulk user imports via CSV (separate feature)
- Email templates for activity notifications (future enhancement)
- Two-factor authentication (future enhancement)

## Decisions

### Decision 1: Reuse Existing Email Infrastructure
**What**: Use existing `backend/email/` package (wneessen/go-mail) instead of new library

**Why**:
- Already has template system with HTML/CSS inlining (premailer)
- Mock mailer for testing
- Proven, well-maintained library
- Just needs wiring to services

**Alternatives considered**:
- Migrate to newer library (jordan-wright/email): Unnecessary churn
- Use external email service directly (SendGrid SDK): Vendor lock-in

### Decision 2: Separate Invitation Service from Auth Service
**What**: Create dedicated `InvitationService` interface instead of adding methods to `AuthService`

**Why**:
- Single Responsibility Principle
- Invitation workflow is distinct from authentication
- Easier testing and mocking
- Clear separation of concerns

**Alternatives considered**:
- Extend AuthService: Would violate SRP, makes auth service too large

### Decision 3: Database Schema Design
**What**: `auth.invitation_tokens` table with email, role_id, created_by, expires_at, used_at

**Why**:
- Mirrors existing `password_reset_tokens` pattern (consistency)
- Tracks who sent invitation (audit requirement)
- Supports optional pre-fill (first_name, last_name)
- Indexes on token, email, expires_at for performance

**Alternatives considered**:
- Store invitation data in accounts table: Cannot pre-create account before user accepts
- Use one-time codes instead of UUIDs: Less secure, harder to track

### Decision 4: Token Expiry Times
**What**: Invitations expire in 48 hours, password resets in 30 minutes (configurable via `INVITATION_TOKEN_EXPIRY_HOURS` and `PASSWORD_RESET_TOKEN_EXPIRY_MINUTES`; defaults shown here)

**Why**:
- Invitations: Longer expiry since users may not check email immediately
- Password resets: Short expiry for security (compromised email = immediate risk)
- Industry standard practices (GitHub, GitLab, etc. use similar)

**Alternatives considered**:
- Same expiry for both: Password reset would be too long (security risk)
- Configurable per invitation: Adds complexity without clear benefit

**Implementation notes**:
- `services.NewFactory` reads the two env values via Viper, clamps them to sane minimum/maximum boundaries, and stores the resulting durations on the shared config struct.
- Both `AuthService` and `InvitationService` receive the derived durations through their constructors so `InitiatePasswordReset` and `CreateInvitation` can apply the correct expiry without re-reading configuration or sprinkling magic numbers.
- The durations are passed along when cloning services with `WithTx` to ensure transactional flows use the same configuration snapshot.

### Decision 5: Email Sending Pattern (Fire-and-Forget)
**What**: Email send failures are logged but don't block API responses

**Why**:
- Invitation token is created regardless of email success
- Admin can manually send link or use "resend" feature
- Prevents email server issues from breaking user creation
- Better UX (no waiting for SMTP timeout)

**Alternatives considered**:
- Block until email sends: Poor UX, timeout issues
- Queue-based async: Over-engineered for current scale

### Decision 6: Per-Email Rate Limiting Strategy
**What**: Track password reset attempts per email in database table `auth.password_reset_rate_limits` with 1-hour sliding window, using email as PRIMARY KEY

**Why**:
- Prevents spam/abuse of password reset feature
- Per-email tracking more accurate than per-IP (shared IPs, VPNs)
- Database-backed (no Redis dependency)
- Sliding window algorithm fair and accurate
- Email as PK simplifies upsert logic and lookups

**Schema**:
```sql
CREATE TABLE auth.password_reset_rate_limits (
    email TEXT PRIMARY KEY,
    attempts INT DEFAULT 1,
    window_start TIMESTAMPTZ DEFAULT NOW()
);
```

**Model Design**:
- Does NOT embed `base.Model` (which requires id column)
- Email field is the primary key directly
- Simpler for rate limiting use case (no need for synthetic id)
- BeforeAppendModel() still needed for ModelTableExpr

**Algorithm**:
1. Atomically upsert the row for the normalized email address (`INSERT ... ON CONFLICT ... DO UPDATE`).
2. The upsert increments `attempts` when the existing `window_start` is within the last hour; otherwise it resets `attempts` to 1 and `window_start` to `NOW()`.
3. The statement returns both the current `attempts` and a calculated `retry_at` timestamp (`window_start + INTERVAL '1 hour'`) so callers know exactly when the cooldown expires.
4. Service code checks the returned `attempts`. If the count exceeds the threshold (≥3), it surfaces `ErrRateLimitExceeded`, includes the `retry_at` hint in the error, and the HTTP handler maps it to 429 with a `Retry-After` header.

**Concurrency requirement**:
- Use a single atomic `INSERT ... ON CONFLICT ... DO UPDATE ... RETURNING attempts, window_start + INTERVAL '1 hour' AS retry_at` so concurrent requests cannot slip past the limiter and consumers always receive consistent rate-limit metadata.

**Alternatives considered**:
- Redis-based: Adds infrastructure dependency
- In-memory cache: Doesn't survive restarts, no cluster support
- Per-IP rate limiting: Already exists, not granular enough
- Add id column + base.Model: Unnecessary complexity for this use case

### Decision 7: Cleanup Scheduler Extension with Type Assertions
**What**: Create new `RunCleanupJobs()` method in scheduler using type assertions on existing `interface{}` fields

**Why**:
- Current scheduler stores `authService` as `interface{}` to avoid circular dependency (line 20)
- Uses reflection to call `CleanupExpiredTokens` in `executeTokenCleanup()` (line ~280)
- Adding invitation cleanup needs same pattern
- Type assertions cleaner than continuing reflection for new methods
- Maintains backward compatibility with existing architecture

**Current Structure**:
```go
// In scheduler.go
type Scheduler struct {
    authService    interface{} // Avoid circular dependency
    // ... other fields
}
```

**Implementation**:
```go
// ADD new field to Scheduler struct
type Scheduler struct {
    activeService     active.Service
    cleanupService    active.CleanupService
    authService       interface{} // Existing - keep as interface{}
    invitationService interface{} // NEW - same pattern
    // ... rest
}

// UPDATE constructor signature
func NewScheduler(
    activeService active.Service,
    cleanupService active.CleanupService,
    authService interface{},
    invitationService interface{}, // NEW parameter
) *Scheduler

// NEW METHOD using type assertions
func (s *Scheduler) RunCleanupJobs() error {
    ctx := context.Background()
    var errors []error

    // Password reset tokens (existing method)
    if s.authService != nil {
        method := reflect.ValueOf(s.authService).MethodByName("CleanupExpiredPasswordResetTokens")
        if method.IsValid() {
            results := method.Call([]reflect.Value{reflect.ValueOf(ctx)})
            if len(results) == 2 {
                count := results[0].Int()
                log.Printf("Token cleanup: %d password reset tokens deleted", count)
                if !results[1].IsNil() {
                    errors = append(errors, results[1].Interface().(error))
                }
            }
        }
    }

    // Invitation tokens (new method, same pattern)
    if s.invitationService != nil {
        method := reflect.ValueOf(s.invitationService).MethodByName("CleanupExpiredInvitations")
        if method.IsValid() {
            results := method.Call([]reflect.Value{reflect.ValueOf(ctx)})
            if len(results) == 2 {
                count := results[0].Int()
                log.Printf("Token cleanup: %d invitation tokens deleted", count)
                if !results[1].IsNil() {
                    errors = append(errors, results[1].Interface().(error))
                }
            }
        }
    }

    // Rate limits (new method on authService)
    if s.authService != nil {
        method := reflect.ValueOf(s.authService).MethodByName("CleanupExpiredRateLimits")
        if method.IsValid() {
            results := method.Call([]reflect.Value{reflect.ValueOf(ctx)})
            if len(results) == 2 {
                count := results[0].Int()
                log.Printf("Token cleanup: %d rate limit records deleted", count)
                if !results[1].IsNil() {
                    errors = append(errors, results[1].Interface().(error))
                }
            }
        }
    }

    if len(errors) > 0 {
        return errors[0] // Return first error for simplicity
    }
    return nil
}

// MODIFY existing executeTokenCleanup() - replace reflection with RunCleanupJobs call
func (s *Scheduler) executeTokenCleanup(task *ScheduledTask) {
    // ... existing locking/timing logic ...

    if err := s.RunCleanupJobs(); err != nil {
        log.Printf("Token cleanup failed: %v", err)
    }

    // ... existing completion logic ...
}
```

**Initialization** (in `backend/api/server.go:58`):
```go
srv.scheduler = scheduler.NewScheduler(
    api.Services.Active,
    api.Services.ActiveCleanup,
    api.Services.Auth,
    api.Services.Invitation, // NEW - pass invitation service as interface{}
)
```

**Note**: Continues using reflection pattern to maintain consistency with existing scheduler architecture and avoid circular dependencies.

**CLI Commands**:
- `go run main.go cleanup invitations` - manual invitation cleanup
- `go run main.go cleanup rate-limits` - manual rate limit cleanup
- Both commands added to `cmd/cleanup.go`

**Alternatives considered**:
- **Change to concrete types**: Would break circular dependency avoidance pattern
- **Pure reflection for all**: Already the pattern, just extending it
- **Separate scheduler jobs**: More complex configuration
- **Middleware-based rate limiting**: Would duplicate logic, less flexible

### Decision 8: Email From Field Configuration
**What**: Configure email From field in service factory, default in SMTPMailer if missing

**Why**:
- `email.Message` requires populated From field (smtp.go:58 will error)
- Centralize configuration in one place
- Support override per message if needed

**Implementation**:
```go
// In services/factory.go
mailer, err := email.NewMailer()
defaultFrom := email.NewEmail(
    viper.GetString("email_from_name"),    // "moto"
    viper.GetString("email_from_address"), // "noreply@domain.com"
)

// In services, populate From when missing:
if message.From.Address == "" {
    message.From = s.defaultFrom
}
```

**Environment management**:
- Mirror the EMAIL_* variables in the repository root `.env`/`.env.example` so docker-compose driven setups pick them up automatically, in addition to `backend/dev.env.example` for direct Go runs.

**Alternatives considered**:
- Always require From in every message: Repetitive, error-prone
- Store in SMTPMailer.from: Already there, just needs wiring

### Decision 9: Frontend Route Structure
**What**:
- Public routes: `/invite?token=` (NEW with query param), `/reset-password?token=` (EXISTING - already uses query param)
- Admin routes: `/invitations` (list/send/manage)

**Why**:
- Reuse existing password reset UX pattern at `/reset-password` (already uses ?token= query param)
- Invitation follows same pattern for consistency
- Query params work better with useSearchParams() hook
- Public routes need no authentication
- Admin routes protected by permission middleware
- Clear separation of concerns

**Existing Assets to Update**:
- `app/reset-password/page.tsx` (already exists, uses searchParams.get("token"))
- `components/ui/password-reset-modal.tsx` (already exists)
- `lib/auth-api.ts` (already has requestPasswordReset, resetPassword)
- `app/api/auth/password-reset/route.ts` (ensure email sending)

**New Public Routes**:
- `/invite?token={uuid}` → validates and displays acceptance form
- Email links use query params: `?token=` (NOT path params like `/invite/{token}`)

**Route placement**:
- Keep validation/acceptance routes outside JWT middleware, and mount create/list/resend/revoke under the existing admin-protected group so permission checks stay centralized.
- Sanitize `FRONTEND_URL` once (trim trailing slash) before building either invitation or reset URLs to avoid `//` artifacts.

**Alternatives considered**:
- Path params `/invite/[token]`: Doesn't match existing password reset pattern
- Nested under `/admin/users/invitations`: Too deep, harder to find

### Decision 10: InvitationService Dependencies
**What**: InvitationService requires repositories for accounts, roles, persons, and transaction handling

**Why**:
- AcceptInvitation creates both Account and Person (needs both repos)
- Role assignment needs accountRoleRepo
- Transaction needed for atomic operations
- Password validation shares helpers with AuthService

**Dependencies**:
```go
type InvitationServiceImpl struct {
    invitationRepo      auth.InvitationTokenRepository
    accountRepo         auth.AccountRepository
    roleRepo            auth.RoleRepository
    accountRoleRepo     auth.AccountRoleRepository
    personRepo          users.PersonRepository
    mailer              email.Mailer
    frontendURL         string
    defaultFrom         email.Email
    db                  *bun.DB
}
```

**Transactional clones**:
- Both AuthService and InvitationService `WithTx` helpers must carry the mailer, frontendURL, and defaultFrom fields forward so transactional flows can still send email and build URLs.

**Shared with AuthService**:
- Password hashing: Use `hashPassword()` helper
- Password validation: Use `validatePasswordStrength()` helper
- Error types: Share `ErrWeakPassword`, add invitation-specific errors

**Alternatives considered**:
- Reuse AuthService methods: Circular dependency risk
- Duplicate password logic: DRY violation

### Decision 11: Structured Error Types
**What**: Add invitation-specific errors to `services/auth/errors.go` for proper HTTP mapping

**Why**:
- API handlers need to distinguish 404 (not found) vs 410 (expired/used)
- Consistent error handling across auth domain
- Easier testing and debugging

**New Errors**:
```go
var (
    ErrInvitationNotFound = errors.New("invitation not found")
    ErrInvitationExpired  = errors.New("invitation has expired")
    ErrInvitationUsed     = errors.New("invitation has already been used")
    ErrRateLimitExceeded  = errors.New("too many password reset requests")
)
```

### Implementation Notes

- Map `ErrPasswordTooWeak` (and other validation errors) to HTTP 400 in the reset-password confirm handler so responses match the spec.
- Include a `Retry-After` header on 429 responses from `InitiatePasswordReset` so the frontend timer can rely on server timing.
- Trim invitation/password-reset URLs off a normalized `FRONTEND_URL` helper to prevent double slashes in outbound email links.
- Sanitize `MockMailer` logging to include only recipient, subject, and template—no token payloads—while SMTP is misconfigured.
- When accepting an invitation, perform the Person + Account creation in a single transaction: create the `users.persons` row (using prefilled names when present), create the `auth.accounts` row, link them via `Person.SetAccount`, assign the role, and mark the invitation as used. If the email already maps to an existing account, short-circuit with `ErrEmailAlreadyExists` so the API can render a helpful message instead of bubbling a database unique constraint violation.

**HTTP Mapping**:
- `ErrInvitationNotFound` → 404 Not Found
- `ErrInvitationExpired` → 410 Gone
- `ErrInvitationUsed` → 410 Gone
- `ErrRateLimitExceeded` → 429 Too Many Requests

**Alternatives considered**:
- Generic errors: Loses status code information
- HTTP errors in service layer: Violates separation of concerns

## Risks / Trade-offs

### Risk 1: SMTP Configuration Errors
**Risk**: Incorrect SMTP settings cause email send failures
**Mitigation**:
- Mock mailer fallback (logs to console in development)
- Clear error messages in logs
- Admin UI shows "invitation sent" even if email fails (token created)
- Resend feature allows retry without re-creating invitation

### Risk 2: Token Cleanup
**Risk**: Expired tokens accumulate in database
**Mitigation**:
- Daily cleanup cron job (similar to `CleanupExpiredPasswordResetTokens`)
- Cleanup command in CLI: `go run main.go cleanup tokens`
- Indexes on expiry columns for efficient deletion

### Risk 3: Email Deliverability
**Risk**: Emails land in spam, users don't receive invitations
**Mitigation**:
- Use reputable SMTP provider (Strato initially, easy to switch to SendGrid)
- Proper email headers (SPF, DKIM, DMARC - infrastructure, not code)
- Clear "from" name (moto) and subject lines
- Admin can view pending invitations and manually share links

### Risk 4: Rate Limiting Password Reset
**Risk**: Attackers spam password reset requests
**Mitigation**:
- Implement middleware: 3 requests per hour per email
- Email enumeration already prevented (always return success)
- Audit logging tracks excessive requests

## Migration Plan

**Phase 1 - Email Integration** (~30 min):
1. Add SMTP config to repo root `.env`/`.env.example` and `backend/dev.env.example`
2. Inject mailer into `services/factory.go`
3. Update `InitiatePasswordReset` to send emails
4. Create `password-reset.html` template
5. Test password reset flow manually

**Phase 2 - Invitation Database** (~45 min):
1. Create migration for `auth.invitation_tokens` table
2. Create model with validation
3. Create repository implementation
4. Add repository to factory
5. Run migration and verify schema

**Phase 3 - Invitation Service** (~1 hour):
1. Create `InvitationService` interface
2. Implement service methods (create/validate/accept/resend/revoke)
3. Integrate mailer for invitation emails
4. Create `invitation.html` template
5. Add service to factory
6. Write unit tests

**Phase 4 - API Endpoints** (~1 hour):
1. Create invitation handlers in `api/auth/`
2. Add routes to auth router
3. Wire to service layer
4. Add permission middleware (admin-only for management routes)
5. Test with Bruno API tests

**Phase 5 - Frontend** (~2 hours):
1. Create public invitation acceptance page
2. Create public password reset page
3. Create admin invitation management page
4. Create API client functions
5. Create reusable form components
6. Test end-to-end flow

**Phase 6 - Testing & Documentation** (~30 min):
1. Write Bruno API tests for all endpoints
2. Update CLAUDE.md with email patterns
3. Update README.md with SMTP setup instructions
4. Run full test suite

**Rollback Plan**:
- Phase 1-2: Simply revert commits (no data created yet)
- Phase 3-6: Invitation tokens soft delete (mark as revoked), keep table for audit

## Open Questions

1. **Email template branding**: Logo URL - use hosted image or base64 embed?
   - **Decision**: Use hosted image at `{FRONTEND_URL}/images/moto_transparent.png` for smaller emails
   - **Fallback**: If hosting issues, embed base64 data URI

2. **Password reset token expiry**: Change from 24h to 30min per spec?
   - **Decision**: YES - update `InitiatePasswordReset` at `backend/services/auth/auth_service.go:1330`
   - **Reasoning**: Shorter expiry reduces attack window, matches industry standard

3. **Rate limit cleanup**: How often should we clean up old rate limit records?
   - **Decision**: Daily cleanup alongside token cleanup
   - **Implementation**: Delete records where `window_start < NOW() - INTERVAL '24 hours'`

4. **Invitation email resending**: Should we extend expiry when resending?
   - **Decision**: NO - keep original expiry, admin can create new invitation if expired
   - **Reasoning**: Simpler logic, prevents indefinite extension

5. **Password validation**: Extract to shared helper or duplicate in InvitationService?
   - **Decision**: Extract `validatePasswordStrength()` and `hashPassword()` to `services/auth/password_helpers.go`
   - **Usage**: Both AuthService and InvitationService import shared helpers
