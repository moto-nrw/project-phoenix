# Cross-stack contracts

Read the relevant section before changing auth, portal routing, enrollment,
tenant scoping, IoT endpoints, or kiosk presence modes. Paths start at the
repository root unless the section says otherwise.

## Ecosystem and IoT

The sibling `../PyrePortal/` repository runs the Raspberry Pi kiosk (Tauri +
React). The Phoenix backend runs on the server, never on the Pi. PRs target
`development`.

PyrePortal consumes `/api/iot/*` using a device API key and staff PIN.
`../PyrePortal/src/services/api.ts` maps backend error strings to German UI
text. Coordinate endpoint, error-string, and auth-header changes across repos.
Backend header and attribution rules: `backend/CLAUDE.md` RFID/IoT Integration.

### Presence mode

`GET /api/iot/config` returns `presence_mode: "detailed" | "binary"`.
In binary mode the kiosk hides room selection and Raumwechsel/WC buttons.
The scan-result modal branches on `checkout.schulhof_enabled`: two buttons
for a door kiosk, three when yard state is enabled. Missing or unknown mode
values default to `detailed` for older kiosks. This wire-compatibility rule is
not permission to default missing infrastructure configuration.
Backend check-in semantics adapt transparently; the kiosk UI branches per mode.

## Tenant boundary

Platform operator → organization (`platform.organizations`) → school
(`platform.schools`). **School ID is tenant ID.** Tenant-scoped tables carry
the school FK. `auth.account_tenants` maps accounts to schools with lifecycle
pending → active → inactive.

| Layer | Scoping mechanism |
|---|---|
| JWT | `tenant_id`, `org_id`, `scope`: `""` tenant, `org`, `platform`, `parent`, `school` |
| Context | `tenant.WithTenantID(ctx, id)` / `tenant.FromContext(ctx)` |
| Database | `TenantTxMiddleware` sets local role and RLS configuration; rolls back on 5xx |
| Models | `base.TenantModel` and `TenantScoped` |
| Repositories | `base.GetDB(ctx, db)` uses the context transaction; `base.EnsureTenantID` populates tenant ID |

Parents' child access also requires relationship-level permissions; read
`.claude/rules/guardian-parent-permissions.md`. Membership alone is insufficient.

## Portal session isolation

Each portal has its own NextAuth instance, cookie, and base path.
Operator, parents, and school cookies are host-only. The tenant cookie is
domain-scoped to support switching between school subdomains (localhost:
host-only). The proxy redirects cross-host paths to their canonical subdomain.

| Portal | Host | Cookie | basePath | JWT scope | Backend login |
|---|---|---|---|---|---|
| Tenant | `{slug}.{TENANT_DOMAIN}` | `{TENANT_DOMAIN, dots→dashes}.session-token` on `.{TENANT_DOMAIN}`; localhost: `authjs.session-token` | `/api/auth` | `""` or `org` | `POST /auth/login` |
| Operator | `{NEXT_PUBLIC_OPERATOR_HOSTNAME}` | `operator.session-token` | `/api/operator/auth` | `platform` | `POST /operator/auth/login` |
| Parents | `{NEXT_PUBLIC_PARENTS_HOSTNAME}` | `parent.session-token` | `/api/parent/auth` | `parent` | `POST /parent/auth/login` |
| School (moto schule) | `{NEXT_PUBLIC_SCHOOL_HOSTNAME}` | `school.session-token`, SameSite=Strict | `/api/school/auth` | `school` | `POST /school/auth/login` |

### Login, refresh, and MFA

Backend paths in this section are relative to `backend/`:

- Tenant login rejects guardian-only accounts with `ErrParentMustUseParentPortal`
  (403) and school-portal-only accounts with `ErrMustUseSchoolPortal` (403,
  `use_school_portal`). Dual-role accounts remain eligible. The school-only
  guard applies at all four tenant-session mint/renew sites, including refresh.
- Parents login requires a guardian role on at least one tenant mapping
  (`ErrAccountNoGuardianRole`, 403). `ParentMiddleware` accepts only `scope=parent`.
- School login requires a school-portal role (currently `lehrkraft`) on an
  active mapping and pins that school's `tenant_id`. `SchoolMiddleware`
  accepts only `scope=school`; `common.ProtectedSchoolGroup` includes tenant
  transactions. Unlike parent scope, school scope is tenant-bound.
- `TenantMiddleware` rejects both parent and school scopes.
  `TestSchoolScopeRejectedOnAllAPIRoutes` checks school-token rejection under `/api`.
- MFA can insert a challenge between credentials and session. Inspect
  `modules/identityaccess/account_mfa.go`, `modules/identityaccess/inbound/auth/mfa_handlers.go`, `modules/identityaccess/inbound/operator/mfa.go`,
  the challenge/enrollment claims in `modules/identityaccess/legacy/jwt/`, and trusted-device settings
  `security.mfa_*`. Include the school frontend's MFA chain when changing its login.

Token lifetimes come from `AUTH_JWT_EXPIRY` / `AUTH_JWT_REFRESH_EXPIRY`;
do not infer them from stale examples. Check credentials, challenge completion,
refresh, tenant switching, and wrong-portal rejection for affected flows.

### Routing and invitations

`frontend/src/proxy.ts` rewrites tenant hosts to `/[tenant]/*`, operator to
`/operator/*`, parents to `/parents/*`, and school to `/school/*`.
`[tenant]/layout.tsx` resolves slugs through `/auth/tenant/resolve?slug=...`
(cached five minutes). `POST /auth/switch-tenant` returns a tenant-session JWT
for the target school, not a `scope=school` portal token.
The backend string `"account does not have access to this tenant"` is mapped in
`frontend/src/lib/tenant-api.ts`; coordinate producer and consumer changes.

Class-day is mounted only at `/school/class-day`; `/api/class-day` and the
tenant `/klassen` page were removed. Its frontend uses `app/school/*`, shared
`components/class-day`, and `server/auth/school*.ts` (under `frontend/src/`).
The PWA identity is moto schule (`schule` / `schule-staging` favicon variants).
Lehrkraft invitations link to `SCHOOL_URL/invite`, not `FRONTEND_URL/invite`.

`TENANT_DOMAIN` is the base domain; `NEXT_PUBLIC_TENANT_DOMAIN` is its client
counterpart. Portal hostnames use the variables in the table above.
`FRONTEND_URL` is for staff/admin email links; `PARENTS_URL` for parent links;
`SCHOOL_URL` for school links. The latter two are required at backend startup
and must use HTTPS in production. Env change checklist:
`.claude/rules/env-docker-sync.md`.

Reserved slug lists in `backend/models/platform/organization.go` and
`frontend/src/lib/reserved-slugs.ts` must stay in sync; verify both when changed.

### Demo access (public demo, #3462)

The routes below are mounted only when `APP_ENV=demo`
(`authAPI.MountDemoRoutes`); elsewhere they answer 404 and the capability is
not composed. The backend routes are public, take no cookies, and rely on
`CORS_ALLOWED_ORIGINS` naming the website origins in the demo environment.

| Route | Contract |
|---|---|
| `POST /demo/access-requests` | `email`, `school_name`, `person_name`, `contact_opt_in`, optional `src` and `role` (a demo role, appended to the mailed link as `&role=`) → always `202 {link_sent: true}`, never the link itself; `422 demo_access_invalid`; `429 demo_access_rate_limited` with `Retry-After` (seconds); `503 demo_capacity_reached` |
| `GET /demo/access/status` | token in `Authorization: Bearer` → `{status: preparing\|ready\|failed, school_name}` (the OGS name the prospect gave, shown while waiting, #3464), plus `school_url` (origin of the demo school) when `ready` |
| `POST /demo/access/sessions` | `{token, role?}` → `{access_token, refresh_token, demo: {access_id, role, src, fixed_role}}` (tenant session; parents portal session for `role: parent`, #3468); `409 demo_school_preparing`; `422 demo_access_invalid` for an unknown role |

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
(owner `identity-access`), valid 14 days, reusable, every use counted. It
travels in the URL fragment, request bodies, or the header above, never in a
URL a server logs.

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
only the OGS name and the person's name, so the address cannot become an
account or guardian address in a demo school, where
`attachExistingAccountByEmail` would hand an existing account of that address
to the inviting tenant. The serving role reads `name`, `status`, `tenant_id`,
`visitor_account_id` and `visitor_parent_account_id` of an order, inserts new ones, and stamps
`last_used_at` when a token is redeemed; `seed_state` and `status` are out of
its reach. The demo process ticks only schools redeemed in the last 30 minutes
(#3464).

Mails (#3465): the entry link leaves by mail only (`demo-access.html`,
Reply-To `kontakt@moto.nrw`), and the answer is the same for every address,
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

`SwitchTenant` refuses any account a demo access signed in
(`403 demo_session`), through a mint guard inside the switch transaction.
Such an account is exempt from the session cap (`capSessionsUnlessDemo`): in
the standing school all visitors share one account, and the sixth visitor
would sign the first one out. `TenantGuard` leaves the entry page alone
(`isDemoEntryPath`) and guards every other tenant route as before.

### Embedded enrollment

The parents portal serves `/parents/anmeldung/{slug}/{phaseId}` with the same
`EnrollmentForm` used by `{slug}.TENANT_DOMAIN/anmeldung/{phaseId}`, injecting
`profileFetcher`, `submitter`, and `skipCaptcha`. Authenticated parent submissions
stamp `enrollment.requests.guardian_account_id`; decisions prefer attachment by
that ID over email matching.
