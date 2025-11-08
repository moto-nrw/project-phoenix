# Bruno API Test Suite - Quick Start Guide

This guide provides high-level workflows and API endpoint examples for manual
exploration of the Project Phoenix API.

## 🚀 Quick Test Execution

The new consolidated test suite consists of 11 numbered test files that run
sequentially:

```bash
# Run all tests in order
bru run --env Local 0*.bru

# Run a single test file
bru run --env Local 05-sessions.bru

# Run tests 01-05 only
bru run --env Local 0[1-5]-*.bru
```

## 📋 Test Suite Overview

| File                | Purpose                                     | Tests | Runtime |
| ------------------- | ------------------------------------------- | ----- | ------- |
| `01-smoke.bru`      | Health checks & connectivity                | 3     | ~50ms   |
| `02-auth.bru`       | Authentication flows                        | 4     | ~100ms  |
| `03-resources.bru`  | Resource listings (groups, students, rooms) | 4     | ~80ms   |
| `04-devices.bru`    | Device-specific endpoints                   | 4     | ~75ms   |
| `05-sessions.bru`   | Session lifecycle & supervisors             | 10    | ~200ms  |
| `06-checkins.bru`   | Check-in/checkout flows                     | 8     | ~150ms  |
| `07-attendance.bru` | RFID + web attendance                       | 6     | ~120ms  |
| `08-rooms.bru`      | Room conflicts regression                   | 5     | ~110ms  |
| `09-rfid.bru`       | RFID assignment/lookup                      | 5     | ~90ms   |
| `10-schulhof.bru`   | Schulhof auto-create workflow               | 5     | ~100ms  |
| `11-claiming.bru`   | Group claiming workflow                     | 5     | ~90ms   |

**Total: 59 tests, ~1165ms estimated runtime**

## 🔐 Authentication Patterns

### Admin/Teacher Authentication (JWT)

```bash
# Login
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "Test1234%"
  }'

# Response
{
  "status": "success",
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc..."
}

# Use token in requests
curl http://localhost:8080/api/groups \
  -H "Authorization: Bearer <access_token>"
```

### Device Authentication (Two-Layer)

Device endpoints require both API key and staff PIN:

```bash
curl -X POST http://localhost:8080/api/iot/ping \
  -H "Authorization: Bearer <device_api_key>" \
  -H "X-Staff-PIN: 1234" \
  -H "Content-Type: application/json"
```

## 📚 Key API Workflows

### 1. Session Management Workflow

```bash
# 1. Start session
POST /api/iot/session/start
{
  "activity_id": 1,
  "room_id": 12,
  "supervisor_ids": [31]
}

# 2. Check current session
GET /api/iot/session/current

# 3. Update supervisors
PUT /api/iot/session/{session_id}/supervisors
{
  "supervisor_ids": [31, 32]
}

# 4. End session
POST /api/iot/session/end
```

### 2. Check-in/Checkout Workflow

```bash
# Check-in student to room
POST /api/iot/checkin
{
  "student_rfid": "AD95A48E",
  "room_id": 3,
  "action": "checkin"
}

# Checkout student
POST /api/iot/checkin
{
  "student_rfid": "AD95A48E",
  "action": "checkout"
}
```

### 3. Schulhof Auto-Create Workflow

```bash
# First student creates Schulhof group automatically
POST /api/iot/checkin
{
  "student_rfid": "AD95A48E",
  "room_id": 25,  # Schulhof room
  "action": "checkin"
}
# Response includes auto-created active_group_id

# Second student reuses the same group
POST /api/iot/checkin
{
  "student_rfid": "DEADBEEF12345678",
  "room_id": 25,
  "action": "checkin"
}
# Response includes same active_group_id
```

### 4. Group Claiming Workflow

```bash
# 1. List unclaimed groups
GET /api/active/groups/unclaimed

# 2. Claim a group as supervisor
POST /api/active/groups/{group_id}/claim
{
  "role": "supervisor"
}

# 3. Verify claim
GET /api/active/groups/{group_id}
```

## 🧪 Testing with Environment Overrides

### Daily Checkout Time Testing

The `dailyCheckoutMode` environment variable controls time-dependent checkout
behavior:

```bash
# In Local.bru, set:
dailyCheckoutMode: after_hours   # Forces "checked_out_daily" action
# OR
dailyCheckoutMode: before_hours  # Forces normal "checked_out" action
# OR
dailyCheckoutMode:              # Empty = accept either outcome
```

This eliminates time-of-day test instability.

## 🔍 Common API Endpoints

### Resource Listings

```bash
GET /api/groups              # Educational groups
GET /api/students            # Students with RFID tags
GET /api/rooms               # Available rooms
GET /api/activities          # Activities (requires teacher token)
```

### Device Endpoints

```bash
GET /api/iot/rooms/available              # Available rooms (no filters)
GET /api/iot/rooms/available?capacity=100 # Capacity-filtered rooms
GET /api/iot/teachers                     # Teachers list
GET /api/iot/rfid/{tag}                   # RFID lookup
POST /api/iot/rfid/assign                 # Assign RFID to student
```

### Attendance

```bash
# RFID-based
POST /api/iot/attendance/toggle
{
  "student_rfid": "AD95A48E",
  "action": "confirm"  # or "cancel"
}

# Web-based
POST /api/active/attendance/check-in
{
  "student_id": 1
}

GET /api/active/attendance/present    # List present students
POST /api/active/attendance/check-out/{id}
```

## 🛠️ Troubleshooting

### Common Issues

1. **Authentication failures**: Ensure tokens are fresh (15min expiry for access
   tokens)
2. **Device auth failures**: Verify both API key AND PIN are correct
3. **Room conflicts**: Check for existing active groups in the room
4. **RFID not found**: Ensure RFID tag exists in database with correct student
   mapping

### Test Data Requirements

All tests assume seed data is populated:

- **Admin account**: admin@example.com / Test1234%
- **Teacher account**: andreas.arndt@schulzentrum.de / Test1234%
- **Students**: 3+ with RFID tags (configured in Local.bru)
- **Rooms**: 24+ rooms including room 25 (Schulhof)
- **Activities**: At least 2 activities for session testing

### Debugging Failed Tests

1. Check backend logs: `docker compose logs -f server`
2. Verify database state: Connect to PostgreSQL and check tables
3. Run individual test file for isolation: `bru run --env Local 05-sessions.bru`
4. Check environment variables in `bruno/environments/Local.bru`

## 📝 Response Format

All API responses follow this structure:

```json
{
  "status": "success", // or "error"
  "message": "Human-readable message",
  "data": {
    // Actual response payload
  }
}
```

## 🧹 Cleanup After Testing

The test suite includes automatic cleanup:

- Sessions ended after lifecycle tests
- Students checked out after check-in tests
- Test groups deleted after room conflict tests
- RFID assignments removed after assignment tests

If cleanup fails, manually reset via:

```bash
# Checkout all students
# End active sessions
# Delete test groups
```

## 🔗 Additional Resources

- **API Documentation**: `backend/docs/routes.md` (generated via
  `go run main.go gendoc`)
- **OpenAPI Spec**: `backend/docs/openapi.yaml`
- **Database Schema**: `backend/database/migrations/`
- **RFID Integration**: `/RFID_IMPLEMENTATION_GUIDE.md`

---

**Note**: This is a documentation-only guide. All executable tests are in the
numbered `.bru` files (01-11).
