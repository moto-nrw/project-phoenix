# Multi-Tenancy: Datenbank-Schema & Migration

Dieses Dokument beschreibt alle Datenbank-Aenderungen: Neue Tabellen, tenant_id Migration, Indexes, RLS-Policies, Drei-Rollen-Architektur und die Production-Migrationsstrategie.

> **Status: historischer Planungsstand (Februar 2026).** Dieses Dokument hält fest, wie das Schema zum Zeitpunkt der Multi-Tenancy-Migration aussah und was diese Migration daran geändert hat. Es wird nicht fortgeschrieben. Alle Tabellenlisten, Zählungen, Constraint- und GRANT-Blöcke sind der Stand von damals und beschreiben nicht das heutige Schema. Maßgeblich für den aktuellen Stand sind die Migrationen unter `backend/database/migrations/` und die Modelle unter `backend/models/`.
>
> Seither entfallen: das Schema `suggestions` mit allen Tabellen (`posts`, `votes`, `comments`, `comment_reads`, `post_reads`, `operator_comments`), gelöscht durch Migration 1.15.315 (#2326). Jede Nennung von `suggestions` weiter unten gehört zum historischen Stand.

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

**Stand Februar 2026.** Die folgenden Listen und Zählungen sind seither überholt, siehe Statushinweis oben. Insbesondere existieren die fünf `suggestions`-Tabellen seit Migration 1.15.315 nicht mehr.

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
| `suggestions.operator_comments` | Operator-Kommentare auf Suggestions — Operator ist Platform-Scope, kein tenant_id auf `platform.operators` |
| `meta.migration_metadata` | Infrastruktur-Tabelle fuer Migrations-Tracking, kein Tenant-Bezug |

**Auth RBAC-Tabellen (D13 revidiert) — gemischte Nullable-Strategie:**

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

**MIT tenant_id NOT NULL (Tenant-Scope, 58 Tabellen):**

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

**Gesamtzaehlung (verifiziert gegen Migrations-Dateien):**

| Kategorie | Anzahl |
|-----------|--------|
| Bestehende Tabellen (14 Schemas) | 70 |
| Davon: MIT tenant_id NOT NULL | 58 |
| Davon: MIT tenant_id NULLABLE | 1 (auth.roles) |
| Davon: OHNE tenant_id | 11 (5× auth-global, 4× platform, meta, suggestions.operator_comments) |
| Neue Tabellen (Multi-Tenancy) | 5 (organizations, schools, account_tenants, cross_tenant_access, operator_organizations) |
| **Gesamt nach Migration** | **75** |

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

### 2.4 UNIQUE Constraints Migration (C1, H2)

**Problem:** Bestehende `UNIQUE`-Constraints sind single-tenant. Bei Multi-Tenancy koennen identische Werte in verschiedenen Tenants existieren (z.B. Raum "Turnhalle" bei jeder OGS). Ohne Migration brechen INSERTs ab dem zweiten Tenant.

**Architektur-Entscheidung:** `auth.accounts` bleibt global (D15) — ein Login fuer alle Tenants. `users.persons`, `users.staff` etc. werden **per-Tenant** dupliziert. Ein Betreuer an 2 OGS hat einen Account aber zwei Person-Records.

#### 2.4.1 MUST FIX — Funktional notwendig

Constraint bricht ohne Aenderung, weil gleiche Werte bei verschiedenen Tenants auftreten:

| Schema.Tabelle | Aktuell | Neu | Beispiel |
|----------------|---------|-----|----------|
| `facilities.rooms` | `UNIQUE(name)` | `UNIQUE(tenant_id, name)` | "Turnhalle" bei jeder OGS |
| `education.groups` | `UNIQUE(name)` | `UNIQUE(tenant_id, name)` | "1a" bei jeder OGS |
| `activities.categories` | `UNIQUE(name)` | `UNIQUE(tenant_id, name)` | "Sport" bei jeder OGS |
| `config.settings` | `UNIQUE(key)` | `UNIQUE(tenant_id, key)` | "school_name" pro Tenant |
| `users.persons` | `UNIQUE(account_id)` | `UNIQUE(tenant_id, account_id)` | Ein Account → N Person-Records |
| `users.persons` | `UNIQUE(tag_id)` | `UNIQUE(tenant_id, tag_id)` | RFID-Tag in beiden Person-Records eines Betreuers an 2 OGS |
| `users.profiles` | `UNIQUE(account_id)` | `UNIQUE(tenant_id, account_id)` | Ein Account → N Profile |
| `users.guardian_profiles` | `UNIQUE(email)` | `UNIQUE(tenant_id, email)` | Elternteil bei mehreren OGS |
| `users.guardian_profiles` | `UNIQUE(account_id)` | `UNIQUE(tenant_id, account_id)` | Ein Account → N Guardian-Profile |
| `auth.accounts_parents` | `UNIQUE(email)` | `UNIQUE(tenant_id, email)` | Eltern-Email bei mehreren OGS |
| `auth.accounts_parents` | `UNIQUE(username)` | `UNIQUE(tenant_id, username)` | Username pro Tenant |
| `auth.account_roles` | `UNIQUE(account_id, role_id)` | `UNIQUE(account_id, role_id, tenant_id)` | Gleiche Rolle bei verschiedenen Tenants (D13) |
| `auth.account_permissions` | `UNIQUE(account_id, permission_id)` | `UNIQUE(account_id, permission_id, tenant_id)` | Gleiche Direct-Permissions bei verschiedenen Tenants |

**Sonderfall `auth.roles.name`** (nullable tenant_id):

```sql
-- System-Rollen (tenant_id IS NULL): Name global unique
CREATE UNIQUE INDEX idx_roles_name_system
    ON auth.roles(name) WHERE tenant_id IS NULL;

-- Tenant-Rollen (tenant_id IS NOT NULL): Name unique pro Tenant
CREATE UNIQUE INDEX idx_roles_name_tenant
    ON auth.roles(tenant_id, name) WHERE tenant_id IS NOT NULL;

-- Alter UNIQUE(name) Index entfernen
DROP INDEX IF EXISTS auth.idx_roles_name;
```

#### 2.4.2 MUST FIX — Defense-in-Depth

Constraint bricht technisch nicht (referenzierte IDs sind global-unique PKs), aber `tenant_id` wird fuer Konsistenz und Sicherheit ergaenzt:

| Schema.Tabelle | Aktuell | Neu |
|----------------|---------|-----|
| `users.staff` | `UNIQUE(person_id)` | `UNIQUE(tenant_id, person_id)` |
| `users.teachers` | `UNIQUE(staff_id)` | `UNIQUE(tenant_id, staff_id)` |
| `users.guests` | `UNIQUE(staff_id)` | `UNIQUE(tenant_id, staff_id)` |
| `users.students` | `UNIQUE(person_id)` | `UNIQUE(tenant_id, person_id)` |
| `users.students_guardians` | `UNIQUE(student_id, guardian_profile_id)` | `UNIQUE(tenant_id, student_id, guardian_profile_id)` |
| `users.persons_guardians` | `UNIQUE(person_id, guardian_account_id, relationship_type)` | `+ tenant_id` |
| `education.group_teachers` | `UNIQUE(group_id, teacher_id)` | `UNIQUE(tenant_id, group_id, teacher_id)` |
| `education.group_substitution` | Partial: `UNIQUE(group_id, substitute_staff_id, start_date)` | `+ tenant_id` |
| `education.grade_transition_mappings` | `UNIQUE(transition_id, from_class)` | `+ tenant_id` |
| `activities.student_enrollments` | `UNIQUE(student_id, activity_group_id)` | `+ tenant_id` |
| `activities.supervisors_planned` | `UNIQUE(staff_id, group_id)` | `+ tenant_id` |
| `activities.schedules` | Partial: `UNIQUE(weekday, timeframe_id, activity_group_id)` | `+ tenant_id` |
| `active.group_supervisors` | Partial: `UNIQUE(staff_id, group_id, role)` | `+ tenant_id` |
| `active.group_mappings` | `UNIQUE(active_combined_group_id, active_group_id)` | `+ tenant_id` |
| `active.scheduled_checkouts` | Partial: `UNIQUE(student_id, status)` | `+ tenant_id` |
| `schedule.pickup_schedules` | `UNIQUE(student_id, weekday)` | `+ tenant_id` |
| `schedule.pickup_schedules` | `UNIQUE(student_id, exception_date)` | `+ tenant_id` |
| `suggestions.votes` | `UNIQUE(post_id, voter_id)` | `+ tenant_id` |

#### 2.4.3 OK — Keine Aenderung noetig

| Schema.Tabelle | Constraint | Grund |
|----------------|-----------|-------|
| `auth.accounts` | `UNIQUE(email)`, `UNIQUE(username)` | Globale Tabelle, kein tenant_id (D15) |
| `auth.permissions` | `UNIQUE(name)`, `UNIQUE(resource, action)` | Globale Definitionen |
| `auth.role_permissions` | `UNIQUE(role_id, permission_id)` | Scoping durch Role-FK |
| `auth.invitation_tokens` | `UNIQUE(token)` | Random Token, global unique |
| `auth.guardian_invitations` | `UNIQUE(token)` | Random Token, global unique |
| `iot.devices` | `UNIQUE(device_id)`, `UNIQUE(api_key)` | Hardware-ID / API-Key global unique |
| `platform.organizations` | `UNIQUE(slug)` | Platform-Tabelle, kein RLS |
| `platform.schools` | `UNIQUE(subdomain)` | Platform-Tabelle, kein RLS |
| `platform.operators` | `UNIQUE(email)` | Platform-Tabelle, kein RLS |

#### 2.4.4 Migration SQL Template

```sql
-- Schritt 1: Alten Constraint/Index entfernen
DROP INDEX IF EXISTS {schema}.{old_index_name};
-- oder: ALTER TABLE {schema}.{table} DROP CONSTRAINT {constraint_name};

-- Schritt 2: Neuen Composite-Constraint erstellen
CREATE UNIQUE INDEX {new_index_name}
    ON {schema}.{table}(tenant_id, {columns});

-- Fuer partielle Indexes:
CREATE UNIQUE INDEX {new_index_name}
    ON {schema}.{table}(tenant_id, {columns})
    WHERE {condition};
```

#### 2.4.5 BUN Model Tag Aenderungen

5 Models haben `bun:"...,unique"` Tags die entfernt werden muessen (Constraint wird via Migration-Index erzwungen, nicht via BUN):

| Model | Datei | Tag entfernen |
|-------|-------|---------------|
| `Room` | `models/facilities/room.go` | `bun:"name,notnull,unique"` → `bun:"name,notnull"` |
| `Group` | `models/education/group.go` | `bun:"name,notnull,unique"` → `bun:"name,notnull"` |
| `Category` | `models/activities/category.go` | `bun:"name,notnull,unique"` → `bun:"name,notnull"` |
| `Role` | `models/auth/role.go` | `bun:"name,notnull,unique"` → `bun:"name,notnull"` |
| `Settings` | `models/config/settings.go` | `bun:"key,notnull,unique"` → `bun:"key,notnull"` |

Zusaetzlich: `models/base/base.go` definiert `NameableUnique` (Zeile 57-59). Diese Struct wird zwar nicht direkt eingebettet, aber das Pattern wird in o.g. Models dupliziert. Bei kuenftigen Models **nicht** `NameableUnique` verwenden — stattdessen Composite-Index via Migration.

**Hinweis:** `auth.accounts.username` und `auth.accounts.email` behalten ihre `unique`-Tags, da `auth.accounts` global bleibt (D15).

### 2.5 Composite Foreign Keys (09-H3)

**Problem:** Alle 64 FKs zwischen Tenant-scoped Tabellen pruefen nur `(id)`, nicht `(tenant_id, id)`. Ein Service-Bug koennte eine Gruppe in Tenant A auf einen Raum in Tenant B verlinken. RLS schuetzt bei SELECT, aber bei Admin-Ops (BYPASSRLS) oder direkten DB-Zugriffen gibt es keinen Schutz.

**Entscheidung:** Composite FKs auf DB-Level. Jeder FK zwischen Tenant-scoped Tabellen wird zu `FK(tenant_id, column) → target(tenant_id, id)`.

#### 2.5.1 Voraussetzung: UNIQUE(tenant_id, id) auf Ziel-Tabellen

Composite FKs erfordern einen UNIQUE-Constraint auf `(tenant_id, id)` der referenzierten Tabelle. Da `id` bereits PK ist, ist `(tenant_id, id)` automatisch unique — PostgreSQL braucht trotzdem einen expliziten Index:

```sql
-- Fuer jede Tabelle die als FK-Ziel referenziert wird:
CREATE UNIQUE INDEX idx_{table}_tenant_pk
    ON {schema}.{table}(tenant_id, id);
```

**Betroffene Ziel-Tabellen (18):**

| Schema | Tabelle | Referenziert von (Anzahl FKs) |
|--------|---------|-------------------------------|
| users | persons | 3 (staff, students, iot.devices) |
| users | staff | 12 (active.*, activities.*, education.*, schedule.*) |
| users | students | 8 (active.*, activities.*, schedule.*, feedback.*, audit.*) |
| users | teachers | 1 (education.group_teacher) |
| users | guardian_profiles | 2 (students_guardians, guardian_phone_numbers) |
| users | rfid_cards | 1 (persons.tag_id) |
| education | groups | 3 (group_teacher, students.group_id, group_substitution) |
| education | grade_transitions | 2 (mappings, history) |
| facilities | rooms | 3 (education.groups, activities.groups, active.groups) |
| activities | categories | 1 (activities.groups) |
| activities | groups | 4 (schedules, supervisors_planned, student_enrollments, active.groups) |
| active | groups | 4 (visits, group_supervisors, group_mappings, combined_groups) |
| active | combined_groups | 1 (group_mappings) |
| active | work_sessions | 2 (breaks, edits) |
| iot | devices | 2 (active.groups, active.attendance) |
| schedule | timeframes | 1 (activities.schedules) |
| suggestions | posts | 3 (comments, votes, reads) |
| auth | accounts_parents | 2 (guardian_profiles, persons_guardians) |

#### 2.5.2 FK-Migration nach Schema (64 FKs)

**active Schema (16 FKs):**

| Quelle | Spalte(n) | Ziel |
|--------|-----------|------|
| active.attendance | student_id | users.students |
| active.attendance | checked_in_by | users.staff |
| active.attendance | checked_out_by | users.staff |
| active.attendance | device_id | iot.devices |
| active.groups | group_id | activities.groups |
| active.groups | device_id | iot.devices |
| active.groups | room_id | facilities.rooms |
| active.visits | student_id | users.students |
| active.visits | active_group_id | active.groups |
| active.group_supervisors | staff_id | users.staff |
| active.group_supervisors | group_id | active.groups |
| active.group_mappings | active_combined_group_id | active.combined_groups |
| active.group_mappings | active_group_id | active.groups |
| active.work_sessions | staff_id, created_by, updated_by | users.staff (3x) |
| active.work_session_breaks | session_id | active.work_sessions |
| active.work_session_edits | session_id | active.work_sessions |
| active.staff_absences | staff_id, approved_by, created_by | users.staff (3x) |

**activities Schema (7 FKs):**

| Quelle | Spalte(n) | Ziel |
|--------|-----------|------|
| activities.groups | category_id | activities.categories |
| activities.groups | planned_room_id | facilities.rooms |
| activities.groups | created_by | users.staff |
| activities.schedules | activity_group_id | activities.groups |
| activities.schedules | timeframe_id | schedule.timeframes |
| activities.supervisors_planned | staff_id, group_id | users.staff, activities.groups |
| activities.student_enrollments | student_id, activity_group_id | users.students, activities.groups |

**education Schema (7 FKs):**

| Quelle | Spalte(n) | Ziel |
|--------|-----------|------|
| education.groups | room_id | facilities.rooms |
| education.group_teacher | group_id, teacher_id | education.groups, users.teachers |
| education.group_substitution | group_id | education.groups |
| education.group_substitution | regular_staff_id, substitute_staff_id | users.staff (2x) |
| education.grade_transition_mappings | transition_id | education.grade_transitions |
| education.grade_transition_history | transition_id | education.grade_transitions |

**users Schema (8 FKs):**

| Quelle | Spalte(n) | Ziel |
|--------|-----------|------|
| users.staff | person_id | users.persons |
| users.students | person_id | users.persons |
| users.students | group_id | education.groups |
| users.teachers | staff_id | users.staff |
| users.persons | tag_id | users.rfid_cards |
| users.persons_guardians | person_id | users.persons |
| users.students_guardians | student_id, guardian_profile_id | users.students, users.guardian_profiles |
| users.privacy_consents | student_id | users.students |
| users.guardian_phone_numbers | guardian_profile_id | users.guardian_profiles |
| users.guests | staff_id | users.staff |

**schedule Schema (6 FKs):**

| Quelle | Spalte(n) | Ziel |
|--------|-----------|------|
| schedule.pickup_schedules | student_id, created_by | users.students, users.staff |
| schedule.pickup_exceptions | student_id, created_by | users.students, users.staff |
| schedule.pickup_notes | student_id, created_by | users.students, users.staff |
| schedule.scheduled_checkouts | student_id, scheduled_by, cancelled_by | users.students, users.staff (2x) |

**Weitere (4 FKs):**

| Quelle | Spalte(n) | Ziel |
|--------|-----------|------|
| feedback.entries | student_id | users.students |
| audit.data_deletions | student_id | users.students |
| iot.devices | registered_by_id | users.persons |
| suggestions.comments, votes, reads | post_id | suggestions.posts (3x) |

#### 2.5.3 Migration SQL Template

```sql
-- Schritt 1: UNIQUE(tenant_id, id) auf Ziel-Tabelle (falls nicht bereits vorhanden)
CREATE UNIQUE INDEX CONCURRENTLY idx_students_tenant_pk
    ON users.students(tenant_id, id);

-- Schritt 2: Alten FK droppen
ALTER TABLE active.visits DROP CONSTRAINT fk_active_visits_student;

-- Schritt 3: Neuen Composite FK erstellen
ALTER TABLE active.visits
    ADD CONSTRAINT fk_active_visits_student
    FOREIGN KEY (tenant_id, student_id)
    REFERENCES users.students(tenant_id, id)
    ON DELETE CASCADE;
```

**Reihenfolge:** Zuerst alle `UNIQUE(tenant_id, id)` Indexes erstellen (Schritt 1 fuer alle 18 Ziel-Tabellen), dann alle FKs migrieren. `CREATE INDEX CONCURRENTLY` vermeidet Locks auf Produktions-Tabellen.

#### 2.5.4 Ausnahmen (KEIN Composite FK)

| FK | Grund |
|----|-------|
| `→ auth.accounts` | Global, kein tenant_id (D15) |
| `→ platform.schools` | Tenant-Tabelle selbst, referenced by tenant_id |
| `→ auth.accounts_parents` | Abhaengig von Entscheidung ob global oder per-tenant |
| `→ auth.permissions`, `auth.role_permissions` | System-Tabellen, global oder nullable tenant_id |

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

**Nicht zum Abtippen.** Die Rollen und Rechte werden von den Migrationen gesetzt (ab 1.14.1), nicht von diesem Block. Er zeigt den Stand von Februar 2026 und nennt unter anderem das Schema `suggestions`, das es nicht mehr gibt; unverändert ausgeführt bricht er deshalb ab.

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

-- SEQUENCE-Rechte (09-H1): Ohne USAGE auf Sequences schlaegt jeder INSERT fehl
-- (BIGSERIAL PKs erstellen implizite Sequences, 60+ im gesamten Schema)
GRANT USAGE ON ALL SEQUENCES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit TO phoenix_tenant;

-- Default Privileges: Zukuenftige Migrationen erben automatisch die Rechte
ALTER DEFAULT PRIVILEGES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO phoenix_tenant;
ALTER DEFAULT PRIVILEGES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit
    GRANT USAGE ON SEQUENCES TO phoenix_tenant;

-- Admin-Rolle: Bypasses RLS (Operator, Migrations, Seeds, Cross-Tenant)
CREATE ROLE phoenix_admin NOLOGIN BYPASSRLS;
GRANT USAGE ON ALL SCHEMAS TO phoenix_admin;
GRANT ALL ON ALL TABLES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit, platform TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit, platform TO phoenix_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit, platform
    GRANT ALL ON TABLES TO phoenix_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit, platform
    GRANT ALL ON SEQUENCES TO phoenix_admin;

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
| CVE-2025-8713 | Optimizer-Statistiken leaken RLS-versteckte Daten | PG 17.6 (NICHT VERIFIZIERT — 09-M5) |

**Hinweis (09-M5):** CVE-2025-8713 konnte nicht verifiziert werden. Mindestversion ist PG 17.1 (fuer CVE-2024-10976, verifiziert). PG 17.6+ als Empfehlung beibehalten (neueste Patch-Version), aber hartes Requirement ist PG 17.1.

Docker-Compose und Deployment-Configs sollten `postgres:17` (latest) oder mindestens `postgres:17.1` verwenden.

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
 4. Migration: tenant_id (NULLABLE, DEFAULT 1) zu allen ~41 Tabellen hinzufuegen
    -> Non-blocking in PostgreSQL (nur Metadaten-Aenderung)
    -> DEFAULT 1 verhindert NULL-Inserts zwischen Step 4 und 6
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
Bei Problemen nach Phase 1 (tenant_id nullable, RLS permissive):
- Code-Rollback auf vorherige Version (ohne Tenant-Middleware)
- tenant_id Spalten bleiben (stoeren nicht, da nullable + default)
- RLS-Policies auf USING(true) zuruecksetzen
- Rollen bleiben bestehen (stoeren nicht)

Bei Problemen nach Phase 2 (tenant_id NOT NULL):
- Zusaetzlich: ALTER COLUMN tenant_id DROP NOT NULL fuer alle Tabellen
- Sonst schlagen Inserts ohne tenant_id fehl
- Script: SELECT 'ALTER TABLE ' || schemaname || '.' || tablename ||
         ' ALTER COLUMN tenant_id DROP NOT NULL;'
         FROM pg_tables WHERE schemaname NOT IN ('pg_catalog', 'information_schema');
```

---

## 9. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
| 2026-02-08 | Aktualisiert gemaess DEBATE-Entscheidungen: Drei-Rollen statt phoenix_app (D7/D8), RLS ohne tenant_id=0 Bypass (D7), account_tenants mit Status/Soft-Delete (D15), PG 17.6 (D16), security_invoker (D16) |
| 2026-02-10 | D13 revidiert: auth.roles, auth.account_roles, auth.account_permissions bekommen tenant_id (Per-Tenant RBAC). Spezielle RLS-Policy fuer nullable tenant_id auf auth.roles. ~44 Tabellen mit tenant_id NOT NULL + 1 mit nullable. |
| 2026-02-10 | UNIQUE Constraints Migration (Sektion 2.4): 13 funktional notwendige + 18 Defense-in-Depth + 1 Sonderfall (auth.roles nullable). 9 Constraints OK ohne Aenderung. BUN Model Tag Aenderungen dokumentiert. |
| 2026-02-10 | SEQUENCE GRANTs ergaenzt (09-H1): `GRANT USAGE ON ALL SEQUENCES` fuer phoenix_tenant + phoenix_admin. `ALTER DEFAULT PRIVILEGES` fuer zukuenftige Migrationen. Ohne diese schlaegt jeder INSERT fehl (60+ Sequences durch BIGSERIAL PKs). |
| 2026-02-10 | Composite Foreign Keys (Sektion 2.5, 09-H3): 64 FKs werden zu `FK(tenant_id, col) → target(tenant_id, id)`. 18 Ziel-Tabellen bekommen `UNIQUE(tenant_id, id)`. Vollstaendige Auflistung nach Schema. |
| 2026-02-10 | Tabellen-Zaehlung korrigiert (06-#2): "~44" → 58 NOT NULL. Gesamtzaehlung gegen Migrations-Dateien verifiziert: 70 bestehende Tabellen in 14 Schemas. `suggestions.operator_comments` und `meta.migration_metadata` als OHNE tenant_id klassifiziert. |
| 2026-08-22 | Statushinweis ergänzt: Dokument ist der Planungsstand Februar 2026 und wird nicht fortgeschrieben. Das Schema `suggestions` ist durch Migration 1.15.315 (#2326) entfallen, die Tabellenlisten, Zählungen und GRANT-Blöcke sind entsprechend als historisch markiert. |
