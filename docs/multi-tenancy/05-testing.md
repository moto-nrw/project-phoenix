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

## 5. Per-Tenant Role Isolation Test (D13 revidiert)

```go
func TestPerTenantRoles_DifferentPermissions(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    // Account erstellen (global)
    account := testpkg.CreateTestAccountInTenant(t, db, tenantA.ID, "maria@example.com")

    // Maria ist Admin bei Tenant A, User bei Tenant B
    adminRole := testpkg.GetSystemRole(t, db, "admin")
    userRole := testpkg.GetSystemRole(t, db, "user")

    testpkg.AssignRoleAtTenant(t, db, account.ID, adminRole.ID, tenantA.ID)
    testpkg.AssignRoleAtTenant(t, db, account.ID, userRole.ID, tenantB.ID)

    // Permissions bei Tenant A: Admin-Rechte
    permsA := loadPermissionsForTenant(t, db, account.ID, tenantA.ID)
    assert.Contains(t, permsA, "users:manage", "Admin at Tenant A must have users:manage")
    assert.Contains(t, permsA, "config:update", "Admin at Tenant A must have config:update")

    // Permissions bei Tenant B: Nur User-Rechte
    permsB := loadPermissionsForTenant(t, db, account.ID, tenantB.ID)
    assert.NotContains(t, permsB, "users:manage", "User at Tenant B must NOT have users:manage")
    assert.NotContains(t, permsB, "config:update", "User at Tenant B must NOT have config:update")
    assert.Contains(t, permsB, "students:read", "User at Tenant B must have students:read")
}

func TestTenantSpecificRole_NotVisibleInOtherTenant(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    // OGS-Admin erstellt eine custom Role bei Tenant A
    customRole := testpkg.CreateTestRoleInTenant(t, db, tenantA.ID, "vertretung")

    // Custom Role ist bei Tenant A sichtbar
    err := tenant.WithTenantTx(context.Background(), db, tenantA.ID,
        func(ctx context.Context, tx bun.Tx) error {
            roles, err := roleRepo.List(ctx)
            require.NoError(t, err)
            assert.True(t, containsRole(roles, "vertretung"),
                "Custom role must be visible at own tenant")
            return nil
        })
    require.NoError(t, err)

    // Custom Role ist bei Tenant B NICHT sichtbar
    err = tenant.WithTenantTx(context.Background(), db, tenantB.ID,
        func(ctx context.Context, tx bun.Tx) error {
            roles, err := roleRepo.List(ctx)
            require.NoError(t, err)
            assert.False(t, containsRole(roles, "vertretung"),
                "Custom role must NOT be visible at other tenant")
            // Aber System-Rollen sind sichtbar
            assert.True(t, containsRole(roles, "admin"),
                "System roles must be visible everywhere")
            return nil
        })
    require.NoError(t, err)
}
```

**Warum:** D13 (revidiert) erlaubt unterschiedliche Rollen pro Tenant. Diese Tests verifizieren: (1) gleicher Account, verschiedene Permissions je nach Tenant, (2) tenant-spezifische Rollen sind nur im eigenen Tenant sichtbar, System-Rollen ueberall.

---

## 6. RowsAffected-Tests (D16)

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
- [ ] Werden Rollen pro Tenant zugewiesen (`account_roles.tenant_id`)? (D13 revidiert)
- [ ] Sind Permission-Loads tenant-spezifisch (nicht global)? (D13 revidiert)
- [ ] Bei Cross-Schema-Joins: Haben beide Seiten des JOINs einen `tenant_id` Filter?
- [ ] Werden Audit-Logs mit der richtigen `tenant_id` geschrieben?
- [ ] Sind SWR Cache-Keys im Frontend tenant-prefixed?
- [ ] Nutzen Advisory Locks die Zwei-Argument-Form `(tenantID, resourceID)`? (D16)

---

## 8. Frontend-Tests (06-#6)

### 8.1 E2E-Tests: Subdomain-Routing + Login

```typescript
// e2e/tenant-routing.spec.ts (Playwright)
test('tenant subdomain resolves correctly', async ({ page }) => {
    await page.goto('http://school-a.localhost:3000/login');
    // resolveTenant() muss "school-a" erkennen
    await expect(page.locator('[data-testid="tenant-name"]')).toContainText('School A');
});

test('unknown subdomain shows not-found', async ({ page }) => {
    await page.goto('http://nichtexistent.localhost:3000/dashboard');
    await expect(page).toHaveTitle(/nicht gefunden|not found/i);
});

test('login sends tenant_slug in body', async ({ page }) => {
    await page.goto('http://school-a.localhost:3000/login');
    const [request] = await Promise.all([
        page.waitForRequest('**/api/auth/login'),
        page.fill('[name="email"]', 'admin@example.com'),
        page.fill('[name="password"]', 'Test1234%'),
        page.click('[type="submit"]'),
    ]);
    const body = JSON.parse(request.postData() || '{}');
    expect(body.tenant_slug).toBe('school-a');
});
```

### 8.2 E2E-Tests: Tenant-Switch

```typescript
test('tenant switch navigates to new subdomain', async ({ page, context }) => {
    // Login bei School A
    await loginAtTenant(page, 'school-a');

    // Switch zu School B
    await page.click('[data-testid="tenant-switcher"]');
    await page.click('[data-testid="tenant-school-b"]');

    // Neue URL muss school-b Subdomain haben
    await expect(page).toHaveURL(/school-b\.localhost:3000\/dashboard/);

    // Daten muessen von School B kommen (nicht gecacht von A)
    const studentNames = await page.locator('[data-testid="student-name"]').allTextContents();
    expect(studentNames).not.toContain('Student aus School A');
});
```

### 8.3 SWR-Cache-Isolationstests

```typescript
// __tests__/use-tenant-swr.test.tsx (React Testing Library)
test('useTenantSWR prefixes key with tenant ID', () => {
    const { result } = renderHook(() => useTenantSWR('/api/students', fetcher), {
        wrapper: ({ children }) => (
            <TenantProvider value={{ tenantId: '42', tenantSlug: 'school-a', /* ... */ }}>
                {children}
            </TenantProvider>
        ),
    });
    // Interner SWR-Key muss "t42:/api/students" sein
    expect(cache.get('t42:/api/students')).toBeDefined();
    expect(cache.get('/api/students')).toBeUndefined();
});

test('tenant switch invalidates SWR cache', async () => {
    // Daten fuer Tenant 42 laden
    await loadStudentsForTenant(42);
    expect(cache.get('t42:/api/students')).toBeDefined();

    // Tenant wechseln zu 99
    await switchTenantTo(99);
    // Alter Cache muss invalidiert sein
    expect(cache.get('t42:/api/students')).toBeUndefined();
});
```

### 8.4 Bruno API-Tests: Multi-Tenant-Szenarien

```bash
# bruno/multi-tenant/ — Neue Test-Suite
# Reihenfolge: Setup → Isolation → Cross-Tenant → Cleanup

# 01-setup.bru: Erstellt 2 Test-Tenants (School A, School B)
# 02-isolation-students.bru: Student in A erstellen, bei B nicht sichtbar
# 03-isolation-rooms.bru: Raum in A erstellen, bei B nicht sichtbar
# 04-tenant-switch.bru: Login bei A, Switch zu B, verify new JWT
# 05-cross-tenant-blocked.bru: B versucht A's Student zu updaten → 404/403
# 06-iot-device-auth.bru: Device-Login mit API-Key + PIN → tenant_id im JWT
# 99-cleanup.bru: Test-Tenants loeschen
```

**Performance-Baseline-Test:** Bruno-Suite mit `--env Multi-Tenant-100` (100 Tenants mit je 50 Students) um Regression durch RLS zu messen. Ziel: < 10% Overhead gegenueber Single-Tenant.

---

## 9. Org-Scope + Cross-Tenant Tests (06-#7)

### 9.1 Org-Scope (D18)

```go
func TestOrgScope_SeesAllTenantsInOrg(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    org := testpkg.CreateTestOrganization(t, db, "Traeger Nord")
    tenantA := testpkg.CreateTestTenantInOrg(t, db, org.ID, "School A")
    tenantB := testpkg.CreateTestTenantInOrg(t, db, org.ID, "School B")

    testpkg.CreateTestStudentInTenant(t, db, tenantA.ID, "Max", "A", "1a")
    testpkg.CreateTestStudentInTenant(t, db, tenantB.ID, "Anna", "B", "2b")

    // OrgScopeService (WithAdminTx + org_id Filter) sieht beide
    orgService := services.NewOrgScopeService(db)
    students, err := orgService.ListStudents(context.Background(), org.ID)
    require.NoError(t, err)
    assert.Len(t, students, 2, "Org scope must see all tenants in org")
}

func TestOrgScope_DoesNotSeeOtherOrg(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    orgNord := testpkg.CreateTestOrganization(t, db, "Traeger Nord")
    orgSued := testpkg.CreateTestOrganization(t, db, "Traeger Sued")
    tenantNord := testpkg.CreateTestTenantInOrg(t, db, orgNord.ID, "School Nord")
    tenantSued := testpkg.CreateTestTenantInOrg(t, db, orgSued.ID, "School Sued")

    testpkg.CreateTestStudentInTenant(t, db, tenantNord.ID, "Max", "Nord", "1a")
    testpkg.CreateTestStudentInTenant(t, db, tenantSued.ID, "Anna", "Sued", "2b")

    orgService := services.NewOrgScopeService(db)
    students, err := orgService.ListStudents(context.Background(), orgNord.ID)
    require.NoError(t, err)
    assert.Len(t, students, 1, "Org scope must NOT see other org's tenants")
    assert.Equal(t, "Max", students[0].FirstName)
}

func TestOrgScope_ReadOnly(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    org := testpkg.CreateTestOrganization(t, db, "Traeger")
    tenantA := testpkg.CreateTestTenantInOrg(t, db, org.ID, "School A")
    student := testpkg.CreateTestStudentInTenant(t, db, tenantA.ID, "Max", "A", "1a")

    // Org-Scope darf NICHT schreiben (D18: read-only)
    orgService := services.NewOrgScopeService(db)
    err := orgService.UpdateStudent(context.Background(), org.ID, student.ID, "2b")
    require.Error(t, err, "Org scope must be read-only")
}
```

### 9.2 Cross-Tenant Access (Ferienbetreuung, D4)

```go
func TestCrossTenantAccess_TemporaryReadAllowed(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "Host OGS")
    tenantB := testpkg.CreateTestTenant(t, db, "Guest OGS")
    account := testpkg.CreateTestAccountInTenant(t, db, tenantB.ID, "betreuer@guest.de")

    // Zeitlich begrenzter Zugriff: Guest-Betreuer darf Host-Daten lesen
    testpkg.CreateCrossTenantAccess(t, db, account.ID, tenantB.ID, tenantA.ID,
        time.Now().Add(-1*time.Hour), time.Now().Add(24*time.Hour))

    // Betreuer kann Students am Host-OGS sehen
    student := testpkg.CreateTestStudentInTenant(t, db, tenantA.ID, "Max", "Host", "1a")
    students, err := crossTenantService.ListStudentsAtHost(context.Background(),
        account.ID, tenantA.ID)
    require.NoError(t, err)
    assert.Len(t, students, 1)
}

func TestCrossTenantAccess_ExpiredDenied(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "Host OGS")
    tenantB := testpkg.CreateTestTenant(t, db, "Guest OGS")
    account := testpkg.CreateTestAccountInTenant(t, db, tenantB.ID, "betreuer@guest.de")

    // Abgelaufener Zugriff
    testpkg.CreateCrossTenantAccess(t, db, account.ID, tenantB.ID, tenantA.ID,
        time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))

    students, err := crossTenantService.ListStudentsAtHost(context.Background(),
        account.ID, tenantA.ID)
    require.NoError(t, err)
    assert.Empty(t, students, "Expired cross-tenant access must return no data")
}
```

---

## 10. Test-Datenbank Kompatibilitaet

Die bestehenden Test-Fixtures (`CreateTestStudent`, `CreateTestStaff` etc.) muessen weiterhin funktionieren. Strategie:

```go
// Bestehende Fixtures nutzen Default-Tenant und WithAdminTx
func CreateTestStudent(t *testing.T, db *bun.DB, first, last, class string) *users.Student {
    return CreateTestStudentInTenant(t, db, defaultTenantID, first, last, class)
}
```

`defaultTenantID` wird in `SetupTestDB` ermittelt (der Default-Tenant der bei Migration erstellt wird). Damit muessen bestehende Tests NICHT sofort umgeschrieben werden — sie laufen alle im Default-Tenant.

---

## 11. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
| 2026-02-08 | Aktualisiert gemaess DEBATE-Entscheidungen: WithTenantTx/WithAdminTx statt Context (D8), kein tenant_id=0 (D7), RowsAffected-Tests (D16), Advisory Lock Tests (D16), PR-Checkliste erweitert |
| 2026-02-10 | D13 revidiert: Per-Tenant Role Isolation Tests (Sektion 5), PR-Checkliste um Rollen-Checks erweitert |
| 2026-02-10 | Frontend-Tests ergaenzt (§8, 06-#6): E2E Subdomain-Routing, Login mit tenant_slug, Tenant-Switch, SWR-Cache-Isolation, Bruno Multi-Tenant-Suite. |
| 2026-02-10 | Org-Scope + Cross-Tenant Tests (§9, 06-#7): OrgScopeService Tests (sieht alle Tenants in Org, nicht andere Orgs, read-only), Ferienbetreuung Tests (temporaerer Zugriff, Ablauf). |
