# Multi-Tenancy: Datenbank-Schema & Migration

Dieses Dokument beschreibt alle Datenbank-Aenderungen: Neue Tabellen, tenant_id Migration, Indexes, RLS-Policies, Drei-Rollen-Architektur und die Production-Migrationsstrategie.

**Verwandte Dokumente:**
- [01-architektur.md](01-architektur.md) - Architektur-Entscheidungen (Shared Schema + RLS, Defense-in-Depth)
- [03-backend.md](03-backend.md) - Backend-Code der diese Tabellen nutzt
- [DEBATE.md](DEBATE.md) - Alle Diskussionspunkte und Entscheidungen

---

## 1. Neue Tabellen

### 1.1 platform.organizations (Traeger)

```sql
CREATE TABLE platform.organizations (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(200) NOT NULL,
    slug            VARCHAR(100) NOT NULL UNIQUE,
    active          BOOLEAN NOT NULL DEFAULT true,
    settings        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 1.2 platform.schools (OGS / Tenants)

```sql
CREATE TABLE platform.schools (
    id                BIGSERIAL PRIMARY KEY,
    organization_id   BIGINT NOT NULL REFERENCES platform.organizations(id),
    name              VARCHAR(200) NOT NULL,
    slug              VARCHAR(100) NOT NULL,
    subdomain         VARCHAR(100) NOT NULL UNIQUE,
    active            BOOLEAN NOT NULL DEFAULT true,
    settings          JSONB DEFAULT '{}',
    address           TEXT,
    city              VARCHAR(100),
    zip               VARCHAR(20),
    phone             VARCHAR(50),
    email             VARCHAR(255),
    device_pin_hash   VARCHAR(255),  -- Argon2id-Hash des OGS-spezifischen Device-PINs
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, slug)
);

CREATE INDEX idx_schools_subdomain ON platform.schools(subdomain);
CREATE INDEX idx_schools_organization ON platform.schools(organization_id);
```

### 1.3 auth.account_tenants (Account -> Tenant N:M) (D15)

Ermoeglicht: Ein Account, mehrere Tenants. Betreuer an mehreren OGS, Buero-Mitarbeiter mit Zugriff auf N OGS. Soft-Delete via `status = 'inactive'`.

```sql
CREATE TABLE auth.account_tenants (
    id                BIGSERIAL PRIMARY KEY,
    account_id        BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
    tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
    status            TEXT NOT NULL DEFAULT 'active'
                      CHECK (status IN ('pending', 'active', 'inactive')),
    invited_at        TIMESTAMPTZ DEFAULT NOW(),
    activated_at      TIMESTAMPTZ,
    deactivated_at    TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(account_id, tenant_id)
);

CREATE INDEX idx_account_tenants_account ON auth.account_tenants(account_id);
CREATE INDEX idx_account_tenants_tenant ON auth.account_tenants(tenant_id);
CREATE INDEX idx_account_tenants_active ON auth.account_tenants(account_id, tenant_id)
    WHERE status = 'active';
```

### 1.4 platform.cross_tenant_access (Ferienbetreuung etc.) (D4)

Zeitlich begrenzter Cross-Tenant-Zugriff, auch traeger-uebergreifend moeglich.

```sql
CREATE TABLE platform.cross_tenant_access (
    id                BIGSERIAL PRIMARY KEY,
    account_id        BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
    source_tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id),
    target_tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id),
    granted_by        BIGINT NOT NULL REFERENCES auth.accounts(id),
    valid_from        TIMESTAMPTZ NOT NULL,
    valid_until       TIMESTAMPTZ NOT NULL,
    reason            VARCHAR(200),
    active            BOOLEAN NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (source_tenant_id != target_tenant_id),
    CHECK (valid_until > valid_from)
);

CREATE INDEX idx_cross_tenant_account ON platform.cross_tenant_access(account_id);
CREATE INDEX idx_cross_tenant_active ON platform.cross_tenant_access(active, valid_until);
```

### 1.5 platform.operator_organizations

```sql
CREATE TABLE platform.operator_organizations (
    operator_id       BIGINT NOT NULL REFERENCES platform.operators(id) ON DELETE CASCADE,
    organization_id   BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
    role              VARCHAR(50) NOT NULL DEFAULT 'viewer',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (operator_id, organization_id)
);
```

---

## 2. tenant_id zu bestehenden Tabellen

### 2.1 Welche Tabellen bekommen tenant_id?

**Grundregel:** Tabellen mit OGS-spezifischen Daten bekommen `tenant_id`. Tabellen mit globalen Definitionen oder Account-Daten bleiben OHNE (D15).

**KEIN tenant_id (Global/Platform-Scope):**

| Tabelle | Grund |
|---------|-------|
| `auth.accounts` | Globale Identitaet, ein Account fuer alle Tenants (D15) |
| `auth.permissions` | Systemweite Permission-Definitionen (Capabilities sind global) |
| `auth.role_permissions` | Zuordnung Rolle → Permissions (Scoping durch Role-FK, nicht durch eigenen tenant_id) |
| `auth.password_reset_tokens` | Per-Account, nicht per-Tenant |
| `auth.password_reset_rate_limits` | Per-Account Rate-Limiting |
| `platform.*` | Platform-Tabellen haben per Definition kein RLS |

**MIT tenant_id NULLABLE (Auth RBAC-Tabellen, D13 revidiert):**

| Tabelle | tenant_id | Grund |
|---------|-----------|-------|
| `auth.roles` | `BIGINT NULL` | NULL = System-Rolle (admin, user, guardian, guest), NOT NULL = Tenant-spezifische Rolle |
| `auth.account_roles` | `BIGINT NOT NULL` | Rollenzuweisung gilt pro Tenant — gleicher Account kann verschiedene Rollen bei verschiedenen Tenants haben |
| `auth.account_permissions` | `BIGINT NOT NULL` | Direct-Permission-Grants gelten pro Tenant (Sonderrechte) |

**Hinweis:** `auth.permissions` und `auth.role_permissions` bleiben OHNE `tenant_id`. Permission-Definitionen (z.B. `students:read`) sind systemweit. Die Zuordnung welche Permissions eine Rolle hat, ist ebenfalls systemweit — Tenant-spezifische Rollen referenzieren die gleichen globalen Permission-Definitionen. Das Scoping erfolgt durch den `tenant_id` auf `auth.roles` selbst.

**RLS-Policy fuer `auth.roles` (Sonderfall nullable tenant_id):**

```sql
CREATE POLICY tenant_isolation_roles ON auth.roles
    FOR ALL
    USING (
        tenant_id IS NULL  -- System-Rollen sind ueberall sichtbar
        OR tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
    );
```

**MIT tenant_id NOT NULL (Tenant-Scope, ~44 Tabellen):**

| Schema | Tabellen | Anzahl |
|--------|----------|--------|
| auth | tokens, invitation_tokens, accounts_parents, guardian_invitations, account_roles, account_permissions | 6 |
| users | rfid_cards, persons, profiles, staff, teachers, guests, persons_guardians, students, guardian_profiles, students_guardians, privacy_consents, guardian_phone_numbers | 12 |
| education | groups, group_teacher, group_substitution, grade_transitions, grade_transition_mappings, grade_transition_history | 6 |
| facilities | rooms | 1 |
| activities | categories, groups, schedules, supervisors, student_enrollments | 5 |
| active | groups, visits, group_supervisors, combined_groups, group_mappings, attendance, scheduled_checkouts, work_sessions, work_session_breaks, staff_absences | 10 |
| schedule | timeframes, dateframes, recurrence_rules, student_pickup_schedules, student_pickup_exceptions, student_pickup_notes | 6 |
| iot | devices | 1 |
| feedback | entries | 1 |
| config | settings | 1 |
| suggestions | posts, votes, comments, comment_reads, post_reads | 5 |
| audit | data_deletions, auth_events, data_imports, work_session_edits | 4 |

**Hinweis zu `auth.tokens`:** Refresh Tokens enthalten `tenant_id` in den JWT Claims (D12). Die DB-Spalte `tenant_id` ermoeglicht gezieltes Revoken aller Tokens fuer einen bestimmten Tenant (z.B. bei Zugriffsentzug).

### 2.2 Migration (3 Schritte pro Tabelle)

```sql
-- Schritt 1: Spalte hinzufuegen (nullable, non-blocking in PostgreSQL)
ALTER TABLE {schema}.{table}
    ADD COLUMN tenant_id BIGINT REFERENCES platform.schools(id);

-- Schritt 2: Bestehende Daten dem Default-Tenant zuweisen
UPDATE {schema}.{table} SET tenant_id = 1;

-- Schritt 3: NOT NULL Constraint setzen
ALTER TABLE {schema}.{table}
    ALTER COLUMN tenant_id SET NOT NULL;
```

### 2.3 Sonderfaelle

- **`suggestions.comment_reads`, `suggestions.post_reads`**: Composite-PKs muessen um tenant_id erweitert werden.
- **`audit.*` Tabellen**: tenant_id fuer Zuordnung. Operators sehen alle Audit-Daten via `WithAdminTx` (D8).

---

## 3. Indexes

Fuer jede Tabelle mit tenant_id:

```sql
-- Standard-Index fuer tenant_id Filterung
CREATE INDEX idx_{table}_tenant ON {schema}.{table}(tenant_id);

-- Composite-Index fuer haeufige Queries (tenant + PK)
CREATE INDEX idx_{table}_tenant_id ON {schema}.{table}(tenant_id, id);
```

### 3.1 Spezielle Composite-Indexes fuer Performance

```sql
CREATE INDEX idx_students_tenant_class ON users.students(tenant_id, school_class);
CREATE INDEX idx_visits_tenant_active ON active.visits(tenant_id, exit_time) WHERE exit_time IS NULL;
CREATE INDEX idx_attendance_tenant_date ON active.attendance(tenant_id, date);
CREATE INDEX idx_devices_tenant ON iot.devices(tenant_id, status);
```

---

## 4. Drei-Rollen-Architektur (D7, D8)

### 4.1 PostgreSQL-Rollen Setup

```sql
-- Verbindungs-Rolle: LOGIN, aber KEINE eigenen Rechte (sicherster Default)
CREATE ROLE phoenix_auth LOGIN NOINHERIT PASSWORD '...';

-- Tenant-Rolle: Subject to RLS, alle CRUD-Rechte auf Tenant-Tabellen
CREATE ROLE phoenix_tenant NOLOGIN;
GRANT USAGE ON ALL SCHEMAS TO phoenix_tenant;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit TO phoenix_tenant;
GRANT SELECT ON platform.schools TO phoenix_tenant;

-- Admin-Rolle: Bypasses RLS (Operator, Migrations, Seeds, Cross-Tenant)
CREATE ROLE phoenix_admin NOLOGIN BYPASSRLS;
GRANT USAGE ON ALL SCHEMAS TO phoenix_admin;
GRANT ALL ON ALL TABLES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit, platform TO phoenix_admin;

-- Verbindungs-Rolle darf zu beiden switchen
GRANT phoenix_tenant TO phoenix_auth;
GRANT phoenix_admin TO phoenix_auth;
```

**Sicherster Default:** `phoenix_auth` hat NOINHERIT → null Rechte. Vergessene Transaktion = Hard-Fail (Permission Denied), nicht stiller Bypass.

### 4.2 Warum nicht `tenant_id=0` Bypass (D7)

`tenant_id=0` Bypass ist fail-open — ein einziger Bug (vergessene Middleware, Default-Wert) gibt alle Daten frei. Kein ernstzunehmender Multi-Tenant-Anbieter nutzt Magic-Value-Bypasses. Stattdessen: `phoenix_admin` (BYPASSRLS) fuer Operator/Platform-Scope.

---

## 5. RLS-Policies

### 5.1 RLS-Policy Template (fuer JEDE tenant-scoped Tabelle)

```sql
ALTER TABLE {schema}.{table} ENABLE ROW LEVEL SECURITY;
ALTER TABLE {schema}.{table} FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_{table}
    ON {schema}.{table}
    FOR ALL
    USING (
        tenant_id = NULLIF(
            current_setting('app.current_tenant_id', true), ''
        )::bigint
    );
```

**Kein `=0` Bypass in der Policy.** `NULLIF` verhindert Cast-Error bei leerem String → NULL → kein Match → zero rows. Fail-closed: Vergessenes `set_config` → kein Zugriff (sicher).

**Performance:** Der `(SELECT ...)` Wrapper um `current_setting()` ist hier nicht noetig — PostgreSQL evaluiert `current_setting()` als stabilen Ausdruck bereits einmalig pro Scan (initPlan). Supabase empfiehlt den Wrapper fuer komplexere Ausdruecke wie `auth.uid()`, nicht fuer `current_setting()`.

### 5.2 Platform-Tabellen (kein RLS)

- `platform.organizations` - kein RLS, nur Operator-Zugriff (via `WithAdminTx`)
- `platform.operators` - kein RLS, nur Operator-Zugriff
- `platform.schools` - kein RLS, aber `phoenix_tenant` hat nur SELECT-Recht
- `platform.announcements` - kein RLS, separates Berechtigungssystem
- `platform.operator_audit_log` - kein RLS, nur Operator-Zugriff

### 5.3 Views mit security_invoker (D16)

Alle Views auf tenant-scoped Tabellen MUESSEN `security_invoker = true` setzen, da Views standardmaessig als Owner ausgefuehrt werden (SECURITY DEFINER) und damit RLS bypassen.

```sql
-- Bestehende View fixen:
CREATE OR REPLACE VIEW users.expired_privacy_consents
WITH (security_invoker = true) AS
SELECT pc.*, s.person_id
FROM users.privacy_consents pc
JOIN users.students s ON pc.student_id = s.id
WHERE ...;
```

**Regel:** Keine neuen Views auf tenant-scoped Tabellen ohne `security_invoker = true`.

---

## 6. PostgreSQL-Anforderungen (D16)

### 6.1 Mindestversion: PostgreSQL 17.6

| CVE | Beschreibung | Gefixt in |
|-----|-------------|-----------|
| CVE-2024-10976 | Plan-Cache ignoriert Role-Wechsel bei Subqueries/CTEs + SET LOCAL ROLE | PG 17.1 |
| CVE-2025-8713 | Optimizer-Statistiken leaken RLS-versteckte Daten | PG 17.6 |

Docker-Compose und Deployment-Configs muessen `postgres:17.6` oder hoeher verwenden.

### 6.2 Verbotene Patterns

| Pattern | Grund |
|---------|-------|
| Materialized Views auf Tenant-Tabellen | Bypassen RLS komplett |
| COPY FROM auf RLS-Tabellen | PostgreSQL blockiert es |
| SECURITY DEFINER Funktionen mit BYPASSRLS-Owner | Bypassen RLS |
| Views ohne `security_invoker = true` | Bypassen RLS |

---

## 7. RLS Rollout in 3 Phasen

```
Phase 1 (Woche 1-2): ENABLE RLS + permissive Policy USING (true)
    -> Alle Queries funktionieren weiterhin
    -> Verifiziert: Kein Query bricht durch RLS-Aktivierung

Phase 2 (Woche 3-4): Echte Policy + Logging bei Violations
    -> SET LOCAL ROLE wird in allen Code-Paths gesetzt (WithTenantTx/WithAdminTx)
    -> Violations werden geloggt (nicht blockiert)
    -> Ziel: 100% der Requests haben korrekten Transaktions-Context

Phase 3 (Woche 5+): Strikte Enforcement
    -> NULLIF-Policy aktiv (fehlender Context = zero rows)
    -> phoenix_auth NOINHERIT erzwingt: ohne Transaktion = Permission Denied
    -> Permissive Fallbacks entfernt
```

---

## 8. Migrations-Strategie fuer Production

### 8.1 Reihenfolge (Zero-Downtime)

```
 1. Migration: platform.organizations + platform.schools erstellen
 2. Migration: Default-Org und Default-School erstellen (ID=1)
 3. Migration: Drei PostgreSQL-Rollen erstellen (phoenix_auth, phoenix_tenant, phoenix_admin)
 4. Migration: tenant_id (NULLABLE) zu allen ~41 Tabellen hinzufuegen
    -> Non-blocking in PostgreSQL (nur Metadaten-Aenderung)
 5. Migration: UPDATE ... SET tenant_id = 1 fuer alle bestehenden Rows
    -> In Batches fuer grosse Tabellen (LIMIT + OFFSET)
 6. Migration: tenant_id NOT NULL Constraint setzen
 7. Migration: Indexes erstellen (CONCURRENTLY)
 8. Migration: Views mit security_invoker = true aktualisieren
 9. Migration: RLS Policies erstellen (Phase 1: permissive)
10. Migration: auth.account_tenants befuellen (alle Accounts -> Tenant 1)
11. Code-Deploy mit Tenant-Middleware + WithTenantTx/WithAdminTx
12. Verification: Alle Queries funktionieren noch
13. Migration: RLS auf strict umstellen (Phase 3)
```

### 8.2 Rollback-Plan

```
Bei Problemen nach Schritt 11:
- Code-Rollback auf vorherige Version (ohne Tenant-Middleware)
- tenant_id Spalten bleiben (stoeren nicht, da nullable oder default)
- RLS-Policies auf USING(true) zuruecksetzen
- Rollen bleiben bestehen (stoeren nicht)
```

---

## 9. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
| 2026-02-08 | Aktualisiert gemaess DEBATE-Entscheidungen: Drei-Rollen statt phoenix_app (D7/D8), RLS ohne tenant_id=0 Bypass (D7), account_tenants mit Status/Soft-Delete (D15), PG 17.6 (D16), security_invoker (D16) |
| 2026-02-10 | D13 revidiert: auth.roles, auth.account_roles, auth.account_permissions bekommen tenant_id (Per-Tenant RBAC). Spezielle RLS-Policy fuer nullable tenant_id auf auth.roles. ~44 Tabellen mit tenant_id NOT NULL + 1 mit nullable. |
