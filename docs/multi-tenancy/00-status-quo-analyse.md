# Status Quo Analyse: Multi-Tenancy Readiness

## Executive Summary

Project Phoenix ist aktuell eine **vollständig Single-Tenant-Anwendung**. Es gibt **keinerlei** Multi-Tenancy-Konzepte in der gesamten Codebase - weder im Backend, Frontend, noch in PyrePortal. Die Umstellung auf Multi-Tenancy betrifft **jeden Layer** des Systems.

---

## 1. Bestandsaufnahme: Was existiert heute

### 1.1 Datenbank (PostgreSQL 17, 12 Schemas)

| Schema | Tabellen (ca.) | Tenant-Relevanz |
|--------|----------------|-----------------|
| `auth` | accounts, tokens, roles, permissions, role_permissions, account_roles, account_permissions, invitation_tokens, password_reset_tokens, password_reset_rate_limits | **KRITISCH** - Alle User/Accounts muessen einem Tenant zugeordnet werden |
| `users` | persons, staff, students, teachers, guardians, rfid_cards, profiles, privacy_consents, guardian_profiles | **KRITISCH** - Kernentitaeten die tenant-isoliert sein muessen |
| `education` | groups, group_substitutions | **KRITISCH** - Gruppen gehoeren zu einer OGS |
| `facilities` | rooms | **KRITISCH** - Raeume gehoeren zu einer OGS |
| `activities` | categories, groups, schedules, supervisors_planned, student_enrollments | **KRITISCH** - Aktivitaeten gehoeren zu einer OGS |
| `active` | groups, visits, group_supervisors, combined_groups, group_mappings, attendance | **KRITISCH** - Echtzeit-Tracking ist OGS-spezifisch |
| `schedule` | timeframes, dateframes, recurrence_rules | HOCH - Stundenplaene pro OGS |
| `iot` | devices | **KRITISCH** - Geraete sind physisch an eine OGS gebunden |
| `feedback` | entries | MITTEL - Feedback pro OGS |
| `config` | settings | HOCH - Einstellungen pro OGS |
| `suggestions` | posts, comments, operator_comments, comment_reads, post_reads | HOCH - Vorschlaege pro OGS |
| `platform` | operators, announcements, announcement_views, operator_audit_log | **SONDER** - Liegt AUSSERHALB der Tenant-Grenze |
| `audit` | auth_events, data_deletions | HOCH - Audit-Logs pro Tenant |

**Fazit:** Alle ~50+ Tabellen in 11 Schemas (ausser `platform`) brauchen eine `tenant_id` Spalte.

### 1.2 Basis-Model (Kein Tenant-Feld)

```go
// models/base/base.go - AKTUELL
type Model struct {
    ID        int64     `bun:"id,pk,autoincrement"`
    CreatedAt time.Time `bun:"created_at,..."`
    UpdatedAt time.Time `bun:"updated_at,..."`
}
```

**Problem:** `int64` auto-increment IDs sind nur innerhalb einer Tabelle einzigartig, NICHT global. Bei Multi-Tenancy koennten verschiedene Tenants die gleichen IDs haben (z.B. Student ID 1 in OGS A und Student ID 1 in OGS B).

### 1.3 JWT Token (Kein Tenant-Feld)

```go
// auth/jwt/claims.go - AKTUELL
type AppClaims struct {
    ID          int      `json:"id"`           // Account-ID
    Sub         string   `json:"sub"`          // Email
    Roles       []string `json:"roles"`        // ["admin", "user"]
    Permissions []string `json:"permissions"`  // ["users:read", ...]
    IsAdmin     bool     `json:"is_admin"`
    Scope       string   `json:"scope"`        // "platform" oder ""
    // FEHLT: TenantID, OrgID, SchoolID
}
```

**Einziger Ansatz von Scope:** `Scope: "platform"` unterscheidet Operator-Tokens von normalen Tokens. Das ist ein guter Startpunkt, aber kein Tenant-Routing.

### 1.4 Account-Model (Keine Org-Zuordnung)

```go
// models/auth/account.go - AKTUELL
type Account struct {
    base.Model
    Email        string    // unique
    PasswordHash *string   // Argon2id
    PINHash      *string   // Fuer RFID
    Active       bool
    // FEHLT: TenantID, OrgID
}
```

**Die Verbindung Account -> Person -> Staff/Student -> Group ist rein relational, ohne jegliche Tenant-Zuordnung.**

### 1.5 Frontend (Zero Tenant Awareness)

| Bereich | Status |
|---------|--------|
| Subdomain-Handling | **Nicht vorhanden** - Kein Hostname-Parsing, kein Middleware |
| Tenant-Context | **Nicht vorhanden** - Kein React Context, kein Provider |
| API-Routes | **Nicht vorhanden** - ~150 Route-Handler ohne Tenant-Parameter |
| Auth-Flow | **Nicht vorhanden** - NextAuth Session hat kein Tenant-Feld |
| Operator-Auth | **Nicht vorhanden** - Cookie-basiert, kein Tenant-Bezug |
| SWR Cache | **Nicht vorhanden** - Cache-Keys ohne Tenant-Prefix |
| URL-Struktur | **Nicht vorhanden** - Keine Tenant-Segmente in URLs |

### 1.6 PyrePortal (Transparent)

**Gute Nachricht:** PyrePortal authentifiziert sich via `DEVICE_API_KEY`. Der Backend-Server scoped bereits alle IoT-Antworten basierend auf dem Device-Record. Multi-Tenancy kann hier **groesstenteils transparent** implementiert werden, indem der API-Key einem Tenant zugeordnet wird.

**Einzige Aenderung:** Eventuell ein `X-Tenant-ID` Header oder die API-Key-zu-Tenant-Zuordnung auf dem Server.

### 1.7 Operator Dashboard (Bereits teilweise isoliert)

Das Operator-Dashboard existiert bereits als **separater Layer**:
- Eigene Auth (`platform.operators`, nicht `auth.accounts`)
- Eigener JWT-Scope (`scope: "platform"`)
- Eigenes Route-Mounting (`/operator/...`)
- Eigene Frontend-Route-Group (`(operator)`)
- `platform.announcements` hatte sogar mal ein `target_school_ids BIGINT[]` Feld (wurde durch `target_roles` ersetzt)

**Fazit:** Der Operator-Layer ist konzeptionell schon "ausserhalb" des Tenants. Er braucht aber die Faehigkeit, Tenants zu verwalten.

---

## 2. Betroffene Code-Bereiche (Quantifiziert)

### Backend (Go)

| Bereich | Dateien (ca.) | Aufwand |
|---------|---------------|---------|
| Models (alle Domains) | ~40 Dateien | Alle brauchen `TenantID` |
| Repositories (alle Domains) | ~35 Dateien | Alle Queries brauchen Tenant-Filter |
| Services (alle Domains) | ~30 Dateien | Tenant-Context durchreichen |
| API Handlers | ~25 Dateien | Tenant aus JWT/Context extrahieren |
| Auth/JWT | ~10 Dateien | TenantID in Claims + Middleware |
| Migrations | ~60 Dateien | Neue Migration fuer tenant_id + RLS |
| Factories | 2 Dateien | Tenant-Context in Factory-Chain |
| Middleware | ~5 Dateien | Neues Tenant-Middleware |
| Tests | ~40 Dateien | Alle Tests brauchen Tenant-Fixtures |
| **TOTAL** | **~250 Dateien** | |

### Frontend (Next.js)

| Bereich | Dateien (ca.) | Aufwand |
|---------|---------------|---------|
| API Route Handlers | ~150 Dateien | Tenant-Header forwarden |
| API Client Libraries | ~20 Dateien | Tenant-Context einfuegen |
| Pages | ~35 Dateien | Wenig Aenderung (data-layer Sache) |
| Components | ~80 Dateien | Minimal (UI bleibt gleich) |
| Auth/Session | ~10 Dateien | Tenant in Session + Cookies |
| Middleware | 1 Datei | Subdomain -> Tenant Routing |
| Contexts/Providers | ~10 Dateien | Tenant-Provider, Cache-Keys |
| **TOTAL** | **~300 Dateien** | |

### PyrePortal (Tauri)

| Bereich | Dateien (ca.) | Aufwand |
|---------|---------------|---------|
| API Service | 1 Datei | Minimal (Header hinzufuegen) |
| Config | 1 Datei | Eventuell `TENANT_ID` |
| **TOTAL** | **~2 Dateien** | |

---

## 3. Risiko-Einschaetzung

### Hoechstes Risiko: Datenlecks zwischen Tenants
- Ohne RLS und ohne Tenant-Filter in JEDER Query kann ein fehlerhafter Code-Path Daten von fremden OGS zurueckgeben
- **Mitigation:** RLS als Safety Net + Tenant-Middleware als erste Verteidigungslinie

### Hohes Risiko: Migration bestehender Daten
- Die aktuelle Production-Datenbank (eine Schule) muss einem "Default-Tenant" zugeordnet werden
- Alle bestehenden Rows brauchen eine `tenant_id`

### Hohes Risiko: Parallele Entwicklung
- 2-3 Developer arbeiten gleichzeitig an einem System das in Production ist
- Feature-Development muss parallel zum Refactoring weitergehen koennen

### Mittleres Risiko: Performance
- Jede Query bekommt einen zusaetzlichen WHERE-Clause
- RLS-Policies haben einen kleinen Overhead
- **Mitigation:** Composite Indexes auf `(tenant_id, id)` fuer alle Tabellen

### Mittleres Risiko: Test-Coverage
- ~40 Test-Dateien muessen alle Tenant-aware gemacht werden
- Bruno API-Tests muessen Multi-Tenant-Szenarien abdecken
