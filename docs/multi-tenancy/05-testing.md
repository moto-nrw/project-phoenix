# Multi-Tenancy: Test-Strategie

Dieses Dokument beschreibt die Test-Strategie: Neue Fixtures, Isolation-Tests, und Pflicht-Tests fuer PRs.

**Verwandte Dokumente:**
- [02-datenbank.md](02-datenbank.md) - Datenbank-Schema das getestet wird
- [03-backend.md](03-backend.md) - Backend-Code der getestet wird

---

## 1. Neue Test-Fixtures

```go
// test/fixtures.go - NEU
func CreateTestOrganization(t *testing.T, db *bun.DB, name string) *platform.Organization
func CreateTestTenant(t *testing.T, db *bun.DB, name string) *platform.School
func CreateTestTenantInOrg(t *testing.T, db *bun.DB, orgID int64, name string) *platform.School
func CreateTestStudentInTenant(t *testing.T, db *bun.DB, tenantID int64, first, last, class string) *users.Student
func CreateTestStaffInTenant(t *testing.T, db *bun.DB, tenantID int64, first, last string) *users.Staff
func CreateTestAccountInTenant(t *testing.T, db *bun.DB, tenantID int64, email string) *auth.Account
```

**Konsistenz mit bestehenden Fixtures:** Die neuen `InTenant`-Varianten folgen dem gleichen Pattern wie die bestehenden `CreateTestStudent`, `CreateTestStaff` etc., erweitert um den `tenantID` Parameter.

---

## 2. Isolation-Tests (PFLICHT fuer jeden PR)

### 2.1 Standard-Pattern: Tenant A sieht Tenant B nicht

```go
func TestTenantIsolation_Students(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    // Daten in Tenant A erstellen
    studentA := testpkg.CreateTestStudentInTenant(t, db, tenantA.ID, "Max", "A", "1a")

    // Tenant B darf Student A NICHT sehen
    ctxB := tenant.WithTenantID(context.Background(), tenantB.ID)
    students, err := repo.List(ctxB)
    require.NoError(t, err)
    assert.Empty(t, students, "Tenant B must not see Tenant A's students")

    // Tenant A sieht eigene Daten
    ctxA := tenant.WithTenantID(context.Background(), tenantA.ID)
    students, err = repo.List(ctxA)
    require.NoError(t, err)
    assert.Len(t, students, 1)
    assert.Equal(t, studentA.ID, students[0].ID)
}
```

### 2.2 Cross-Schema-Join Isolation

```go
func TestTenantIsolation_VisitsWithAttendance(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    // Visit in Tenant A
    visitA := testpkg.CreateTestVisitInTenant(t, db, tenantA.ID, ...)

    // Query mit Tenant B Context: Darf Visit A NICHT sehen
    ctxB := tenant.WithTenantID(context.Background(), tenantB.ID)
    visits, err := visitRepo.ListActive(ctxB)
    require.NoError(t, err)
    assert.Empty(t, visits, "Cross-schema join must respect tenant isolation")
}
```

---

## 3. Fehlender Tenant-Context Test

```go
func TestMissingTenantContext_Fails(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    // Query OHNE Tenant-Context muss fehlschlagen (nach Phase 3 RLS)
    ctx := context.Background() // Kein tenant_id!
    _, err := repo.List(ctx)
    require.Error(t, err, "Query without tenant context must fail")
}
```

**Hinweis:** Dieser Test funktioniert erst nach RLS Phase 3 (strikte Enforcement). In Phase 1-2 gibt er ein leeres Ergebnis zurueck statt einen Error.

---

## 4. Platform-Scope Bypass Test

```go
func TestPlatformScope_SeesAllTenants(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    testpkg.CreateTestStudentInTenant(t, db, tenantA.ID, "Max", "A", "1a")
    testpkg.CreateTestStudentInTenant(t, db, tenantB.ID, "Anna", "B", "2b")

    // Platform-Scope (tenant_id=0) sieht alle Daten
    ctxPlatform := tenant.WithTenantID(context.Background(), 0)
    students, err := repo.List(ctxPlatform)
    require.NoError(t, err)
    assert.Len(t, students, 2, "Platform scope must see all tenants")
}
```

---

## 5. PR-Checkliste (Code-Review Pflicht)

Jeder PR der eine Repository-Methode aendert **MUSS** geprueft werden auf:

- [ ] Hat die Query einen `tenant_id` Filter? (Defense-in-Depth)
- [ ] Wird bei `Create` die `tenant_id` aus dem Context gesetzt?
- [ ] Gibt es einen Test der verifiziert, dass Tenant A nicht Daten von Tenant B sieht?
- [ ] Bei Cross-Schema-Joins: Haben beide Seiten des JOINs einen `tenant_id` Filter?
- [ ] Werden Audit-Logs mit der richtigen `tenant_id` geschrieben?
- [ ] Sind SWR Cache-Keys im Frontend tenant-prefixed?

---

## 6. Test-Datenbank Kompatibilitaet

Die bestehenden Test-Fixtures (`CreateTestStudent`, `CreateTestStaff` etc.) muessen weiterhin funktionieren. Strategie:

```go
// Bestehende Fixtures nutzen Default-Tenant (ID=1)
func CreateTestStudent(t *testing.T, db *bun.DB, first, last, class string) *users.Student {
    return CreateTestStudentInTenant(t, db, 1, first, last, class)
}
```

Damit muessen bestehende Tests NICHT sofort umgeschrieben werden. Sie laufen alle im Default-Tenant.

---

## 7. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
