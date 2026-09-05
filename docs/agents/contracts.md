# Cross-stack contracts

Read the relevant section before changing auth, portal routing, enrollment,
tenant scoping, IoT endpoints, or kiosk presence modes. Paths start at the
repository root unless the section says otherwise.

## Ecosystem and IoT

The sibling `../PyrePortal/` repository runs the Raspberry Pi kiosk (Tauri +
React). `../moto-balenaOS/` deploys it on Pi 5 hardware; the Phoenix backend
runs on the server, never on the Pi. PRs target `development` except in
moto-balenaOS (`main`).

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
  `services/auth/mfa_service.go`, `api/auth/mfa_handlers.go`, `api/operator/mfa.go`,
  the challenge/enrollment claims in `auth/jwt/`, and trusted-device settings
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

### Embedded enrollment

The parents portal serves `/parents/enroll/{slug}/{phaseId}` with the same
`EnrollmentForm` used by `{slug}.TENANT_DOMAIN/enroll/{phaseId}`, injecting
`profileFetcher`, `submitter`, and `skipCaptcha`. Authenticated parent submissions
stamp `enrollment.requests.guardian_account_id`; decisions prefer attachment by
that ID over email matching.
