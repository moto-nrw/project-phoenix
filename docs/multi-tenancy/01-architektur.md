# Multi-Tenancy: Architektur-Entscheidungen

Dieses Dokument beschreibt die grundlegenden Architektur-Entscheidungen fuer die Multi-Tenancy-Implementierung. Es basiert auf einer vollstaendigen Codebase-Analyse (55 Tabellen, 54 Repositories, 100+ Frontend-Routes) und Best-Practice-Research (Supabase, Shopify, Slack, PostgreSQL RLS, BUN ORM, Next.js).

**Verwandte Dokumente:**
- [00-anforderungen.md](00-anforderungen.md) - Business-Anforderungen
- [02-datenbank.md](02-datenbank.md) - Datenbank-Schema & RLS
- [03-backend.md](03-backend.md) - Backend-Implementierung
- [04-frontend.md](04-frontend.md) - Frontend-Implementierung
- [05-testing.md](05-testing.md) - Test-Strategie

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

## 2. Defense-in-Depth

**Entscheidung:** RLS als Safety Net + explizite `WHERE tenant_id = ?` Clauses in allen Repositories.

**Begruendung:**
- Doppelte Absicherung: Selbst wenn ein Repository die tenant_id vergisst, blockiert RLS den Zugriff
- Waehrend der Migration kann RLS schrittweise aktiviert werden (permissiv -> logging -> enforced)
- BUN-Maintainer empfiehlt explizite Filterung: "Better to write a little bit more code rather than relying on hooks & hacks"

**Auswirkung:**
- Jedes Repository MUSS `WHERE tenant_id = ?` in allen Queries haben
- RLS faengt vergessene Filter ab (Safety Net)
- Doppelter Check hat minimalen Performance-Impact (tenant_id ist immer ge-indexed)

---

## 3. Tenant = OGS (School)

- `platform.schools.id` ist die `tenant_id` auf allen Tabellen
- `platform.organizations.id` ist die `org_id` (Traeger-Umbrella)
- Die OGS ist die **Daten-Isolationsgrenze**, der Traeger ist die organisatorische Klammer

**Scope-Modell:**

| Scope | tenant_id | Bedeutung | Zugriff |
|-------|-----------|-----------|---------|
| `""` (leer) | > 0 | Normaler User (Betreuer, OGS-Buero) | Nur eigener Tenant |
| `"org"` | > 0 (Haupt-Tenant) | Traeger-Buero | Alle Tenants der Organization |
| `"platform"` | 0 | Operator | Alles (kein RLS) |

---

## 4. TenantModel Mixin (statt base.Model Erweiterung)

**Entscheidung:** Separates `base.TenantModel` Struct, das nur in tenant-scoped Models eingebettet wird.

**Begruendung:**
- Platform-Modelle (Operator, Announcement, Organization, School) haben KEINE tenant_id
- base.Model zu aendern wuerde alle Models betreffen, auch die Platform-Models
- Saubere Trennung: Model embeddet `base.TenantModel` = tenant-scoped, andernfalls = platform-scoped

---

## 5. Per-Tenant Device-PIN (statt globaler Env-Var)

**Entscheidung:** Der IoT Device-PIN wird pro OGS in `platform.schools.device_pin_hash` gespeichert, statt als globale Umgebungsvariable.

**Begruendung:**
- Aktuell: Alle OGS teilen denselben PIN via `OGS_DEVICE_PIN` Env-Var
- Bei Multi-Tenancy: Jede OGS muss ihren eigenen PIN haben
- PIN-Hash wird beim Tenant-Provisioning gesetzt (Operator Dashboard)

---

## 6. Aufwand-Schaetzung

| Bereich | Dateien | Aufwand |
|---------|---------|--------|
| DB Migrations (neue Tabellen, tenant_id, Indexes, RLS) | ~8-10 | 2-3 Wochen |
| tenant/ Package + Middleware | ~5 | 2-3 Tage |
| JWT Claims + Login-Flow | ~10 | 1 Woche |
| base.TenantModel + alle Models | ~50 | 1 Woche |
| Repositories (Defense-in-Depth WHERE) | ~54 | 2-3 Wochen |
| Services (Context-Propagation) | ~29 | 1 Woche |
| IoT Device-Auth + Per-Tenant PIN | ~5 | 3-4 Tage |
| SSE/Realtime Tenant-Isolation | ~3 | 2-3 Tage |
| Frontend Middleware + Auth | ~10 | 1 Woche |
| Frontend API Routes | ~100+ | 1-2 Wochen |
| Frontend SWR Cache-Keys | ~20 | 3-4 Tage |
| Tests (Isolation + Fixtures) | ~40 | 2 Wochen |
| Production Migration + Testing | - | 2-3 Wochen |
| **TOTAL** | **~350 Dateien** | **~14-18 Wochen** |

---

## 7. Risiken

| Risiko | Schwere | Mitigation |
|--------|---------|------------|
| Datenleck zwischen Tenants | KRITISCH | Defense-in-Depth (RLS + WHERE), Isolation-Tests als Pflicht |
| Performance-Regression durch RLS | MITTEL | initPlan-Wrapper, Composite-Indexes, Benchmark bei 100 Tenants |
| Migration bricht Production | HOCH | Zero-Downtime-Strategie, nullable tenant_id, phased RLS |
| Vergessene tenant_id in neuem Code | HOCH | PR-Checkliste, Test-Template, RLS als Safety Net |
| Cross-Schema-Joins ohne tenant_id | MITTEL | Code-Review-Regel: Alle JOINs muessen tenant_id auf beiden Seiten haben |
| SWR-Cache Collision | NIEDRIG | Tenant-prefixed Cache-Keys |
| Cookie-Domain Probleme | NIEDRIG | Testen mit Wildcard-Domain auf Staging |

---

## 8. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse und Best-Practice-Research |
