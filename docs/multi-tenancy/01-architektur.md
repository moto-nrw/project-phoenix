# Multi-Tenancy: Architektur-Entscheidungen

Dieses Dokument beschreibt die grundlegenden Architektur-Entscheidungen fuer die Multi-Tenancy-Implementierung. Es basiert auf einer vollstaendigen Codebase-Analyse (55 Tabellen, 54 Repositories, 100+ Frontend-Routes) und Best-Practice-Research (Supabase, PostgREST, Shopify, Slack, PostgreSQL RLS, BUN ORM, Next.js).

**Verwandte Dokumente:**
- [00-anforderungen.md](00-anforderungen.md) - Business-Anforderungen
- [02-datenbank.md](02-datenbank.md) - Datenbank-Schema & RLS
- [03-backend.md](03-backend.md) - Backend-Implementierung
- [04-frontend.md](04-frontend.md) - Frontend-Implementierung
- [05-testing.md](05-testing.md) - Test-Strategie
- [DEBATE.md](DEBATE.md) - Alle Diskussionspunkte und Entscheidungen

---

## 1. Isolationsstrategie: Shared Schema + RLS

**Entscheidung:** Alle Tenants teilen sich dasselbe Datenbank-Schema. Row Level Security (RLS) erzwingt die Datenisolation auf DB-Ebene.

**Begruendung:**
- 100-500 Tenants ist klar im Bereich einer Single-DB-Loesung (Shopify, Slack, Supabase bestaetigen das)
- Schema-per-Tenant ist mit BUN ORM nicht praktikabel (59 Models mit Hardcoded Schema-Tags, 55 BeforeAppendModel-Hooks)
- RLS bietet DSGVO-konforme Isolation auf Datenbankebene, nicht nur im Application-Code

**Verworfene Alternativen:**
| Ansatz | Grund fuer Ablehnung |
|--------|---------------------|
| Schema-per-Tenant | BUN ORM bindet Schema-Namen per Struct-Tag, kein dynamisches Switching moeglich |
| Database-per-Tenant | Massiver Overhead bei 100+ Tenants, Cross-Tenant-Queries unmoeglich |
| Application-only Filtering | Kein DB-Level-Schutz, ein vergessener Filter = Datenleck |

---

## 2. Drei-Rollen-Architektur + SET LOCAL ROLE (D7, D8)

**Entscheidung:** Ein Connection Pool als `phoenix_auth` (NOINHERIT) + `SET LOCAL ROLE` pro Transaktion. Kein `tenant_id=0` Bypass in RLS-Policies.

**Begruendung:** `tenant_id=0` Bypass ist fail-open — ein einziger Bug gibt alle Tenant-Daten frei. Kein ernstzunehmender Multi-Tenant-Anbieter (Supabase, PostgREST, AWS, Citus, Crunchy Data, Nile) nutzt Magic-Value-Bypasses.

**PostgreSQL-Rollen:**

| Rolle | Attribute | Zweck |
|-------|-----------|-------|
| `phoenix_auth` | LOGIN, NOINHERIT | Verbindungs-Rolle, keine eigenen Rechte |
| `phoenix_tenant` | NOLOGIN | Tenant-scoped Queries, subject to RLS |
| `phoenix_admin` | NOLOGIN, BYPASSRLS | Operator, Migrations, Seeds, Cross-Tenant |

**Transaktions-Wrapper:**
- `WithTenantTx(ctx, db, tenantID, fn)` — setzt `SET LOCAL ROLE phoenix_tenant` + `set_config('app.current_tenant_id', ...)`. Fuer 99% aller Requests.
- `WithAdminTx(ctx, db, fn)` — setzt `SET LOCAL ROLE phoenix_admin`. Fuer Operator-Routes, Migrations, Cleanup, Cross-Tenant.

**Sicherster Default:** `phoenix_auth` hat NOINHERIT → null Rechte. Vergessene Transaktion = Hard-Fail (Permission Denied), nicht stiller Bypass.

**Voraussetzung:** PostgreSQL >= 17.6 (CVE-2024-10976 + CVE-2025-8713 gefixt). Siehe D16.

Siehe [DEBATE.md D7](DEBATE.md#d7-tenant_id0-rls-bypass-ist-ein-sicherheitsrisiko) und [D8](DEBATE.md#d8-set_config-transaction-sicherheit--base-repository-transaction-gap) fuer vollstaendige Begruendung.

---

## 3. Defense-in-Depth (vier unabhaengige Schichten)

**Entscheidung:** Vier unabhaengige Sicherheitsschichten. Jede Schicht kann einzeln versagen, ohne Datenleck.

| Schicht | Mechanismus | Bei Versagen |
|---------|-------------|-------------|
| 1. RLS | `WHERE tenant_id = current_setting(...)` | Zero rows (silent filter) |
| 2. Repository | `WHERE tenant_id = ?` (explizit) | Zero rows (explicit filter) |
| 3. Policy Engine | `Subject.TenantID != Resource.TenantID` → Deny | Hard deny + error log |
| 4. RowsAffected | Check nach UPDATE/DELETE | Fehler statt stille Nicht-Aenderung |

**Doppelter Check hat minimalen Performance-Impact** (tenant_id ist immer ge-indexed).

---

## 4. Tenant = OGS (School)

- `platform.schools.id` ist die `tenant_id` auf allen Tabellen
- `platform.organizations.id` ist die `org_id` (Traeger-Umbrella)
- Die OGS ist die **Daten-Isolationsgrenze**, der Traeger ist die organisatorische Klammer

**Scope-Modell:**

| Scope | Bedeutung | DB-Rolle | Zugriff |
|-------|-----------|----------|---------|
| `""` (leer) | Normaler User (Betreuer, OGS-Buero) | `phoenix_tenant` (RLS aktiv) | Nur eigener Tenant |
| `"org"` | Traeger-Buero | `phoenix_tenant` (RLS aktiv, Haupt-Tenant) | Alle Tenants der Organization via Tenant-Switch (D4) |
| `"platform"` | Operator | `phoenix_admin` (BYPASSRLS) | Alles |

**Wichtig:** Operators nutzen `phoenix_admin` (BYPASSRLS) — nicht `tenant_id=0`. Kein Magic-Value-Bypass in RLS-Policies (D7).

---

## 5. TenantModel Mixin (statt base.Model Erweiterung) (D2)

**Entscheidung:** Separates `base.TenantModel` Struct mit `GetTenantID()` und `SetTenantID()`, das nur in tenant-scoped Models eingebettet wird.

**Begruendung:**
- Platform-Modelle (Operator, Announcement, Organization, School) haben KEINE tenant_id
- base.Model zu aendern wuerde alle Models betreffen, auch die Platform-Models
- Saubere Trennung: Model embeddet `base.TenantModel` = tenant-scoped, andernfalls = platform-scoped

**Kein BeforeAppendModel auf TenantModel** (D10): 55 von 57 Models shadowen den Hook. Service-Layer setzt `tenant_id` explizit via `SetTenantID()`. Defense-in-Depth: NOT NULL Constraint + RLS.

---

## 6. Kein QueryHook (D9)

**Entscheidung:** QueryHook entfaellt komplett. `set_config()` wird einmal pro Transaktion im `WithTenantTx` Wrapper gesetzt (D8).

**Begruendung:** BUN QueryHook ist fundamental kaputt mit Connection-Pooling — Hook bekommt Connection A, Query bekommt Connection B. SET LOCAL ROLE in expliziten Transaktionen (D8) loest das Problem und macht QueryHook obsolet.

**Performance-Gewinn:** 1 `set_config()` pro Transaktion statt N+1 (einmal pro Query).

---

## 7. Two-Tier Authorization (D14)

**Entscheidung:** Middleware fuer statische JWT-Checks, Service-Layer fuer dynamische DB-basierte Policy-Evaluation.

| Tier | Frage | Schicht | DB noetig? |
|------|-------|---------|------------|
| Tier 1 (statisch) | "Hat dieser User Permission `visits:read`?" | Middleware (`RequiresPermission`) | Nein (JWT) |
| Tier 2 (dynamisch) | "Ist dieser Lehrer in der Gruppe dieses Schuelers?" | Service (Policy Engine) | Ja |

**Fail-closed Tenant-Assert in Engine.Authorize():** `Subject.TenantID > 0` aber `Resource.TenantID == 0` → Deny. Vergessenes TenantID-Tagging wird erkannt statt still uebersprungen.

D8 erzwingt diese Migration: Policy-Middleware ruft Services auf, die `r.getDB(ctx)` nutzen. Ohne Transaktion faellt getDB auf `r.DB` zurueck (`phoenix_auth`, NOINHERIT) → Permission Denied. Policy-Eval mit DB-Zugriff MUSS innerhalb von `WithTenantTx` laufen.

---

## 8. Per-Tenant Device-PIN (statt globaler Env-Var)

**Entscheidung:** Der IoT Device-PIN wird pro OGS in `platform.schools.device_pin_hash` gespeichert, statt als globale Umgebungsvariable.

**Begruendung:**
- Aktuell: Alle OGS teilen denselben PIN via `OGS_DEVICE_PIN` Env-Var
- Bei Multi-Tenancy: Jede OGS muss ihren eigenen PIN haben
- PIN-Hash wird beim Tenant-Provisioning gesetzt (Operator Dashboard)

---

## 9. PostgreSQL-Anforderungen (D16)

- **Mindestversion: PostgreSQL 17.6** (CVE-2024-10976 + CVE-2025-8713 gefixt)
- **RowsAffected-Checks:** Alle UPDATE/DELETE Operationen muessen `RowsAffected()` pruefen (72% tun es aktuell nicht — Silent Failures unter RLS)
- **Views mit `security_invoker = true`:** Views bypassen RLS standardmaessig
- **Advisory Locks: Zwei-Argument-Form** `pg_advisory_xact_lock(tenantID, activityID)` statt Multiplikation

---

## 10. Aufwand-Schaetzung

| Bereich | Dateien | Aufwand |
|---------|---------|--------|
| DB Migrations (neue Tabellen, tenant_id, Indexes, RLS) | ~8-10 | 2-3 Wochen |
| tenant/ Package + WithTenantTx/WithAdminTx | ~5 | 2-3 Tage |
| JWT Claims + Login-Flow + RefreshClaims | ~10 | 1 Woche |
| base.TenantModel + alle Models | ~50 | 1 Woche |
| Repositories (Defense-in-Depth WHERE + getDB + RowsAffected) | ~54 | 2-3 Wochen |
| Services (WithTenantTx Wrapping + Policy Migration) | ~29 | 1-2 Wochen |
| IoT Device-Auth + Per-Tenant PIN | ~5 | 3-4 Tage |
| SSE/Realtime Tenant-Isolation | ~3 | 2-3 Tage |
| Frontend Middleware (Rewrite) + [tenant]/layout.tsx | ~10 | 1 Woche |
| Frontend API Routes | ~100+ | 1-2 Wochen |
| Frontend SWR Cache-Keys | ~20 | 3-4 Tage |
| Tests (Isolation + Fixtures) | ~40 | 2 Wochen |
| Production Migration + Testing | - | 2-3 Wochen |
| **TOTAL** | **~350 Dateien** | **~14-18 Wochen** |

---

## 11. Risiken

| Risiko | Schwere | Mitigation |
|--------|---------|------------|
| Datenleck zwischen Tenants | KRITISCH | Defense-in-Depth (4 Schichten), Isolation-Tests als Pflicht |
| Performance-Regression durch RLS | MITTEL | SET LOCAL ROLE (1x pro Tx), Composite-Indexes, Benchmark bei 100 Tenants |
| Migration bricht Production | HOCH | Zero-Downtime-Strategie, nullable tenant_id, phased RLS |
| Vergessene tenant_id in neuem Code | HOCH | CI-Check (D10), NOT NULL Constraint, RLS als Safety Net |
| Cross-Schema-Joins ohne tenant_id | MITTEL | Code-Review-Regel: Alle JOINs muessen tenant_id auf beiden Seiten haben |
| SWR-Cache Collision | NIEDRIG | Tenant-prefixed Cache-Keys |
| Cookie-Domain Probleme | NIEDRIG | Testen mit Wildcard-Domain auf Staging |
| RowsAffected nicht geprueft | HOCH | assertRowsAffected Helper (D16), 72% der UPDATE/DELETE betroffen |

---

## 12. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse und Best-Practice-Research |
| 2026-02-08 | Aktualisiert gemaess DEBATE-Entscheidungen D7-D17: Drei-Rollen-Architektur, SET LOCAL ROLE, kein QueryHook, kein tenant_id=0 Bypass, Two-Tier Authorization, PG 17.6+ |
