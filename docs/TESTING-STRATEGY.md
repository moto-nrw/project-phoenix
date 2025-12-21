# Testing Strategy - Project Phoenix

**Last Updated**: 2025-12-21
**Version**: 1.0.0

## Table of Contents

1. [Testing Philosophy](#1-testing-philosophy)
2. [Testing Pyramid & Strategy](#2-testing-pyramid--strategy)
3. [Unit Testing Strategy](#3-unit-testing-strategy)
4. [Integration Testing Strategy](#4-integration-testing-strategy)
5. [End-to-End Testing Strategy](#5-end-to-end-testing-strategy)
6. [API Testing Strategy](#6-api-testing-strategy)
7. [Performance Testing Strategy](#7-performance-testing-strategy)
8. [Security Testing Strategy](#8-security-testing-strategy)
9. [Accessibility Testing Strategy](#9-accessibility-testing-strategy)
10. [Visual Regression Testing](#10-visual-regression-testing)
11. [Test Data Management](#11-test-data-management)
12. [Continuous Testing in CI/CD](#12-continuous-testing-in-cicd)
13. [Code Coverage Strategy](#13-code-coverage-strategy)
14. [Test Maintenance & Quality](#14-test-maintenance--quality)
15. [Implementation Roadmap](#15-implementation-roadmap)

---

## 1. Testing Philosophy

### Core Principles

**1.1 Test Value Over Coverage**
- Prioritize high-value tests over arbitrary coverage targets
- Focus on critical business flows and user journeys
- Write tests that catch real bugs, not just increase metrics

**1.2 Fast Feedback Loops**
- Unit tests run in <5 seconds
- Integration tests complete in <30 seconds
- Full test suite (excluding E2E) runs in <2 minutes
- E2E tests complete in <10 minutes

**1.3 Test Reliability**
- Tests must be deterministic (no flaky tests)
- Isolated tests with no shared state
- Hermetic tests with proper setup/teardown
- Clear failure messages for quick debugging

**1.4 Maintainability**
- Tests are first-class code (apply same quality standards)
- DRY principle: Reusable test helpers and fixtures
- Self-documenting tests (clear naming and structure)
- Test code reviews as rigorous as production code

**1.5 Shift-Left Testing**
- Catch bugs early in development cycle
- Write tests alongside production code (TDD encouraged)
- Automated testing before manual testing
- Developer-driven quality (not QA-dependent)

### Testing Mindset

**What to Test**:
- ✅ Business logic and critical paths
- ✅ Edge cases and error handling
- ✅ Data validation and constraints
- ✅ Permission and authorization rules
- ✅ API contracts and integrations
- ✅ User-facing functionality

**What NOT to Test**:
- ❌ Third-party libraries (trust but verify)
- ❌ Trivial getters/setters
- ❌ Framework internals
- ❌ Auto-generated code
- ❌ Simple pass-through functions

---

## 2. Testing Pyramid & Strategy

### The Testing Pyramid

```
              /\
             /  \
            / E2E \         10% - End-to-End Tests
           /--------\              (Slow, Brittle, High-Level)
          /          \
         / Integration \    20% - Integration Tests
        /--------------\          (Medium Speed, Component Interaction)
       /                \
      /   Unit Tests     \  70% - Unit Tests
     /____________________\        (Fast, Isolated, Fine-Grained)
```

### Project Phoenix Testing Strategy

**Target Distribution** (by test count):
- **70% Unit Tests**: Fast, isolated, fine-grained validation
- **20% Integration Tests**: Component interaction, API contracts, database
- **10% E2E Tests**: Critical user flows, full-stack validation

**Target Distribution** (by execution time):
- **20% Unit Tests**: <5 seconds total
- **30% Integration Tests**: <30 seconds total
- **50% E2E Tests**: <10 minutes total

### Complementary Testing Strategies

Beyond the pyramid, implement:
- **API Testing** (Bruno) - Hermetic, contract-based validation
- **Performance Testing** - Load tests, benchmarks, stress tests
- **Security Testing** - Vulnerability scanning, penetration testing
- **Accessibility Testing** - WCAG compliance, screen reader support
- **Visual Regression** - UI consistency, cross-browser validation

---

## 3. Unit Testing Strategy

### Backend Unit Testing (Go)

#### What to Test

**Models & Validation**:
```go
// Test business rules and validation
func TestStudent_Validate(t *testing.T) {
    tests := []struct {
        name    string
        student *users.Student
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid student",
            student: &users.Student{
                PersonID: 1,
                DataRetentionDays: ptr(30),
            },
            wantErr: false,
        },
        {
            name: "invalid retention period",
            student: &users.Student{
                PersonID: 1,
                DataRetentionDays: ptr(0),
            },
            wantErr: true,
            errMsg: "data_retention_days must be between 1 and 31",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.student.Validate()
            if tt.wantErr {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

**Services (Business Logic)**:
```go
// Test business logic with mocked dependencies
func TestSessionService_CreateSession(t *testing.T) {
    // Setup
    mockRepo := new(MockSessionRepository)
    service := NewSessionService(mockRepo)

    // Test data
    input := &CreateSessionInput{
        GroupID: 1,
        RoomID: 2,
        SupervisorIDs: []int64{3},
    }

    // Mock expectations
    mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(s *Session) bool {
        return s.GroupID == 1 && s.RoomID == 2
    })).Return(nil)

    // Execute
    session, err := service.CreateSession(context.Background(), input)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, int64(1), session.GroupID)
    mockRepo.AssertExpectations(t)
}
```

**Error Handling & Edge Cases**:
```go
func TestAuthService_Login_EdgeCases(t *testing.T) {
    tests := []struct {
        name      string
        email     string
        password  string
        setupMock func(*MockUserRepository)
        wantErr   error
    }{
        {
            name:     "account locked",
            email:    "locked@example.com",
            password: "valid_password",
            setupMock: func(m *MockUserRepository) {
                m.On("FindByEmail", "locked@example.com").Return(&User{
                    IsLocked: true,
                }, nil)
            },
            wantErr: ErrAccountLocked,
        },
        {
            name:     "invalid credentials",
            email:    "user@example.com",
            password: "wrong_password",
            setupMock: func(m *MockUserRepository) {
                m.On("FindByEmail", "user@example.com").Return(&User{
                    PasswordHash: "hashed_password",
                }, nil)
            },
            wantErr: ErrInvalidCredentials,
        },
    }
    // ... test execution
}
```

#### Best Practices

**Table-Driven Tests**:
```go
// Use table-driven tests for multiple scenarios
func TestCalculateRetentionDate(t *testing.T) {
    baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

    tests := []struct {
        name              string
        retentionDays     *int
        expected          time.Time
    }{
        {"default 30 days", nil, baseTime.AddDate(0, 0, 30)},
        {"custom 7 days", ptr(7), baseTime.AddDate(0, 0, 7)},
        {"max 31 days", ptr(31), baseTime.AddDate(0, 0, 31)},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := CalculateRetentionDate(baseTime, tt.retentionDays)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

**Test Helpers & Fixtures**:
```go
// Centralize test data creation
func createTestStudent(t *testing.T, overrides ...func(*users.Student)) *users.Student {
    t.Helper()

    student := &users.Student{
        PersonID:          1,
        DataRetentionDays: ptr(30),
        CreatedAt:         time.Now(),
    }

    for _, override := range overrides {
        override(student)
    }

    return student
}

// Usage
student := createTestStudent(t, func(s *users.Student) {
    s.DataRetentionDays = ptr(7)
})
```

**Subtests for Organization**:
```go
func TestUserService(t *testing.T) {
    t.Run("Create", func(t *testing.T) {
        t.Run("ValidInput", func(t *testing.T) { /* ... */ })
        t.Run("DuplicateEmail", func(t *testing.T) { /* ... */ })
        t.Run("InvalidData", func(t *testing.T) { /* ... */ })
    })

    t.Run("Update", func(t *testing.T) {
        t.Run("SuccessfulUpdate", func(t *testing.T) { /* ... */ })
        t.Run("NotFound", func(t *testing.T) { /* ... */ })
    })
}
```

### Frontend Unit Testing (React/Next.js)

#### What to Test

**Components - User Interactions**:
```typescript
// Test user interactions and state changes
describe("StudentCheckInForm", () => {
  it("should submit form with valid data", async () => {
    const mockOnSubmit = vi.fn();
    const user = userEvent.setup();

    render(<StudentCheckInForm onSubmit={mockOnSubmit} />);

    // Fill form
    await user.type(screen.getByLabelText(/student/i), "John Doe");
    await user.selectOptions(screen.getByLabelText(/room/i), "101");
    await user.click(screen.getByRole("button", { name: /check in/i }));

    // Assert
    expect(mockOnSubmit).toHaveBeenCalledWith({
      studentId: "1",
      roomId: "101",
    });
  });

  it("should show validation errors for invalid data", async () => {
    const user = userEvent.setup();
    render(<StudentCheckInForm onSubmit={vi.fn()} />);

    // Submit without filling form
    await user.click(screen.getByRole("button", { name: /check in/i }));

    // Assert error messages
    expect(screen.getByText(/student is required/i)).toBeInTheDocument();
    expect(screen.getByText(/room is required/i)).toBeInTheDocument();
  });
});
```

**Hooks - Custom Logic**:
```typescript
describe("useSessionTimer", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("should increment elapsed time every second", () => {
    const { result } = renderHook(() => useSessionTimer({
      startTime: new Date("2025-01-01T10:00:00Z"),
    }));

    expect(result.current.elapsed).toBe(0);

    // Fast-forward 1 second
    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(result.current.elapsed).toBe(1);

    // Fast-forward 5 more seconds
    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(result.current.elapsed).toBe(6);
  });
});
```

**API Clients - Mock Service Worker**:
```typescript
// Setup MSW for API mocking
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";

const server = setupServer(
  http.get("/api/students", () => {
    return HttpResponse.json({
      status: "success",
      data: [
        { id: "1", name: "John Doe" },
        { id: "2", name: "Jane Smith" },
      ],
    });
  })
);

beforeAll(() => server.listen());
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("fetchStudents", () => {
  it("should fetch and transform student data", async () => {
    const students = await fetchStudents();

    expect(students).toHaveLength(2);
    expect(students[0]).toEqual({
      id: "1",
      name: "John Doe",
    });
  });

  it("should handle API errors", async () => {
    server.use(
      http.get("/api/students", () => {
        return new HttpResponse(null, { status: 500 });
      })
    );

    await expect(fetchStudents()).rejects.toThrow("Failed to fetch students");
  });
});
```

**Utilities & Helpers**:
```typescript
describe("formatDuration", () => {
  it.each([
    [0, "0s"],
    [59, "59s"],
    [60, "1m 0s"],
    [3661, "1h 1m 1s"],
    [7200, "2h 0m 0s"],
  ])("should format %i seconds as %s", (seconds, expected) => {
    expect(formatDuration(seconds)).toBe(expected);
  });
});
```

#### Best Practices

**Testing Library Principles**:
```typescript
// ✅ GOOD - Test like a user
const submitButton = screen.getByRole("button", { name: /submit/i });
await user.click(submitButton);

// ❌ BAD - Test implementation details
const submitButton = wrapper.find('[data-testid="submit-button"]');
submitButton.simulate("click");
```

**Arrange-Act-Assert Pattern**:
```typescript
it("should display success message after form submission", async () => {
  // Arrange - Setup test conditions
  const mockOnSubmit = vi.fn().mockResolvedValue({ success: true });
  render(<MyForm onSubmit={mockOnSubmit} />);

  // Act - Perform user actions
  await userEvent.type(screen.getByLabelText(/name/i), "John");
  await userEvent.click(screen.getByRole("button", { name: /submit/i }));

  // Assert - Verify outcomes
  await waitFor(() => {
    expect(screen.getByText(/success/i)).toBeInTheDocument();
  });
});
```

**Avoid Testing Implementation Details**:
```typescript
// ✅ GOOD - Test behavior
expect(screen.getByText("3 items")).toBeInTheDocument();

// ❌ BAD - Test state
expect(component.state.itemCount).toBe(3);
```

---

## 4. Integration Testing Strategy

### Backend Integration Testing

#### Database Integration Tests

**Real Database Tests**:
```go
func TestStudentRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup test database
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    repo := repositories.NewStudentRepository(db)

    t.Run("Create and Retrieve", func(t *testing.T) {
        // Create student
        student := &users.Student{
            PersonID: 1,
            DataRetentionDays: ptr(30),
        }

        err := repo.Create(context.Background(), student)
        require.NoError(t, err)
        assert.NotZero(t, student.ID)

        // Retrieve student
        retrieved, err := repo.GetByID(context.Background(), student.ID)
        require.NoError(t, err)
        assert.Equal(t, student.PersonID, retrieved.PersonID)
    })

    t.Run("List with Filters", func(t *testing.T) {
        // Create test data
        createTestStudents(t, repo, 10)

        // Test pagination
        students, err := repo.List(context.Background(), &ListOptions{
            Limit:  5,
            Offset: 0,
        })

        require.NoError(t, err)
        assert.Len(t, students, 5)
    })
}
```

**Transaction Testing**:
```go
func TestSessionService_CreateWithVisits_Transaction(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    service := NewSessionService(db)

    t.Run("Rollback on Visit Creation Failure", func(t *testing.T) {
        // Start transaction
        ctx := context.Background()

        // Attempt to create session with invalid visit data
        _, err := service.CreateSessionWithVisits(ctx, &CreateInput{
            GroupID: 1,
            RoomID: 2,
            Visits: []*Visit{
                {StudentID: 999}, // Non-existent student
            },
        })

        // Should fail
        require.Error(t, err)

        // Verify session was NOT created (transaction rolled back)
        sessions, _ := service.ListSessions(ctx)
        assert.Empty(t, sessions)
    })
}
```

**Multi-Schema Tests**:
```go
func TestCrossSchemaQueries(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    t.Run("Join Across Schemas", func(t *testing.T) {
        // Query across users, education, and active schemas
        var results []struct {
            StudentName string
            GroupName   string
            RoomName    string
        }

        err := db.NewSelect().
            TableExpr("users.students s").
            Join("JOIN education.groups g ON g.id = s.group_id").
            Join("JOIN active.sessions sess ON sess.group_id = g.id").
            Join("JOIN facilities.rooms r ON r.id = sess.room_id").
            Column("s.name AS student_name").
            Column("g.name AS group_name").
            Column("r.name AS room_name").
            Scan(context.Background(), &results)

        require.NoError(t, err)
    })
}
```

### Frontend Integration Testing

#### Component Integration Tests

**Multi-Component Interactions**:
```typescript
describe("SessionManagementPage Integration", () => {
  it("should create session and update active sessions list", async () => {
    const user = userEvent.setup();

    // Mock API
    server.use(
      http.post("/api/sessions", async ({ request }) => {
        const body = await request.json();
        return HttpResponse.json({
          status: "success",
          data: { id: "1", ...body },
        });
      }),
      http.get("/api/sessions", () => {
        return HttpResponse.json({
          status: "success",
          data: [{ id: "1", groupName: "Group A", roomName: "Room 101" }],
        });
      })
    );

    render(<SessionManagementPage />);

    // Fill form
    await user.type(screen.getByLabelText(/group/i), "Group A");
    await user.type(screen.getByLabelText(/room/i), "Room 101");
    await user.click(screen.getByRole("button", { name: /create/i }));

    // Verify session appears in list
    await waitFor(() => {
      expect(screen.getByText("Group A")).toBeInTheDocument();
      expect(screen.getByText("Room 101")).toBeInTheDocument();
    });
  });
});
```

**Form Validation & Error Handling**:
```typescript
describe("CheckInForm Error Handling", () => {
  it("should display server validation errors", async () => {
    server.use(
      http.post("/api/checkins", () => {
        return HttpResponse.json(
          {
            status: "error",
            message: "Student is already checked in",
            code: "ALREADY_CHECKED_IN",
          },
          { status: 400 }
        );
      })
    );

    render(<CheckInForm />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/student/i), "John Doe");
    await user.click(screen.getByRole("button", { name: /check in/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/student is already checked in/i)
      ).toBeInTheDocument();
    });
  });
});
```

---

## 5. End-to-End Testing Strategy

### E2E Testing with Playwright

#### Critical User Flows

**Priority 1 - Authentication & Authorization**:
```typescript
// tests/e2e/auth.spec.ts
import { test, expect } from "@playwright/test";

test.describe("Authentication Flow", () => {
  test("should login as admin and access dashboard", async ({ page }) => {
    // Navigate to login
    await page.goto("/login");

    // Fill credentials
    await page.fill('input[name="email"]', "admin@example.com");
    await page.fill('input[name="password"]', "Test1234%");
    await page.click('button[type="submit"]');

    // Verify redirect to dashboard
    await expect(page).toHaveURL("/dashboard");
    await expect(page.locator("h1")).toContainText("Dashboard");

    // Verify user menu
    await page.click('[data-testid="user-menu"]');
    await expect(page.locator("text=admin@example.com")).toBeVisible();
  });

  test("should deny access to unauthorized pages", async ({ page }) => {
    // Navigate without authentication
    await page.goto("/admin/users");

    // Should redirect to login
    await expect(page).toHaveURL("/login");
  });

  test("should handle invalid credentials", async ({ page }) => {
    await page.goto("/login");

    await page.fill('input[name="email"]', "admin@example.com");
    await page.fill('input[name="password"]', "WrongPassword");
    await page.click('button[type="submit"]');

    // Verify error message
    await expect(page.locator("text=/invalid credentials/i")).toBeVisible();
    await expect(page).toHaveURL("/login");
  });
});
```

**Priority 2 - Session Management**:
```typescript
// tests/e2e/sessions.spec.ts
test.describe("Session Management", () => {
  test.beforeEach(async ({ page }) => {
    // Login as teacher
    await loginAsTeacher(page);
  });

  test("should create active session", async ({ page }) => {
    await page.goto("/sessions");

    // Click create button
    await page.click('button:has-text("New Session")');

    // Fill form
    await page.selectOption('select[name="groupId"]', "1");
    await page.selectOption('select[name="roomId"]', "101");
    await page.click('button[type="submit"]');

    // Verify success
    await expect(page.locator("text=/session created/i")).toBeVisible();

    // Verify appears in list
    const sessionRow = page.locator('[data-testid="session-row"]').first();
    await expect(sessionRow).toContainText("Group A");
    await expect(sessionRow).toContainText("Room 101");
  });

  test("should prevent room conflict", async ({ page }) => {
    // Create first session
    await createSession(page, { groupId: "1", roomId: "101" });

    // Try to create second session with same room
    await page.click('button:has-text("New Session")');
    await page.selectOption('select[name="groupId"]', "2");
    await page.selectOption('select[name="roomId"]', "101"); // Same room
    await page.click('button[type="submit"]');

    // Verify error
    await expect(
      page.locator("text=/room is already occupied/i")
    ).toBeVisible();
  });
});
```

**Priority 3 - RFID Check-In Flow**:
```typescript
// tests/e2e/checkin.spec.ts
test.describe("Student Check-In", () => {
  test("should check in student via web interface", async ({ page }) => {
    await loginAsTeacher(page);
    await page.goto("/checkins");

    // Select student
    await page.fill('input[name="search"]', "John Doe");
    await page.click('button:has-text("John Doe")');

    // Select room
    await page.selectOption('select[name="roomId"]', "101");

    // Submit check-in
    await page.click('button:has-text("Check In")');

    // Verify success
    await expect(page.locator("text=/checked in successfully/i")).toBeVisible();

    // Verify student appears in active list
    await page.goto("/active");
    await expect(page.locator("text=John Doe")).toBeVisible();
    await expect(page.locator("text=Room 101")).toBeVisible();
  });

  test("should check out student", async ({ page }) => {
    await loginAsTeacher(page);

    // Navigate to active students
    await page.goto("/active");

    // Find student row
    const studentRow = page.locator('tr:has-text("John Doe")');
    await studentRow.locator('button:has-text("Check Out")').click();

    // Confirm checkout
    await page.click('button:has-text("Confirm")');

    // Verify success
    await expect(
      page.locator("text=/checked out successfully/i")
    ).toBeVisible();

    // Verify student removed from list
    await expect(page.locator("text=John Doe")).not.toBeVisible();
  });
});
```

#### E2E Best Practices

**Page Object Model**:
```typescript
// tests/pages/LoginPage.ts
export class LoginPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto("/login");
  }

  async login(email: string, password: string) {
    await this.page.fill('input[name="email"]', email);
    await this.page.fill('input[name="password"]', password);
    await this.page.click('button[type="submit"]');
  }

  async expectLoginSuccess() {
    await expect(this.page).toHaveURL("/dashboard");
  }

  async expectLoginFailure() {
    await expect(this.page.locator("text=/invalid credentials/i")).toBeVisible();
  }
}

// Usage
test("should login successfully", async ({ page }) => {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login("admin@example.com", "Test1234%");
  await loginPage.expectLoginSuccess();
});
```

**Test Fixtures**:
```typescript
// tests/fixtures.ts
import { test as base } from "@playwright/test";

type Fixtures = {
  authenticatedPage: Page;
};

export const test = base.extend<Fixtures>({
  authenticatedPage: async ({ page }, use) => {
    // Login before each test
    await page.goto("/login");
    await page.fill('input[name="email"]', "admin@example.com");
    await page.fill('input[name="password"]', "Test1234%");
    await page.click('button[type="submit"]');
    await page.waitForURL("/dashboard");

    await use(page);

    // Logout after test
    await page.click('[data-testid="user-menu"]');
    await page.click("text=Logout");
  },
});

// Usage
test("should access protected page", async ({ authenticatedPage }) => {
  await authenticatedPage.goto("/admin/users");
  await expect(authenticatedPage.locator("h1")).toContainText("Users");
});
```

**Visual Testing**:
```typescript
test("should match dashboard screenshot", async ({ page }) => {
  await loginAsAdmin(page);
  await page.goto("/dashboard");

  // Wait for content to load
  await page.waitForSelector('[data-testid="dashboard-content"]');

  // Take screenshot
  await expect(page).toHaveScreenshot("dashboard.png", {
    fullPage: true,
    threshold: 0.2, // Allow 20% difference
  });
});
```

---

## 6. API Testing Strategy

### Bruno API Testing (Current)

#### Hermetic Test Design

**Self-Contained Tests**:
```bruno
# 05-sessions.bru

# Pre-request: Create test data
pre-request {
  // Get fresh auth token
  const authResponse = await api.post("/auth/login", {
    email: "andreas.arndt@schulzentrum.de",
    password: "Test1234%"
  });
  bru.setVar("accessToken", authResponse.data.accessToken);

  // Create group for testing
  const group = await api.post("/api/groups", {
    name: "Test Group"
  });
  bru.setVar("testGroupId", group.data.id);
}

# Test 1: Create session
POST {{baseUrl}}/api/active/sessions
Authorization: Bearer {{accessToken}}

{
  "group_id": {{testGroupId}},
  "room_id": "101",
  "supervisor_ids": ["1"]
}

# Test cleanup
post-response {
  if (res.status === 201) {
    const sessionId = res.body.data.id;

    // Delete session after test
    await api.delete(`/api/active/sessions/${sessionId}`);
  }
}
```

#### Contract Testing

**Validate Response Schemas**:
```bruno
# Test: Get student list
GET {{baseUrl}}/api/students
Authorization: Bearer {{accessToken}}

# Post-response validation
post-response {
  const { status, data } = res.body;

  // Validate response structure
  assert.equal(status, "success");
  assert.isArray(data);

  // Validate student schema
  if (data.length > 0) {
    const student = data[0];
    assert.hasAllKeys(student, [
      "id",
      "person_id",
      "data_retention_days",
      "created_at",
      "updated_at"
    ]);
    assert.isString(student.id);
    assert.isNumber(student.person_id);
  }
}
```

#### Edge Case Testing

**Boundary Conditions**:
```bruno
# Test: Data retention validation
POST {{baseUrl}}/api/students
Authorization: Bearer {{accessToken}}

{
  "person_id": 1,
  "data_retention_days": 0  // Invalid: must be 1-31
}

# Post-response
post-response {
  assert.equal(res.status, 400);
  assert.equal(res.body.status, "error");
  assert.include(res.body.message, "data_retention_days");
}
```

**Concurrent Requests**:
```bruno
# Test: Room conflict detection
pre-request {
  // Create two sessions simultaneously
  const requests = [
    api.post("/api/active/sessions", {
      group_id: "1",
      room_id: "101"
    }),
    api.post("/api/active/sessions", {
      group_id: "2",
      room_id: "101"  // Same room
    })
  ];

  const results = await Promise.allSettled(requests);

  // One should succeed, one should fail
  const succeeded = results.filter(r => r.status === "fulfilled");
  const failed = results.filter(r => r.status === "rejected");

  assert.equal(succeeded.length, 1);
  assert.equal(failed.length, 1);
}
```

### Alternative: REST Client (VS Code Extension)

**HTTP File Format**:
```http
### Login
# @name login
POST {{baseUrl}}/auth/login
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "Test1234%"
}

###
@accessToken = {{login.response.body.$.data.accessToken}}

### Get Students
GET {{baseUrl}}/api/students
Authorization: Bearer {{accessToken}}

### Create Student
POST {{baseUrl}}/api/students
Authorization: Bearer {{accessToken}}
Content-Type: application/json

{
  "person_id": 1,
  "data_retention_days": 30
}
```

---

## 7. Performance Testing Strategy

### Go Benchmarks

**Backend Performance Tests**:
```go
// services/active/session_service_bench_test.go
func BenchmarkSessionService_CreateSession(b *testing.B) {
    db := setupTestDB(b)
    defer db.Close()

    service := NewSessionService(db)
    input := &CreateSessionInput{
        GroupID: 1,
        RoomID: 2,
        SupervisorIDs: []int64{3},
    }

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        _, err := service.CreateSession(context.Background(), input)
        if err != nil {
            b.Fatal(err)
        }

        // Cleanup
        _ = service.DeleteSession(context.Background(), session.ID)
    }
}

func BenchmarkStudentRepository_List(b *testing.B) {
    db := setupTestDB(b)
    defer db.Close()

    repo := repositories.NewStudentRepository(db)

    // Create test data
    for i := 0; i < 1000; i++ {
        createTestStudent(b, repo)
    }

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        _, err := repo.List(context.Background(), &ListOptions{
            Limit: 50,
            Offset: 0,
        })
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

**Run Benchmarks**:
```bash
go test -bench=. ./services/active
go test -bench=BenchmarkSessionService -benchmem
go test -bench=. -benchtime=10s -cpuprofile=cpu.prof
```

### Load Testing with k6

**API Load Test**:
```javascript
// loadtests/api_load_test.js
import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  stages: [
    { duration: "30s", target: 10 },  // Ramp up to 10 users
    { duration: "1m", target: 50 },   // Ramp up to 50 users
    { duration: "2m", target: 50 },   // Stay at 50 users
    { duration: "30s", target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ["p(95)<500"],  // 95% of requests < 500ms
    http_req_failed: ["rate<0.01"],    // Error rate < 1%
  },
};

export default function () {
  // Login
  const loginRes = http.post(`${__ENV.BASE_URL}/auth/login`, {
    email: "admin@example.com",
    password: "Test1234%",
  });

  check(loginRes, {
    "login status 200": (r) => r.status === 200,
  });

  const token = loginRes.json("data.accessToken");

  // Get students
  const studentsRes = http.get(`${__ENV.BASE_URL}/api/students`, {
    headers: { Authorization: `Bearer ${token}` },
  });

  check(studentsRes, {
    "students status 200": (r) => r.status === 200,
    "response time < 500ms": (r) => r.timings.duration < 500,
  });

  sleep(1);
}
```

**Run Load Test**:
```bash
k6 run loadtests/api_load_test.js
k6 run --vus 50 --duration 30s loadtests/api_load_test.js
```

### Database Performance Testing

**Query Performance Tests**:
```go
func BenchmarkComplexQuery_ActiveStudents(b *testing.B) {
    db := setupTestDB(b)
    defer db.Close()

    // Create realistic test data
    createTestData(b, db, 1000) // 1000 students

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        var results []struct {
            StudentID   int64
            StudentName string
            RoomName    string
            Duration    int
        }

        err := db.NewSelect().
            TableExpr("active.visits v").
            Join("JOIN users.students s ON s.id = v.student_id").
            Join("JOIN facilities.rooms r ON r.id = v.room_id").
            Column("v.student_id", "s.name AS student_name").
            Column("r.name AS room_name").
            ColumnExpr("EXTRACT(EPOCH FROM (NOW() - v.entry_time))::int AS duration").
            Where("v.exit_time IS NULL").
            Scan(context.Background(), &results)

        if err != nil {
            b.Fatal(err)
        }
    }
}
```

**Analyze Query Plans**:
```go
func TestQueryPerformance_ExplainAnalyze(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    query := db.NewSelect().
        TableExpr("active.visits v").
        Join("JOIN users.students s ON s.id = v.student_id").
        Where("v.exit_time IS NULL")

    // Get EXPLAIN ANALYZE output
    var plans []struct {
        Plan string
    }

    _, err := db.Query(&plans, "EXPLAIN ANALYZE "+query.String())
    require.NoError(t, err)

    // Log query plan for analysis
    t.Logf("Query Plan:\n%s", plans[0].Plan)
}
```

---

## 8. Security Testing Strategy

### Static Application Security Testing (SAST)

**Go Security Scanner (gosec)**:
```bash
# Install
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run security scan
gosec ./...
gosec -fmt=json -out=security-report.json ./...
```

**Configuration** (`.gosec.json`):
```json
{
  "exclude": [],
  "tests": true,
  "exclude-dirs": [
    "vendor",
    "node_modules"
  ],
  "severity": "medium",
  "confidence": "medium"
}
```

### Dynamic Application Security Testing (DAST)

**OWASP ZAP Integration**:
```yaml
# .github/workflows/security.yml
name: Security Scan

on: [push, pull_request]

jobs:
  zap-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Start Application
        run: docker compose up -d

      - name: Wait for Application
        run: sleep 30

      - name: ZAP Baseline Scan
        uses: zaproxy/action-baseline@v0.7.0
        with:
          target: "http://localhost:3000"
          rules_file_name: ".zap/rules.tsv"
          cmd_options: "-a"
```

### Vulnerability Scanning

**Dependency Scanning**:
```bash
# Go dependencies
go install github.com/sonatypeoss/nancy@latest
go list -json -m all | nancy sleuth

# Node dependencies
npm audit
npm audit --audit-level=moderate
npm audit fix
```

**Container Scanning**:
```bash
# Install trivy
brew install aquasecurity/trivy/trivy

# Scan Docker images
trivy image project-phoenix-backend:latest
trivy image project-phoenix-frontend:latest
```

### Penetration Testing Checklist

**Authentication & Authorization**:
- [ ] SQL injection in login forms
- [ ] JWT token manipulation
- [ ] Session fixation attacks
- [ ] Brute force protection
- [ ] Password reset vulnerabilities
- [ ] CSRF protection
- [ ] XSS in user inputs

**API Security**:
- [ ] Rate limiting bypass
- [ ] Mass assignment vulnerabilities
- [ ] Insecure direct object references (IDOR)
- [ ] API endpoint enumeration
- [ ] Authentication bypass
- [ ] Privilege escalation

**Data Protection**:
- [ ] SSL/TLS configuration
- [ ] Sensitive data exposure
- [ ] PII encryption
- [ ] GDPR compliance (data retention, right to erasure)

---

## 9. Accessibility Testing Strategy

### Automated Accessibility Testing

**axe-core Integration (Frontend)**:
```typescript
// tests/a11y/accessibility.test.ts
import { axe, toHaveNoViolations } from "jest-axe";

expect.extend(toHaveNoViolations);

describe("Accessibility", () => {
  it("should have no violations on login page", async () => {
    const { container } = render(<LoginPage />);
    const results = await axe(container);
    expect(results).toHaveNoViolations();
  });

  it("should have no violations on dashboard", async () => {
    const { container } = render(<Dashboard />);
    const results = await axe(container);
    expect(results).toHaveNoViolations();
  });
});
```

**Playwright Accessibility Tests**:
```typescript
// tests/e2e/a11y.spec.ts
import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

test.describe("Accessibility", () => {
  test("should not have accessibility violations on homepage", async ({
    page,
  }) => {
    await page.goto("/");

    const accessibilityScanResults = await new AxeBuilder({ page }).analyze();

    expect(accessibilityScanResults.violations).toEqual([]);
  });

  test("should have proper heading hierarchy", async ({ page }) => {
    await page.goto("/dashboard");

    // Check heading levels
    const h1Count = await page.locator("h1").count();
    expect(h1Count).toBe(1); // Only one H1 per page

    // Verify heading order
    const headings = await page.locator("h1, h2, h3, h4, h5, h6").all();
    let previousLevel = 0;

    for (const heading of headings) {
      const tagName = await heading.evaluate((el) => el.tagName);
      const level = parseInt(tagName[1]);

      // Headings should not skip levels
      expect(level).toBeLessThanOrEqual(previousLevel + 1);
      previousLevel = level;
    }
  });
});
```

### Manual Accessibility Testing

**Keyboard Navigation**:
```typescript
test("should be fully keyboard navigable", async ({ page }) => {
  await page.goto("/sessions");

  // Tab through interactive elements
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toHaveAttribute("href", "/dashboard");

  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toHaveAttribute("href", "/sessions");

  // Enter on focused button should activate
  await page.keyboard.press("Enter");
  await expect(page.locator('[role="dialog"]')).toBeVisible();

  // Escape should close modal
  await page.keyboard.press("Escape");
  await expect(page.locator('[role="dialog"]')).not.toBeVisible();
});
```

**Screen Reader Testing**:
```typescript
test("should have proper ARIA labels", async ({ page }) => {
  await page.goto("/checkins");

  // Verify form has accessible name
  const form = page.locator("form");
  await expect(form).toHaveAttribute("aria-label", "Student check-in form");

  // Verify buttons have accessible text
  const submitButton = page.locator('button[type="submit"]');
  await expect(submitButton).toHaveAccessibleName("Check in student");

  // Verify error messages are announced
  await submitButton.click();
  const errorMessage = page.locator('[role="alert"]');
  await expect(errorMessage).toHaveAttribute("aria-live", "polite");
});
```

### WCAG Compliance Checklist

**Level A (Must Have)**:
- [ ] Non-text content has text alternatives
- [ ] Captions for prerecorded audio/video
- [ ] Content can be presented in different ways
- [ ] Color is not the only visual means of conveying information
- [ ] Keyboard accessible (all functionality available via keyboard)
- [ ] Enough time to read and use content
- [ ] Content does not cause seizures (no flashing > 3 times/second)
- [ ] Navigable (skip navigation, page titles, focus order, link purpose)
- [ ] Readable (language of page identified, unusual words explained)
- [ ] Predictable (consistent navigation and identification)
- [ ] Input assistance (error identification, labels/instructions, error suggestions)

**Level AA (Should Have)**:
- [ ] Captions for live audio
- [ ] Audio description for prerecorded video
- [ ] Color contrast ratio at least 4.5:1
- [ ] Text can be resized up to 200% without loss of functionality
- [ ] Images of text avoided (except logos)
- [ ] Multiple ways to find pages
- [ ] Headings and labels describe topic/purpose
- [ ] Keyboard focus is visible
- [ ] Language of parts identified
- [ ] On input, context changes are predictable
- [ ] Error prevention for legal/financial/data transactions

---

## 10. Visual Regression Testing

### Percy or Chromatic Setup

**Percy Configuration** (`.percy.yml`):
```yaml
version: 2
static:
  include:
    - "**/*.png"
    - "**/*.jpg"
snapshot:
  widths:
    - 375  # Mobile
    - 768  # Tablet
    - 1280 # Desktop
  min-height: 1024
```

**Visual Tests with Playwright + Percy**:
```typescript
// tests/visual/visual.spec.ts
import { test } from "@playwright/test";
import percySnapshot from "@percy/playwright";

test.describe("Visual Regression", () => {
  test("login page", async ({ page }) => {
    await page.goto("/login");
    await percySnapshot(page, "Login Page");
  });

  test("dashboard with data", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/dashboard");
    await page.waitForSelector('[data-testid="dashboard-content"]');
    await percySnapshot(page, "Dashboard - Loaded");
  });

  test("session creation modal", async ({ page }) => {
    await loginAsTeacher(page);
    await page.goto("/sessions");
    await page.click('button:has-text("New Session")');
    await page.waitForSelector('[role="dialog"]');
    await percySnapshot(page, "Session Creation Modal");
  });
});
```

### Responsive Design Testing

**Multi-Viewport Tests**:
```typescript
const viewports = [
  { name: "mobile", width: 375, height: 667 },
  { name: "tablet", width: 768, height: 1024 },
  { name: "desktop", width: 1280, height: 720 },
  { name: "wide", width: 1920, height: 1080 },
];

test.describe("Responsive Design", () => {
  for (const viewport of viewports) {
    test(`dashboard should render correctly on ${viewport.name}`, async ({
      page,
    }) => {
      await page.setViewportSize(viewport);
      await loginAsAdmin(page);
      await page.goto("/dashboard");

      await expect(page.locator("h1")).toBeVisible();
      await percySnapshot(page, `Dashboard - ${viewport.name}`);
    });
  }
});
```

---

## 11. Test Data Management

### Test Data Strategy

**Fixture-Based Approach**:
```go
// backend/test/fixtures/students.go
package fixtures

type StudentFixture struct {
    ID                int64
    PersonID          int64
    DataRetentionDays *int
}

var Students = []StudentFixture{
    {
        ID:                1,
        PersonID:          1,
        DataRetentionDays: ptr(30),
    },
    {
        ID:                2,
        PersonID:          2,
        DataRetentionDays: ptr(7),
    },
}

func LoadStudents(db *bun.DB) error {
    for _, s := range Students {
        _, err := db.NewInsert().
            Model(&s).
            Exec(context.Background())
        if err != nil {
            return err
        }
    }
    return nil
}
```

**Factory Pattern (Backend)**:
```go
// backend/test/factories/student_factory.go
package factories

type StudentFactory struct {
    defaults *users.Student
}

func NewStudentFactory() *StudentFactory {
    return &StudentFactory{
        defaults: &users.Student{
            PersonID:          1,
            DataRetentionDays: ptr(30),
        },
    }
}

func (f *StudentFactory) Create(overrides ...func(*users.Student)) *users.Student {
    student := *f.defaults

    for _, override := range overrides {
        override(&student)
    }

    return &student
}

// Usage
student := factories.NewStudentFactory().Create(func(s *users.Student) {
    s.DataRetentionDays = ptr(7)
})
```

**Builder Pattern (Frontend)**:
```typescript
// frontend/src/test/builders/student.builder.ts
export class StudentBuilder {
  private student: Partial<Student> = {
    id: "1",
    personId: "1",
    dataRetentionDays: 30,
  };

  withId(id: string): this {
    this.student.id = id;
    return this;
  }

  withRetention(days: number): this {
    this.student.dataRetentionDays = days;
    return this;
  }

  build(): Student {
    return this.student as Student;
  }
}

// Usage
const student = new StudentBuilder()
  .withId("123")
  .withRetention(7)
  .build();
```

### Database Seeding

**Idempotent Seed Script**:
```go
// backend/cmd/seed.go
func seedDatabase(db *bun.DB, reset bool) error {
    ctx := context.Background()

    if reset {
        // Clear existing data
        if err := clearAllData(ctx, db); err != nil {
            return err
        }
    }

    // Seed in dependency order
    if err := seedPersons(ctx, db); err != nil {
        return err
    }

    if err := seedStaff(ctx, db); err != nil {
        return err
    }

    if err := seedStudents(ctx, db); err != nil {
        return err
    }

    if err := seedGroups(ctx, db); err != nil {
        return err
    }

    return nil
}

func seedStudents(ctx context.Context, db *bun.DB) error {
    students := []users.Student{
        {PersonID: 1, DataRetentionDays: ptr(30)},
        {PersonID: 2, DataRetentionDays: ptr(7)},
        // ... more students
    }

    for _, student := range students {
        _, err := db.NewInsert().
            Model(&student).
            On("CONFLICT (person_id) DO NOTHING").
            Exec(ctx)
        if err != nil {
            return err
        }
    }

    return nil
}
```

---

## 12. Continuous Testing in CI/CD

### Comprehensive CI/CD Pipeline

**Full Testing Workflow**:
```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main, development]
  pull_request:
    branches: [main, development]

jobs:
  # Stage 1: Lint & Static Analysis
  lint:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        component: [backend, frontend]
    steps:
      - uses: actions/checkout@v4

      - name: Backend Lint
        if: matrix.component == 'backend'
        run: |
          cd backend
          golangci-lint run --timeout=10m

      - name: Frontend Lint
        if: matrix.component == 'frontend'
        run: |
          cd frontend
          npm ci
          npm run lint
          npm run typecheck

  # Stage 2: Unit & Integration Tests
  test:
    runs-on: ubuntu-latest
    needs: lint
    strategy:
      matrix:
        component: [backend, frontend]
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

      - name: Backend Tests
        if: matrix.component == 'backend'
        run: |
          cd backend
          go test -v -race -coverprofile=coverage.out ./...
          go tool cover -html=coverage.out -o coverage.html

      - name: Upload Backend Coverage
        if: matrix.component == 'backend'
        uses: codecov/codecov-action@v4
        with:
          files: ./backend/coverage.out
          flags: backend

      - name: Frontend Tests
        if: matrix.component == 'frontend'
        run: |
          cd frontend
          npm ci
          npm run test:run -- --coverage

      - name: Upload Frontend Coverage
        if: matrix.component == 'frontend'
        uses: codecov/codecov-action@v4
        with:
          files: ./frontend/coverage/coverage-final.json
          flags: frontend

  # Stage 3: API Tests
  api-tests:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v4

      - name: Start Services
        run: docker compose up -d

      - name: Wait for Services
        run: |
          timeout 60 sh -c 'until docker compose ps | grep healthy; do sleep 2; done'

      - name: Run Bruno Tests
        run: |
          npm install -g @usebruno/cli
          cd bruno
          bru run --env Local 0*.bru

      - name: Stop Services
        if: always()
        run: docker compose down

  # Stage 4: E2E Tests
  e2e:
    runs-on: ubuntu-latest
    needs: api-tests
    steps:
      - uses: actions/checkout@v4

      - name: Install Playwright
        run: |
          cd frontend
          npm ci
          npx playwright install --with-deps

      - name: Start Application
        run: docker compose up -d

      - name: Wait for Application
        run: sleep 30

      - name: Run E2E Tests
        run: |
          cd frontend
          npx playwright test

      - name: Upload Test Results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-report
          path: frontend/playwright-report

  # Stage 5: Performance Tests
  performance:
    runs-on: ubuntu-latest
    needs: test
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4

      - name: Start Application
        run: docker compose up -d

      - name: Install k6
        run: |
          curl https://github.com/grafana/k6/releases/download/v0.50.0/k6-v0.50.0-linux-amd64.tar.gz -L | tar xvz
          sudo mv k6-v0.50.0-linux-amd64/k6 /usr/local/bin/

      - name: Run Load Tests
        run: k6 run loadtests/api_load_test.js

  # Stage 6: Security Scan
  security:
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4

      - name: Run gosec
        run: |
          cd backend
          go install github.com/securego/gosec/v2/cmd/gosec@latest
          gosec -fmt=json -out=security-report.json ./...

      - name: Run npm audit
        run: |
          cd frontend
          npm audit --audit-level=moderate

      - name: OWASP ZAP Scan
        uses: zaproxy/action-baseline@v0.7.0
        with:
          target: "http://localhost:3000"
```

### Branch-Specific Strategies

**Development Branch**:
- All tests run on every push
- Coverage reports generated
- E2E tests run (subset)

**Main Branch**:
- Full test suite (including long-running tests)
- Performance regression tests
- Security scans
- Visual regression tests

**Pull Requests**:
- Required: Lint + Unit + Integration tests
- Optional: E2E tests (can be triggered manually)
- Coverage must not decrease

---

## 13. Code Coverage Strategy

### Coverage Targets

**Backend (Go)**:
- **Overall**: 80% line coverage
- **Critical Paths**: 95% coverage (auth, permissions, payments)
- **Repositories**: 70% coverage
- **Services**: 85% coverage
- **API Handlers**: 75% coverage

**Frontend (TypeScript)**:
- **Overall**: 70% line coverage
- **Components**: 75% coverage
- **Hooks**: 90% coverage
- **Utilities**: 85% coverage
- **API Clients**: 80% coverage

### Coverage Collection

**Backend Coverage**:
```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View in terminal
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html

# Filter by package
go test -coverprofile=coverage.out ./services/...
go tool cover -func=coverage.out | grep -E '(total|services)'
```

**Frontend Coverage**:
```bash
# Generate coverage with Vitest
npm run test:run -- --coverage

# View report
open coverage/index.html

# Coverage thresholds in vitest.config.ts
coverage: {
  lines: 70,
  functions: 70,
  branches: 70,
  statements: 70,
  include: ['src/**/*.{ts,tsx}'],
  exclude: ['src/**/*.test.{ts,tsx}', 'src/test/**']
}
```

### Coverage Enforcement

**CI/CD Coverage Check**:
```yaml
- name: Check Coverage
  run: |
    cd backend
    go test -coverprofile=coverage.out ./...
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print substr($3, 1, length($3)-1)}')
    echo "Current coverage: $COVERAGE%"
    if (( $(echo "$COVERAGE < 80" | bc -l) )); then
      echo "Coverage below threshold (80%)"
      exit 1
    fi
```

**Coverage Badges**:
```markdown
<!-- README.md -->
![Backend Coverage](https://codecov.io/gh/org/project-phoenix/branch/main/graph/badge.svg?flag=backend)
![Frontend Coverage](https://codecov.io/gh/org/project-phoenix/branch/main/graph/badge.svg?flag=frontend)
```

---

## 14. Test Maintenance & Quality

### Flaky Test Prevention

**Common Causes & Solutions**:

**1. Time-Dependent Tests**:
```typescript
// ❌ BAD - Depends on current time
test("should expire after 1 hour", () => {
  const token = createToken();
  expect(token.expiresAt).toBe(Date.now() + 3600000);
});

// ✅ GOOD - Use mocked time
test("should expire after 1 hour", () => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2025-01-01T10:00:00Z"));

  const token = createToken();
  expect(token.expiresAt).toBe(new Date("2025-01-01T11:00:00Z"));

  vi.useRealTimers();
});
```

**2. Async Race Conditions**:
```typescript
// ❌ BAD - Race condition
test("should update list after creation", async () => {
  await createStudent();
  const list = await fetchStudents(); // Might not include new student yet
  expect(list).toHaveLength(1);
});

// ✅ GOOD - Wait for specific condition
test("should update list after creation", async () => {
  const student = await createStudent();

  await waitFor(() => {
    const list = await fetchStudents();
    expect(list.find((s) => s.id === student.id)).toBeDefined();
  });
});
```

**3. Test Isolation**:
```go
// ❌ BAD - Shared state between tests
var db *bun.DB

func TestA(t *testing.T) {
    // Uses shared db
}

func TestB(t *testing.T) {
    // Also uses shared db - can interfere with TestA
}

// ✅ GOOD - Isolated database per test
func TestA(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    // ...
}

func TestB(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    // ...
}
```

### Test Code Quality

**Test Naming Conventions**:
```go
// Format: Test<Function>_<Scenario>_<ExpectedBehavior>

func TestCreateSession_ValidInput_ReturnsSession(t *testing.T) { /* ... */ }
func TestCreateSession_RoomConflict_ReturnsError(t *testing.T) { /* ... */ }
func TestCreateSession_UnauthorizedUser_ReturnsForbidden(t *testing.T) { /* ... */ }
```

**Test Structure (AAA Pattern)**:
```go
func TestStudentService_Create(t *testing.T) {
    // Arrange - Setup test conditions
    mockRepo := new(MockStudentRepository)
    service := NewStudentService(mockRepo)
    input := &CreateStudentInput{
        PersonID: 1,
        DataRetentionDays: ptr(30),
    }

    mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

    // Act - Execute the function under test
    student, err := service.Create(context.Background(), input)

    // Assert - Verify the outcome
    require.NoError(t, err)
    assert.NotNil(t, student)
    assert.Equal(t, int64(1), student.PersonID)
    mockRepo.AssertExpectations(t)
}
```

**DRY in Tests**:
```go
// Extract common setup
func setupSessionTest(t *testing.T) (*SessionService, *MockRepository) {
    t.Helper()

    mockRepo := new(MockRepository)
    service := NewSessionService(mockRepo)

    return service, mockRepo
}

// Use in tests
func TestSessionService_Create(t *testing.T) {
    service, mockRepo := setupSessionTest(t)

    // Test logic...
}
```

### Test Maintenance Checklist

**Weekly**:
- [ ] Review flaky tests (rerun failed tests)
- [ ] Update test data if schema changed
- [ ] Check for slow tests (> 1s unit tests)

**Monthly**:
- [ ] Review code coverage trends
- [ ] Refactor duplicated test code
- [ ] Update outdated test helpers
- [ ] Remove tests for deleted features

**Per Release**:
- [ ] Run full test suite (including manual tests)
- [ ] Update E2E tests for new features
- [ ] Review and update test documentation
- [ ] Verify all critical paths are tested

---

## 15. Implementation Roadmap

### Phase 1: Foundation (Weeks 1-2)

**Goals**: Establish baseline coverage and infrastructure

**Backend**:
- [x] 48 unit tests already exist
- [ ] Add code coverage reporting to CI/CD
- [ ] Set coverage baseline (current state)
- [ ] Add 5-10 missing service tests

**Frontend**:
- [x] Vitest configured
- [ ] Add 10-15 component tests (high-value components)
- [ ] Add test scripts to CI/CD
- [ ] Setup MSW for API mocking

**API Testing**:
- [x] 115+ Bruno tests exist
- [ ] Integrate Bruno into CI/CD
- [ ] Add missing endpoint coverage

**Deliverables**:
- Coverage reports in CI/CD
- 15+ new frontend component tests
- Bruno running in GitHub Actions

---

### Phase 2: E2E Coverage (Weeks 3-4)

**Goals**: Implement critical path E2E tests

**Setup**:
- [ ] Install and configure Playwright
- [ ] Create test environment setup scripts
- [ ] Define critical user flows (5-7 flows)

**E2E Tests**:
- [ ] Authentication flow (login, logout, session)
- [ ] Session management (create, update, delete)
- [ ] Student check-in/check-out
- [ ] RFID operations
- [ ] Admin user management

**Infrastructure**:
- [ ] Add E2E tests to CI/CD
- [ ] Setup test data seeding
- [ ] Configure parallel test execution

**Deliverables**:
- 15-20 E2E test scenarios
- E2E tests running in CI
- Page Object Model for reusability

---

### Phase 3: Performance & Security (Weeks 5-6)

**Goals**: Establish performance baselines and security testing

**Performance**:
- [ ] Add Go benchmarks for critical services
- [ ] Setup k6 for API load testing
- [ ] Define performance budgets
- [ ] Add database query performance tests

**Security**:
- [ ] Integrate gosec for Go code scanning
- [ ] Add npm audit to CI/CD
- [ ] Setup OWASP ZAP baseline scan
- [ ] Conduct manual penetration testing

**Infrastructure**:
- [ ] Add performance tests to CI (main branch only)
- [ ] Setup performance regression detection
- [ ] Add security scan to PR checks

**Deliverables**:
- 10+ benchmark tests
- k6 load test suite
- Security scans in CI/CD

---

### Phase 4: Quality & Accessibility (Weeks 7-8)

**Goals**: Improve test quality and add accessibility coverage

**Accessibility**:
- [ ] Add axe-core to component tests
- [ ] Create E2E accessibility tests with Playwright
- [ ] Manual screen reader testing
- [ ] Keyboard navigation tests

**Test Quality**:
- [ ] Refactor duplicated test code
- [ ] Add test documentation
- [ ] Create test writing guide
- [ ] Setup flaky test detection

**Coverage Enforcement**:
- [ ] Set minimum coverage thresholds (backend: 80%, frontend: 70%)
- [ ] Block PRs below coverage threshold
- [ ] Add coverage trend tracking

**Deliverables**:
- WCAG Level AA compliance
- Test documentation (TESTING.md)
- Coverage enforcement in CI/CD

---

### Phase 5: Advanced Testing (Weeks 9-12)

**Goals**: Add advanced testing capabilities

**Visual Regression**:
- [ ] Setup Percy or Chromatic
- [ ] Add visual tests for key pages
- [ ] Configure responsive design tests
- [ ] Add visual tests to CI/CD

**Contract Testing**:
- [ ] Define API contracts (OpenAPI)
- [ ] Add contract tests for backend
- [ ] Add contract tests for frontend API clients
- [ ] Setup contract validation in CI

**Chaos Engineering**:
- [ ] Add resilience tests (network failures, timeouts)
- [ ] Test graceful degradation
- [ ] Add circuit breaker tests
- [ ] Test recovery scenarios

**Deliverables**:
- Visual regression testing suite
- API contract validation
- Resilience test suite

---

### Ongoing: Continuous Improvement

**Monthly**:
- [ ] Review test metrics and trends
- [ ] Identify and fix flaky tests
- [ ] Update test documentation
- [ ] Refactor test code

**Quarterly**:
- [ ] Review testing strategy
- [ ] Update coverage targets
- [ ] Evaluate new testing tools
- [ ] Conduct testing retrospective

**Annually**:
- [ ] Comprehensive testing audit
- [ ] Update testing guidelines
- [ ] Team testing training
- [ ] Benchmark against industry standards

---

## Appendix: Testing Tools Reference

### Backend Tools

| Tool | Purpose | Installation |
|------|---------|--------------|
| **Go Testing** | Unit tests | Built-in |
| **Testify** | Assertions | `go get github.com/stretchr/testify` |
| **go-sqlmock** | SQL mocking | `go get github.com/DATA-DOG/go-sqlmock` |
| **gosec** | Security scanning | `go install github.com/securego/gosec/v2/cmd/gosec@latest` |
| **gofmt** | Code formatting | Built-in |
| **golangci-lint** | Linting | `brew install golangci-lint` |

### Frontend Tools

| Tool | Purpose | Installation |
|------|---------|--------------|
| **Vitest** | Test runner | `npm install -D vitest` |
| **React Testing Library** | Component testing | `npm install -D @testing-library/react` |
| **jest-dom** | DOM matchers | `npm install -D @testing-library/jest-dom` |
| **MSW** | API mocking | `npm install -D msw` |
| **Playwright** | E2E testing | `npm install -D @playwright/test` |
| **axe-core** | Accessibility | `npm install -D @axe-core/playwright` |

### API Testing Tools

| Tool | Purpose | Installation |
|------|---------|--------------|
| **Bruno** | API testing | `npm install -g @usebruno/cli` |
| **k6** | Load testing | `brew install k6` |
| **Postman** | Manual API testing | Download from postman.com |

### DevOps Tools

| Tool | Purpose | Installation |
|------|---------|--------------|
| **Docker** | Containerization | `brew install docker` |
| **GitHub Actions** | CI/CD | Built-in to GitHub |
| **Codecov** | Coverage reporting | GitHub Action |
| **OWASP ZAP** | Security testing | Docker image |

---

## Conclusion

This testing strategy provides a comprehensive, industry-standard approach to ensuring Project Phoenix maintains high quality and reliability. By following the testing pyramid (70% unit, 20% integration, 10% E2E) and implementing the phased roadmap, the project will achieve:

- **Fast feedback loops** (<2 minutes for full suite)
- **High confidence** (80%+ coverage on critical paths)
- **Automated quality gates** (CI/CD enforcement)
- **Security & compliance** (GDPR, WCAG, OWASP)
- **Maintainable test suite** (DRY, clear structure)

**Next Steps**: Begin Phase 1 implementation focusing on frontend component tests and coverage reporting.
