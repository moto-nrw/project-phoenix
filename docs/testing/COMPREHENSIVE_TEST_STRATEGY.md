# Project Phoenix - Comprehensive Test Strategy for SaaS Launch

**Version**: 1.0
**Date**: 2025-12-22
**Target**: Production-ready SaaS with 100+ customers

---

## Executive Summary

Project Phoenix is preparing for SaaS launch serving hundreds of educational institutions. This document defines a comprehensive testing strategy to ensure reliability, security, and scalability.

### Current State Assessment

| Layer | Coverage | Status | Risk Level |
|-------|----------|--------|------------|
| **Backend Unit Tests** | 24% avg | 48 test files | HIGH |
| **Frontend Tests** | <1% | 1 test file | CRITICAL |
| **API Tests (Bruno)** | 21% | 46/220 endpoints | HIGH |
| **E2E Tests** | 0% | Not configured | CRITICAL |
| **Load/Stress Tests** | 0% | Not configured | CRITICAL |
| **Security Tests** | 0% | Not configured | CRITICAL |

### Risk Summary

- **17 of 19 API handler packages** have 0% test coverage
- **141 frontend components** have no tests
- **No E2E tests** for critical user flows
- **No load testing** infrastructure for multi-tenant scalability
- **No security testing** for authentication/authorization

---

## Part 1: Test Types Overview

### 1.1 Test Pyramid for SaaS

```
                    ┌─────────────┐
                    │   Manual    │  ← Exploratory, Acceptance
                    │   Testing   │
                   ─┼─────────────┼─
                  ╱ │    E2E      │ ╲  ← Critical User Journeys
                 ╱  │   Tests     │  ╲
                ───┼─────────────┼───
               ╱   │ Integration │   ╲  ← API, Database, SSE
              ╱    │   Tests     │    ╲
             ─────┼─────────────┼─────
            ╱     │    Unit     │     ╲  ← Components, Services, Models
           ╱      │   Tests     │      ╲
          ───────┴─────────────┴───────
```

### 1.2 Test Categories for Project Phoenix

| Category | Purpose | Tools | Current Status |
|----------|---------|-------|----------------|
| **Unit Tests** | Test individual functions/components in isolation | Go test, Vitest | Partial |
| **Integration Tests** | Test module interactions (API + DB) | Go test, Bruno | Minimal |
| **Component Tests** | Test React components with mocked dependencies | Vitest + RTL | Missing |
| **API Tests** | Test HTTP endpoints with real/mock backends | Bruno | 21% |
| **E2E Tests** | Test complete user workflows in browser | Playwright | Missing |
| **Performance Tests** | Test response times under load | k6 | Missing |
| **Load Tests** | Test system capacity limits | k6, Locust | Missing |
| **Stress Tests** | Test system behavior beyond capacity | k6 | Missing |
| **Security Tests** | Test auth, injection, data isolation | OWASP ZAP, custom | Missing |
| **Contract Tests** | Test API schema compatibility | OpenAPI validation | Missing |

---

## Part 2: Current Test Inventory

### 2.1 Backend (Go) - 48 Test Files

#### Well-Tested Areas (60%+ Coverage)
- ✅ Authorization Policy Engine (100%)
- ✅ Real-time SSE Hub (89.6%)
- ✅ Permission Checking (82.8%)
- ✅ Model Validation (60-82%)

#### Critical Gaps (0-10% Coverage)
- ❌ All 17 API Handlers (0%)
- ❌ Active Service - Core Attendance (3.5%)
- ❌ JWT Authentication (0%)
- ❌ Device/RFID Auth (0%)
- ❌ Repository Layer (3-11%)
- ❌ Email Service (0%)

### 2.2 Frontend (Next.js/React) - 1 Test File

#### Infrastructure Status
- ✅ Vitest 4.0.16 configured
- ✅ React Testing Library installed
- ❌ MSW (Mock Service Worker) not installed
- ❌ Playwright/Cypress not configured

#### Coverage
- 1 test file: `use-sse.test.ts` (20 test cases)
- 0/141 components tested
- 0/32 pages tested

### 2.3 API Tests (Bruno) - 18 Test Files

#### Covered Domains
- Auth, Sessions, Check-ins, RFID, Feedback, Invitations, Password Reset

#### Missing Coverage (79% of endpoints)
- User CRUD, Group CRUD, Activity CRUD
- Substitutions, Schedules, Config
- Device management, Analytics
- Error scenarios (401, 403, 500)

---

## Part 3: Test Strategy by Category

### 3.1 Unit Tests

#### Backend (Go)

**Target Coverage**: 70%+ for critical packages

**Priority 1 - Immediate (Week 1-2)**
```
Package                          Current   Target   Action
─────────────────────────────────────────────────────────
services/active/                 3.5%      60%      Test check-in/out, sessions
auth/jwt/                        0%        80%      Test token generation/validation
auth/device/                     0%        70%      Test device authentication
api/auth/                        0%        50%      Test login, refresh, logout handlers
api/active/                      0%        50%      Test active session handlers
```

**Priority 2 - High (Week 3-4)**
```
Package                          Current   Target
─────────────────────────────────────────────────
services/education/              22%       50%
services/users/                  0%        40%
database/repositories/           3-11%     40%
api/groups/                      0%        40%
api/students/                    0%        40%
```

#### Frontend (React)

**Target Coverage**: 60%+ for critical components

**Phase 1: Setup (Week 1)**
```bash
npm install --save-dev msw @testing-library/user-event
```

Create test utilities:
- `src/test/test-utils.tsx` - Custom render with providers
- `src/test/mocks/handlers.ts` - MSW API handlers
- `src/test/mocks/server.ts` - MSW server setup

**Phase 2: Components (Week 2-4)**
```
Component                        Priority   Complexity
─────────────────────────────────────────────────────
UI Base (Button, Input, Modal)   HIGH       Low
Form Components (student, group) HIGH       High
List Components                  MEDIUM     Medium
Modal Components                 MEDIUM     High
Page Components                  MEDIUM     Medium
```

### 3.2 Integration Tests

#### API Integration (Bruno)

**Current**: 46/220 endpoints (21%)
**Target**: 150/220 endpoints (68%)

**Priority Actions**:

1. **Auth Comprehensive Suite** (New file: `14-auth-comprehensive.bru`)
   - 401 Unauthorized scenarios
   - 403 Forbidden scenarios
   - Token expiry handling
   - Account deactivation

2. **CRUD Operations Suite** (New file: `15-crud-operations.bru`)
   - Create/Read/Update/Delete for all entities
   - Validation error responses
   - Cascade deletion verification

3. **Error Scenarios Suite** (New file: `16-error-scenarios.bru`)
   - Invalid input data
   - Missing required fields
   - Type mismatches
   - Resource not found

#### Database Integration

**Missing Tests**:
- Repository SQL query validation
- Transaction handling
- Relationship loading (Teacher→Staff→Person)
- Complex queries (pagination, filtering)

### 3.3 E2E Tests (NEW)

**Framework**: Playwright (recommended for Next.js 15)

**Setup**:
```bash
npm install --save-dev @playwright/test
npx playwright install
```

**Configuration**: `playwright.config.ts`
```typescript
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
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

**Critical User Journeys to Test**:

| Journey | Priority | Scenarios |
|---------|----------|-----------|
| **Invitation → Signup → Login** | CRITICAL | Accept invite, set password, login, access dashboard |
| **Student Check-in Flow** | CRITICAL | RFID scan → Visit creation → Room update → SSE notification |
| **CSV Import** | HIGH | Upload → Preview → Validate → Import → Verify |
| **Password Reset** | HIGH | Request → Email → Confirm → Login |
| **Group Management** | MEDIUM | Create → Add students → Assign supervisor → Start session |
| **Activity Scheduling** | MEDIUM | Create → Set schedule → Enroll students → Start/end |

### 3.4 Performance & Load Tests (NEW)

**Framework**: k6 (recommended for modern SaaS)

**Installation**:
```bash
brew install k6  # macOS
# or
docker pull grafana/k6
```

**Test Scenarios**:

#### Baseline Performance Test
```javascript
// k6/baseline.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 10,
  duration: '1m',
  thresholds: {
    http_req_duration: ['p(95)<500'],  // 95% requests < 500ms
    http_req_failed: ['rate<0.01'],    // Error rate < 1%
  },
};

export default function () {
  const res = http.get('http://localhost:8080/api/groups');
  check(res, { 'status is 200': (r) => r.status === 200 });
  sleep(1);
}
```

#### Load Test (Concurrent Users)
```javascript
// k6/load.js
export const options = {
  stages: [
    { duration: '2m', target: 50 },   // Ramp up to 50 users
    { duration: '5m', target: 50 },   // Stay at 50 users
    { duration: '2m', target: 100 },  // Ramp up to 100 users
    { duration: '5m', target: 100 },  // Stay at 100 users
    { duration: '2m', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    http_req_failed: ['rate<0.05'],
  },
};
```

#### Stress Test (Beyond Capacity)
```javascript
// k6/stress.js
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

#### Multi-Tenant Simulation
```javascript
// k6/multi-tenant.js
const TENANTS = ['school-a', 'school-b', 'school-c'];

export default function () {
  const tenant = TENANTS[Math.floor(Math.random() * TENANTS.length)];
  const headers = { 'X-Tenant-ID': tenant };

  http.get('http://localhost:8080/api/students', { headers });
  http.get('http://localhost:8080/api/groups', { headers });
  http.post('http://localhost:8080/api/iot/checkin',
    JSON.stringify({ rfid: 'TEST123', room_id: 1 }),
    { headers: { ...headers, 'Content-Type': 'application/json' } }
  );
}
```

**SaaS Performance Targets**:

| Metric | Target | Critical |
|--------|--------|----------|
| Response Time (p50) | <100ms | <200ms |
| Response Time (p95) | <300ms | <500ms |
| Response Time (p99) | <500ms | <1000ms |
| Error Rate | <0.1% | <1% |
| Throughput | 1000 req/s | 500 req/s |
| Concurrent Users | 500 | 200 |
| SSE Connections | 1000 | 500 |

### 3.5 Security Tests (NEW)

**Framework**: OWASP ZAP + Custom Scripts

**Security Test Categories**:

| Category | Tools | Tests |
|----------|-------|-------|
| **Authentication** | Custom Bruno tests | Brute force, session fixation, token tampering |
| **Authorization** | Custom Go tests | Role bypass, privilege escalation, IDOR |
| **Injection** | OWASP ZAP, SQLMap | SQL injection, XSS, command injection |
| **Data Isolation** | Custom tests | Cross-tenant data leakage |
| **GDPR Compliance** | Custom tests | Data deletion, consent enforcement, audit logs |

**Critical Security Tests**:

1. **Cross-Tenant Data Isolation**
```javascript
// Verify tenant A cannot access tenant B's data
test('tenant isolation - students', async () => {
  const tenantAToken = await loginAsTenantA();
  const tenantBStudentId = 'tenant-b-student-1';

  const response = await fetch(`/api/students/${tenantBStudentId}`, {
    headers: { Authorization: `Bearer ${tenantAToken}` }
  });

  expect(response.status).toBe(403); // or 404
});
```

2. **JWT Token Security**
```go
// Test expired token rejection
func TestExpiredTokenRejected(t *testing.T) {
    expiredToken := createToken(time.Now().Add(-1 * time.Hour))
    req := httptest.NewRequest("GET", "/api/students", nil)
    req.Header.Set("Authorization", "Bearer "+expiredToken)

    resp := executeRequest(req)
    assert.Equal(t, 401, resp.Code)
}
```

3. **Rate Limiting**
```javascript
// Bruno test for rate limiting
for (let i = 0; i < 100; i++) {
  const response = await fetch('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email: 'test@test.com', password: 'wrong' })
  });

  if (i > 10) {
    expect(response.status).toBe(429); // Too Many Requests
  }
}
```

### 3.6 Contract Tests

**Purpose**: Ensure API compatibility between frontend and backend

**Tool**: OpenAPI Schema Validation

**Implementation**:
1. Generate OpenAPI spec: `go run main.go gendoc --openapi`
2. Validate frontend API calls against spec
3. Fail CI if schema violations detected

---

## Part 4: Test Infrastructure

### 4.1 CI/CD Pipeline

```yaml
# .github/workflows/test.yml
name: Test Suite

on: [push, pull_request]

jobs:
  backend-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - name: Run Go Tests
        run: |
          cd backend
          go test ./... -coverprofile=coverage.out
          go tool cover -func=coverage.out
      - name: Upload Coverage
        uses: codecov/codecov-action@v4

  frontend-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
      - name: Install Dependencies
        run: cd frontend && npm ci
      - name: Run Vitest
        run: cd frontend && npm run test:run -- --coverage
      - name: Upload Coverage
        uses: codecov/codecov-action@v4

  api-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:17
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - name: Start Backend
        run: |
          cd backend
          go run main.go serve &
          sleep 10
      - name: Run Bruno Tests
        run: |
          npm install -g @usebruno/cli
          cd bruno
          bru run --env Local 0*.bru

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
      - name: Install Playwright
        run: npx playwright install --with-deps
      - name: Run E2E Tests
        run: cd frontend && npx playwright test

  load-tests:
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      - name: Run k6 Load Test
        uses: grafana/k6-action@v0.3.1
        with:
          filename: k6/baseline.js
```

### 4.2 Test Database Strategy

**Development**: SQLite in-memory (fast, isolated)
**Integration**: PostgreSQL container (realistic)
**Production**: Separate test schema with cleanup

```go
// test/helpers.go
func SetupTestDB(t *testing.T) *bun.DB {
    db := setupInMemoryDB()
    t.Cleanup(func() { db.Close() })
    return db
}
```

### 4.3 Test Data Management

**Seed Data**: `go run main.go seed`
**Test Fixtures**: Factory functions for each entity
**Cleanup**: Automatic teardown after each test

---

## Part 5: Implementation Roadmap

### Phase 1: Foundation (Weeks 1-2)

| Task | Owner | Effort | Priority |
|------|-------|--------|----------|
| Install MSW for frontend | Dev | 4h | CRITICAL |
| Create frontend test utilities | Dev | 8h | CRITICAL |
| Fix 6 skipped Go integration tests | Dev | 8h | HIGH |
| Add JWT authentication tests | Dev | 16h | CRITICAL |
| Add Device auth tests | Dev | 8h | HIGH |

**Deliverables**:
- Frontend test infrastructure complete
- JWT/Device auth 70%+ coverage
- All Go tests passing (no skips)

### Phase 2: Core Coverage (Weeks 3-4)

| Task | Owner | Effort | Priority |
|------|-------|--------|----------|
| Test Active Service (check-in/out) | Dev | 20h | CRITICAL |
| Test API Handlers (auth, active) | Dev | 20h | CRITICAL |
| Test 20 critical React components | Dev | 40h | HIGH |
| Add Bruno error scenario tests | Dev | 8h | HIGH |

**Deliverables**:
- Active service 60%+ coverage
- API handlers 40%+ coverage
- 20 component tests
- Bruno coverage 40%+

### Phase 3: E2E & Integration (Weeks 5-6)

| Task | Owner | Effort | Priority |
|------|-------|--------|----------|
| Setup Playwright | Dev | 8h | HIGH |
| Implement 5 critical E2E journeys | Dev | 40h | HIGH |
| Add Repository layer tests | Dev | 16h | MEDIUM |
| Add remaining API CRUD tests | Dev | 16h | MEDIUM |

**Deliverables**:
- 5 E2E tests for critical flows
- Repository layer 40%+ coverage
- Bruno coverage 60%+

### Phase 4: Performance & Security (Weeks 7-8)

| Task | Owner | Effort | Priority |
|------|-------|--------|----------|
| Setup k6 load testing | Dev | 8h | HIGH |
| Create baseline/load/stress scripts | Dev | 16h | HIGH |
| Implement security tests | Dev | 20h | CRITICAL |
| Add multi-tenant isolation tests | Dev | 16h | CRITICAL |

**Deliverables**:
- Performance baselines established
- Load test passing (500 concurrent users)
- Security test suite
- Multi-tenant isolation verified

### Phase 5: Production Readiness (Weeks 9-10)

| Task | Owner | Effort | Priority |
|------|-------|--------|----------|
| CI/CD pipeline complete | DevOps | 16h | HIGH |
| Coverage monitoring (Codecov) | DevOps | 4h | MEDIUM |
| Performance regression detection | DevOps | 8h | MEDIUM |
| Documentation complete | Dev | 8h | MEDIUM |

**Deliverables**:
- Automated test pipeline
- Coverage dashboards
- Performance monitoring
- Complete test documentation

---

## Part 6: Coverage Targets

### Target Coverage by Week 10

| Layer | Current | Target | Metric |
|-------|---------|--------|--------|
| Backend Unit Tests | 24% | 60% | Line coverage |
| Frontend Unit Tests | <1% | 50% | Line coverage |
| API Integration | 21% | 70% | Endpoint coverage |
| E2E Tests | 0% | 5 journeys | User flow coverage |
| Load Tests | 0% | 500 users | Concurrent capacity |
| Security Tests | 0% | OWASP Top 10 | Vulnerability coverage |

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
        target: 80%  # New code must have 80% coverage
```

---

## Part 7: Tools Summary

| Category | Tool | Purpose | Status |
|----------|------|---------|--------|
| **Go Unit Tests** | go test | Backend unit/integration | ✅ Configured |
| **Go Coverage** | go test -cover | Coverage reporting | ✅ Configured |
| **Frontend Unit** | Vitest | React component tests | ✅ Configured |
| **Component Testing** | React Testing Library | DOM testing | ✅ Installed |
| **API Mocking** | MSW | Network-level mocking | ❌ Not installed |
| **API Tests** | Bruno | HTTP endpoint tests | ✅ Configured |
| **E2E Tests** | Playwright | Browser automation | ❌ Not installed |
| **Load Tests** | k6 | Performance testing | ❌ Not installed |
| **Security Scan** | OWASP ZAP | Vulnerability scanning | ❌ Not installed |
| **Coverage Report** | Codecov | Coverage tracking | ❌ Not configured |

---

## Part 8: SaaS-Specific Testing Considerations

### Multi-Tenancy Testing

Per [SaaS testing best practices](https://www.browserstack.com/guide/saas-application-testing-best-practices):

1. **Data Isolation**: Verify tenant A cannot access tenant B's data
2. **Performance Fairness**: Ensure one tenant can't monopolize resources
3. **Configuration Isolation**: Tenant-specific settings don't leak
4. **Backup/Restore**: Per-tenant backup and restore works

### Scalability Testing

Per [multi-tenant SaaS architecture](https://acropolium.com/blog/build-scale-a-multi-tenant-saas/):

- Target 99.99% availability (52 min downtime/year)
- Kubernetes-based orchestration for 70% better resource utilization
- Dynamic resource allocation for 50% better peak performance

### Compliance Testing (GDPR)

- Data deletion verification (right to be forgotten)
- Consent tracking and enforcement
- Audit log completeness (50% faster breach detection)
- Data export functionality

### Disaster Recovery

- RTO target: 1-2 hours
- RPO target: 15 minutes
- Quarterly DR simulation tests

---

## Appendix A: Test File Templates

### Go Test Template
```go
package mypackage_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

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

### React Component Test Template
```typescript
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MyComponent } from './my-component';

describe('MyComponent', () => {
  describe('Rendering', () => {
    it('renders with default props', () => {
      render(<MyComponent />);
      expect(screen.getByRole('button')).toBeInTheDocument();
    });
  });

  describe('User Interactions', () => {
    it('handles click events', async () => {
      const handleClick = vi.fn();
      const user = userEvent.setup();

      render(<MyComponent onClick={handleClick} />);
      await user.click(screen.getByRole('button'));

      expect(handleClick).toHaveBeenCalledOnce();
    });
  });
});
```

### Bruno API Test Template
```
meta {
  name: Feature Test
  type: http
  seq: 100
}

post {
  url: {{baseUrl}}/api/resource
  body: json
  auth: bearer
}

auth:bearer {
  token: {{accessToken}}
}

body:json {
  {
    "field": "value"
  }
}

tests {
  test("creates resource successfully", function() {
    expect(res.status).to.equal(201);
    expect(res.body.data).to.have.property('id');
  });

  test("returns proper structure", function() {
    expect(res.body).to.have.property('status', 'success');
    expect(res.body.data).to.have.property('field', 'value');
  });
}
```

---

## Appendix B: Quick Reference Commands

```bash
# Backend Tests
cd backend
go test ./...                           # Run all tests
go test ./... -cover                    # With coverage
go test ./... -v                        # Verbose
go test -run TestName ./path/to/pkg     # Specific test

# Frontend Tests
cd frontend
npm run test                            # Watch mode
npm run test:run                        # Single run
npm run test:run -- --coverage          # With coverage

# Bruno API Tests
cd bruno
bru run --env Local 0*.bru              # All tests
bru run --env Local 05-sessions.bru     # Specific file

# E2E Tests (after setup)
cd frontend
npx playwright test                     # All tests
npx playwright test --ui                # UI mode
npx playwright test auth.spec.ts        # Specific file

# Load Tests (after setup)
k6 run k6/baseline.js                   # Baseline test
k6 run k6/load.js                       # Load test
k6 run k6/stress.js                     # Stress test
```

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-22 | Claude | Initial comprehensive strategy |

---

**Sources**:
- [SaaS Application Testing Best Practices - BrowserStack](https://www.browserstack.com/guide/saas-application-testing-best-practices)
- [Multi-Tenant SaaS Best Practices - Acropolium](https://acropolium.com/blog/build-scale-a-multi-tenant-saas/)
- [SaaS Testing Tools 2025 - LambdaTest](https://www.lambdatest.com/blog/saas-testing-tools/)
- [Multi-Tenancy Testing - AWS SaaS Lens](https://wa.aws.amazon.com/saas.question.REL_3.en.html)
- [SaaS Security Testing Guide - Qualysec](https://qualysec.com/a-complete-guide-to-conduct-a-saas-application-security-testing/)
