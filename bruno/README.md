# Project Phoenix Bruno API Tests

Consolidated, deterministic API test suite for Project Phoenix using [Bruno](https://usebruno.com/).

## 🎯 Design Principles

- **Hermetic Tests**: Each test file is self-contained, sets up its own state, and performs cleanup
- **No External Dependencies**: Pure Bruno CLI execution, no shell script orchestration
- **Deterministic**: Environment-driven assertions eliminate time-dependent brittleness
- **Comprehensive Coverage**: 59 tests across 11 domains covering all critical API workflows

## 🚀 Quick Start

### Prerequisites

```bash
# Install Bruno CLI
brew install bruno-cli
# OR
pnpm add -g @usebruno/cli

# Install jq (optional, for manual token inspection)
brew install jq

# Ensure backend is running
docker compose up -d
```

### Running Tests

```bash
cd bruno

# Run all tests (recommended)
bru run --env Local 0*.bru

# Run a single test file
bru run --env Local 05-sessions.bru

# Run specific range (tests 01-05)
bru run --env Local 0[1-5]-*.bru
```

## 📁 Test Suite Structure

### Consolidated Test Files

| File | Purpose | Tests | Coverage |
|------|---------|-------|----------|
| **00-cleanup.bru** | Pre-test cleanup | 1 | Ends all active sessions before testing |
| **01-smoke.bru** | Health checks & connectivity | 3 | Admin auth, groups API, device ping |
| **02-auth.bru** | Authentication flows | 4 | Admin login, token refresh, teacher auth, device auth |
| **03-resources.bru** | Resource listings | 4 | Groups, students, rooms, activities |
| **04-devices.bru** | Device endpoints | 4 | Available rooms, capacity filters, teachers, activities |
| **05-sessions.bru** | Session lifecycle | 10 | Start, conflict, current, supervisors, end |
| **06-checkins.bru** | Check-in/out flows | 8 | Happy path, errors, capacity, multi-student, cleanup |
| **07-attendance.bru** | RFID + web attendance | 6 | RFID toggle, web check-in/out, present list |
| **08-rooms.bru** | Room conflicts regression | 5 | Create, conflict, self-exclusion, occupied move |
| **09-rfid.bru** | RFID assignment | 5 | Lookup, errors, assignment, validation, cleanup |
| **10-schulhof.bru** | Schulhof auto-create | 5 | Auto-create, reuse, checkout, verification |
| **11-claiming.bru** | Group claiming | 5 | List unclaimed, claim, verify, duplicate, cleanup |

**Total**: 60 tests, ~340ms actual runtime (includes automatic cleanup)

### Directory Structure

```
bruno/
├── 00-cleanup.bru            # Pre-test cleanup (runs first)
├── 01-smoke.bru              # Health checks
├── 02-auth.bru               # Authentication
├── 03-resources.bru          # Resource listings
├── 04-devices.bru            # Device endpoints
├── 05-sessions.bru           # Session lifecycle
├── 06-checkins.bru           # Check-in/out flows
├── 07-attendance.bru         # Attendance (RFID + web)
├── 08-rooms.bru              # Room conflicts
├── 09-rfid.bru               # RFID assignment
├── 10-schulhof.bru           # Schulhof workflow
├── 11-claiming.bru           # Group claiming
├── environments/
│   └── Local.bru             # Environment configuration
├── examples/
│   └── quick-start.md        # API workflow documentation
└── bruno.json                # Collection metadata
```

## 🔧 Environment Configuration

### Setup Environment File

Environment variables are configured in `bruno/environments/Local.bru`:

```bruno
vars {
  baseUrl: http://localhost:8080
  accessToken:                          # Auto-populated by tests
  refreshToken:                         # Auto-populated by tests
  deviceApiKey: 9YUQWdt4dLa013foUTRKdnaeEUPBsWj7
  staffPIN: 1234
  devicePin: 1234                       # For device authentication
  globalOgsPin: 1234
  staffID: 1
  testStaffEmail: andreas.krueger@example.com
  testStaffPassword: Test1234%
  testStudent1RFID: AD95A48E
  testStudent1Name: Leon Lang
  testStudent2RFID: DEADBEEF12345678
  testStudent2Name: Emma Horn
  testStudent3RFID: 89D72485
  testStudent3Name: Paul Brandt
  dailyCheckoutMode:                    # See "Time-Dependent Testing" below
}
```

### Time-Dependent Testing

The `dailyCheckoutMode` variable controls checkout behavior for deterministic testing:

```bruno
dailyCheckoutMode: after_hours   # Forces "checked_out_daily" action
# OR
dailyCheckoutMode: before_hours  # Forces normal "checked_out" action
# OR
dailyCheckoutMode:              # Empty = accept either outcome (default)
```

This eliminates test brittleness from time-of-day dependent logic.

## 🔐 Authentication

### Two Authentication Patterns

**1. Admin/Teacher Endpoints (JWT Bearer)**
```bruno
headers {
  Authorization: Bearer {{accessToken}}
}
```
- Access tokens auto-populated by pre-request scripts
- 15-minute expiry, automatically refreshed when needed
- Used for: /api/groups, /api/students, /api/active/*

**2. Device Endpoints (Two-Layer Auth)**
```bruno
headers {
  Authorization: Bearer {{deviceApiKey}}
  X-Staff-PIN: {{devicePin}}
}
```
- Requires both device API key AND staff PIN
- Used for: /api/iot/*

### Test Accounts

```
Admin:   admin@example.com / Test1234%
Teacher: andreas.krueger@example.com / Test1234% (PIN: 1234)
Device:  API Key in environment + PIN 1234
```

## ✅ Test Execution Flow

### Self-Contained Test Pattern

Each `.bru` file follows this pattern:

1. **Pre-request Script**: Ensures authentication tokens exist
2. **Main Request**: Primary test endpoint
3. **Post-response Script**: Additional test requests + assertions
4. **Cleanup**: Removes any created state to prevent leakage

Example from `05-sessions.bru`:
```javascript
// Pre-request: Auto-login if no token
if (!bru.getEnvVar("accessToken")) {
  // Perform admin login...
}

// Main request: Start session
POST /api/iot/session/start { ... }

// Post-response: Additional tests + cleanup
async function() {
  // Test conflict scenarios
  // Test supervisor updates
  // Test session end (cleanup)
}
```

### Parallel vs Sequential Execution

- **Within a file**: Async script requests run in parallel where possible
- **Across files**: Bruno executes files sequentially (01 → 02 → ... → 11)
- **No dependencies**: Each file is independent and can run standalone

## 🧪 Key Testing Workflows

### Session Lifecycle (05-sessions.bru)

1. ✅ Start session with activity + room + supervisors
2. ✅ Conflict detection (duplicate room)
3. ✅ Current session verification
4. ✅ Update supervisors (add, remove, deduplicate)
5. ✅ Validation (empty list, invalid IDs)
6. ✅ Cleanup (session end)

### Check-in/Checkout Flow (06-checkins.bru)

1. ✅ Check-in student to room
2. ✅ Checkout student
3. ✅ Error handling (missing room, invalid RFID)
4. ✅ Multi-student check-ins
5. ✅ Automatic cleanup (checkout all test students)

### Schulhof Auto-Create (10-schulhof.bru)

1. ✅ First student → auto-creates "Schulhof" group
2. ✅ Second student → reuses same group
3. ✅ Both students checkout cleanly

## 🛠️ Troubleshooting

### Common Issues

**Tests fail with authentication errors:**
- Ensure backend is running: `docker compose ps`
- Verify admin account exists: `admin@example.com / Test1234%`
- Check Local.bru has correct credentials

**All tests consistently pass** thanks to 00-cleanup.bru:
- Automatically ends active sessions before each test run
- No manual cleanup required
- Ensures reproducible test results

**Device auth failures:**
- Verify `deviceApiKey` matches database: `SELECT api_key FROM iot.devices;`
- Ensure `devicePin` and `staffPIN` are correct (default: 1234)

**Time-dependent failures (checkout actions):**
- Set `dailyCheckoutMode` in Local.bru to force specific behavior
- Or leave empty to accept either outcome

### Test Isolation

To debug a specific test in isolation:

```bash
# Run single test file
bru run --env Local 06-checkins.bru

# Check backend logs
docker compose logs -f server

# Inspect database state
docker compose exec postgres psql -U your_user -d project_phoenix
```

## 📊 Test Coverage

### API Coverage Map

**Authentication** → 02-auth.bru
**Resources** → 03-resources.bru (groups, students, rooms, activities)
**Sessions** → 05-sessions.bru (start, update, end)
**Check-ins** → 06-checkins.bru (RFID check-in/out)
**Attendance** → 07-attendance.bru (RFID toggle + web dashboard)
**RFID** → 09-rfid.bru (lookup, assignment, validation)
**Room Conflicts** → 08-rooms.bru (Issue #3 regression)
**Schulhof** → 10-schulhof.bru (auto-create + reuse)
**Claiming** → 11-claiming.bru (deviceless group supervision)

### Seed Data Requirements

Tests assume the following seed data exists:

- **Admin account**: admin@example.com / Test1234%
- **Teacher account**: First teacher from seed (Staff ID: 1)
- **Students**: 50+ with RFID tags (3 configured in Local.bru)
- **Rooms**: 25 rooms (including room 12 for sessions, room 25 for Schulhof)
- **Activities**: 20+ activities for session testing
- **Device**: API key in iot.devices table

To repopulate seed data:
```bash
cd backend
go run main.go seed --reset
```

**Automatic Cleanup**: The test suite includes `00-cleanup.bru` which automatically ends all active sessions before running tests. This ensures reliable, repeatable test execution without manual intervention.

### ⚠️ IMPORTANT: After Database Reset

The seed data uses deterministic random generation (seed: 42), but **values still change between resets** because:
- Random last name selection from pool
- Random RFID byte generation
- Random device API key generation

**After each `seed --reset`, you MUST update `environments/Local.bru`:**

```bash
# 1. Get device API key
docker compose exec -T postgres psql -U postgres -d postgres -c \
  "SELECT device_id, api_key FROM iot.devices WHERE device_id = 'RFID-MAIN-001';"

# 2. Get first teacher email
docker compose exec -T postgres psql -U postgres -d postgres -c \
  "SELECT a.email FROM auth.accounts a
   JOIN users.persons p ON p.account_id = a.id
   JOIN users.staff s ON s.person_id = p.id
   JOIN users.teachers t ON t.staff_id = s.id
   ORDER BY t.id LIMIT 1;"

# 3. Get first 3 student RFID tags and names
docker compose exec -T postgres psql -U postgres -d postgres -c \
  "SELECT p.first_name, p.last_name, p.tag_id
   FROM users.persons p
   JOIN users.students s ON s.person_id = p.id
   ORDER BY s.id LIMIT 3;"

# 4. Update environments/Local.bru with these values:
# - deviceApiKey: <from step 1>
# - testStaffEmail: <from step 2>
# - testStudent1RFID, testStudent2RFID, testStudent3RFID: <from step 3>
# - testStudent1Name, testStudent2Name, testStudent3Name: <from step 3>
```

**⚠️ SECURITY WARNING: TEST DATA ONLY ⚠️**

**These values are for local development/testing ONLY. They are generated from deterministic seed data (seed: 42) and are NOT secure for production use. DO NOT copy these values to production environments!**

**Current test values (with seed 42, 4-byte RFIDs only):**
```
deviceApiKey: ejpSOD5EEyMtbgsWBFNEoPU8MX0z553E (TEST ONLY - DO NOT USE IN PRODUCTION)
testStaffEmail: andreas.krueger@example.com
testStudent1RFID: E83BE72F (Leon Huber)
testStudent2RFID: CA5DE789 (Emma Schreiber)
testStudent3RFID: 43385429 (Ben Sauer)
```

## 🔗 Additional Resources

- **API Examples**: `bruno/examples/quick-start.md` - High-level workflow documentation
- **API Routes**: `backend/docs/routes.md` - Generated route documentation
- **OpenAPI Spec**: `backend/docs/openapi.yaml` - Machine-readable API spec
- **RFID Guide**: `/RFID_IMPLEMENTATION_GUIDE.md` - Device integration details

## 🎯 Design Rationale

### Why This Structure?

**Problem**: Previous structure had 58+ scattered `.bru` files, brittle shell scripts, time-dependent assertions, and manual state management.

**Solution**: 11 consolidated test files following these principles:

1. **Hermetic Testing**: Each file manages its own state and cleanup
2. **No Shell Scripts**: Pure Bruno CLI, no external orchestration
3. **Environment Overrides**: `dailyCheckoutMode` replaces time-dependent logic
4. **Intentional Duplication**: Bruno lacks includes, so auth scripts are duplicated per file for self-containment

### Bruno Limitations

- **No shared scripts**: Pre-request logic duplicated across files (intentional)
- **Single request per file**: Additional requests made via post-response scripts
- **No test dependencies**: Files must be independent (enforced by design)
- **Async limitations**: Some script requests may not wait properly (logged as warnings)

## 📝 Contributing

### Adding New Tests

1. Create new numbered file: `12-new-feature.bru`
2. Follow existing patterns (pre-request auth, post-response tests, cleanup)
3. Document in `docs` section: prerequisites, coverage, cleanup strategy
4. Update this README with test count and coverage

### Modifying Existing Tests

1. Preserve hermetic principle: state setup + cleanup within same file
2. Update environment variables if needed: `environments/Local.bru`
3. Ensure backward compatibility: don't break other tests
4. Test isolation: verify test passes standalone

---

**Clean. Consolidated. Deterministic.**

62 files → 12 files (81% reduction) | 8 shell scripts → Pure Bruno CLI | Time-dependent → Environment-driven | Automatic cleanup | ~340ms execution
