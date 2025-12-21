# Testing State - Project Phoenix

**Last Updated**: 2025-12-21
**Version**: 1.0.0

## Executive Summary

Project Phoenix has a **solid testing foundation** but with **significant gaps** in frontend component testing and end-to-end coverage. The project demonstrates strong backend unit testing practices and comprehensive API testing via Bruno, but lacks E2E automation, performance testing, and frontend component coverage.

### Quick Stats

| Category | Status | Coverage |
|----------|--------|----------|
| **Backend Unit Tests** | ✅ Strong | 48 test files across domains |
| **Frontend Component Tests** | ⚠️ Critical Gap | Only 1 test file (useSSE hook) |
| **API Integration Tests** | ✅ Excellent | 115+ scenarios via Bruno (~340ms) |
| **E2E Tests** | ❌ Missing | No Playwright/Cypress |
| **Performance Tests** | ❌ Missing | No benchmarks or load tests |
| **Code Coverage** | ⚠️ Not Measured | No coverage reporting configured |
| **CI/CD Testing** | ✅ Good | GitHub Actions with lint/test stages |

---

## 1. Backend Testing (Go)

### Current State: ✅ Strong Foundation

#### Test Coverage by Domain

```
backend/
├── models/          18 test files   (Model validation & business rules)
├── services/         8 test files   (Business logic layer)
├── api/              3 test files   (HTTP handlers)
├── auth/authorize/   3 test files   (Authorization & permissions)
├── database/repos/   2 test files   (Data access layer)
├── backend/test/     1 integration  (Authorization integration)
└── Other domains     13 test files  (Realtime, cmd, utilities)
────────────────────────────────────────────────────────────────
TOTAL:               48 test files
```

#### Testing Framework & Tools

**Primary Tools**:
- **Go standard library** (`testing` package)
- **Testify** (`github.com/stretchr/testify v1.11.1`)
  - `require.*` - Hard assertions (fail immediately)
  - `assert.*` - Soft assertions (continue test)
- **SQL mocking** (`github.com/DATA-DOG/go-sqlmock`)
- **BUN ORM** (`github.com/uptrace/bun`) - Used in tests

**Test Patterns**:
- ✅ **Table-driven tests** - Common throughout
- ✅ **Mock repositories** - Service isolation
- ✅ **Test helpers** - Centralized in `test/helpers.go` (397 lines)
- ✅ **Integration tests** - Authorization flow testing
- ❌ **Benchmarks** - Not implemented
- ❌ **Coverage reporting** - Not configured

#### Test Helpers (`backend/test/helpers.go`)

**Key Utilities**:
```go
// Authentication & Authorization
CreateTestJWTAuth()                    // JWT token generation
CreateTestAuthorizationService()       // Auth service setup
MockJWTContext()                       // Add JWT claims to context

// Test Data Setup
CreateTestData()                       // Comprehensive test data
CreateTestStudent()                    // Individual entity creation
CreateTestGroup()
CreateTestPerson()
SetupTestDatabase()                    // Database configuration

// Permission Testing
TestPermissionScenario                 // Permission test structure
RunPermissionScenarios()               // Batch permission tests
```

#### Example Test Structure

**Model Validation** (`models/active/visit_test.go`):
```go
func TestVisit_Validate(t *testing.T) {
    tests := []struct {
        name    string
        visit   *active.Visit
        wantErr bool
    }{
        {"valid visit", &active.Visit{...}, false},
        {"missing student_id", &active.Visit{...}, true},
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.visit.Validate()
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

**Service Testing** (`services/auth/password_reset_rate_limit_test.go`):
```go
func TestRateLimitService_CheckRateLimit(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()

    // Mock SQL expectations
    mock.ExpectQuery("SELECT (.+) FROM rate_limits").
        WillReturnRows(sqlmock.NewRows(...))

    // Test service logic
    service := NewRateLimitService(repo)
    allowed, err := service.CheckRateLimit(ctx, email)

    require.NoError(t, err)
    assert.True(t, allowed)
}
```

#### Integration Tests

**Authorization Integration** (`backend/test/authorization_integration_test.go`):
- Full permission-based access control testing
- Multi-user scenario testing
- Database-backed permission validation
- Role hierarchy testing

#### Test Execution

```bash
# All tests
go test ./...                          # Standard run
go test -v ./...                       # Verbose output
go test -race ./...                    # Race detection

# Specific packages
go test ./api/auth                     # Auth API tests
go test ./services/active              # Active service tests

# Specific tests
go test -run TestLogin                 # Run tests matching name
go test -run TestLogin -v              # Verbose specific test

# With coverage (manual)
go test -cover ./...                   # Show coverage %
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

#### CI/CD Integration

**GitHub Actions** (`.github/workflows/test.yml`):
```yaml
- name: test
  run: cd backend && go test -v ./...
```

**Status**: ✅ Tests run in CI pipeline

---

## 2. Frontend Testing (Next.js/React)

### Current State: ⚠️ Critical Gap

#### Test Coverage

```
frontend/
├── src/lib/hooks/__tests__/
│   └── use-sse.test.ts        714 lines  (SSE hook only)
└── src/components/            0 tests    (No component tests)
    src/app/                   0 tests    (No page tests)
    src/lib/*-api.ts          0 tests    (No API client tests)
────────────────────────────────────────────────────────────────
TOTAL:                         1 test file
```

**Only Tested**: `useSSE` hook (Server-Sent Events)

#### Testing Framework & Tools

**Configured & Ready**:
- ✅ **Vitest** (`v4.0.16`) - Test runner
- ✅ **React Testing Library** (`v16.3.1`) - Component testing
- ✅ **@testing-library/jest-dom** (`v6.9.1`) - DOM matchers
- ✅ **Happy-dom** (`v20.0.11`) - Lightweight DOM environment
- ✅ **jsdom** (`v27.3.0`) - Alternative DOM environment
- ✅ **V8 coverage provider** - Built-in coverage

**Configuration** (`vitest.config.ts`):
```typescript
export default defineConfig({
  plugins: [react()],
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      exclude: [
        "node_modules/",
        "src/test/",
        "**/*.config.*",
        "**/types.ts"
      ]
    }
  }
})
```

**Test Setup** (`src/test/setup.ts`):
```typescript
import "@testing-library/jest-dom";
```

#### Example Test: useSSE Hook

**File**: `src/lib/hooks/__tests__/use-sse.test.ts` (714 lines)

**Coverage**:
- ✅ Connection lifecycle (idle → connecting → connected → failed)
- ✅ Reconnection logic with exponential backoff
- ✅ Error handling and recovery
- ✅ Event parsing (JSON and non-JSON)
- ✅ Cleanup on unmount
- ✅ Status transitions
- ✅ Mock EventSource implementation

**Example Structure**:
```typescript
describe("useSSE", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockEventSource.mockClear();
  });

  describe("Connection Lifecycle", () => {
    it("should initialize in idle state", () => {
      const { result } = renderHook(() => useSSE("/test-endpoint"));
      expect(result.current.status).toBe("idle");
    });

    it("should transition to connecting when connect is called", () => {
      const { result } = renderHook(() => useSSE("/test-endpoint"));
      act(() => result.current.connect());
      expect(result.current.status).toBe("connecting");
    });
  });

  describe("Reconnection Logic", () => {
    it("should implement exponential backoff", async () => {
      // Test implementation
    });
  });
});
```

#### Test Scripts

**Available Commands** (`package.json`):
```bash
npm run test              # Vitest watch mode
npm run test:ui          # Vitest UI dashboard
npm run test:run         # CI mode (single run)
```

**Quality Checks**:
```bash
npm run check            # Lint + typecheck (required before commit)
npm run lint             # ESLint (max-warnings: 0)
npm run typecheck        # TypeScript strict mode
```

#### CI/CD Integration

**GitHub Actions** (`.github/workflows/lint.yml`):
```yaml
frontend:
  - name: lint
    run: cd frontend && npm run lint
  - name: typecheck
    run: cd frontend && npm run typecheck
```

**Status**: ⚠️ Lint/typecheck run in CI, but NO tests executed

---

## 3. API Testing (Bruno)

### Current State: ✅ Excellent Coverage

#### Test Suite Overview

**Location**: `bruno/`
**Total Files**: 18 consolidated `.bru` files
**Total Scenarios**: 115+ test scenarios
**Execution Time**: ~340ms (full suite)

#### Test Files by Domain

| File | Scenarios | Domain | Purpose |
|------|-----------|--------|---------|
| `00-cleanup.bru` | 1 | Setup | End all active sessions |
| `01-smoke.bru` | 3 | Health | Admin auth, groups API, device ping |
| `02-auth.bru` | 4 | Auth | Login, refresh, teacher, device auth |
| `03-resources.bru` | 4 | Lists | Groups, students, rooms, activities |
| `04-devices.bru` | 4 | IoT | Device endpoints & filters |
| `05-sessions.bru` | 10 | Active | Session lifecycle & conflicts |
| `06-checkins.bru` | 8 | Active | Check-in/checkout flows |
| `06a-supervisor-rfid.bru` | 6 | Active | Supervisor RFID operations |
| `07-attendance.bru` | 6 | Active | RFID + web attendance tracking |
| `07a-feedback.bru` | 12 | Feedback | Submission & validation |
| `08-rooms.bru` | 5 | Facilities | Room conflict regression |
| `09-rfid.bru` | 5 | IoT | RFID assignment operations |
| `09a-staff-rfid.bru` | 6 | IoT | Staff RFID device operations |
| `10-schulhof.bru` | 5 | Active | Schulhof auto-create workflow |
| `11-claiming.bru` | 5 | Active | Group claiming workflow |
| `12-password-reset.bru` | 4 | Auth | Password reset flows |
| `13-invitations.bru` | 15 | Auth | Invitation creation & acceptance |
| `20-csv-import.bru` | 2 | Import | CSV import functionality |

#### Bruno Features & Patterns

**Architecture**:
- ✅ **Hermetic tests** - Self-contained setup and cleanup
- ✅ **Environment-driven** - `environments/Local.bru` configuration
- ✅ **Pre-request scripts** - Auto-authentication
- ✅ **Post-response scripts** - Additional assertions
- ✅ **Async testing** - Parallel requests within files
- ✅ **Time-dependent overrides** - `dailyCheckoutMode` variable

**Authentication Methods**:
```bruno
# JWT Bearer (Teacher/Admin)
Authorization: Bearer {{accessToken}}

# Two-layer Device Auth (RFID)
Authorization: Bearer {{deviceApiKey}}
X-Staff-PIN: {{devicePin}}
```

**Test Accounts**:
- Admin: `admin@example.com` / `Test1234%`
- Teacher: `andreas.arndt@schulzentrum.de` / `Test1234%` (Staff ID: 1, PIN: 1234)

#### Example Test Structure

**Session Lifecycle** (`05-sessions.bru`):
```bruno
# Test 1: Create active session
POST {{baseUrl}}/api/active/sessions
{
  "group_id": "{{groupId}}",
  "room_id": "{{roomId}}",
  "supervisor_ids": ["{{staffId}}"]
}

# Test 2: Verify room conflict detection
POST {{baseUrl}}/api/active/sessions
{
  "group_id": "{{anotherGroupId}}",
  "room_id": "{{roomId}}"  // Same room - should fail
}

# Test 3: Update session
PUT {{baseUrl}}/api/active/sessions/{{sessionId}}
{
  "room_id": "{{newRoomId}}"
}

# Test 4: End session
DELETE {{baseUrl}}/api/active/sessions/{{sessionId}}
```

#### Test Execution

**Run All Tests**:
```bash
cd bruno
bru run --env Local 0*.bru              # All numbered tests (~340ms)
```

**Run Specific Domains**:
```bash
bru run --env Local 05-sessions.bru    # Session tests only
bru run --env Local 06-checkins.bru    # Check-in tests only
bru run --env Local 0[1-5]-*.bru       # Tests 01-05 only
```

**GUI Testing**:
```bash
# Open Bruno desktop app
# File → Open Collection → Select bruno/ directory
```

#### Test Coverage by API Domain

| Domain | Endpoints Tested | Coverage |
|--------|------------------|----------|
| **Auth** | Login, Refresh, Logout, Device, Reset, Invitations | Excellent |
| **Active Sessions** | Create, Update, Delete, Conflict detection | Excellent |
| **Check-ins/Check-outs** | RFID, Web, Validation, Supervisor | Excellent |
| **Groups** | List, Filter, Claiming | Good |
| **Students** | List, Filter | Basic |
| **Rooms** | List, Conflict detection | Good |
| **Devices** | List, Ping, RFID operations | Good |
| **Feedback** | Submit, Validation | Excellent |
| **CSV Import** | Import operations | Basic |

#### CI/CD Integration

**Status**: ❌ Not integrated into GitHub Actions

**Recommendation**: Add Bruno tests to CI pipeline:
```yaml
- name: API Tests
  run: |
    cd bruno
    bru run --env Local 0*.bru
```

---

## 4. Integration & E2E Testing

### Current State: ❌ Missing

#### What's NOT Available

**E2E Frameworks**:
- ❌ **Playwright** - Not installed or configured
- ❌ **Cypress** - Not installed or configured
- ❌ **Selenium** - Not installed or configured

**Integration Testing**:
- ⚠️ **Backend integration** - Limited to authorization test
- ❌ **Frontend integration** - No full-stack flow tests
- ❌ **Database integration** - Tests use SQL mocks, not real DB

**Visual Regression**:
- ❌ **Percy** - Not configured
- ❌ **Chromatic** - Not configured
- ❌ **Snapshot tests** - Not implemented

#### What Exists (Partial)

**Backend Authorization Integration**:
- File: `backend/test/authorization_integration_test.go`
- Tests: Permission-based access control
- Database: Uses test database (real DB, not mocked)

**SQL Mocking**:
- Tool: `github.com/DATA-DOG/go-sqlmock`
- Usage: Service layer testing
- Limitation: Not full integration (isolated from real DB)

---

## 5. Performance & Load Testing

### Current State: ❌ Missing

#### What's NOT Available

**Performance Testing**:
- ❌ **Go benchmarks** - No `*_bench.go` files
- ❌ **Load testing** - No k6, Apache JMeter, Gatling
- ❌ **Stress testing** - No chaos engineering tools
- ❌ **Database profiling** - No query performance tracking

**Monitoring**:
- ❌ **Performance budgets** - Not defined
- ❌ **Performance regression detection** - Not configured
- ❌ **Real-time monitoring** - No application performance monitoring (APM)

#### Bruno Performance Metrics

**Current Baseline** (from test execution):
```
Full test suite: ~340ms (115+ scenarios)
- 00-cleanup.bru: ~10ms
- 01-smoke.bru: ~15ms
- 05-sessions.bru: ~50ms
- 06-checkins.bru: ~40ms
- ... (other tests)
```

**Note**: These are API response times, NOT load testing

---

## 6. Code Coverage

### Current State: ⚠️ Not Measured

#### Backend Coverage

**Status**: ❌ Not configured

**Manual Coverage**:
```bash
# Generate coverage report (manual)
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Issues**:
- No automated coverage collection
- No coverage reporting in CI/CD
- No minimum coverage enforcement
- No coverage trends tracking

#### Frontend Coverage

**Status**: ✅ Configured, ❌ Not utilized

**Configuration**:
```typescript
// vitest.config.ts
coverage: {
  provider: "v8",
  reporter: ["text", "json", "html"],
  exclude: ["node_modules/", "src/test/", "**/*.config.*"]
}
```

**Generate Coverage**:
```bash
npm run test:run -- --coverage
```

**Issue**: Only 1 test file exists, so coverage would be minimal

#### Coverage Reporting Tools

**NOT Configured**:
- ❌ **Codecov** - No integration
- ❌ **Coveralls** - No integration
- ❌ **SonarQube** - No integration

---

## 7. CI/CD Testing Pipeline

### Current State: ✅ Good Foundation

#### GitHub Actions Workflow

**Main Workflow** (`.github/workflows/main.yml`):
```
main.yml (orchestrator)
├── dependencies.yml (setup Go + Node)
├── lint.yml (parallel)
│   ├── Backend: golangci-lint (10m timeout)
│   └── Frontend: ESLint + TypeScript check (0 warnings)
└── test.yml (parallel)
    └── Backend: go test -v ./...
```

#### Lint Workflow (`.github/workflows/lint.yml`)

**Backend Linting**:
```yaml
backend:
  steps:
    - name: golangci-lint
      uses: golangci/golangci-lint-action@v6
      with:
        version: latest
        args: --timeout=10m
```

**Frontend Linting**:
```yaml
frontend:
  steps:
    - name: lint
      run: cd frontend && npm run lint
    - name: typecheck
      run: cd frontend && npm run typecheck
```

**Zero Warnings Policy**: `eslintConfig.maxWarnings = 0`

#### Test Workflow (`.github/workflows/test.yml`)

**Backend Testing**:
```yaml
backend:
  steps:
    - name: test
      run: cd backend && go test -v ./...
```

**Frontend Testing**: ❌ Not configured

#### What's Missing in CI/CD

- ❌ Frontend test execution
- ❌ API tests (Bruno)
- ❌ E2E tests
- ❌ Code coverage collection
- ❌ Coverage enforcement (minimum threshold)
- ❌ Performance regression tests
- ❌ Security testing (OWASP ZAP, etc.)

---

## 8. Test Environment Management

### Current State: ⚠️ Manual Setup

#### Environment Configuration

**Backend** (`backend/dev.env`):
```bash
# Database
DB_DSN=postgres://user:pass@localhost:5432/db?sslmode=require

# Authentication
AUTH_JWT_SECRET=test_secret_key
AUTH_JWT_EXPIRY=15m

# Admin Account
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=Test1234%
```

**Frontend** (`frontend/.env.local`):
```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXTAUTH_URL=http://localhost:3000
NEXTAUTH_SECRET=test_nextauth_secret
```

**Bruno** (`bruno/environments/Local.bru`):
```bruno
vars {
  baseUrl: http://localhost:8080
  adminEmail: admin@example.com
  adminPassword: Test1234%
  teacherEmail: andreas.arndt@schulzentrum.de
}
```

#### Test Data Management

**Seed Data**:
```bash
go run main.go seed             # Populate test data
go run main.go seed --reset     # Clear and re-seed
```

**Issues**:
- No test data versioning
- No isolated test databases per developer
- No automated test environment provisioning
- Manual cleanup required between test runs

---

## 9. Testing Documentation

### Current State: ⚠️ Limited

#### Existing Documentation

**Main Documentation** (`CLAUDE.md`):
- Testing strategy section (brief)
- Bruno API testing overview
- Backend testing examples
- Frontend testing configuration

**Bruno README** (`bruno/README.md`):
- Test execution instructions
- Test account credentials
- Environment setup

#### What's Missing

- ❌ Comprehensive testing guide for contributors
- ❌ Testing best practices document
- ❌ Test writing guidelines
- ❌ Coverage requirements and goals
- ❌ Test maintenance guide
- ❌ Troubleshooting common test issues

---

## 10. Summary: Strengths & Gaps

### ✅ Strengths

1. **Backend Unit Testing** - 48 test files with solid patterns
2. **API Testing** - 115+ Bruno scenarios with hermetic design
3. **Test Helpers** - Comprehensive utilities in `test/helpers.go`
4. **CI/CD Foundation** - GitHub Actions with lint/test stages
5. **Modern Frontend Setup** - Vitest + React Testing Library ready
6. **Zero Warnings Policy** - Strict quality enforcement

### ⚠️ Gaps Requiring Attention

1. **Frontend Component Testing** - Only 1 test file (critical gap)
2. **Code Coverage** - No metrics or enforcement
3. **E2E Testing** - No Playwright/Cypress automation
4. **Performance Testing** - No benchmarks or load tests
5. **Frontend CI Testing** - Tests not run in pipeline

### ❌ Missing Entirely

1. **Visual Regression Testing** - No Percy/Chromatic
2. **Accessibility Testing** - No aXe integration
3. **Security Testing** - No OWASP ZAP or similar
4. **Mobile Testing** - No mobile automation
5. **Chaos Engineering** - No resilience testing
6. **Test Documentation** - No comprehensive guide

---

## 11. Recommended Priorities

### Immediate (Next Sprint)

1. ✅ **Add frontend component tests** - Target 10-15 high-value components
2. ✅ **Enable coverage reporting** - Backend (go test -cover) + Frontend (Vitest)
3. ✅ **Add frontend tests to CI** - Update GitHub Actions workflow
4. ✅ **Document testing patterns** - Create TESTING.md guide

### Short-term (1-2 Months)

5. ✅ **Add E2E tests** - Playwright for critical user flows
6. ✅ **Integrate Bruno into CI** - Automated API testing in pipeline
7. ✅ **Set coverage thresholds** - Enforce minimum coverage (e.g., 70%)
8. ✅ **Add performance benchmarks** - Go benchmarks for critical paths

### Medium-term (3-6 Months)

9. ✅ **Visual regression testing** - Percy or Chromatic setup
10. ✅ **Load testing** - k6 for API stress testing
11. ✅ **Accessibility testing** - aXe integration
12. ✅ **Security testing** - OWASP ZAP or similar

---

## 12. Metrics & KPIs

### Current Metrics (Estimated)

| Metric | Backend | Frontend | Target |
|--------|---------|----------|--------|
| **Unit Test Coverage** | Unknown (~60%?) | <5% | 80% |
| **Integration Test Coverage** | ~10% | 0% | 40% |
| **E2E Test Coverage** | 0% (via Bruno) | 0% | 20% |
| **Test Execution Time** | <5s | <1s | <10s |
| **API Test Coverage** | ~80% (Bruno) | N/A | 90% |

### Proposed Targets (6 Months)

| Metric | Target | Measurement |
|--------|--------|-------------|
| **Backend Unit Coverage** | 80% | `go test -cover` |
| **Frontend Unit Coverage** | 70% | Vitest coverage |
| **E2E Coverage** | 30% critical paths | Playwright reports |
| **API Coverage** | 95% endpoints | Bruno test count |
| **Build Time** | <3 minutes | CI/CD duration |
| **Test Success Rate** | 99% | CI/CD pass rate |

---

## Conclusion

Project Phoenix demonstrates **strong backend testing fundamentals** with 48 test files and **excellent API coverage** via 115+ Bruno scenarios. However, the **frontend testing gap is critical** with only 1 test file covering a single hook.

The infrastructure is in place (Vitest, React Testing Library) to rapidly expand frontend coverage. Adding E2E tests with Playwright and enabling code coverage reporting should be top priorities.

**Overall Grade**: B- (Strong foundation, critical gaps in frontend and E2E)

---

**Next Steps**: See `TESTING-STRATEGY.md` for comprehensive testing approach and implementation roadmap.
