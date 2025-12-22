# Project Phoenix - Comprehensive Test Strategy for SaaS Launch

**Version**: 2.0
**Date**: 2025-12-22
**Target**: Production-ready SaaS with 100+ customers

---

## Executive Summary

Project Phoenix is preparing for SaaS launch serving hundreds of educational institutions. This document defines a comprehensive testing strategy covering **18 test categories** across unit, integration, E2E, performance, security, accessibility, and resilience testing.

### Current State Assessment

| Layer | Coverage | Status | Risk Level |
|-------|----------|--------|------------|
| Backend Unit Tests | 24% avg | 48 test files | HIGH |
| Frontend Tests | <1% | 1 test file | CRITICAL |
| API Tests (Bruno) | 21% | 46/220 endpoints | HIGH |
| E2E Tests | 0% | Not configured | CRITICAL |
| Load/Stress Tests | 0% | Not configured | CRITICAL |
| Security Tests | 0% | Not configured | CRITICAL |
| Accessibility (WCAG) | 0% | Not configured | CRITICAL |
| Chaos/Resilience | 0% | Not configured | CRITICAL |
| i18n Testing | 0% | Not configured | HIGH |
| Email Testing | 0% | Not configured | HIGH |

---

## Part 1: Complete Test Categories

### 1.1 Test Pyramid

```
                        ┌───────────────┐
                        │    Manual     │  ← Exploratory, Acceptance
                        │   + Chaos     │  ← Resilience Testing
                       ─┼───────────────┼─
                      ╱ │     E2E       │ ╲  ← User Journeys + Accessibility
                     ╱  │ + Visual Reg  │  ╲
                    ───┼───────────────┼───
                   ╱   │  Integration   │   ╲  ← API, DB, Email, SSE
                  ╱    │  + Contract    │    ╲
                 ─────┼───────────────┼─────
                ╱     │     Unit       │     ╲  ← Components, Services, i18n
               ╱      │   + Security   │      ╲
              ───────┴───────────────┴───────
```

### 1.2 All 18 Test Categories

| # | Category | Purpose | Tools | Priority |
|---|----------|---------|-------|----------|
| 1 | Unit Tests (Backend) | Test Go functions/services | go test | CRITICAL |
| 2 | Unit Tests (Frontend) | Test React components | Vitest + RTL | CRITICAL |
| 3 | Integration Tests | Test module interactions | Go test, Bruno | CRITICAL |
| 4 | API Tests | Test HTTP endpoints | Bruno | CRITICAL |
| 5 | E2E Tests | Test complete user workflows | Playwright | CRITICAL |
| 6 | Security Tests | Auth, injection, OWASP Top 10 | Custom, ZAP | CRITICAL |
| 7 | **Accessibility Tests** | WCAG 2.1 AA compliance | axe-core, Pa11y | CRITICAL |
| 8 | **Chaos/Resilience** | Failure injection, recovery | Gremlin, custom | CRITICAL |
| 9 | Load Tests | System capacity limits | k6 | HIGH |
| 10 | Stress Tests | Beyond capacity behavior | k6 | HIGH |
| 11 | **i18n Tests** | German locale, text expansion | pseudo-loc | HIGH |
| 12 | **Email Tests** | Delivery, templates, links | Mailosaur | HIGH |
| 13 | **Rate Limiting Tests** | Per-tenant API limits | Bruno, k6 | HIGH |
| 14 | **DR Tests** | Backup/restore validation | Custom scripts | HIGH |
| 15 | Contract Tests | API schema compatibility | OpenAPI | MEDIUM |
| 16 | **Cross-Browser Tests** | Multi-browser compatibility | Playwright | MEDIUM |
| 17 | **Visual Regression** | UI screenshot comparison | Playwright | MEDIUM |
| 18 | **Concurrency Tests** | Race conditions, deadlocks | Go test | MEDIUM |

---

## Part 2: Current Test Inventory

### 2.1 Backend (Go) - 48 Test Files

**Well-Tested (60%+)**:
- ✅ Authorization Policy Engine (100%)
- ✅ Real-time SSE Hub (89.6%)
- ✅ Permission Checking (82.8%)
- ✅ Model Validation (60-82%)

**Critical Gaps (0-10%)**:
- ❌ All 17 API Handlers (0%)
- ❌ Active Service - Attendance (3.5%)
- ❌ JWT Authentication (0%)
- ❌ Device/RFID Auth (0%)
- ❌ Repository Layer (3-11%)
- ❌ Email Service (0%)

### 2.2 Frontend (Next.js) - 1 Test File

- ✅ Vitest 4.0.16 configured
- ✅ React Testing Library installed
- ❌ MSW not installed
- ❌ Playwright not configured
- ❌ 0/141 components tested
- ❌ 0/32 pages tested

### 2.3 API Tests (Bruno) - 18 Files

**Covered**: Auth, Sessions, Check-ins, RFID, Feedback, Invitations
**Missing (79%)**: User CRUD, Group CRUD, Activity CRUD, Error scenarios

---

## Part 3: Detailed Test Specifications

### 3.1 Unit Tests

#### Backend (Go) - Target: 70%

```
Priority 1 (Week 1-2):
├── services/active/           3.5% → 60%   Check-in/out, sessions
├── auth/jwt/                  0%   → 80%   Token generation/validation
├── auth/device/               0%   → 70%   Device authentication
├── api/auth/                  0%   → 50%   Login, refresh handlers
└── api/active/                0%   → 50%   Active session handlers

Priority 2 (Week 3-4):
├── services/education/        22%  → 50%
├── services/users/            0%   → 40%
├── database/repositories/     3%   → 40%
├── api/groups/                0%   → 40%
└── api/students/              0%   → 40%
```

**Go Test Template**:
```go
func TestFeatureName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"happy path", "input", "expected", false},
        {"error case", "bad", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := MyFunction(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

#### Frontend (React) - Target: 60%

**Setup Required**:
```bash
npm install --save-dev msw @testing-library/user-event
```

**Priority Components**:
```
Week 2-3:
├── UI Base (Button, Input, Modal)     HIGH
├── Auth (LoginForm, InvitationForm)   HIGH
├── Student (StudentForm, StudentList) HIGH
└── Active (CheckInCard, RoomStatus)   HIGH

Week 4-5:
├── Group (GroupForm, GroupList)       MEDIUM
├── Activity (ActivityForm)            MEDIUM
└── Dashboard (StatsCard, Charts)      MEDIUM
```

**React Test Template**:
```typescript
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MyComponent } from './my-component';

describe('MyComponent', () => {
  it('renders correctly', () => {
    render(<MyComponent />);
    expect(screen.getByRole('button')).toBeInTheDocument();
  });

  it('handles click', async () => {
    const handleClick = vi.fn();
    const user = userEvent.setup();
    render(<MyComponent onClick={handleClick} />);
    await user.click(screen.getByRole('button'));
    expect(handleClick).toHaveBeenCalledOnce();
  });
});
```

---

### 3.2 E2E Tests (Playwright)

**Setup**:
```bash
cd frontend
npm install --save-dev @playwright/test
npx playwright install
```

**Configuration** (`playwright.config.ts`):
```typescript
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
  },
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:3000',
    reuseExistingServer: !process.env.CI,
  },
});
```

**Critical User Journeys** (8 total):

| # | Journey | Priority | Scenarios |
|---|---------|----------|-----------|
| 1 | Invitation → Signup → Login | CRITICAL | Accept invite, set password, login |
| 2 | Student Check-in Flow | CRITICAL | RFID scan → Visit → Room update → SSE |
| 3 | Password Reset | CRITICAL | Request → Email → Confirm → Login |
| 4 | CSV Import | HIGH | Upload → Preview → Validate → Import |
| 5 | Group Management | HIGH | Create → Add students → Start session |
| 6 | Activity Scheduling | MEDIUM | Create → Schedule → Enroll → Start |
| 7 | Supervisor Dashboard | MEDIUM | View rooms → See students → Filter |
| 8 | Admin User Management | MEDIUM | Create user → Assign role → Verify |

**E2E Test Template**:
```typescript
import { test, expect } from '@playwright/test';

test.describe('Student Check-in', () => {
  test('RFID check-in creates visit', async ({ page }) => {
    await page.goto('/supervisor/room/1');

    // Simulate RFID scan via API
    const response = await page.request.post('/api/iot/checkin', {
      data: { rfid_tag: 'TEST123', room_id: 1 },
      headers: { 'Authorization': 'Bearer device-key' }
    });
    expect(response.ok()).toBeTruthy();

    // Verify UI updates via SSE
    await expect(page.locator('[data-testid="student-card"]')).toBeVisible();
  });
});
```

---

### 3.3 Accessibility Tests (WCAG 2.1 AA) 🆕

**Legal Requirement**: BITV 2.0 for German educational institutions.

**Setup**:
```bash
npm install --save-dev @axe-core/playwright
```

**Automated Checks** (add to every E2E test):
```typescript
import AxeBuilder from '@axe-core/playwright';

test('dashboard is accessible', async ({ page }) => {
  await page.goto('/dashboard');

  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa'])
    .analyze();

  expect(results.violations).toEqual([]);
});
```

**Manual Audit Checklist**:
```
WCAG 2.1 Level AA Requirements:
□ 1.1.1 All images have alt text
□ 1.3.1 Content structure uses semantic HTML
□ 1.4.3 Color contrast ratio ≥ 4.5:1
□ 1.4.4 Text resizable to 200% without loss
□ 2.1.1 All functions keyboard accessible
□ 2.1.2 No keyboard traps
□ 2.4.3 Focus order is logical
□ 2.4.7 Focus indicator visible
□ 3.1.1 Page language declared (lang="de")
□ 3.3.1 Input errors identified
□ 3.3.2 Labels provided for inputs
□ 4.1.2 All controls have accessible names
```

**Tools**:
- Automated: axe-core, Pa11y CI
- Browser: WAVE extension, axe DevTools
- Screen Reader: VoiceOver (macOS), NVDA (Windows)

---

### 3.4 Internationalization Tests (i18n) 🆕

**Why**: App is German, text expansion causes layout issues.

**Setup**:
```bash
npm install --save-dev pseudo-localization
```

**Test Categories**:

| Category | Test | Tool |
|----------|------|------|
| Hardcoded strings | Pseudo-localization pass | pseudo-loc |
| Text expansion | German text +25% fits | Playwright |
| Date formats | DD.MM.YYYY displays correctly | Vitest |
| Number formats | 1.234,56 not 1,234.56 | Vitest |
| Timezone | DST transitions handled | Vitest |
| Unicode | äöüß, émojis render | Vitest |

**Pseudo-localization Test**:
```typescript
import { pseudoLocalize } from 'pseudo-localization';

test('no hardcoded strings in UI', async ({ page }) => {
  // Enable pseudo-localization mode
  await page.addInitScript(() => {
    window.__PSEUDO_LOCALE__ = true;
  });

  await page.goto('/dashboard');

  // All visible text should be pseudo-localized
  const body = await page.textContent('body');
  expect(body).not.toMatch(/\b(Save|Cancel|Delete|Edit)\b/);
});
```

**Date/Number Format Tests**:
```typescript
describe('German locale formatting', () => {
  it('formats dates as DD.MM.YYYY', () => {
    const date = new Date('2025-12-22');
    expect(formatDate(date, 'de-DE')).toBe('22.12.2025');
  });

  it('formats numbers with comma decimal', () => {
    expect(formatNumber(1234.56, 'de-DE')).toBe('1.234,56');
  });
});
```

---

### 3.5 Email Testing 🆕

**Why**: Password reset and invitations are first user touchpoints.

**Setup** (using Mailosaur):
```bash
npm install --save-dev mailosaur
```

**Test Scenarios**:

| Email Type | Tests |
|------------|-------|
| Password Reset | Delivered, link valid, expires correctly |
| Invitation | Delivered, link valid, pre-fills name |
| Notifications | Content correct, unsubscribe works |

**Email E2E Test**:
```typescript
import MailosaurClient from 'mailosaur';

const mailosaur = new MailosaurClient(process.env.MAILOSAUR_API_KEY);
const serverId = process.env.MAILOSAUR_SERVER_ID;

test('password reset email', async ({ page }) => {
  const emailAddress = `test.${Date.now()}@${serverId}.mailosaur.net`;

  // Trigger password reset
  await page.goto('/forgot-password');
  await page.fill('[name="email"]', emailAddress);
  await page.click('button[type="submit"]');

  // Verify email received
  const email = await mailosaur.messages.get(serverId, {
    sentTo: emailAddress,
    timeout: 10000
  });

  expect(email.subject).toContain('Passwort');
  expect(email.html.links[0].href).toContain('/reset-password?token=');

  // Click reset link
  await page.goto(email.html.links[0].href);
  await expect(page.locator('h1')).toContainText('Neues Passwort');
});
```

---

### 3.6 Chaos/Resilience Testing 🆕

**Why**: 99.9% availability requires testing failure scenarios.

**Failure Scenarios to Test**:

| Scenario | Expected Behavior | Test Method |
|----------|-------------------|-------------|
| Database unavailable 30s | Graceful error, auto-reconnect | Kill postgres |
| SSE hub crashes | Clients auto-reconnect | Kill SSE process |
| Email service 500s | Queue retry, user notified | Mock SMTP |
| Network latency 5000ms | Timeout handling, UI feedback | tc netem |
| Redis unavailable | Fallback to DB | Kill redis |
| Primary DB failover | Automatic switch to replica | Simulate failure |

**Chaos Test Script**:
```bash
#!/bin/bash
# chaos/test-db-outage.sh

echo "Starting chaos test: Database outage"

# Start monitoring
curl -s http://localhost:8080/health &

# Kill database for 30 seconds
docker stop postgres
sleep 30
docker start postgres

# Verify recovery
sleep 10
HEALTH=$(curl -s http://localhost:8080/health)
if [[ "$HEALTH" == *"healthy"* ]]; then
  echo "✅ PASS: System recovered from DB outage"
else
  echo "❌ FAIL: System did not recover"
  exit 1
fi
```

**Go Resilience Test**:
```go
func TestDatabaseReconnection(t *testing.T) {
    db := setupTestDB(t)
    service := NewActiveService(db)

    // Simulate connection loss
    db.Close()

    // Attempt operation
    _, err := service.GetActiveGroups(ctx)
    require.Error(t, err)

    // Reconnect
    db = setupTestDB(t)
    service = NewActiveService(db)

    // Should work now
    groups, err := service.GetActiveGroups(ctx)
    require.NoError(t, err)
    assert.NotNil(t, groups)
}
```

---

### 3.7 Load & Stress Testing (k6)

**Installation**:
```bash
brew install k6  # macOS
```

**Baseline Test** (`k6/baseline.js`):
```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 10,
  duration: '1m',
  thresholds: {
    http_req_duration: ['p(95)<500'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const res = http.get('http://localhost:8080/api/groups');
  check(res, { 'status is 200': (r) => r.status === 200 });
  sleep(1);
}
```

**Load Test** (`k6/load.js`):
```javascript
export const options = {
  stages: [
    { duration: '2m', target: 50 },
    { duration: '5m', target: 50 },
    { duration: '2m', target: 100 },
    { duration: '5m', target: 100 },
    { duration: '2m', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    http_req_failed: ['rate<0.05'],
  },
};
```

**Stress Test** (`k6/stress.js`):
```javascript
export const options = {
  stages: [
    { duration: '2m', target: 100 },
    { duration: '5m', target: 200 },
    { duration: '5m', target: 300 },
    { duration: '5m', target: 400 },
    { duration: '2m', target: 0 },
  ],
};
```

**Multi-Tenant Simulation** (`k6/multi-tenant.js`):
```javascript
const TENANTS = ['school-a', 'school-b', 'school-c'];

export default function () {
  const tenant = TENANTS[Math.floor(Math.random() * TENANTS.length)];
  const headers = { 'X-Tenant-ID': tenant };

  http.get('http://localhost:8080/api/students', { headers });
  http.get('http://localhost:8080/api/groups', { headers });
}
```

**Performance Targets**:

| Metric | Target | Critical |
|--------|--------|----------|
| Response Time (p50) | <100ms | <200ms |
| Response Time (p95) | <300ms | <500ms |
| Response Time (p99) | <500ms | <1000ms |
| Error Rate | <0.1% | <1% |
| Concurrent Users | 500 | 200 |
| SSE Connections | 1000 | 500 |

---

### 3.8 Security Testing

**Categories**:

| Category | Tools | Tests |
|----------|-------|-------|
| Authentication | Custom Bruno | Brute force, session fixation |
| Authorization | Custom Go | Role bypass, IDOR |
| Injection | OWASP ZAP | SQL injection, XSS |
| Data Isolation | Custom | Cross-tenant leakage |
| Rate Limiting | Bruno, k6 | API abuse prevention |

**Cross-Tenant Isolation Test**:
```go
func TestCrossTenantIsolation(t *testing.T) {
    tenantAToken := loginAsTenant(t, "school-a")
    tenantBStudentID := createStudent(t, "school-b")

    req := httptest.NewRequest("GET",
        fmt.Sprintf("/api/students/%d", tenantBStudentID), nil)
    req.Header.Set("Authorization", "Bearer "+tenantAToken)

    resp := executeRequest(req)

    // Tenant A should NOT see Tenant B's student
    assert.Equal(t, 403, resp.Code)
}
```

**JWT Security Test**:
```go
func TestExpiredTokenRejected(t *testing.T) {
    expiredToken := createToken(time.Now().Add(-1 * time.Hour))
    req := httptest.NewRequest("GET", "/api/students", nil)
    req.Header.Set("Authorization", "Bearer "+expiredToken)

    resp := executeRequest(req)
    assert.Equal(t, 401, resp.Code)
}
```

**Rate Limiting Test** (Bruno):
```javascript
// 11-rate-limiting.bru
for (let i = 0; i < 100; i++) {
  const res = await bru.sendRequest({
    method: 'POST',
    url: bru.getEnvVar("baseUrl") + "/auth/login",
    data: { email: 'test@test.com', password: 'wrong' }
  });

  if (i > 10) {
    test(`Request ${i} should be rate limited`, function() {
      expect(res.status).to.equal(429);
    });
  }
}
```

---

### 3.9 Disaster Recovery Testing 🆕

**RTO/RPO Targets**:
- RTO (Recovery Time): 1-2 hours
- RPO (Recovery Point): 15 minutes

**Quarterly DR Drill Checklist**:
```
Pre-Drill:
□ Schedule maintenance window
□ Notify stakeholders
□ Prepare rollback plan

Drill Steps:
□ 1. Create backup snapshot
□ 2. Simulate primary DB failure
□ 3. Initiate failover to replica
□ 4. Measure actual recovery time
□ 5. Verify data integrity
□ 6. Test per-tenant data restoration
□ 7. Validate application functionality

Post-Drill:
□ Document actual RTO achieved
□ Identify gaps
□ Update runbooks
```

**Backup Validation Script**:
```bash
#!/bin/bash
# dr/validate-backup.sh

BACKUP_FILE=$1
TEST_DB="phoenix_dr_test"

echo "Validating backup: $BACKUP_FILE"

# Restore to test database
pg_restore -d $TEST_DB $BACKUP_FILE

# Run integrity checks
psql -d $TEST_DB -c "SELECT COUNT(*) FROM users.students;"
psql -d $TEST_DB -c "SELECT COUNT(*) FROM active.visits;"

# Verify relationships
psql -d $TEST_DB -c "
  SELECT COUNT(*) FROM active.visits v
  LEFT JOIN users.students s ON v.student_id = s.id
  WHERE s.id IS NULL;
" | grep -q "0" && echo "✅ Referential integrity OK"

# Cleanup
dropdb $TEST_DB
```

---

### 3.10 Rate Limiting Testing 🆕

**Test Scenarios**:

| Endpoint | Limit | Test |
|----------|-------|------|
| POST /auth/login | 10/min | Brute force prevention |
| POST /api/* | 100/min | General API abuse |
| GET /api/sse/events | 5 connections/user | SSE flood prevention |

**Bruno Rate Limit Test** (`11-rate-limiting.bru`):
```
meta {
  name: Rate Limiting Tests
  type: http
  seq: 11
}

post {
  url: {{baseUrl}}/auth/login
  body: json
}

body:json {
  {
    "email": "ratelimit@test.com",
    "password": "wrongpassword"
  }
}

script:post-response {
  // Run 15 requests rapidly
  const results = [];
  for (let i = 0; i < 15; i++) {
    const res = await bru.sendRequest({
      method: 'POST',
      url: bru.getEnvVar("baseUrl") + "/auth/login",
      data: { email: "ratelimit@test.com", password: "wrong" }
    });
    results.push(res.status);
  }

  test("Rate limiting kicks in after 10 requests", function() {
    const rateLimited = results.filter(s => s === 429);
    expect(rateLimited.length).to.be.greaterThan(0);
  });
}
```

---

### 3.11 Concurrency Testing 🆕

**Race Condition Scenarios**:

| Scenario | Risk | Test |
|----------|------|------|
| Concurrent RFID check-ins | Duplicate visits | Parallel requests |
| Simultaneous session start | Duplicate sessions | Parallel requests |
| Concurrent group updates | Lost updates | Parallel requests |

**Go Concurrency Test**:
```go
func TestConcurrentCheckIn(t *testing.T) {
    var wg sync.WaitGroup
    var successCount atomic.Int32
    var errorCount atomic.Int32

    studentID := 1
    roomID := 1

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := service.CheckIn(ctx, studentID, roomID)
            if err != nil {
                errorCount.Add(1)
            } else {
                successCount.Add(1)
            }
        }()
    }

    wg.Wait()

    // Exactly ONE check-in should succeed
    assert.Equal(t, int32(1), successCount.Load())
    assert.Equal(t, int32(99), errorCount.Load())

    // Verify only one visit exists
    visits, _ := repo.GetActiveVisits(ctx, studentID)
    assert.Len(t, visits, 1)
}
```

---

## Part 4: CI/CD Pipeline

```yaml
# .github/workflows/test.yml
name: Test Suite

on: [push, pull_request]

jobs:
  backend-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:17
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - name: Run Go Tests
        run: |
          cd backend
          go test ./... -race -coverprofile=coverage.out
          go tool cover -func=coverage.out
      - uses: codecov/codecov-action@v4

  frontend-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
      - run: cd frontend && npm ci
      - name: Run Vitest
        run: cd frontend && npm run test:run -- --coverage
      - uses: codecov/codecov-action@v4

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
      - run: npx playwright install --with-deps
      - name: Run E2E + Accessibility
        run: cd frontend && npx playwright test

  api-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Start Backend
        run: |
          cd backend && go run main.go serve &
          sleep 10
      - name: Run Bruno Tests
        run: |
          npm install -g @usebruno/cli
          cd bruno && bru run --env Local

  load-tests:
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      - uses: grafana/k6-action@v0.3.1
        with:
          filename: k6/baseline.js

  security-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run gosec
        run: |
          go install github.com/securego/gosec/v2/cmd/gosec@latest
          cd backend && gosec ./...
```

---

## Part 5: Implementation Roadmap (12 Weeks)

### Phase 1: Foundation (Weeks 1-2)

| Task | Effort | Deliverable |
|------|--------|-------------|
| Install MSW for frontend | 4h | Mock server ready |
| Create test utilities | 8h | Render helpers, factories |
| Fix skipped Go tests | 8h | All tests passing |
| Add JWT auth tests | 16h | 80% auth coverage |
| Add Device auth tests | 8h | 70% device coverage |
| **Accessibility audit** | 16h | WCAG gap report |
| **i18n baseline test** | 8h | Hardcoded strings found |

**Week 2 Checkpoint**: Auth tests complete, accessibility gaps documented.

### Phase 2: Core Coverage (Weeks 3-4)

| Task | Effort | Deliverable |
|------|--------|-------------|
| Test Active Service | 20h | 60% coverage |
| Test API Handlers | 20h | 50% coverage |
| Test 20 React components | 40h | Core UI tested |
| Bruno error scenarios | 8h | 40% API coverage |
| **Email testing setup** | 8h | Mailosaur integrated |
| **Fix accessibility issues** | 16h | Critical fixes done |

**Week 4 Checkpoint**: Core functionality tested, emails verified.

### Phase 3: E2E & Integration (Weeks 5-6)

| Task | Effort | Deliverable |
|------|--------|-------------|
| Setup Playwright | 8h | E2E infrastructure |
| 5 critical E2E journeys | 40h | User flows tested |
| Repository layer tests | 16h | 40% DB coverage |
| Remaining Bruno CRUD | 16h | 60% API coverage |
| **Cross-browser tests** | 8h | Chrome, Safari, Firefox |
| **SSE stability tests** | 16h | 24h connection test |

**Week 6 Checkpoint**: E2E covering critical paths, cross-browser OK.

### Phase 4: Performance & Security (Weeks 7-8)

| Task | Effort | Deliverable |
|------|--------|-------------|
| Setup k6 | 8h | Load test infrastructure |
| Baseline/load/stress scripts | 16h | Performance baselines |
| Security tests | 20h | OWASP Top 10 covered |
| Multi-tenant isolation | 16h | Data isolation verified |
| **Rate limiting tests** | 8h | API abuse prevented |
| **Concurrency tests** | 16h | Race conditions fixed |

**Week 8 Checkpoint**: 500 concurrent users OK, security verified.

### Phase 5: Production Readiness (Weeks 9-10)

| Task | Effort | Deliverable |
|------|--------|-------------|
| CI/CD pipeline complete | 16h | Automated testing |
| Coverage monitoring | 4h | Codecov dashboards |
| Performance regression | 8h | Alerts configured |
| Documentation | 8h | Runbooks complete |
| **Feature flag setup** | 16h | Gradual rollout ready |
| **Config isolation tests** | 8h | Per-tenant config OK |

**Week 10 Checkpoint**: CI/CD running, monitoring active.

### Phase 6: Resilience & DR (Weeks 11-12) 🆕

| Task | Effort | Deliverable |
|------|--------|-------------|
| Chaos test infrastructure | 16h | Failure injection ready |
| 5 chaos scenarios | 24h | Resilience verified |
| DR drill execution | 16h | RTO/RPO validated |
| Backup validation scripts | 8h | Restore tested |
| Observability testing | 8h | Alerts validated |
| Final review | 8h | Launch readiness |

**Week 12 Checkpoint**: System survives failures, DR validated.

---

## Part 6: Coverage Targets

### Final Targets (Week 12)

| Layer | Current | Target |
|-------|---------|--------|
| Backend Unit Tests | 24% | 70% |
| Frontend Unit Tests | <1% | 60% |
| API Integration | 21% | 80% |
| E2E Tests | 0 | 8 journeys |
| Load Tests | 0 | 500 users |
| Security Tests | 0 | OWASP Top 10 |
| Accessibility | 0 | WCAG 2.1 AA |
| i18n Tests | 0 | All UI strings |
| Chaos Tests | 0 | 5 scenarios |
| DR Validation | 0 | Quarterly drills |

### Coverage Enforcement

```yaml
# codecov.yml
coverage:
  status:
    project:
      default:
        target: 60%
        threshold: 5%
    patch:
      default:
        target: 80%  # New code must have 80%
```

---

## Part 7: Tools Summary

| Category | Tool | Status |
|----------|------|--------|
| Go Unit Tests | go test | ✅ Ready |
| Frontend Unit | Vitest + RTL | ✅ Ready |
| API Mocking | MSW | ❌ Install |
| API Tests | Bruno | ✅ Ready |
| E2E Tests | Playwright | ❌ Install |
| Accessibility | axe-core | ❌ Install |
| Load Tests | k6 | ❌ Install |
| Email Tests | Mailosaur | ❌ Install |
| Security Scan | gosec, ZAP | ❌ Install |
| Coverage | Codecov | ❌ Configure |
| Chaos | Custom scripts | ❌ Create |

---

## Quick Reference Commands

```bash
# Backend Tests
cd backend
go test ./...                          # All tests
go test ./... -cover                   # With coverage
go test -race ./...                    # Race detection

# Frontend Tests
cd frontend
npm run test                           # Watch mode
npm run test:run -- --coverage         # Single run

# E2E Tests (after setup)
cd frontend
npx playwright test                    # All tests
npx playwright test --ui               # UI mode

# Bruno API Tests
cd bruno
bru run --env Local                    # All tests

# Load Tests (after setup)
k6 run k6/baseline.js                  # Baseline
k6 run k6/load.js                      # Load test
k6 run k6/stress.js                    # Stress test

# Accessibility
npx pa11y http://localhost:3000        # Single page
```

---

## Document History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-12-22 | Initial strategy |
| 2.0 | 2025-12-22 | Integrated review findings: +8 test categories, extended to 12 weeks |
