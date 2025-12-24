# Testing Types Explained - Quick Reference

**Last Updated**: 2025-12-21

A practical guide to understanding different types of tests, when to use them, and how they're implemented.

---

## Test Types Overview

```
Scope/Complexity
    ↑
    │                          🎭 E2E Tests
    │                         (Full system, slow)
    │
    │                  🔗 Integration Tests
    │                 (Multiple components)
    │
    │        🧩 Unit Tests
    │       (Single function/component)
    │
    └──────────────────────────────────────→ Speed
      Fast                              Slow
```

---

## 1. Unit Tests

### What They Test
**Single function, method, or component in isolation**
- One thing at a time
- No external dependencies (database, API, file system)
- Uses mocks/stubs for dependencies

### When to Use
- Testing business logic
- Validating data transformations
- Checking edge cases and error handling
- Most common type (70% of your tests)

### Implementation

**Backend (Go) - Testing a Service Function**:
```go
// What we're testing: CalculateRetentionDate function
func TestCalculateRetentionDate(t *testing.T) {
    // Arrange - Setup test data
    baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

    // Act - Call the function
    result := CalculateRetentionDate(baseTime, ptr(7))

    // Assert - Verify result
    expected := baseTime.AddDate(0, 0, 7)
    assert.Equal(t, expected, result)
}

// Testing with mocked dependencies
func TestCreateStudent(t *testing.T) {
    // Arrange - Create mock repository
    mockRepo := new(MockStudentRepository)
    service := NewStudentService(mockRepo)

    // Setup mock expectation
    mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

    // Act - Call service method
    student, err := service.Create(ctx, &CreateInput{
        PersonID: 1,
        DataRetentionDays: ptr(30),
    })

    // Assert
    require.NoError(t, err)
    assert.NotNil(t, student)
    mockRepo.AssertExpectations(t) // Verify mock was called correctly
}
```

**Frontend (React) - Testing a Component**:
```typescript
// What we're testing: Login button behavior
describe("LoginButton", () => {
  it("should call onLogin when clicked", async () => {
    // Arrange - Create mock function
    const mockOnLogin = vi.fn();
    const user = userEvent.setup();

    // Render component with mock
    render(<LoginButton onLogin={mockOnLogin} />);

    // Act - Simulate user click
    await user.click(screen.getByRole("button", { name: /login/i }));

    // Assert - Verify mock was called
    expect(mockOnLogin).toHaveBeenCalledTimes(1);
  });

  it("should disable button when loading", () => {
    // Arrange
    render(<LoginButton onLogin={vi.fn()} isLoading={true} />);

    // Assert - Check button state
    const button = screen.getByRole("button", { name: /login/i });
    expect(button).toBeDisabled();
  });
});
```

**Characteristics**:
- ⚡ **Very Fast** (milliseconds per test)
- 🔒 **Isolated** (no external dependencies)
- 🎯 **Focused** (tests one thing)
- 🔄 **Easy to debug** (small scope)

---

## 2. Integration Tests

### What They Test
**Multiple components working together**
- Database interactions
- API endpoints with real data
- Multiple services collaborating
- Component interactions in UI

### When to Use
- Testing data flow between layers
- Verifying database queries work correctly
- Testing API endpoints with real database
- Checking multiple components interact properly

### Implementation

**Backend - Database Integration**:
```go
// What we're testing: Repository + Database interaction
func TestStudentRepository_Integration(t *testing.T) {
    // Arrange - Setup REAL test database
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    repo := repositories.NewStudentRepository(db)

    // Act - Perform real database operation
    student := &users.Student{
        PersonID: 1,
        DataRetentionDays: ptr(30),
    }
    err := repo.Create(context.Background(), student)

    // Assert - Verify in database
    require.NoError(t, err)
    assert.NotZero(t, student.ID) // Database assigned ID

    // Act - Retrieve from database
    retrieved, err := repo.GetByID(context.Background(), student.ID)

    // Assert - Verify data persisted correctly
    require.NoError(t, err)
    assert.Equal(t, student.PersonID, retrieved.PersonID)
}
```

**Backend - API Handler Integration**:
```go
// What we're testing: HTTP Handler + Service + Repository + Database
func TestLoginHandler_Integration(t *testing.T) {
    // Arrange - Setup real components (not mocked)
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    // Create test user in database
    createTestUser(t, db, "test@example.com", "Password123!")

    // Create real HTTP request
    reqBody := `{"email":"test@example.com","password":"Password123!"}`
    req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(reqBody))
    w := httptest.NewRecorder()

    // Act - Call handler (hits service, repository, database)
    handler := NewAuthHandler(db)
    handler.Login(w, req)

    // Assert - Check HTTP response
    assert.Equal(t, 200, w.Code)

    var response LoginResponse
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.NotEmpty(t, response.AccessToken)
}
```

**Frontend - Multi-Component Integration**:
```typescript
// What we're testing: Form + API + State Management
describe("StudentCheckInForm Integration", () => {
  it("should submit form and update student list", async () => {
    // Arrange - Setup API mock
    server.use(
      http.post("/api/checkins", async ({ request }) => {
        const body = await request.json();
        return HttpResponse.json({
          status: "success",
          data: { id: "1", ...body },
        });
      }),
      http.get("/api/students/active", () => {
        return HttpResponse.json({
          status: "success",
          data: [{ id: "1", name: "John Doe", room: "101" }],
        });
      })
    );

    const user = userEvent.setup();

    // Render multiple components together
    render(<CheckInPage />);

    // Act - Fill and submit form
    await user.type(screen.getByLabelText(/student/i), "John Doe");
    await user.selectOptions(screen.getByLabelText(/room/i), "101");
    await user.click(screen.getByRole("button", { name: /check in/i }));

    // Assert - Verify student appears in active list (different component)
    await waitFor(() => {
      expect(screen.getByText("John Doe")).toBeInTheDocument();
      expect(screen.getByText("Room 101")).toBeInTheDocument();
    });
  });
});
```

**Characteristics**:
- 🐢 **Slower** (seconds per test)
- 🔗 **Multiple components** (tests interaction)
- 💾 **Real dependencies** (database, file system)
- 🧪 **More realistic** (closer to production)

---

## 3. End-to-End (E2E) Tests

### What They Test
**Entire user flow from start to finish**
- Real browser interactions
- Full application stack (frontend + backend + database)
- User journeys and workflows
- Cross-page navigation

### When to Use
- Testing critical user paths
- Validating complete workflows
- Regression testing for major features
- Smoke testing after deployment

### Implementation

**Playwright - Full User Journey**:
```typescript
// What we're testing: Complete login → create session → logout flow
test("complete session management workflow", async ({ page }) => {
  // Step 1: Login
  await page.goto("http://localhost:3000/login");
  await page.fill('input[name="email"]', "admin@example.com");
  await page.fill('input[name="password"]', "Test1234%");
  await page.click('button[type="submit"]');

  // Assert: Redirected to dashboard
  await expect(page).toHaveURL("/dashboard");
  await expect(page.locator("h1")).toContainText("Dashboard");

  // Step 2: Navigate to sessions
  await page.click('a[href="/sessions"]');
  await expect(page).toHaveURL("/sessions");

  // Step 3: Create new session
  await page.click('button:has-text("New Session")');
  await page.selectOption('select[name="groupId"]', "1");
  await page.selectOption('select[name="roomId"]', "101");
  await page.click('button:has-text("Create Session")');

  // Assert: Success message and session appears
  await expect(page.locator("text=/session created/i")).toBeVisible();
  await expect(page.locator("text=Group A")).toBeVisible();
  await expect(page.locator("text=Room 101")).toBeVisible();

  // Step 4: End session
  await page.click('button:has-text("End Session")');
  await page.click('button:has-text("Confirm")');

  // Assert: Session removed
  await expect(page.locator("text=/session ended/i")).toBeVisible();
  await expect(page.locator("text=Group A")).not.toBeVisible();

  // Step 5: Logout
  await page.click('[data-testid="user-menu"]');
  await page.click("text=Logout");

  // Assert: Back to login
  await expect(page).toHaveURL("/login");
});
```

**E2E with Page Object Model** (Better organization):
```typescript
// pages/LoginPage.ts - Encapsulate page interactions
class LoginPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto("/login");
  }

  async login(email: string, password: string) {
    await this.page.fill('input[name="email"]', email);
    await this.page.fill('input[name="password"]', password);
    await this.page.click('button[type="submit"]');
  }

  async expectSuccess() {
    await expect(this.page).toHaveURL("/dashboard");
  }
}

// tests/auth.spec.ts - Use page objects
test("should login and access dashboard", async ({ page }) => {
  const loginPage = new LoginPage(page);
  const dashboardPage = new DashboardPage(page);

  await loginPage.goto();
  await loginPage.login("admin@example.com", "Test1234%");
  await loginPage.expectSuccess();

  await dashboardPage.expectToBeLoaded();
  await expect(dashboardPage.getWelcomeMessage()).toContainText("Welcome");
});
```

**Characteristics**:
- 🐌 **Slowest** (minutes for full suite)
- 🌐 **Real browser** (Chrome, Firefox, Safari)
- 🎬 **Full stack** (all services running)
- 💰 **Expensive** (takes time and resources)
- 🔍 **Finds real bugs** (catches integration issues)

---

## 4. API Tests

### What They Test
**HTTP endpoints and API contracts**
- Request/response validation
- Status codes and error handling
- API performance and reliability
- Authentication and authorization

### When to Use
- Validating API contracts
- Testing backend without UI
- Integration testing for microservices
- Contract testing between teams

### Implementation

**Bruno - API Test File**:
```bruno
# 01-auth.bru - Authentication Tests

# Test 1: Successful login
POST {{baseUrl}}/auth/login
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "Test1234%"
}

# Assertions
post-response {
  const { status, data } = res.body;

  // Verify response structure
  assert.equal(res.status, 200);
  assert.equal(status, "success");

  // Verify token returned
  assert.isDefined(data.accessToken);
  assert.isDefined(data.refreshToken);

  // Save token for subsequent requests
  bru.setVar("accessToken", data.accessToken);
}

---

# Test 2: Invalid credentials
POST {{baseUrl}}/auth/login
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "WrongPassword"
}

# Assertions
post-response {
  assert.equal(res.status, 401);
  assert.equal(res.body.status, "error");
  assert.include(res.body.message, "Invalid credentials");
}

---

# Test 3: Protected endpoint (requires auth)
GET {{baseUrl}}/api/students
Authorization: Bearer {{accessToken}}

# Assertions
post-response {
  assert.equal(res.status, 200);
  assert.isArray(res.body.data);

  // Validate schema
  if (res.body.data.length > 0) {
    const student = res.body.data[0];
    assert.hasAllKeys(student, ["id", "person_id", "data_retention_days"]);
  }
}
```

**Go - HTTP Handler Test**:
```go
// Testing API endpoint directly
func TestStudentHandler_List(t *testing.T) {
    // Arrange - Setup test server
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    handler := NewStudentHandler(db)
    req := httptest.NewRequest("GET", "/api/students?limit=10", nil)
    w := httptest.NewRecorder()

    // Add authentication
    ctx := addAuthToContext(req.Context(), &User{ID: 1})
    req = req.WithContext(ctx)

    // Act - Call handler
    handler.List(w, req)

    // Assert - Check response
    assert.Equal(t, 200, w.Code)

    var response APIResponse
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.Equal(t, "success", response.Status)
    assert.NotNil(t, response.Data)
}
```

**Characteristics**:
- ⚡ **Fast** (milliseconds to seconds)
- 🔌 **No UI** (tests backend only)
- 📝 **Contract validation** (ensures API stability)
- 🔄 **Easy to automate** (simple HTTP requests)

---

## 5. Performance Tests

### What They Test
**Speed, scalability, and resource usage**
- Response times under load
- Throughput (requests per second)
- Resource consumption (CPU, memory)
- Breaking points and bottlenecks

### When to Use
- Before production deployment
- After major changes
- Establishing performance baselines
- Finding bottlenecks

### Implementation

**Go Benchmarks - Micro Performance**:
```go
// Testing function performance
func BenchmarkCalculateRetentionDate(b *testing.B) {
    baseTime := time.Now()
    retentionDays := ptr(30)

    // b.N automatically adjusted by Go to get accurate results
    for i := 0; i < b.N; i++ {
        CalculateRetentionDate(baseTime, retentionDays)
    }
}

// Testing with memory profiling
func BenchmarkStudentList(b *testing.B) {
    db := setupTestDB(b)
    defer db.Close()

    repo := repositories.NewStudentRepository(db)

    // Create 1000 students for realistic test
    createTestStudents(b, repo, 1000)

    b.ResetTimer()
    b.ReportAllocs() // Track memory allocations

    for i := 0; i < b.N; i++ {
        _, err := repo.List(context.Background(), &ListOptions{
            Limit: 50,
        })
        if err != nil {
            b.Fatal(err)
        }
    }
}

// Run: go test -bench=. -benchmem
// Output:
// BenchmarkStudentList-8    1000   1250000 ns/op   524288 B/op   1024 allocs/op
//                           ^      ^                ^             ^
//                           runs   time per op      bytes/op      allocations
```

**k6 - Load Testing**:
```javascript
// Testing API under load
import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  stages: [
    { duration: "30s", target: 10 },   // Ramp to 10 users
    { duration: "1m", target: 50 },    // Ramp to 50 users
    { duration: "2m", target: 50 },    // Stay at 50 users
    { duration: "30s", target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ["p(95)<500"],  // 95% of requests < 500ms
    http_req_failed: ["rate<0.01"],    // Error rate < 1%
  },
};

export default function () {
  // Login
  const loginRes = http.post("http://localhost:8080/auth/login", {
    email: "admin@example.com",
    password: "Test1234%",
  });

  check(loginRes, {
    "status is 200": (r) => r.status === 200,
  });

  const token = loginRes.json("data.accessToken");

  // API calls
  const studentsRes = http.get("http://localhost:8080/api/students", {
    headers: { Authorization: `Bearer ${token}` },
  });

  check(studentsRes, {
    "status is 200": (r) => r.status === 200,
    "response time < 500ms": (r) => r.timings.duration < 500,
  });

  sleep(1); // Think time between requests
}

// Run: k6 run loadtest.js
// Output shows RPS, response times, error rates
```

**Characteristics**:
- 📊 **Metrics-focused** (numbers, not pass/fail)
- ⏱️ **Time-intensive** (minutes to hours)
- 📈 **Reveals bottlenecks** (scalability issues)
- 💰 **Resource-heavy** (needs dedicated environment)

---

## 6. Security Tests

### What They Test
**Vulnerabilities and security weaknesses**
- SQL injection
- XSS (Cross-Site Scripting)
- Authentication/authorization flaws
- Dependency vulnerabilities
- Sensitive data exposure

### When to Use
- Before every release
- After dependency updates
- Regular security audits
- Before production deployment

### Implementation

**Static Analysis - gosec (Go)**:
```bash
# Install
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run security scan
gosec ./...

# Example output identifies issues:
# [G401] Use of weak cryptographic primitive
# [G304] Potential file inclusion via variable
# [G201] SQL string formatting
```

**Dependency Scanning**:
```bash
# Go dependencies
go list -json -m all | nancy sleuth

# Node dependencies
npm audit
npm audit --audit-level=moderate

# Fix automatically (if possible)
npm audit fix
```

**Manual Security Test (Playwright)**:
```typescript
test("should prevent XSS injection", async ({ page }) => {
  await loginAsAdmin(page);
  await page.goto("/students/new");

  // Try to inject script
  const xssPayload = '<script>alert("XSS")</script>';
  await page.fill('input[name="name"]', xssPayload);
  await page.click('button[type="submit"]');

  // Verify script is NOT executed (HTML escaped)
  await page.goto("/students");
  const studentCell = page.locator(`td:has-text("${xssPayload}")`);

  // Should display as text, not execute
  await expect(studentCell).toBeVisible();
  await expect(page.locator("alert")).not.toBeVisible();
});

test("should prevent SQL injection", async ({ page }) => {
  await page.goto("/login");

  // Try SQL injection
  await page.fill('input[name="email"]', "admin' OR '1'='1");
  await page.fill('input[name="password"]', "anything");
  await page.click('button[type="submit"]');

  // Should NOT login
  await expect(page.locator("text=/invalid credentials/i")).toBeVisible();
  await expect(page).toHaveURL("/login");
});
```

**Characteristics**:
- 🛡️ **Critical for production** (prevents breaches)
- 🔍 **Automated + Manual** (tools + human testing)
- 📋 **Compliance-focused** (OWASP, GDPR)
- 🔄 **Continuous** (ongoing activity)

---

## 7. Accessibility Tests

### What They Test
**Usability for people with disabilities**
- Screen reader compatibility
- Keyboard navigation
- Color contrast
- ARIA labels and semantic HTML

### When to Use
- For every UI component
- Before major releases
- WCAG compliance requirements
- Legal/regulatory compliance

### Implementation

**Automated - axe-core**:
```typescript
import { axe, toHaveNoViolations } from "jest-axe";

expect.extend(toHaveNoViolations);

test("login page has no accessibility violations", async () => {
  const { container } = render(<LoginPage />);

  // Run automated accessibility check
  const results = await axe(container);

  // Fails if violations found
  expect(results).toHaveNoViolations();
});
```

**Playwright - Keyboard Navigation**:
```typescript
test("should be fully keyboard navigable", async ({ page }) => {
  await page.goto("/sessions");

  // Tab through elements
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toHaveText("Dashboard");

  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toHaveText("Sessions");

  // Enter to activate
  await page.keyboard.press("Enter");
  await expect(page.locator('[role="dialog"]')).toBeVisible();

  // Escape to close
  await page.keyboard.press("Escape");
  await expect(page.locator('[role="dialog"]')).not.toBeVisible();
});
```

**Manual Testing Checklist**:
```typescript
test("should have proper ARIA labels", async ({ page }) => {
  await page.goto("/checkins");

  // Form accessibility
  const form = page.locator("form");
  await expect(form).toHaveAttribute("aria-label", "Student check-in form");

  // Button accessibility
  const button = page.locator('button[type="submit"]');
  await expect(button).toHaveAccessibleName("Check in student");

  // Error announcements
  await button.click();
  const error = page.locator('[role="alert"]');
  await expect(error).toHaveAttribute("aria-live", "polite");
});
```

**Characteristics**:
- ♿ **Legally required** (ADA, WCAG)
- 🤖 **Automated + Manual** (tools catch 30-40%)
- 🎨 **Design-focused** (color, layout, semantics)
- 👥 **User-centric** (real users testing)

---

## Quick Comparison Table

| Test Type | Speed | Scope | Dependencies | Cost | When to Use |
|-----------|-------|-------|--------------|------|-------------|
| **Unit** | ⚡⚡⚡ | Single function | None (mocked) | 💰 | Always (70% of tests) |
| **Integration** | ⚡⚡ | Multiple components | Real (DB, API) | 💰💰 | Data flow validation |
| **E2E** | ⚡ | Full application | All (Browser, DB) | 💰💰💰 | Critical user paths |
| **API** | ⚡⚡ | HTTP endpoints | Backend only | 💰 | Contract validation |
| **Performance** | ⚡ | System under load | Production-like | 💰💰💰 | Before releases |
| **Security** | ⚡⚡ | Vulnerabilities | Varies | 💰💰 | Every release |
| **Accessibility** | ⚡⚡ | UI usability | Browser | 💰💰 | UI components |

---

## Testing Strategy (70/20/10 Rule)

```
Write tests in this proportion:

70% Unit Tests
└─ Fast, isolated, many tests
└─ Test business logic, edge cases
└─ Easy to maintain and debug

20% Integration Tests
└─ Medium speed, realistic
└─ Test component interaction
└─ Database, API integration

10% E2E Tests
└─ Slow, comprehensive
└─ Test critical user flows
└─ Highest confidence
```

---

## Best Practices Summary

### ✅ DO:
- **Write unit tests first** (fastest feedback)
- **Test behavior, not implementation** (test what users see)
- **Keep tests independent** (no shared state)
- **Use descriptive names** (`TestCreateStudent_InvalidEmail_ReturnsError`)
- **Follow AAA pattern** (Arrange, Act, Assert)
- **Mock external dependencies** (databases, APIs in unit tests)
- **Run tests in CI/CD** (automated quality gates)

### ❌ DON'T:
- **Test third-party libraries** (trust but verify)
- **Write flaky tests** (inconsistent pass/fail)
- **Skip test cleanup** (always clean up resources)
- **Over-mock** (use real dependencies in integration tests)
- **Test implementation details** (test public interfaces)
- **Ignore slow tests** (optimize or move to integration)
- **Commit commented-out tests** (fix or delete)

---

## Common Pitfalls

### 1. Testing Too Much
```typescript
// ❌ BAD - Testing React internals
expect(component.state.count).toBe(3);

// ✅ GOOD - Testing user-visible behavior
expect(screen.getByText("Count: 3")).toBeInTheDocument();
```

### 2. Shared State Between Tests
```go
// ❌ BAD - Shared database between tests
var db *bun.DB

func TestA(t *testing.T) {
    // Uses shared db - can affect TestB
}

// ✅ GOOD - Isolated database per test
func TestA(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
}
```

### 3. Not Testing Edge Cases
```go
// ❌ BAD - Only testing happy path
func TestDivide(t *testing.T) {
    result := Divide(10, 2)
    assert.Equal(t, 5, result)
}

// ✅ GOOD - Testing edge cases
func TestDivide(t *testing.T) {
    tests := []struct{
        name string
        a, b int
        want int
        wantErr bool
    }{
        {"normal", 10, 2, 5, false},
        {"divide by zero", 10, 0, 0, true},
        {"negative", -10, 2, -5, false},
    }
    // ... run tests
}
```

---

## Project Phoenix Test Commands

```bash
# Backend
cd backend
go test ./...                    # Unit + Integration
go test -race ./...              # Race detection
go test -bench=. ./...           # Benchmarks
go test -cover ./...             # Coverage

# Frontend
cd frontend
npm run test                     # Unit (watch mode)
npm run test:run                 # Unit (CI mode)
npm run test:run -- --coverage   # With coverage

# API
cd bruno
bru run --env Local 0*.bru       # All API tests

# E2E (when implemented)
cd frontend
npx playwright test              # All E2E tests
npx playwright test --headed     # With browser visible
npx playwright test --ui         # Interactive mode

# Quality Checks
npm run check                    # Lint + Typecheck
golangci-lint run                # Go linter
```

---

## Summary

Each test type serves a specific purpose:

1. **Unit Tests** = Fast feedback on individual functions
2. **Integration Tests** = Verify components work together
3. **E2E Tests** = Validate complete user workflows
4. **API Tests** = Ensure backend contracts
5. **Performance Tests** = Check speed and scalability
6. **Security Tests** = Find vulnerabilities
7. **Accessibility Tests** = Ensure usability for all

**Golden Rule**: Write the simplest test that gives you confidence. Start with unit tests, add integration tests for critical paths, and use E2E tests sparingly for the most important user journeys.
