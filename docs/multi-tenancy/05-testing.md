# Multi-Tenancy: Test-Strategie

Dieses Dokument beschreibt die Test-Strategie: Neue Fixtures, Isolation-Tests, und Pflicht-Tests fuer PRs. Alle Tests nutzen `WithTenantTx`/`WithAdminTx` (D8) fuer korrekten Transaktions-Context.

**Verwandte Dokumente:**
- [02-datenbank.md](02-datenbank.md) - Datenbank-Schema das getestet wird
- [03-backend.md](03-backend.md) - Backend-Code der getestet wird
- [DEBATE.md](DEBATE.md) - Alle Diskussionspunkte und Entscheidungen

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

**Fixtures erstellen Daten mit `WithAdminTx`** (BYPASSRLS), damit sie unabhaengig vom Tenant-Context angelegt werden koennen.

---

## 2. Isolation-Tests (PFLICHT fuer jeden PR)

### 2.1 Standard-Pattern: Tenant A sieht Tenant B nicht

```go
func TestTenantIsolation_Students(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    // Daten in Tenant A erstellen (via WithAdminTx, da Fixtures)
    studentA := testpkg.CreateTestStudentInTenant(t, db, tenantA.ID, "Max", "A", "1a")

    // Tenant B darf Student A NICHT sehen (via WithTenantTx — D8)
    var studentsB []users.Student
    err := tenant.WithTenantTx(context.Background(), db, tenantB.ID,
        func(ctx context.Context, tx bun.Tx) error {
            var err error
            studentsB, err = repo.List(ctx)
            return err
        })
    require.NoError(t, err)
    assert.Empty(t, studentsB, "Tenant B must not see Tenant A's students")

    // Tenant A sieht eigene Daten (via WithTenantTx — D8)
    var studentsA []users.Student
    err = tenant.WithTenantTx(context.Background(), db, tenantA.ID,
        func(ctx context.Context, tx bun.Tx) error {
            var err error
            studentsA, err = repo.List(ctx)
            return err
        })
    require.NoError(t, err)
    assert.Len(t, studentsA, 1)
    assert.Equal(t, studentA.ID, studentsA[0].ID)
}
```

### 2.2 Cross-Schema-Join Isolation

```go
func TestTenantIsolation_VisitsWithAttendance(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    // Visit in Tenant A
    testpkg.CreateTestVisitInTenant(t, db, tenantA.ID, ...)

    // Query mit Tenant B Context: Darf Visit A NICHT sehen
    err := tenant.WithTenantTx(context.Background(), db, tenantB.ID,
        func(ctx context.Context, tx bun.Tx) error {
            visits, err := visitRepo.ListActive(ctx)
            require.NoError(t, err)
            assert.Empty(t, visits, "Cross-schema join must respect tenant isolation")
            return nil
        })
    require.NoError(t, err)
}
```

---

## 3. Fehlender Transaktions-Context Test (D8)

```go
func TestMissingTransaction_PermissionDenied(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    // Query OHNE WithTenantTx/WithAdminTx → phoenix_auth (NOINHERIT) hat keine Rechte
    ctx := context.Background()
    _, err := repo.List(ctx)
    require.Error(t, err, "Query without transaction must fail with permission denied")
    // phoenix_auth NOINHERIT = kein Zugriff auf Tabellen = Hard-Fail
}
```

**Warum:** `phoenix_auth` hat NOINHERIT und keine eigenen Rechte (D8). Ohne explizite Transaktion mit `SET LOCAL ROLE` schlaegt jeder Query fehl — fail-closed by design.

---

## 4. Admin-Scope Test (WithAdminTx)

```go
func TestAdminScope_SeesAllTenants(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    testpkg.CreateTestStudentInTenant(t, db, tenantA.ID, "Max", "A", "1a")
    testpkg.CreateTestStudentInTenant(t, db, tenantB.ID, "Anna", "B", "2b")

    // Admin-Scope (WithAdminTx, BYPASSRLS) sieht alle Daten
    var allStudents []users.Student
    err := tenant.WithAdminTx(context.Background(), db,
        func(ctx context.Context, tx bun.Tx) error {
            var err error
            allStudents, err = repo.List(ctx)
            return err
        })
    require.NoError(t, err)
    assert.Len(t, allStudents, 2, "Admin scope must see all tenants")
}
```

---

## 5. RowsAffected-Tests (D16)

```go
func TestRowsAffected_CrossTenantUpdateFails(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    // Student in Tenant A erstellen
    studentA := testpkg.CreateTestStudentInTenant(t, db, tenantA.ID, "Max", "A", "1a")

    // Versuch: Tenant B versucht Student A zu updaten
    err := tenant.WithTenantTx(context.Background(), db, tenantB.ID,
        func(ctx context.Context, tx bun.Tx) error {
            return repo.UpdateSchoolClass(ctx, studentA.ID, "2b")
        })

    // Muss fehlschlagen (RowsAffected = 0, assertRowsAffected wirft Error)
    require.Error(t, err, "Cross-tenant update must fail via RowsAffected check")
}
```

---

## 6. Advisory Lock Tenant-Isolation (D16)

```go
func TestAdvisoryLock_TenantIsolated(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")
    activityID := int64(42)

    // Lock in Tenant A
    err := tenant.WithTenantTx(context.Background(), db, tenantA.ID,
        func(ctx context.Context, tx bun.Tx) error {
            _, err := tx.ExecContext(ctx,
                "SELECT pg_advisory_xact_lock(?, ?)", tenantA.ID, activityID)
            require.NoError(t, err)

            // Parallel: Lock in Tenant B mit gleichem activityID — darf NICHT blockieren
            done := make(chan error, 1)
            go func() {
                done <- tenant.WithTenantTx(context.Background(), db, tenantB.ID,
                    func(ctx2 context.Context, tx2 bun.Tx) error {
                        _, err := tx2.ExecContext(ctx2,
                            "SELECT pg_advisory_xact_lock(?, ?)", tenantB.ID, activityID)
                        return err
                    })
            }()

            select {
            case err := <-done:
                assert.NoError(t, err, "Different tenant must not block")
            case <-time.After(2 * time.Second):
                t.Fatal("Advisory lock blocked across tenants — missing tenant isolation")
            }

            return nil
        })
    require.NoError(t, err)
}
```

---

## 7. PR-Checkliste (Code-Review Pflicht)

Jeder PR der eine Repository-Methode aendert **MUSS** geprueft werden auf:

- [ ] Laeuft die Query innerhalb von `WithTenantTx` oder `WithAdminTx`? (D8)
- [ ] Hat die Query einen `tenant_id` Filter? (Defense-in-Depth)
- [ ] Wird bei `Create` die `tenant_id` im Service via `SetTenantID()` gesetzt? (D10)
- [ ] Wird `RowsAffected()` nach UPDATE/DELETE geprueft? (D16)
- [ ] Gibt es einen Test der verifiziert, dass Tenant A nicht Daten von Tenant B sieht?
- [ ] Bei Cross-Schema-Joins: Haben beide Seiten des JOINs einen `tenant_id` Filter?
- [ ] Werden Audit-Logs mit der richtigen `tenant_id` geschrieben?
- [ ] Sind SWR Cache-Keys im Frontend tenant-prefixed?
- [ ] Nutzen Advisory Locks die Zwei-Argument-Form `(tenantID, resourceID)`? (D16)

---

## 8. Test-Datenbank Kompatibilitaet

Die bestehenden Test-Fixtures (`CreateTestStudent`, `CreateTestStaff` etc.) muessen weiterhin funktionieren. Strategie:

```go
// Bestehende Fixtures nutzen Default-Tenant und WithAdminTx
func CreateTestStudent(t *testing.T, db *bun.DB, first, last, class string) *users.Student {
    return CreateTestStudentInTenant(t, db, defaultTenantID, first, last, class)
}
```

`defaultTenantID` wird in `SetupTestDB` ermittelt (der Default-Tenant der bei Migration erstellt wird). Damit muessen bestehende Tests NICHT sofort umgeschrieben werden — sie laufen alle im Default-Tenant.

---

## 9. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
| 2026-02-08 | Aktualisiert gemaess DEBATE-Entscheidungen: WithTenantTx/WithAdminTx statt Context (D8), kein tenant_id=0 (D7), RowsAffected-Tests (D16), Advisory Lock Tests (D16), PR-Checkliste erweitert |
