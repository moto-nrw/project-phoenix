# Multi-Tenancy: Datenbank-Schema & Migration

Dieses Dokument beschreibt alle Datenbank-Aenderungen: Neue Tabellen, tenant_id Migration, Indexes, RLS-Policies und die Production-Migrationsstrategie.

**Verwandte Dokumente:**
- [01-architektur.md](01-architektur.md) - Architektur-Entscheidungen (Shared Schema + RLS, Defense-in-Depth)
- [03-backend.md](03-backend.md) - Backend-Code der diese Tabellen nutzt

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

### 1.3 auth.account_tenants (Account -> Tenant N:M)

Ermoeglicht: Betreuer an mehreren OGS, Buero-Mitarbeiter mit Zugriff auf N OGS.

```sql
CREATE TABLE auth.account_tenants (
    account_id        BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
    tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
    is_primary        BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, tenant_id)
);
```

### 1.4 platform.cross_tenant_access (Ferienbetreuung etc.)

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

**Alle 49 Nicht-Platform-Tabellen** brauchen eine `tenant_id` Spalte. Migration in 3 Schritten:

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

### 2.1 Betroffene Tabellen nach Schema (49 total)

| Schema | Tabellen | Anzahl |
|--------|----------|--------|
| auth | accounts, tokens, password_reset_tokens, roles, permissions, role_permissions, account_roles, account_permissions, accounts_parents, password_reset_rate_limits, invitation_tokens, guardian_invitations | 12 |
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

### 2.2 Sonderfaelle

- **`auth.roles`, `auth.permissions`, `auth.role_permissions`**: Koennten global bleiben (systemweite Rollen). **Empfehlung:** tenant_id hinzufuegen fuer Custom-Rollen pro OGS, aber System-Rollen (`is_system = true`) sind global (tenant_id = NULL oder 0).
- **`suggestions.comment_reads`, `suggestions.post_reads`, `platform.announcement_views`**: Composite-PKs muessen um tenant_id erweitert werden.
- **`audit.*` Tabellen**: tenant_id fuer Zuordnung, aber RLS-Policy erlaubt Operator-Zugriff auf alle Audit-Daten.

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
-- Haeufigste Queries: Studenten einer OGS, aktive Besuche einer OGS
CREATE INDEX idx_students_tenant_class ON users.students(tenant_id, school_class);
CREATE INDEX idx_visits_tenant_active ON active.visits(tenant_id, exit_time) WHERE exit_time IS NULL;
CREATE INDEX idx_attendance_tenant_date ON active.attendance(tenant_id, date);
CREATE INDEX idx_devices_tenant ON iot.devices(tenant_id, status);
CREATE INDEX idx_accounts_tenant_email ON auth.accounts(tenant_id, email);
```

---

## 4. RLS-Policies

### 4.1 Datenbank-Rollen Setup

```sql
-- Migrations-Rolle (besitzt Tabellen, nur fuer DDL)
-- Dies ist der aktuelle DB-User - bleibt wie er ist

-- Applikations-Rolle (fuer den Go-Server)
CREATE ROLE phoenix_app WITH LOGIN PASSWORD '...' NOBYPASSRLS;
GRANT USAGE ON ALL SCHEMAS TO phoenix_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit, platform
    TO phoenix_app;
```

### 4.2 RLS-Policy Template (fuer JEDE tenant-scoped Tabelle)

```sql
ALTER TABLE {schema}.{table} ENABLE ROW LEVEL SECURITY;
ALTER TABLE {schema}.{table} FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_{table}
    ON {schema}.{table}
    FOR ALL
    USING (
        -- Platform scope (tenant_id=0) bypasses RLS (fuer Operator)
        (SELECT current_setting('app.current_tenant_id', true))::bigint = 0
        OR tenant_id = (SELECT current_setting('app.current_tenant_id', true))::bigint
    );
```

**Wichtig:** Der `(SELECT ...)` Wrapper erzwingt, dass PostgreSQL `current_setting()` als initPlan (einmalig) evaluiert statt pro Zeile. Das bringt laut Supabase >100x Performance-Verbesserung auf grossen Tabellen.

### 4.3 Platform-Tabellen (kein RLS)

- `platform.organizations` - kein RLS, nur Operator-Zugriff
- `platform.operators` - kein RLS, nur Operator-Zugriff
- `platform.schools` - kein RLS, aber App liest nur eigenen Tenant
- `platform.announcements` - kein RLS, separates Berechtigungssystem
- `platform.operator_audit_log` - kein RLS, nur Operator-Zugriff

---

## 5. RLS Rollout in 3 Phasen

```
Phase 1 (Woche 1-2): ENABLE RLS + permissive Policy USING (true)
    -> Alle Queries funktionieren weiterhin
    -> Verifiziert: Kein Query bricht durch RLS-Aktivierung

Phase 2 (Woche 3-4): Echte Policy + Logging bei Violations
    -> SET LOCAL wird in allen Code-Paths gesetzt
    -> Violations werden geloggt (nicht blockiert)
    -> Ziel: 100% der Requests haben Tenant-Context

Phase 3 (Woche 5+): Strikte Enforcement
    -> current_setting(..., false) statt true
    -> Fehlender Tenant-Context = Error statt leeres Ergebnis
    -> Permissive Fallbacks entfernt
```

---

## 6. Migrations-Strategie fuer Production

### 6.1 Reihenfolge (Zero-Downtime)

```
 1. Migration: platform.organizations + platform.schools erstellen
 2. Migration: Default-Org und Default-School erstellen (ID=1)
 3. Migration: tenant_id (NULLABLE) zu allen Tabellen hinzufuegen
    -> Non-blocking in PostgreSQL (nur Metadaten-Aenderung)
 4. Migration: UPDATE ... SET tenant_id = 1 fuer alle bestehenden Rows
    -> In Batches fuer grosse Tabellen (LIMIT + OFFSET)
 5. Migration: tenant_id NOT NULL Constraint setzen
 6. Migration: Indexes erstellen (CONCURRENTLY)
 7. Migration: RLS Policies erstellen (Phase 1: permissive)
 8. Migration: auth.account_tenants befuellen (alle Accounts -> Tenant 1)
 9. Code-Deploy mit Tenant-Middleware
10. Verification: Alle Queries funktionieren noch
11. Migration: RLS auf strict umstellen (Phase 3)
```

### 6.2 Rollback-Plan

```
Bei Problemen nach Schritt 9:
- Code-Rollback auf vorherige Version (ohne Tenant-Middleware)
- tenant_id Spalten bleiben (stoeren nicht, da nullable oder default)
- RLS-Policies auf USING(true) zuruecksetzen
```

---

## 7. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
