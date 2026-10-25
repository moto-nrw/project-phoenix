# Cross-stack contracts

Read the relevant section before changing auth, portal routing, enrollment,
tenant scoping, IoT endpoints, or kiosk presence modes. Paths start at the
repository root unless the section says otherwise.

## Ecosystem and IoT

The sibling `../PyrePortal/` repository runs the Raspberry Pi kiosk (Tauri +
React). The Phoenix backend runs on the server, never on the Pi. PRs target
`development`.

PyrePortal consumes `/api/iot/*` using a device API key and staff PIN.
`../PyrePortal/src/services/apiErrors.ts` maps registry error codes (its
`errorCodes.generated.ts` comes from `error-registry.json` via
`pnpm run sync:error-codes`) to German UI text. Since #2508 the kiosk matches
no backend text; the refusal body is the RFC 9457 envelope of ADR 0006 with
`status` as the HTTP status number, the diagnostic text in `detail` and the
identity in `code` (#2520). Coordinate endpoint, code and auth-header changes
across repos. Changing a code the kiosk branches on is a two-repo change.
PyrePortal's `CODE_MESSAGES` maps 35 codes; the backend pins them in two
places. `backend/api/testdata/iot_error_codes.golden` pins the 15 check-in
conflict, staff-clock and open-room codes to the declaration that emits them
(`TestFullProductionRouterGolden/contracts/IoT_error_code_declarations`). The
21 codes of the table below are pinned on the wire (see after the table);
`iot.rfid_tag_not_found` appears in both lists. These tests fail until the
backend and its pin move together; the PyrePortal side is a manual
coordination step.
Backend header and attribution rules: `backend/CLAUDE.md` RFID/IoT Integration.

### Device credentials (#3841, slice 3841-a)

Device Fleet stores SHA-256 fingerprints and a masked suffix, never raw keys.
Migration 1.15.474 hashes existing values exactly (no trimming) and drops the
plaintext column; old bearer credentials keep working. New operator keys are
server-generated only. Creation and rotation return `api_key` once;
operator lists, details and school transfers omit it and expose only
`masked_api_key`. Transfers move the fingerprint, preserving kiosk access.
Deploy the matching backend and operator frontend together; do not run an old
backend against the migrated schema. Migration rollback requires a forward fix.

PyrePortal imports `#key=` into localStorage and removes it with
`history.replaceState`. It temporarily accepts and scrubs `?key=` too;
fragments take precedence when both are present. Restarts contain no key.
The backend still accepts the same Bearer header, never a URL key. HTTP
statuses, staff PIN headers and `iot.*` codes are unchanged.

Human slice 3841-g switches configured start URLs, rotates all legacy keys and
cleans up old kiosk-host access logs. Follow-up 3841-k removes query import
only after that inventory is complete. Implementation is not production
evidence: [rollout and recovery](../operations/geraete-zugang-3841.md).

### Device PIN resolution (slice 3841-b)

The school setting `security.ogs_device_pin` is authoritative, including its
unchanged registry default. Settings read errors and explicit empty values reject
PIN-protected requests with the existing `401 / iot.device_pin_invalid` envelope;
no `OGS_DEVICE_PIN` fallback exists. Missing resolver wiring fails at construction.
Device-only routes remain PIN-free. Already deployed kiosks handle the same code
and headers, so no wire transition or compatibility-removal slice is needed.

### Kiosk error codes

Each kiosk refusal answers with its own `iot.*` code (#3939). The text in
`detail` is diagnostics and may change. PyrePortal branches on every code
in this table; the tests named after the table pin them.

| Code | Status | Detail |
|---|---|---|
| `iot.device_api_key_required` | 401 | device API key is required |
| `iot.device_api_key_format_invalid` | 401 | invalid API key format - use Bearer token |
| `iot.device_api_key_invalid` | 401 | invalid device API key |
| `iot.device_pin_required` | 401 | staff PIN is required |
| `iot.device_pin_invalid` | 401 | invalid staff PIN |
| `iot.device_inactive` | 403 | device is not active |
| `iot.rfid_tag_not_found` | 404 | RFID tag not found (scan, pickup, attendance) |
| `iot.rfid_tag_unassigned` | 404 | RFID tag not assigned to any person |
| `iot.rfid_tag_not_student_or_staff` | 404 | RFID tag not assigned to student or staff |
| `iot.person_not_student` | 404 | person is not a student |
| `iot.pickup_student_rfid_required` | 400 | student RFID tag required for pickup query |
| `iot.no_active_session` | 404 | no active session - please start an activity first |
| `iot.no_session_to_end` | 400 | no active session to end |
| `iot.no_active_groups_in_room` | 404 | no active groups in specified room |
| `iot.no_room_available` | 400 | no room available for this activity |
| `iot.no_attendance_record_today` | 404 | student has no attendance record for today |
| `iot.schulhof_not_configured` | 500 | schulhof activity not configured |
| `iot.wc_not_configured` | 500 | WC activity not configured |
| `iot.wc_needs_staff` | 500 | WC activity auto-create requires staff context |
| `iot.staff_not_found` | 404 | staff not found |
| `iot.staff_rfid_tag_unassigned` | 404 | staff has no RFID tag assigned |

Device authentication codes come from `backend/modules/devicefleet/internal/adapters/deviceauth/errors.go`, the
others from `backend/modules/devicescan/scan.go`.
`checkIoTErrorCodes` (`backend/api/iot_error_codes_contract_test.go`, run as
`TestFullProductionRouterGolden/contracts/IoT_error_codes`) drives 17 of them
through the production router. The Schulhof, WC and unassigned-card codes
need a missing system activity or a card without a person behind it;
`TestWireFormat_ClassifiedFailures`
(`backend/modules/devicescan/http/checkin/wire_test.go`) pins these four.
Other kiosk programming errors and 5xx faults keep their class code. `device is already running an activity session` has
no code: no backend path returns it, and a second start answers the session
conflict instead.

### Error reports relay

`POST /api/iot/error-reports` is PyrePortal's Sentry tunnel (#3645, SDK option
`tunnel`). It authenticates with `Authorization: Bearer <device API key>`
alone, without `X-Staff-PIN`, and opens no tenant transaction. The body is one
Sentry envelope (`application/x-sentry-envelope`) of at most 1 MB.

The relay accepts only envelopes whose header `dsn` names the project of
`SENTRY_PYREPORTAL_DSN` (`pyreportal`). It writes the tags `device_id` (the
device's `device_id`) and `school_id` into every event item, replacing what
the kiosk sent under these keys, and posts the envelope to that project's
envelope endpoint, never to a host the envelope names. The device key does not
leave the backend. Code: `backend/api/iot/error_reports.go` (handler and
texts), `backend/api/iot_error_report_composition.go` (forwarding),
`backend/observability/sentry_envelope.go` (envelope check and tags).

| Status | `error` | When |
|---|---|---|
| Sentry's | Sentry's body | Forwarded; `Retry-After` and `X-Sentry-Rate-Limits` pass through |
| 400 | `invalid error report` | The body is no envelope |
| 400 | `error report project is not allowed` | The envelope names another project or no DSN |
| 401 / 403 | Device key strings above | Key missing, invalid, or device inactive |
| 429 | `error report too large` | Envelope over 1 MB |
| 429 | `too many error reports` | Over 60 envelopes per minute and device (burst 60, per process); `Retry-After: 60` |
| 502 | `error reporting service unavailable` | Sentry unreachable or answering 5xx; logged, never a Sentry event |
| 503 | `error reporting is not configured` | The backend runs without Sentry (local development) |

The 502 is the only 5xx that `ServerErrorReporting` does not report
(`common.SkipServerErrorReport`); reporting it could only loop. PyrePortal's
SDK transport consumes these answers and shows no text, so they are not part
of `iot_error_codes.golden`. Changing a status is still a two-repo change. `serve` refuses to start with `SENTRY_DSN` set
and `SENTRY_PYREPORTAL_DSN` empty, and a malformed DSN stops the start.
With `SENTRY_DSN` set, `APP_ENV` must be `production`, `staging`, `demo` or
`development`; it is the Sentry environment of backend events.

### Presence mode

`GET /api/iot/config` returns `presence_mode: "detailed" | "binary"`.
In binary mode the kiosk hides room selection and Raumwechsel/WC buttons.
The scan-result modal branches on `checkout.schulhof_enabled`: two buttons
for a door kiosk, three when yard state is enabled. Missing or unknown mode
values default to `detailed` for older kiosks. This wire-compatibility rule is
not permission to default missing infrastructure configuration.
Backend check-in semantics adapt transparently; the kiosk UI branches per mode.

### Destination choice into released rooms (#3067)

`GET /api/iot/rooms/available` flags each room with `is_open_room` (released
by the administration) and `is_schulhof` (the system Schulhof room). After a
checkout scan the kiosk offers every released room as a destination. The
Schulhof keeps its `POST /api/iot/checkin` flow (ADR 0019 block rosters and
yard state). Every other released room goes through
`POST /api/iot/move-to-room` with `{student_rfid, room_id}`. That route runs
the phone's open-room move (ADR 0018), with the kiosk's device trust: it
records an independent stay in the room's own session and needs no device,
second scan or supervision in the destination. It never joins an activity
that runs in that room. A repeated booking answers `moved: false`.

Refusals carry stable codes PyrePortal maps:
- `iot.room_not_found` (404, also a foreign tenant's room)
- `rooms.not_released` (409)
- `iot.student_not_present` (409)
- `iot.open_room_binary_mode` (409)
- the check-in capacity and `iot.student_already_active` bodies

`backend/api/testdata/iot_error_codes.golden` pins the codes. Deploy the
backend first. A kiosk that finds neither flag nor route (404) keeps the old
Schulhof/WC buttons.

## Tenant boundary

Platform operator → organization (`platform.organizations`) → school
(`platform.schools`). **School ID is tenant ID.** Tenant-scoped tables carry
the school FK. `auth.account_tenants` maps accounts to schools with lifecycle
pending → active → inactive.

| Layer | Scoping mechanism |
|---|---|
| JWT | `tenant_id`, `org_id`, `scope`: `""` tenant, `org`, `platform`, `parent`, `school` |
| Context | `tenant.WithTenantID(ctx, id)` / `tenant.FromContext(ctx)` |
| Database | `common.TenantTxMiddleware` (`backend/api/common/tenant_middleware.go`) runs the request through the root-bound `tenant.UnitOfWork`; rolls back on 5xx or a rollback marker. `PostgresUnitOfWork.WithinTenant` (`backend/modules/transactionruntime/internal/adapters/postgres/tenant_runtime.go`) sets `SET LOCAL ROLE phoenix_tenant` and transaction-local `app.current_tenant_id` |
| RLS | `provisionTenantRLS` (`backend/database/migrations/rls_provisioning.go`) enables and forces RLS; `USING` and `WITH CHECK` match `tenant_id` to `app.current_tenant_id` |
| Rows | Owner-local tenant fields, e.g. `TenantModel.TenantID` with `bun:"tenant_id,notnull"` in `backend/modules/peopledirectory/internal/rows/model.go`; no shared model base |
| Repositories | Owner-private bindings consume `tenant.TransactionFromContext`. People Directory's `NewDatabase` and `ResolveDatabase` validate the ambient transaction and school scope (`backend/modules/peopledirectory/internal/adapters/postgres/native_compose_bindings_callback_database.go`, `scope.go` in that package); `StudentStore.InsertRecord` / `UpdateRecord` in `student_write.go` require a positive tenant, attribute inserts from scope and filter updates by tenant ID |

Parents' child access also requires relationship-level permissions; read
`.claude/rules/guardian-parent-permissions.md`. Membership alone is insufficient.

`TestEveryTenantTableHasRLS` in `backend/database/migrations/rls_coverage_test.go`
checks all non-system, non-partition-leaf tables with a live `tenant_id` column
after migrations: RLS enabled and forced, a policy, and a policy applicable to
`phoenix_tenant` (including PUBLIC/inherited roles). Its shrink-only exceptions
name administrative tables and their access boundary; effective table or column
privileges for `phoenix_tenant`, missing tables and repaired exceptions fail.
The guard checks coverage, not arbitrary SQL predicate semantics: text matching
cannot prove isolation across OR clauses, functions and combined policies.
Keep behavioral cross-school tests alongside policies. The code/test evidence
does not prove the deployed catalog; human verification is described in
[RLS coverage verification](../operations/rls-coverage-3842.md).

## Portal session isolation

Each portal has its own NextAuth instance, cookie, and base path.
Operator, parents, school, and Träger cookies are host-only. The tenant cookie is
domain-scoped to support switching between school subdomains (localhost:
host-only). The proxy redirects cross-host paths to their canonical subdomain.

| Portal | Host | Cookie | basePath | JWT scope | Backend login |
|---|---|---|---|---|---|
| Tenant | `{slug}.{TENANT_DOMAIN}` | `{TENANT_DOMAIN, dots→dashes}.session-token` on `.{TENANT_DOMAIN}`; localhost: `authjs.session-token` | `/api/auth` | `""` | `POST /auth/login` |
| Operator | `{NEXT_PUBLIC_OPERATOR_HOSTNAME}` | `operator.session-token` | `/api/operator/auth` | `platform` | `POST /operator/auth/login` |
| Parents | `{NEXT_PUBLIC_PARENTS_HOSTNAME}` | `parent.session-token` | `/api/parent/auth` | `parent` | `POST /parent/auth/login` |
| School (moto schule) | `{NEXT_PUBLIC_SCHOOL_HOSTNAME}` | `school.session-token`, SameSite=Strict | `/api/school/auth` | `school` | `POST /school/auth/login` |
| Träger | `{NEXT_PUBLIC_TRAEGER_HOSTNAME}` | `traeger.session-token` | `/api/traeger/auth` | `org` | `POST /traeger/auth/login` |

### Login, refresh, and MFA

Backend paths in this section are relative to `backend/`:

- Tenant login rejects guardian-only accounts with `ErrParentMustUseParentPortal`
  (403) and school-portal-only accounts with `ErrMustUseSchoolPortal` (403,
  `identity.use_school_portal`). Dual-role accounts remain eligible. The school-only
  guard applies at all four tenant-session mint/renew sites, including refresh.
- Parents login requires a guardian role on at least one tenant mapping
  (`ErrAccountNoGuardianRole`, 403). `ParentMiddleware` accepts only `scope=parent`.
- School login requires a school-portal role (currently `lehrkraft`) on an
  active mapping and pins that school's `tenant_id`. `SchoolMiddleware`
  accepts only `scope=school`; `common.ProtectedSchoolGroup` includes tenant
  transactions. Unlike parent scope, school scope is tenant-bound.
- Träger login (ADR 0084, #4649) requires an active membership in
  `platform.organization_accounts` of an active Träger and mints `scope=org`
  with `org_id` and no `tenant_id`; an account in several Träger chooses one
  (`organization_selection_required`). Refresh and validation re-check the
  membership. `TraegerMiddleware` accepts only `scope=org`; the Träger rights
  come from fixed role bundles in `auth.organization_account_roles`. Reads
  across schools run through the `traeger-view` projection in an admin
  transaction filtered on the Träger's schools. No MFA yet (#4651).
- `TenantMiddleware` rejects parent, school, and org scopes.
  `TestSchoolScopeRejectedOnAllAPIRoutes` checks school-token rejection under `/api`.
- MFA can insert a challenge between credentials and session. Inspect
  `modules/identityaccess/account_mfa.go`, `modules/identityaccess/inbound/account/mfa_handlers.go`, `modules/identityaccess/inbound/operator/mfa.go`,
  the challenge/enrollment claims in `modules/identityaccess/sessiontoken/`, and trusted-device settings
  `security.mfa_*`. Include the school frontend's MFA chain when changing its login.

#### Privileged MFA floor (#3841, slice 3841-f)

Effective school permissions, not role names, impose the MFA minimum in
`modules/identityaccess/internal/domain/mfa_privilege.go`: administrative
account/role/auth/security/system/staff rights, health permissions and
`staff:financial` and `enrollment_uploads:read` (parents’ income documents),
including matching wildcards. `off` and `required_admins`
cannot lower it. No new `force_off` exemption is accepted for privileged
targets, even from operators; existing exemptions lose to the runtime floor.

Every account session mint and refresh rechecks policy and permissions under
policy/account locks. Migration 1.15.484 marks existing refresh sessions
unverified; their next refresh refuses privileged access and requires login.
Completed email/passkey exchanges and valid school-bound trusted devices
produce `auth.tokens.mfa_verified`; rotation and bounded hand-off recovery
preserve it. MFA reset clears proof across existing sessions. Password reset
issues no session.

Refresh-session, invitation, password-reset and e-mail-change tokens are
stored only as hex SHA-256; migration 1.15.521 hashed existing rows and its
CHECK constraints refuse raw values (#4902). Lookups hash the presented value.
A refresh successor is HMAC-derived from the presented token, so hand-off
recovery recomputes it. A resend or guardian approval mails a new link and
voids the previous one; listings return no token. Do not run an old backend
against the migrated schema; migration rollback requires a forward fix. Tenant switching preserves proof only from the presented
account's live same-portal session family, read under the account lock shared with
MFA reset. Missing, expired, revoked, unverified or foreign-portal families
cannot satisfy target-school MFA. Such a switch returns 401 (`identity.mfa_session_required`)
and requires direct login at the target school; MFA lookup failures return 503.
School-portal switching uses the same live-family proof, restricted to school scope;
it cannot reuse a tenant-portal family. The staff switcher shows the refusal without
expiring the source session. Its explicit target-login action first requests
revocation of the source refresh family through `/api/auth/logout`, then deletes
the shared domain Auth.js cookie via `signOut({ callbackUrl: targetLoginUrl })`. Auth.js itself
then navigates to the target school’s existing login page before notifying local
required-session consumers; manual navigation after `redirect: false` can race
their source-school redirects. Navigation alone is not reauthentication:
the domain cookie would reach the target guard unchanged. The guard handles the
same MFA refusal without SessionExpired redirects or automatic retries and
offers this same explicit logout-and-login action. An HTTP error returned by
the logout BFF stays visible and prevents navigation; 401 is treated as already
unauthenticated. This does not confirm database revocation: the existing backend
`/auth/logout` logs `LogoutWithAudit` failures, including database revocation
failures, and still responds 204. The browser cannot distinguish that failure
from success.
If Auth.js fails to clear the cookie, the target guard still blocks access and
offers explicit reauthentication instead of redirecting in a loop.
School-portal cookies remain host-only and are neither used nor cleared by the tenant action; the school portal currently has no
frontend school-switch/auto-switch consumer. Its backend switch still requires
school-scoped live-family proof.
Parent-scope sessions remain separate and carry no staff permissions.

Enrollment uses the existing email-code flow in tenant and school portals;
mail delivery/verification failures issue no completed session. Passkeys for
school personnel already require verified email registration and WebAuthn user
verification at login. Local seed tools complete the same email flow through
local Mailpit, with no production bypass. Human rollout/recovery and count-only
inventory: [MFA runbook](../operations/mfa-pflicht-3841.md).

Token lifetimes come from `AUTH_JWT_EXPIRY` / `AUTH_JWT_REFRESH_EXPIRY`;
do not infer them from stale examples. Check credentials, challenge completion,
refresh, tenant switching, and wrong-portal rejection for affected flows.

### Routing and invitations

`frontend/src/proxy.ts` rewrites tenant hosts to `/[tenant]/*`, operator to
`/operator/*`, parents to `/parents/*`, school to `/school/*`, and Träger to
`/traeger/*`.
`[tenant]/layout.tsx` resolves slugs through `/auth/tenant/resolve?slug=...`
(cached five minutes). `POST /auth/switch-tenant` returns a tenant-session JWT
for the target school, not a `scope=school` portal token.
Verified MFA belongs to the live tenant-session family of the same account,
not its school or organization. A permitted switch across organizations preserves
that proof while rechecking the target school's permissions and MFA policy.
After reset, logout, expiry or elapsed switch-retirement grace, a privileged
target requires a fresh MFA login: 401 `identity.mfa_session_required`. Clients
must replace the presented session with the returned tokens; login caches must
invalidate the retired login after a successful switch. See
[ADR 0093](../adr/0093-privilegierte-rechte-erzwingen-einen-zweiten-anmeldenachweis.md).
A switch to a school the account cannot access answers 401 with the code
`identity.tenant_access_denied`, which `frontend/src/lib/tenant-api.ts` reads;
coordinate producer and consumer changes.

Class-day is mounted only at `/school/class-day`; `/api/class-day` and the
tenant `/klassen` page were removed. Its frontend uses `app/school/*`, shared
`components/class-day`, and `server/auth/school*.ts` (under `frontend/src/`).
The PWA identity is moto schule (`schule` / `schule-staging` favicon variants).
Lehrkraft invitations link to `SCHOOL_URL/invite`, not `FRONTEND_URL/invite`.

`TENANT_DOMAIN` is the base domain; `NEXT_PUBLIC_TENANT_DOMAIN` is its client
counterpart. Portal hostnames use the variables in the table above.
`FRONTEND_URL` is for staff/admin email links; `PARENTS_URL` for parent links;
`SCHOOL_URL` for school links; `TRAEGER_URL` for Träger links. The latter three are required at backend startup
and must use HTTPS in production. Env change checklist:
`.claude/rules/env-docker-sync.md`.

Reserved slug lists in `backend/modules/organizationtenancy/internal/domain/organization.go` and
`frontend/src/lib/reserved-slugs.ts` must stay in sync; verify both when changed.

### Demo access (public demo, #3462)

The routes below are mounted only when `APP_ENV=demo`
(`authAPI.MountDemoRoutes`); elsewhere they answer 404 and the capability is
not composed. The backend routes are public, take no cookies, and rely on
`CORS_ALLOWED_ORIGINS` naming the website origins in the demo environment.

| Route | Contract |
|---|---|
| `POST /demo/access-requests` | `email`, `school_name`, `first_name`, `last_name` (both required, at most 120 bytes each, whitespace runs collapsed; the demo school's caregiver and parent carry exactly these names), `contact_opt_in`, optional `src` and `role` (a demo role, appended to the mailed link as `&role=`) → always `202 {link_sent: true}`, never the link itself; `422 demo_access_invalid`; `429 demo_access_rate_limited` with `Retry-After` (seconds); `503 demo_capacity_reached` |
| `GET /demo/access/status` | token in `Authorization: Bearer` → `{status: preparing\|ready\|failed, school_name}` (the OGS name the prospect gave, shown while waiting, #3464), plus `school_url` (origin of the demo school) when `ready` |
| `POST /demo/access/sessions` | `{token, role?}` → `{access_token, refresh_token, demo: {access_id, role, src, fixed_role}}` (tenant session; parents portal session for `role: parent`, #3468); `409 demo_school_preparing`; `422 demo_access_invalid` for an unknown role |
| `POST /demo/access/reset` | `{token}` → `202 {status: "preparing", entry_url}` (#3470): a fresh demo school is queued for the same access with the same names, the old one is soft-deleted and its sessions are revoked; `entry_url` is the waiting room with the token in the fragment, as in the mailed link. `409 demo_school_preparing` while the current school is still being seeded; `422 demo_access_invalid` for the shared standing school; `429 demo_access_rate_limited` with `Retry-After`, because a restart counts against the same per-address and per-IP windows as a request; `503 demo_capacity_reached` never for a restart, because the old school gives its place back first |

Demo roles (#3467): `caregiver`, `lead`, `all`. A role first becomes the only
role of the visitor's own caregiver in its school (`user` for `caregiver`,
`admin` for `lead` and `all` until reduced permission sets exist), then the
session is minted, so the banner's role switch is the same call as the entry.

The shared administrator of the standing school keeps its role and answers
`role: "all", fixed_role: true`; the banner then shows the role without a
menu. The frontend route `/api/demo/access/sessions` keeps the
redeemed token in the httpOnly cookie `moto-demo-token` (path `/api/demo`,
14 days); a switch sends only the role.

Demo role `parent` (#3468): the same route with the same token answers a
parents portal token pair (`scope=parent`) for the school's parent of the
visitor's name (`visitor_parent_account_id`), a primary guardian with full
parent portal rights for exactly one child; the caregiver's role stays as it
is. A school without that parent answers `422 demo_access_invalid`. The
parents host serves the entry page `app/parents/demo` (`/demo`, public in
`ParentAuthGuard`), which signs in with the `parent-credentials`
`internalRefresh` path. Every app redeems on its own host, so switching apps
is a navigation with the token in the fragment, not a separate hand-over:
the banner navigates to `GET /api/demo/access/handoff?role=…`, which reads
the `moto-demo-token` cookie and answers `303` to the other app's `/demo`
entry page (`#token=…&role=…&switched=1`; for an OGS role it asks the
backend for `school_url` first). Without a usable token it redirects to this
host's `/demo`. The waiting room and the OGS entry page send the role
`parent` straight to the parents host. The standing school has no parent of
its own: `parent` there answers `role: "all", fixed_role: true`, and the
parents entry page sends the visitor on to the school.

Unknown token: `404 demo_access_unknown`; expired: `410 demo_access_expired`.
The token is opaque, stored as SHA-256 fingerprint in `auth.demo_accesses`
(owner `identity-access`), reusable, every use counted. It is valid 14 days
after its last use (#3470): a redemption moves `expires_at` forward by the
full lifetime. It travels in the URL fragment, request bodies, or the header
above, never in a URL a server logs.

Restart and expiry (#3470). „Demo neu anfangen" in the banner asks first
(`ConfirmationModal`), then posts the role to the frontend route
`/api/demo/access/reset`, which reads the `moto-demo-token` cookie, calls the
backend and answers `{entry_url}` with `&restarted=1` and the role appended
to the fragment. The banner navigates there: the waiting room shows the setup
screen as on the first entry, the school's entry page skips the role cards
and reports `demo_restarted` instead of `demo_entered`. Every access of the
old school moves to the new one (`school_slug`), so earlier links of the
address lead there too, and the old school's `account_id` and
`parent_account_id` are forgotten. Expiry runs in the demo process once an
hour on an injected clock (`cmd/devtools/demo_expiry.go`): accesses past `expires_at`
are deleted, and a demo school no remaining access enters is soft-deleted
through its owner. The standing school is never hidden. A hidden school
holds no place against the capacity, gets no ticker, and cannot be entered:
its subdomain resolves no tenant. An expired or deleted link answers `410` or
`404`, and both entry pages show „Dieser Link funktioniert nicht mehr" with
„Neuen Link anfordern" to the website's demo page.

Every address enters a demo school of its own (#3463). Its first request
queues an order in `platform.demo_school_states` (owner
`organization-tenancy`); the school's slug is the OGS name plus a random
suffix. A demo school, and with it `{slug}.TENANT_DOMAIN`, exists only after
its seed: the tenant layout resolves the slug and would send a visitor of an
unknown subdomain away. So the mailed link is always the waiting room on the
main domain, `FRONTEND_URL/demo#token=…` (`app/demo/page.tsx`). It polls the
status and, once `ready`, hands the token on to
`{school_url}/demo#token=…`, where the entry page
`[tenant]/(public)/demo` redeems it through `/api/demo/access/*` and signs in
with the `internalRefresh` credentials path. Both pages take their texts
from `lib/demo-access.ts`; while the school is `preparing` they show
`DemoSetupScreen` („Wir richten {school_name} für Sie ein" with progress
lines, #3464), and a wait that went wrong offers „Noch einmal versuchen". The demo process seeds the school; `docs/operations/standing-demo.md` has the queue,
the limit of three seeds at a time, the single repetition and the fallback to
the standing school `messe-demo` (`--demo-standing-school` on `serve` and
`demo`). The access row remembers its school (`school_slug`), so status and
redemption never take a slug from the caller.

- `preparing`: queued, being seeded, or seeded without its first tick.
- `ready`: seeded, first tick done, the school exists and is active. The
  session signs in the prospect's own caregiver (`visitor_account_id`); a
  school without one (the standing school) signs in its oldest administrator.
- `failed`: the seed failed twice. The entry page sends the prospect back to
  the website; the address no longer counts as active and may ask again.

An address is active while its newest unexpired access enters a school that
did not fail. A further request of an active address stores an access into
that same school; only an inactive address queues a new one. The prospect's
address stays in `auth.demo_accesses`. The order carries
only the OGS name and the person's first and last name, so the address cannot become an
account or guardian address in a demo school, where
`attachExistingAccountByEmail` would hand an existing account of that address
to the inviting tenant. The serving role reads `name`, `status`, `tenant_id`,
`visitor_account_id` and `visitor_parent_account_id` of an order, inserts new ones, and stamps
`last_used_at` when a token is redeemed; `seed_state` and `status` are out of
its reach. A restart soft-deletes the school through `platform.schools`, on
which the serving role holds every right; the order row is not touched. The
demo process ticks only schools redeemed in the last 30 minutes (#3464) and,
for the expiry (#3470), may read `id`, `school_slug` and `expires_at` of
`auth.demo_accesses` and delete rows, and read and set `deleted_at` of
`platform.schools`; it never sees an address or a name (migration 1.15.412).

Mails (#3465): the entry link leaves by mail only (`demo-access.html`,
Reply-To `kontakt@moto-ogs.de`), and the answer is the same for every address,
so it neither hands a demo to somebody who typed a foreign address nor tells
who asked before. The mail carries nothing the form submitted. Every request
stores its own access with the submitted details; earlier links stay valid
until they expire. An address waits 10 minutes for its next link: within
that cooldown a request stores and mails nothing. The team is mailed
(`demo-lead.html`) for an address without an active access and when the
contact consent changed. The website shows „Wir haben Ihnen den Link
geschickt" for `link_sent`.

Limits (#3466): per IP address 60 requests and per address 3 requests within
any hour; over either, `429` with `Retry-After`. Only a valid request counts,
so invalid requests fill no window; they touch no table either. The IP limit
is high because fair visitors share one WLAN address. The windows live in the serving process (one demo server; a
restart forgives them). At most `--demo-max-active-schools` demo schools hold
a place (queued, or ready and not deleted; a failed one frees it); a request
that needs a new school beyond that answers `503 demo_capacity_reached`,
while an active address still gets its link. `serve` refuses to start under
`APP_ENV=demo` without the flag; `environments/demo.compose.yml` sets 300.

Mail lock: under `APP_ENV=demo`, `email.NewMailer` wraps the one SMTP
transport in `email.RestrictToDemoMails`. Every template but the two above
is dropped and reported as sent, whoever the caller is; only
`mfa-email-code.html` reports `email.ErrNotDeliveredInDemo`, because a
sign-in waits for that code. A new mail that must leave the demo environment
needs its template added there. `email.IsDemoEnvironment` decides for the
routes, the capability and the lock alike.

Because the lock drops the operator's sign-in code, the `APP_ENV=demo`
composition leaves the operator login without a second factor
(`DemoDependencies.OperatorWithoutSecondFactor`, #3460); otherwise the demo
process could never sign in to seed a school. The demo host's Caddy answers
404 for the operator host and every `/operator` and `/api/operator` path, so
the operator surface stays internal. Every other environment keeps operator
MFA mandatory.

`SwitchTenant` refuses any account a demo access signed in
(`403 demo_session`), through a mint guard inside the switch transaction.
Such an account is exempt from the session cap (`capSessionsUnlessDemo`): in
the standing school all visitors share one account, and the sixth visitor
would sign the first one out. The role parent signs in another account than
the caregiver, so the redemption notes it in
`auth.demo_accesses.parent_account_id`; switching between the apps therefore
ends no parent session either. `TenantGuard` leaves the entry page alone
(`isDemoEntryPath`) and guards every other tenant route as before.

### Embedded enrollment

The parents portal serves `/parents/anmeldung/{slug}/{phaseId}` with the same
`EnrollmentForm` used by `{slug}.TENANT_DOMAIN/anmeldung/{phaseId}`, injecting
`profileFetcher`, `submitter`, and `skipCaptcha`. Authenticated parent submissions
stamp `enrollment.requests.guardian_account_id`; decisions prefer attachment by
that ID over email matching.

A form template with `core_requirements.signature` requires the parent's
signature on every parent submission and form edit (#4786); the server refuses
a missing one with `enrollment.signature_required`. The status token is the
only authority for the parent's proof (`GET /api/enrollment/requests/{token}?format=pdf`):
it resolves one request of one school, every later read stays on that request
and school, and an unknown, expired or altered token answers 404
`enrollment.status_link_invalid`. Details:
[docs/reference/elternerklaerungen.md](../reference/elternerklaerungen.md#anmeldung-mit-unterschrift-4786).
