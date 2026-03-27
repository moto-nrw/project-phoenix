# Review: `settings-v1-plan.md`

**Author:** yungweng
**Date:** 2026-03-27T16:00:00+02:00

Review of Chris's v1 plan (`docs/settings-v1-plan.md`) against the actual codebase.

---

## Verdict

Good plan. The RFC → v1 trimming landed in the right place. Six issues found — two are high severity and should be resolved before implementation starts.

---

## Issues

### 1. Per-tenant scheduling design is missing (HIGH)

The plan says "scheduler iterates all active tenants for each job" — but the real question is: **how do you fire a job at 18:00 for Tenant A and 16:30 for Tenant B?**

The scheduler currently registers a single cron trigger per job (e.g., session-end fires at the global `SESSION_END_TIME`). Per-tenant times break this model.

The scheduler already has `forEachTenant()` at `services/scheduler/scheduler.go:125-159` using `schoolRepo.ListActive()` + `tenant.WithTenantTx()`. The per-tenant execution path exists. The per-tenant **trigger timing** does not.

**Options to evaluate:**
- **(a) Minute-polling:** Run a check every minute, resolve each tenant's configured time, fire if `now == tenant_time`. Simple, slight delay (up to 60s), easy to reason about.
- **(b) Earliest-trigger:** Schedule one global job at the earliest tenant time. On each tick, resolve all tenants, skip those whose time hasn't come, re-schedule next tick for the next tenant's time.
- **(c) Dynamic cron per tenant:** Register/deregister cron entries when tenant settings change. Most precise, most complex.

**Recommendation:** Option (a). It's the simplest and the 60s delay is acceptable for session-end and cleanup jobs. Document this decision in the plan.

---

### 2. Existing `config.settings` compat wrapper belongs in Phase 1, not Phase 4 (HIGH)

The current `config.settings` table is **actively used in production**. `GetTimeoutSettings()` at `services/config/config_service.go:538-579` reads session timeout, warning threshold, and check interval from the old table on every call. The IoT resource injects `ConfigService` at `api/iot/api.go:44,176`.

The plan puts the compat wrapper in Phase 4 ("Not in v1"). This means the old `config.settings` table runs in parallel with the new `config.setting_values` table indefinitely, with no connection between them.

**Fix:** Move the compat wrapper into Phase 1. It's a thin layer — the existing `GetStringValue()` / `GetBoolValue()` methods delegate to the new resolver instead of the old repo. Without this, you ship a new settings system that ignores the settings already in production.

---

### 3. PIN hashing is pointless for 4-digit codes (MEDIUM)

The plan says "Password type means PIN stored hashed (Argon2id)" (line 194). This doesn't work well:

- The OGS PIN is a **4-digit numeric code** (10,000 possibilities). Argon2id hashing doesn't meaningfully protect it — brute force takes seconds regardless.
- The plan says "show '••••••' when set" — but school admins need to **share the PIN with staff**. If it's hashed, there's no way to reveal it. The plan doesn't include a "reveal" button or alternative flow.
- The current comparison in `auth/device/device_auth.go:232` uses `SecureCompareStrings()` (constant-time). Switching to `userpass.VerifyPassword()` (Argon2id verify) adds latency to every device auth request for no real security gain.

**Recommendation:** Store the PIN as plain text in the DB. It's already behind RLS + tenant isolation. Mask it in the settings UI if desired (password field type for display), but don't hash it on the backend. The security win is making it per-tenant, not hashing a 4-digit code.

---

### 4. Init-time vs runtime env vars need different refactors (MEDIUM)

The plan treats all 11 settings as equivalent. They're not. The scheduler has two consumption patterns:

**Init-time (stored in struct fields, used by goroutines):**
- `SESSION_CLEANUP_INTERVAL_MINUTES` — parsed at `scheduler.go:710-723`, stored in `s.sessionCleanupIntervalMinutes`, captured by goroutine at line 736-737
- `BREAK_AUTO_END_INTERVAL_SECONDS` — parsed at `scheduler.go:830-835`, stored in `s.breakAutoEndIntervalSeconds`

Changing these per-tenant means changing the goroutine's tick interval, not just swapping a value lookup. This is a structural refactor.

**Runtime (re-read every execution):**
- `CLEANUP_SCHEDULER_TIMEOUT_MINUTES` — re-read at `scheduler.go:300-306` every cleanup run
- `SESSION_END_TIME` — read at `scheduler.go:198-200` for scheduling

These are easy to swap for a settings resolver call.

**Per-request (no caching):**
- `OGS_DEVICE_PIN` — read at `device_auth.go:224` on every auth attempt
- `STUDENT_DAILY_CHECKOUT_TIME` — parsed at `checkin/helpers.go:16-40` on every checkout check

These are trivial to swap.

**Recommendation:** Categorize the settings in the plan by consumption pattern. The init-time ones need design attention; the runtime and per-request ones are straightforward.

---

### 5. `BREAK_AUTO_END_INTERVAL_SECONDS` is missing (LOW)

The scheduler also reads `BREAK_AUTO_END_INTERVAL_SECONDS` at `scheduler.go:830-835` — same pattern as the other scheduler env vars (init-time, struct field). It controls how often the break auto-end check runs.

Not in the 11 settings list. Either add it as a 12th setting or explicitly note it's deferred.

---

### 6. Frontend needs re-fetch after DELETE/reset (LOW)

The plan says "Save on change (debounced PUT)" and "Reset button → DELETE". But after a DELETE, the parent setting reverts to its registry default, which may change `DependsOn` visibility for child settings.

The frontend needs to either:
- Re-fetch the schema after a successful DELETE (simplest)
- Optimistically update the local value to the known default from the schema response

Minor, but should be noted in the Phase 3 spec to avoid a bug where child fields stay hidden/visible after a parent reset.

---

## Things That Are Correct (Don't Change)

- **Migration number `001015019`** — confirmed next in sequence after `001015018`.
- **Device auth has tenant context** — `device_auth.go:245-247` injects tenant ID after API key lookup. The settings resolver will work without extra plumbing.
- **6 field types** — password and select are justified additions over the original 4.
- **Audit table with no UI** — cheap writes, useful for debugging PIN changes later.
- **3 API endpoints** — sufficient for v1.
- **`DependsOn` kept** — 3 of the 11 settings groups genuinely use the enabled-toggle pattern.
- **Scheduler refactor in Phase 2** — correct ordering. Settings backend first, then wire it into the real consumers.

---

## Should NOT Address in v1

- Per-device or per-room settings
- PIN complexity validation (it's a 4-digit code, not a password)
- Dynamic select options
- Audit API or UI
- Settings search, i18n, import/export
- Custom renderers

---

## Action Items

| # | Issue | Severity | Action |
|---|-------|----------|--------|
| 1 | Per-tenant scheduling design missing | **High** | Decide on minute-polling vs earliest-trigger vs dynamic cron |
| 2 | Compat wrapper for existing config.settings | **High** | Move from Phase 4 to Phase 1 |
| 3 | PIN hashing pointless for 4-digit codes | Medium | Store plain text, mask in UI only |
| 4 | Init-time vs runtime env var patterns | Medium | Categorize settings by consumption pattern in the plan |
| 5 | `BREAK_AUTO_END_INTERVAL_SECONDS` missing | Low | Add as 12th setting or explicitly defer |
| 6 | Frontend re-fetch after DELETE | Low | Note in Phase 3 spec |
