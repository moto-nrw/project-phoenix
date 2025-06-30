# Device Multi-Supervisor Implementation Plan

## Overview
This document tracks the implementation of multiple supervisor support for RFID device sessions. Previously, devices authenticated with individual staff PINs and sessions had single supervisors. The new system uses a global OGS PIN and supports multiple supervisors per session.

**Branch**: `feature/global-ogs-pin`
**Related PR**: #219 (Multiple educational group supervisors - merged)

---

## Current Status

### ✅ Completed (All Objectives Done!)
- [x] Global PIN authentication implemented (Objective 1)
- [x] Device authentication without staff context (Objective 1)
- [x] Teacher list endpoint for supervisor selection (Objective 2)
- [x] Session creation with multiple supervisors (Objective 3)
- [x] Dynamic supervisor management via PUT endpoint (Objectives 5 & 6)
- [x] Business rules implemented and verified (Objective 7)
- [x] Multiple supervisor database schema (from PR #219)
- [x] Comprehensive Bruno test suite

### ❌ Not Needed
- Backward compatibility (Objective 4) - Devices will be updated

---

## Objectives & Test Criteria

### Objective 1: Global PIN Authentication ✅
**Status**: COMPLETE - Implemented and tested
**Completed**: 2025-06-30

**Implementation**:
- Modified `DeviceAuthenticator` middleware to use `OGS_DEVICE_PIN` environment variable
- Removed staff context requirement from device authentication
- Updated all IoT handlers to work without staff context

**Test Results**:
- ✅ Device ping endpoint works with global PIN
- ✅ Bruno test `dev/device-auth.bru` passes
- ✅ Environment variable properly configured in all .env files

---

### Objective 2: Fetch Available Teachers ✅
**Status**: COMPLETE - Endpoint working correctly
**Completed**: 2025-06-30

**Endpoint**: `GET /api/iot/teachers`

**Implementation**:
- Removed PIN check filter from `getAvailableTeachers` function
- Endpoint now returns all teachers regardless of PIN status
- Uses `DeviceOnlyAuthenticator` (no PIN required, only API key)

**Test Results**:
- ✅ Returns all 20 teachers in test database
- ✅ Works with device API key only (no PIN needed)
- ✅ Returns staff_id, person_id, first_name, last_name, display_name
- ✅ Bruno test created and passing

**Testing Checklist**:
- [x] Create Bruno test file `dev/device-teachers-list.bru`
- [x] Verify response includes all teachers (20 returned)
- [x] Confirm no staff context required

---

### Objective 3: Start Session with Multiple Supervisors ✅
**Status**: COMPLETE - Multi-supervisor session creation implemented  
**Completed**: 2025-06-30

**API Design**:
```json
POST /api/iot/session/start
{
  "activity_id": 123,
  "room_id": 456,              // optional
  "supervisor_ids": [1, 2, 3], // REQUIRED: array of staff IDs
  "force": false
}
```

**Response includes supervisors**:
```json
{
  "active_group_id": 789,
  "activity_id": 123,
  "device_id": 456,
  "start_time": "2025-06-30T15:30:00Z",
  "supervisors": [
    {
      "staff_id": 1,
      "first_name": "Ben",
      "last_name": "Klein",
      "display_name": "Ben Klein",
      "role": "supervisor"
    },
    {
      "staff_id": 2,
      "first_name": "Julian",
      "last_name": "Müller",
      "display_name": "Julian Müller",
      "role": "supervisor"
    }
  ],
  "status": "started",
  "message": "Activity session started successfully"
}
```

**Implementation Details**:
- ✅ **NO BACKWARD COMPATIBILITY** - `supervisor_ids` is REQUIRED (returns 400 if missing)
- ✅ Added service methods: `StartActivitySessionWithSupervisors` and `ForceStartActivitySessionWithSupervisors`
- ✅ Implemented supervisor validation in `validateSupervisorIDs` helper
- ✅ Fixed `FindWithPerson` in staff repository to load person data
- ✅ Supervisor deduplication handled automatically
- ✅ All operations wrapped in database transaction

**Key Implementation Challenges Solved**:
1. **BUN ORM Person Loading**: Fixed "relation people does not exist" error by implementing proper query in `FindWithPerson`
2. **Response Enhancement**: Added `SupervisorInfo` struct to include staff details in response
3. **Data Integrity**: All supervisor assignments verified in database with proper foreign keys

**Database Verification**:
- Groups created with correct activity_id, device_id, room_id
- All supervisors properly assigned with role="supervisor"
- No orphaned records or duplicates
- Staff-Person relationships intact

**Test Results**:
- [x] Single supervisor: ✅ Works
- [x] Multiple supervisors [1,2,3]: ✅ All assigned correctly
- [x] Different supervisor sets [4,5]: ✅ Works with any valid staff
- [x] Empty array []: ✅ Returns 400 error as expected
- [x] Missing supervisor_ids field: ✅ Returns 400 error (no backward compatibility)

---

### Objective 4: Backward Compatibility ❌
**Status**: NOT NEEDED - Per user decision: "we do not need backwards compatibility as the device will also change!!"
**Decision Date**: 2025-06-30

**Rationale**: 
- Devices will be updated to use the new multi-supervisor API
- No need to maintain backward compatibility
- `supervisor_ids` is required in all session start requests
- This is an intentional breaking change

---

### Objectives 5 & 6: Update Supervisors for Active Session ✅
**Status**: COMPLETE - Merged into single PUT endpoint
**Completed**: 2025-06-30

**API Design** (Final Implementation):
```json
PUT /api/iot/session/{session_id}/supervisors
{
  "supervisor_ids": [1, 4, 5]  // Complete new list of supervisors
}
```

**Implementation Details**:
- ✅ Single PUT endpoint replaces entire supervisor list (RESTful best practice)
- ✅ Validates session exists and is active
- ✅ Validates all supervisor IDs are valid staff members
- ✅ Atomic transaction for all changes
- ✅ Handles unique constraint by reactivating ended supervisors
- ✅ Automatic deduplication of supervisor IDs
- ✅ Returns updated supervisor list with full details

**Service Method**: `UpdateActiveGroupSupervisors(ctx, activeGroupID, supervisorIDs)`

**Key Implementation Challenges Solved**:
1. **Schema-qualified tables**: Fixed repository Update method to use proper ModelTableExpr
2. **Unique constraint handling**: Reactivates existing supervisors instead of creating duplicates
3. **BUN ORM relations**: Manually load supervisors to avoid schema issues with relations

**Test Results**:
- [x] Update supervisors (normal case): ✅ Works
- [x] Empty supervisor list: ✅ Returns 400 error "at least one supervisor is required"
- [x] Invalid supervisor ID: ✅ Returns error "staff member with ID 999999 not found"
- [x] Duplicate IDs: ✅ Automatically deduplicated
- [x] Non-existent session: ✅ Returns error

**Bruno Tests Created**:
- `dev/device-supervisor-update.bru` - Main update test
- `dev/device-supervisor-update-edge.bru` - Edge case tests
- `dev/device-supervisor-update-invalid.bru` - Invalid session test
- `bruno/test-supervisor-update.sh` - Automated test script

---

### Objective 7: Session Management Rules ✅
**Status**: COMPLETE - All actual requirements already implemented
**Completed**: 2025-06-30

**Business Rules Implemented**:
- [x] Cannot start session with 0 supervisors - Validated in session start and supervisor update
- [x] "Springerkraft" can supervise multiple rooms simultaneously - Already supported
- [x] Session lifecycle controlled by end session API - No automatic termination

**Implementation Notes**:
- Minimum 1 supervisor enforced in both session start and PUT update endpoint
- Staff can be assigned to multiple active sessions (no restrictions)
- Sessions only end via explicit API call to `/api/iot/session/end`
- No requirement for automatic session termination when supervisors removed

**What Was NOT Required**:
- ❌ "Session remains active if at least 1 supervisor remains" - Never requested
- ❌ "Session continues even if all supervisors removed" - Conflicts with min supervisor rule

**Verification**:
- Starting session with 0 supervisors returns 400 error ✅
- Updating to 0 supervisors returns 400 error ✅
- One staff member can supervise multiple rooms ✅
- Sessions persist until explicitly ended ✅

---

## Implementation Timeline

### Phase 1: Core Implementation (Completed)
- [x] Global PIN authentication
- [x] Verify teacher list endpoint
- [x] Implement multi-supervisor session start
- [x] ~~Ensure backward compatibility~~ (Not needed - devices will update)

### Phase 2: Dynamic Management (Completed)
- [x] ~~Add supervisor endpoint~~ → Merged into PUT endpoint
- [x] ~~Remove supervisor endpoint~~ → Merged into PUT endpoint
- [x] Update supervisors endpoint (PUT)
- [x] Edge case handling

### Phase 3: Integration & Testing (Completed)
- [x] Complete test suite (Bruno tests created)
- [x] Business rules verified
- [x] Documentation updated

---

## Testing Matrix

| Scenario | O1 | O2 | O3 | O4 | O5 | O6 | O7 |
|----------|----|----|----|----|----|----|----|
| Device auth with global PIN | ✅ | | | | | | |
| List all teachers | | ✅ | | | | | |
| Start with 1 supervisor | | | ✅ | | | | |
| Start with 3 supervisors | | | ✅ | | | | |
| Old API still works | | | | ❌ | | | |
| Update supervisors (PUT) | | | | | ✅ | | |
| Empty supervisor validation | | | | | ✅ | | |
| Invalid supervisor validation | | | | | ✅ | | |
| Duplicate handling | | | | | ✅ | | |
| Multiple rooms per person | | | | | | | ✅ |

Legend: ✅ Complete | ⏳ Pending | ❌ Not Needed/Skipped

---

## Code Locations

### Backend Files Modified:
- `backend/auth/device/device_auth.go` - Global PIN authentication
- `backend/api/iot/api.go` - Device endpoints, multi-supervisor handling, PUT update endpoint
- `backend/services/active/interface.go` - Added multi-supervisor service methods
- `backend/services/active/active_service.go` - Implemented multi-supervisor logic and update method
- `backend/database/repositories/active/group_supervisor.go` - Fixed Update method for schema-qualified tables
- `backend/database/repositories/active/group.go` - Fixed FindWithSupervisors for manual loading
- `backend/database/repositories/users/staff.go` - FindWithPerson implementation

### Environment Files:
- `.env.example` - Added OGS_DEVICE_PIN
- `backend/dev.env.example` - Added OGS_DEVICE_PIN
- `docker-compose.yml` - Added OGS_DEVICE_PIN to server environment
- `docker-compose.example.yml` - Added OGS_DEVICE_PIN

### Database Tables (from PR #219):
- `active.groups` - Activity sessions
- `active.group_supervisors` - Supervisor assignments (many-to-many)

### Bruno Test Files:
- `bruno/dev/device-teachers-list.bru` - Teacher list endpoint test
- `bruno/dev/device-session-start-multi.bru` - Multi-supervisor session test
- `bruno/dev/device-session-start-multi-edge.bru` - Edge case tests
- `bruno/dev/device-supervisor-update.bru` - Supervisor update endpoint test
- `bruno/dev/device-supervisor-update-edge.bru` - Update edge case tests
- `bruno/dev/device-supervisor-update-invalid.bru` - Invalid session test
- `bruno/test-supervisor-update.sh` - Automated test script for supervisor updates

---

## Notes & Decisions

1. **Global PIN Choice**: Decided to use environment variable `OGS_DEVICE_PIN` for simplicity
2. **No Staff Tracking**: Device actions no longer tracked to specific staff members
3. **Session Persistence**: Sessions continue even without supervisors (per requirements)
4. **Springerkraft Support**: One person can supervise multiple rooms simultaneously

---

## Implementation Complete! 🎉

All objectives have been successfully implemented:
1. ✅ Global PIN authentication 
2. ✅ Teacher list endpoint
3. ✅ Multi-supervisor session creation
4. ❌ Backward compatibility (not needed)
5. ✅ Dynamic supervisor management (merged with 6)
6. ✅ Dynamic supervisor management (merged with 5)
7. ✅ Session management rules

The multi-supervisor RFID device implementation is now ready for deployment.

---

## API Reference for Device Implementation

### Authentication Headers
All device endpoints require these headers:
- `Authorization: Bearer {device_api_key}` - Device API key
- `X-Staff-PIN: {global_pin}` - Global OGS PIN (currently: 1234)

### 1. Device Authentication Check
**Endpoint:** `POST /api/iot/ping`

**Purpose:** Verify device is authenticated and online

**Headers:**
```
Authorization: Bearer {device_api_key}
X-Staff-PIN: {global_pin}
```

**Request Body:** None

**Response (200 OK):**
```json
{
  "status": "success",
  "data": {
    "device_id": "test",
    "device_name": "test",
    "is_online": true,
    "last_seen": "2025-06-30T16:00:55.446617302+02:00",
    "ping_time": "2025-06-30T16:00:55.448689761+02:00",
    "status": "active"
  },
  "message": "Device ping successful"
}
```

### 2. Get Available Teachers
**Endpoint:** `GET /api/iot/teachers`

**Purpose:** Retrieve list of all teachers for supervisor selection

**Headers:**
```
Authorization: Bearer {device_api_key}
```

**Request Body:** None

**Response (200 OK):**
```json
{
  "status": "success",
  "data": [
    {
      "staff_id": 1,
      "person_id": 1,
      "first_name": "Ben",
      "last_name": "Klein",
      "display_name": "Ben Klein"
    },
    {
      "staff_id": 2,
      "person_id": 2,
      "first_name": "Julian",
      "last_name": "Müller",
      "display_name": "Julian Müller"
    }
    // ... more teachers
  ],
  "message": "Teachers retrieved successfully"
}
```

### 3. Start Session with Multiple Supervisors
**Endpoint:** `POST /api/iot/session/start`

**Purpose:** Start a new activity session with multiple supervisors

**Headers:**
```
Authorization: Bearer {device_api_key}
X-Staff-PIN: {global_pin}
Content-Type: application/json
```

**Request Body:**
```json
{
  "activity_id": 1,                    // Required: Activity ID
  "room_id": 1,                       // Optional: Room ID (can be null)
  "supervisor_ids": [1, 2, 3],        // Required: Array of staff IDs (min 1)
  "force": false                      // Optional: Force start even if conflicts
}
```

**Response (200 OK):**
```json
{
  "status": "success",
  "data": {
    "active_group_id": 43,
    "activity_id": 1,
    "device_id": 8,
    "start_time": "2025-06-30T16:02:06.101830585+02:00",
    "supervisors": [
      {
        "staff_id": 1,
        "first_name": "Ben",
        "last_name": "Klein",
        "display_name": "Ben Klein",
        "role": "supervisor"
      },
      {
        "staff_id": 2,
        "first_name": "Julian",
        "last_name": "Müller",
        "display_name": "Julian Müller",
        "role": "supervisor"
      }
    ],
    "status": "started",
    "message": "Activity session started successfully"
  },
  "message": "Activity session started successfully"
}
```

**Error Response (400):**
```json
{
  "status": "error",
  "error": "supervisor_ids is required and must contain at least one supervisor"
}
```

### 4. Update Session Supervisors
**Endpoint:** `PUT /api/iot/session/{active_group_id}/supervisors`

**Purpose:** Replace entire supervisor list for an active session

**Headers:**
```
Authorization: Bearer {device_api_key}
X-Staff-PIN: {global_pin}
Content-Type: application/json
```

**Request Body:**
```json
{
  "supervisor_ids": [2, 4, 5]         // Required: New complete list (min 1)
}
```

**Response (200 OK):**
```json
{
  "status": "success",
  "data": {
    "active_group_id": 43,
    "supervisors": [
      {
        "staff_id": 2,
        "first_name": "Julian",
        "last_name": "Müller",
        "display_name": "Julian Müller",
        "role": "supervisor"
      },
      {
        "staff_id": 4,
        "first_name": "Mia",
        "last_name": "Werner",
        "display_name": "Mia Werner",
        "role": "supervisor"
      },
      {
        "staff_id": 5,
        "first_name": "Amelie",
        "last_name": "Schulze",
        "display_name": "Amelie Schulze",
        "role": "supervisor"
      }
    ],
    "status": "success",
    "message": "Supervisors updated successfully"
  },
  "message": "Supervisors updated successfully"
}
```

**Error Responses:**
- 400: "at least one supervisor is required"
- 400: "supervisor_ids must be an array"
- 404: "active group not found"
- 400: "staff member with ID {id} not found"

### 5. Student Check-in/Check-out
**Endpoint:** `POST /api/iot/checkin`

**Purpose:** Check student in or out using RFID tag

**Headers:**
```
Authorization: Bearer {device_api_key}
X-Staff-PIN: {global_pin}
Content-Type: application/json
```

**Request Body (Check-in):**
```json
{
  "student_rfid": "0717E589DBE0C0",   // Required: RFID tag number
  "action": "checkin",                // Required: "checkin" or "checkout"
  "room_id": 1                        // Required for checkin only
}
```

**Request Body (Check-out):**
```json
{
  "student_rfid": "0717E589DBE0C0",   // Required: RFID tag number
  "action": "checkout"                // Required: "checkin" or "checkout"
}
```

**Response (200 OK - Check-in):**
```json
{
  "status": "success",
  "data": {
    "action": "checked_in",
    "message": "Hallo Paula!",
    "processed_at": "2025-06-30T16:04:48.537708216+02:00",
    "room_name": "101",
    "status": "success",
    "student_id": 1,
    "student_name": "Paula Vogel",
    "visit_id": 1
  },
  "message": "Student checked_in successfully"
}
```

**Response (200 OK - Check-out):**
```json
{
  "status": "success",
  "data": {
    "action": "checked_out",
    "message": "Tschüss Paula!",
    "processed_at": "2025-06-30T16:05:04.27045275+02:00",
    "room_name": "",
    "status": "success",
    "student_id": 1,
    "student_name": "Paula Vogel",
    "visit_id": 1
  },
  "message": "Student checked_out successfully"
}
```

### 6. Get Current Session
**Endpoint:** `GET /api/iot/session/current`

**Purpose:** Get details of the current active session for this device

**Headers:**
```
Authorization: Bearer {device_api_key}
X-Staff-PIN: {global_pin}
```

**Response (200 OK):**
```json
{
  "status": "success",
  "data": {
    "active_group_id": 43,
    "activity_id": 1,
    "activity_name": "Hausaufgabenbetreuung",
    "room_id": 1,
    "room_name": "101",
    "start_time": "2025-06-30T16:02:06.101830585+02:00",
    "device_id": 8,
    "supervisors": [
      {
        "staff_id": 1,
        "first_name": "Ben",
        "last_name": "Klein",
        "role": "supervisor"
      }
    ]
  },
  "message": "Current session retrieved"
}
```

**Response (404 - No Session):**
```json
{
  "status": "error",
  "error": "no active session found for this device"
}
```

### 7. End Session
**Endpoint:** `POST /api/iot/session/end`

**Purpose:** End the current active session

**Headers:**
```
Authorization: Bearer {device_api_key}
X-Staff-PIN: {global_pin}
```

**Request Body:** None

**Response (200 OK):**
```json
{
  "status": "success",
  "data": {
    "active_group_id": 43,
    "activity_id": 1,
    "device_id": 8,
    "duration": "4m13.95581105s",
    "ended_at": "2025-06-30T16:06:20.057640633+02:00",
    "message": "Activity session ended successfully",
    "status": "ended"
  },
  "message": "Activity session ended successfully"
}
```

**Response (400 - No Session):**
```json
{
  "status": "error",
  "error": "no active session found for this device"
}
```

### Important Implementation Notes

1. **Breaking Change**: `supervisor_ids` is now REQUIRED when starting sessions (no backward compatibility)

2. **Supervisor Management**: 
   - Minimum 1 supervisor required at all times
   - Use PUT endpoint to update entire supervisor list
   - Supervisors are automatically deduplicated
   - One supervisor can manage multiple rooms (Springerkraft)

3. **Session Lifecycle**:
   - Sessions only end via explicit API call
   - No automatic termination when supervisors change
   - Device can only have one active session at a time

4. **Error Handling**:
   - All errors return JSON with `status: "error"` and `error` message
   - HTTP status codes: 200 (success), 400 (bad request), 401 (auth), 404 (not found)

5. **RFID Format**:
   - RFID tags are case-insensitive
   - Various formats supported (with/without colons, dashes)

---

## Current Implementation Status (2025-06-30)

### ✅ What's Working:
1. **Global PIN Authentication**: Devices authenticate with `OGS_DEVICE_PIN` environment variable
2. **Teacher List Endpoint**: Returns all teachers for supervisor selection
3. **Multi-Supervisor Sessions**: Can start sessions with multiple supervisors
4. **Data Integrity**: All supervisor assignments properly stored in database
5. **API Response**: Includes full supervisor details (name, role)

### ⚠️ Important Notes:
1. **Breaking Change**: `supervisor_ids` is REQUIRED - no backward compatibility
2. **Minimum Supervisors**: At least 1 supervisor must be specified
3. **Validation**: All supervisor IDs must exist as valid staff members
4. **Transaction Safety**: Group and supervisor creation is atomic

### 🔧 Technical Details to Remember:
1. **FindWithPerson Fix**: Staff repository method modified to avoid BUN ORM relation issues
2. **Debug Logging**: Extensive logging added for troubleshooting (can be removed later)
3. **Deduplication**: Supervisor IDs automatically deduplicated before insertion
4. **Response Structure**: Added `SupervisorInfo` struct for API responses

### 📝 Deployment Ready:
1. ✅ Dynamic supervisor endpoints implemented (PUT)
2. ✅ Business rules for session management added
3. ✅ "Springerkraft" support working (staff can supervise multiple rooms)
4. Frontend integration (not in current scope)

Last Updated: 2025-06-30 14:45





# More context

1. yes it should present all teachers we have in our database
2. no no limit minimum always 1 and max is free
3. all should be able to assign all no verification needed
4.   4. Dynamic Supervisor Changes: Can supervisors be added/removed from an active session, or
is the supervisor list fixed at session creation? it should be dynmaic. on the device there will be a settings page where we can add and remove supervisors
5.   5. Session Ownership: If all supervisors leave/log out, should the session automatically
end, or continue running? the session should end with a special end session button. we already have a api endpoint for that right?
6. Activity Instances: When selecting "Hausaufgabenbetreuung", does the device create a new
instance or join an existing one? How do we differentiate between instances? it always creates a new instances, no merges. but we still should be able to continue a session on the same device

7. Action Attribution: When multiple supervisors are assigned, how should we track which
supervisor performed specific actions (like checking in a student)? lets for now just share the specific actions and no tracking. this is currently not so important. so if teacher a is group supervisor for
group 1 then all other added personal should be able to assign tags

8.   8. Supervisor Availability: Should we prevent selecting supervisors who are already assigned
to another active session, or allow one person to supervise multiple rooms? so there is usually people who supervise one room but there is also 1 "Springerkraft" that goes from room to room and should
therefore be assigned to both rooms

Frontend Integration

9. MyRoom Page Logic: Currently it likely queries by single supervisor_id. Should it show
data for ALL rooms where the user is a supervisor, or have a room selector?
10. Real-time Updates: Should all supervisors see real-time updates when any supervisor
performs an action in their shared session?

lets for now ignore the frontend integration only backend and api integration neccessary

---

so i approved chris pull request and merged it into development (current branch). for more information you might look at pr Feat/multiple educational group supervisors #219
