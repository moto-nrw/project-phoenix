# Org-Layer (Träger) im Code: Bestand und Lücken

Stand 2026-08-29, Branch `ideation/traegerapp`. Pfade relativ zum Repo-Root.

## 1. Datenmodell und `scope: "org"`: Skelett vorhanden, nirgends aktiv

### Tabellen

`platform.organizations` (`backend/database/migrations/001013001_create_organizations_and_schools.go:48-56`): id, name, slug UNIQUE, active, settings JSONB, created_at, updated_at; `deleted_at` später (`001015035`). Model `backend/models/platform/organization.go:52-60`.

`platform.schools.organization_id NOT NULL` FK, `UNIQUE(organization_id, slug)`, Index `idx_schools_organization`. Model-Relation `backend/models/platform/school.go:31`.

Keine weitere Tabelle auf Org-Ebene. Einzige Org-Bezüge sonst:
- `platform.announcements.target_org_ids BIGINT[]` (`backend/models/platform/announcement.go:47`)
- `platform.organizations.settings` ist toter Speicher: kein Go-Code liest oder schreibt das Feld. Das Settings-System (`config.setting_values`) ist rein tenant-scoped.

### JWT

`backend/auth/jwt/claims.go:33` trägt `OrgID`. Scope-Konstanten `backend/tenant/context.go:24-30`: `""`, `"org"`, `"platform"`, `"parent"`, `"school"`. Refresh-Token-Pendant `backend/models/auth/token.go:32-38` (`PortalScopeOrg`).

Wichtig: `CapPortalScopes` (token.go:41-47) wirft tenant und org in einen gemeinsamen Session-Cap-Topf ("staff portal"), school und parent haben eigene. Ebenso `backend/database/repositories/iot/push_subscription.go:330` (`PushPortalStaff` = tenant + org). Der ursprüngliche Entwurf ging davon aus, dass Träger-Nutzer im Staff-Portal sitzen, nicht in einem eigenen Portal.

### Wer wertet `ScopeOrg` aus? Genau zwei Stellen

1. `backend/services/auth/auth_login.go:906-976`, `resolveAccountTenantBySlug`: Org-Caller darf per `POST /auth/switch-tenant` in jede Schule seiner Org wechseln, ohne `auth.account_tenants`-Eintrag (`school.OrganizationID == callerOrgID`).
2. `backend/database/repositories/auth/accounts.go`: `accountMembershipScope` (22-38), `FindByRole` (481-492, läuft unter `tenant.WithAdminTx` mit Org-Filter), `applyRoleFilter` (563-580).

Es fehlen:

| Bereich | Status |
|---|---|
| Org-Login-Endpoint | fehlt; kein Code mintet je `Scope: "org"` |
| `OrgMiddleware` | fehlt (nur tenant/parent/school in `backend/auth/jwt/`) |
| Org-Systemrolle | fehlt (Muster: `001015278_lehrkraft_role.go`) |
| Org-Permission (`org:dashboard:read` aus DEBATE D18) | fehlt |
| Endpoint mit Org-Scope | keiner; `TenantMiddleware` lässt `org` durch (`tenant_middleware.go:31-42` lehnt nur parent/school ab), aber kein Handler prüft darauf |
| `OrgScopeService` (DEBATE D18) | fehlt |

Fazit: `scope "org"` ist ein toter Pfad. Erreichbar heute nur aus Tests (`backend/api/auth/account_tenant_isolation_test.go:152ff`).

### Tenant-Switching (existiert, tenant-scoped)

`POST /auth/switch-tenant` (`backend/api/auth/api.go:164`, Service `auth_switch_tenant.go:16-59`), `GET /auth/tenants`, `GET /auth/account/tenants`. Frontend `components/tenant/tenant-switcher.tsx`, `lib/tenant-api.ts:426/312`. Die Schulliste ist schon org-angereichert (`repositories/auth/account_tenant.go:185-211`, sortiert nach org.name, school.name), solange pro Schule ein `account_tenants`-Mapping existiert.

## 2. Operator-Portal

Routen `backend/api/operator/api.go:272-290` (`/organizations` CRUD, Soft-Delete/Restore, `/{id}/schools`; `/schools`). Service-Interface `backend/services/platform/operator_provisioning_service.go:100-131`. Alles unter `tenant.WithAdminTx`, mit Operator-Audit. Org mit Schulen ist nicht löschbar (`provisioning.go:655`). `organization_id` ist Pflicht beim Anlegen einer Schule (`school.go:70-72`) und wird nur dort gesetzt.

Frontend: `app/operator/organizations/page.tsx`, `[slug]/page.tsx`, `[slug]/schools/[schoolSlug]/page.tsx`, Modals unter `app/operator/provisioning/`, Filter-Hook `use-org-school-filter.ts`. Kennzahl `traeger_count` auf dem Operator-Dashboard.

## 3. Das Schul-Portal-Muster (#2207) als Blaupause

### Backend

| Datei | Rolle |
|---|---|
| `backend/api/school/api.go` | Portal-Resource, eigener chi-Router unter `/school`; Public-Gruppe (login, password-reset, mfa/verify, mfa/resend) mit Rate-Limiter, MFA-Enrollment-Gruppe, geschützte Gruppe mit `jwt.SchoolMiddleware`, switch-school |
| `backend/api/school/auth_handlers.go`, `password_handlers.go` | Login, Switch, MFA, Reset |
| `backend/auth/jwt/school_middleware.go` | Scope-Prüfung, setzt WithTenant/WithOrgID/WithScope |
| `backend/auth/jwt/tenant_middleware.go:37-42` | Gegen-Guard |
| `backend/api/common/router.go:40-56` | `ProtectedSchoolGroup` (gleiche Kette wie Tenant, dann `TenantTxMiddleware`) |
| `backend/services/auth/auth_login_school.go` | Login-Flow, Tenant-Pinning, MFA-Gate |
| `backend/services/auth/errors.go`, `auth_login.go` | `ErrMustUseSchoolPortal` an allen Tenant-Mint-Stellen |
| `backend/models/auth/token.go` | eigener Session-Cap-Topf |
| `001015282_mfa_challenge_portal_binding.go` | MFA-Challenge ans Portal gebunden |
| `backend/api/base.go:742-745, 920-938` | Mount `/school`, `/school-sse` |
| `backend/api/school_scope_matrix_test.go` | pinnt: School-Token schliesst unter `/api` nichts auf |
| `backend/models/platform/organization.go:38-40` | reservierte Slugs |

### Frontend

`server/auth/school-config.ts`, `school.ts`, `school-route.ts`; `app/api/school/auth/**` (9 BFF-Routen); `app/school/{page,layout,providers,auth-guard,school-shell}.tsx` plus Seiten; `components/school/shell/*`; `components/ui/portal-shell.tsx` (geteilter Rahmen, direkt wiederverwendbar); `proxy.ts:33-36, 369-460, 529-531, 570-612` (Host-Routing, fail-fast Env); `env.js:41`; `manifest.ts`, `layout.tsx`, `public/favicons/schule*`; `lib/reserved-slugs.ts:18`.

### Infrastruktur

`.env.example`, `frontend/.env.example`, `backend/dev.env.example`, `docker-compose.example.yml`, beide `*.sops.env`, `frontend/Dockerfile.prod` (ARG+ENV), `.github/workflows/{build,lint,test}.yml`, `playwright.config.ts`, backend-seitig `SCHOOL_URL` (`services/factory.go`).

### Checkliste Träger-Portal

Backend
1. `ScopeOrg` existiert.
2. `backend/auth/jwt/org_middleware.go` (neu): darf keinen `WithTenant` setzen, sonst per RLS auf eine Schule eingesperrt. Nur `WithOrgID` + `WithScope`.
3. Gegen-Guard in `tenant_middleware.go` für `org`? Bruch: heute lässt TenantMiddleware `org` durch, `auth_login.go:961` baut darauf auf (Org-Nutzer wechselt per switch-tenant in eine Schule). Entscheidung: Org-Token nur auf `/org/*` (Guard, kein Switch) oder tenant-fähig (kein Guard, kein Scope-Matrix-Test möglich).
4. `ProtectedOrgGroup` in `router.go`: nicht `TenantTxMiddleware` anhängen, braucht eigene Tx-Variante (Admin-Tx + Org-Filter).
5. `backend/api/org/{api,auth_handlers,password_handlers}.go`.
6. `services/auth/auth_login_org.go`: Datenbasis fehlt, keine `auth.account_organizations`. Ableitung aus `account_tenants` + `schools.organization_id` ist mehrdeutig bei Accounts in zwei Trägern.
7. Migration: Systemrolle + Permission `org:dashboard:read`.
8. `token.go`: `CapPortalScopes` Topf trennen; ebenso `push_subscription.go:330`.
9. Slug `traeger` reservieren (beide Listen).
10. `base.go`: Mount `/org`, Rate-Limiter, ggf. `/org-sse`.
11. `route_table.golden` + Scope-Matrix-Test.

Frontend
12. `server/auth/org-config.ts`, `org.ts`, `org-route.ts`
13. `app/api/org/auth/**`
14. `app/org/*`
15. `components/org/shell/*` (nutzt `portal-shell.tsx`)
16. `proxy.ts` Host-Routing
17. `env.js`: `NEXT_PUBLIC_ORG_HOSTNAME` (Pflicht, kein Default)
18. PWA-Identität, Favicons

Infrastruktur
19. Env-Dateien, sops, Dockerfile.prod, build.yml
20. `ORG_URL` für Einladungsmails

## 4. Cross-School-Features, die ein Träger sehen wollen würde

Alles strikt ein-tenant (ProtectedTenantGroup + TenantTxMiddleware + RLS):

| Bereich | Backend | Frontend |
|---|---|---|
| Zeiterfassung / Dienstplan | `api/staff-shifts/` (overview, series, export), `schedule.staff_shifts`, `staff_shift_series` | `time-tracking/`, `dienstplan/`, `payroll/` |
| Lohn-Export | `services/active/staff_time_export_datev.go` | `payroll/page.tsx` |
| Personal | `api/staff/`, `users.staff` | `staff/` |
| Statistik | `api/statistics/api.go:49-58` (`/report`, `/export`), `services/statistics` | `statistics/` |
| Settings | `config.setting_values`, ~86 Settings | `settings/` |
| Anmeldung | enrollment-Schema | `enrollment-phases/`, `anfragen/` |
| Vertretung / Abwesenheiten | `api/absence-types/`, substitutions | `vertretung/`, `absences/` |

Anschlussfähig: `services/statistics` hat saubere Service/Report-Trennung (Org-Variante wäre zweiter Aufrufer mit anderem Tx-Rahmen). DATEV-Export ist der plausibelste erste Träger-Use-Case (Lohnbuchhaltung sitzt beim Träger). Es gibt keine einzige Aggregations-Query über `tenant_id` hinweg im Staff-Bereich.

## 5. Dokumentation: spezifiziert, nie gebaut

- `docs/multi-tenancy/00-anforderungen.md` Z. 18-22, 47-57, 76-93 (Träger-Büro vs OGS-Büro), 131-153 (Ferien, Springer), 216-218 (10-20 Träger bis 2027), 225-226 (Follow-ups, nicht umgesetzt).
- `docs/multi-tenancy/DEBATE.md` D18 (Z. 1098-1160, Zusammenfassung 1476): Träger-Büro = `scope: org`, eigener Mechanismus; bewusst kein RLS für Org (3-10 Nutzer pro Träger); Tagesgeschäft per Tenant-Switch mit RLS; Permission `org:dashboard:read`; keine Impersonation; "Application-Layer mit WithAdminTx + org_id-Filter. Dedizierter OrgScopeService, read-only". Entschieden 2026-02-10.
- `11-implementierungsplan.md:155`: nur "JWT Claims erweitern" (erledigt). Rest von D18 in keinem Plan.
- `06-offene-punkte.md:179`: Ferienbetreuung träger-übergreifend braucht AVV (D21).
- DSGVO (DEBATE 1371-1439): jeder Träger eigener Verantwortlicher, moto Auftragsverarbeiter (Art. 28).
- Kein ADR zu Träger/Org-Scope. Branch `ideation/traegerapp` war inhaltlich leer.

## 6. RLS: der Kern des Problems

Zwei Rollen: CLI als Superuser (bypass), HTTP als `phoenix_auth` mit `SET LOCAL ROLE phoenix_tenant` + `app.current_tenant_id` pro Request (`backend/database/tenant_runtime.go:72-76`); Admin-Variante `phoenix_admin` (BYPASSRLS, Z. 99). Policy (`rls_provisioning.go:32-60`) vergleicht ausschliesslich `tenant_id` gegen einen Einzelwert. Keine Org-Variante, kein `IN`, keine `app.current_org_id`.

`backend/api/common/tenant_middleware.go:96-118`: Tenant-Kontext -> WithinCurrentTenant; Platform-Scope -> platformAdminTransaction; sonst reject. Ein Org-Token ohne tenant_id fällt heute in reject.

Cross-Tenant-Reads existieren via `tenant.WithAdminTx` (`backend/tenant/runtime.go:316-337`): Eltern-Portal (`repositories/parent/child_repository.go:3` "MUST run inside a tenant.WithAdminTx"), `parent_message_read.go`, Operator-Provisioning, `accounts.go:481` (FindByRole im Org-Zweig: genau das Muster für ein Träger-Dashboard). Lektion aus den Eltern-Repos: Queries so bauen, dass die führende Index-Spalte ohne Tenant-Prädikat greift.

Bedeutung: laut D18 Org-Reads unter WithAdminTx mit explizitem `organization_id`-Filter, Schreiben nur per Tenant-Switch mit RLS. Grösster Risikoposten: jede vergessene `organization_id`-Bedingung ist ein tenant-übergreifendes Datenleck ohne Netz. Alternative (zweite Policy-Familie `tenant_id IN (SELECT id FROM platform.schools WHERE organization_id = ...)` auf 58+ Tabellen) wurde in D18 verworfen.

## Kurzfassung

Existiert: Datenmodell org<->school, org_id + scope im JWT, Org-Zweig in Login-Slug-Auflösung, Org-Filter in drei Account-Repo-Methoden, switch-tenant inkl. Switcher, Operator-Trägerverwaltung, Org-Targeting bei Announcements, Portal-Muster (#2207, ~60 Dateien, `portal-shell.tsx` extrahiert), etablierter Cross-Tenant-Lesepfad im Eltern-Portal, ausführliche Doku (D18).

Fehlt: jeder Weg zu einem Org-Token, Account->Organization-Zuordnung, Org-Rolle/Permission, OrgMiddleware/ProtectedOrgGroup/org-taugliche Tx-Middleware, jeder Org-Endpoint und jede aggregierende Query, reservierter Slug, getrennte Session-Cap/Push-Töpfe, kompletter Frontend-Stack, Nutzung von `organizations.settings`, org-weites RLS (bewusst nicht vorgesehen).
