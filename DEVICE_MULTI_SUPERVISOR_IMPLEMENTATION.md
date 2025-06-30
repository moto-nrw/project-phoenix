# Device Multi-Supervisor Implementation Plan

## Overview
This document tracks the implementation of multiple supervisor support for RFID device sessions. Previously, devices authenticated with individual staff PINs and sessions had single supervisors. The new system uses a global OGS PIN and supports multiple supervisors per session.

**Branch**: `feature/global-ogs-pin`
**Related PR**: #219 (Multiple educational group supervisors - merged)

---

## Current Status

### ✅ Completed
- [x] Global PIN authentication implemented
- [x] Device authentication without staff context
- [x] Multiple supervisor database schema (from PR #219)
- [x] Teacher list endpoint for supervisor selection
- [x] Session creation with multiple supervisors

### 🚧 In Progress
- [ ] Backward compatibility testing

### 📋 Pending
- [ ] Dynamic supervisor management endpoints
- [ ] Business rule implementation
- [ ] Integration testing

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

### Objective 4: Backward Compatibility
**Status**: PENDING
**Target**: Ensure existing workflows continue to function

**Acceptance Criteria**:
- [ ] Existing Bruno tests pass without modification
- [ ] Single supervisor mode still works
- [ ] No breaking changes to existing endpoints

**Implementation Tasks**:
1. [ ] Make `supervisor_ids` optional in request
2. [ ] Default behavior when field not provided
3. [ ] Support both old and new patterns

**Testing Checklist**:
- [ ] Run existing Bruno test suite
- [ ] Verify no regressions
- [ ] Document any migration needs

---

### Objective 5: Add Supervisors to Active Session
**Status**: PENDING
**Target**: Dynamic supervisor addition during active session

**API Design**:
```json
POST /api/iot/session/{session_id}/supervisors
{
  "supervisor_ids": [4, 5]  // Staff IDs to add
}
```

**Acceptance Criteria**:
- [ ] Validate session exists and is active
- [ ] Validate supervisor IDs are valid staff
- [ ] Create new `group_supervisors` records
- [ ] Prevent duplicate assignments (idempotent)
- [ ] Return updated supervisor list

**Implementation Tasks**:
1. [ ] Create new endpoint in `api/iot/api.go`
2. [ ] Add route to router
3. [ ] Implement service method for adding supervisors
4. [ ] Handle duplicate prevention logic
5. [ ] Create response structure

**Test Scenarios**:
- [ ] Add single supervisor to active session
- [ ] Add multiple supervisors in one request
- [ ] Add already assigned supervisor (no error, idempotent)
- [ ] Add to non-existent session → Error
- [ ] Add invalid staff ID → Error

**Testing Checklist**:
- [ ] Create Bruno test `dev/device-supervisor-add.bru`
- [ ] Test all scenarios
- [ ] Verify database state after operations

---

### Objective 6: Remove Supervisors from Active Session
**Status**: PENDING
**Target**: Dynamic supervisor removal during active session

**API Design**:
```json
DELETE /api/iot/session/{session_id}/supervisors
{
  "supervisor_ids": [2]  // Staff IDs to remove
}
```

**Acceptance Criteria**:
- [ ] Validate session exists and is active
- [ ] Set end_date on `group_supervisors` records
- [ ] Return remaining supervisor list
- [ ] Handle removing non-existent supervisor gracefully

**Implementation Tasks**:
1. [ ] Create endpoint for supervisor removal
2. [ ] Implement soft delete (set end_date)
3. [ ] Check remaining supervisors count
4. [ ] Handle edge cases

**Test Scenarios**:
- [ ] Remove one supervisor (others remain active)
- [ ] Remove multiple supervisors
- [ ] Remove non-assigned supervisor (no error)
- [ ] Remove last supervisor (check business rules)

**Testing Checklist**:
- [ ] Create Bruno test `dev/device-supervisor-remove.bru`
- [ ] Test all scenarios
- [ ] Verify end_date set correctly

---

### Objective 7: Session Management Rules
**Status**: PENDING
**Target**: Implement business rules for supervisor management

**Business Rules**:
- [ ] Cannot start session with 0 supervisors
- [ ] Session remains active if at least 1 supervisor remains
- [ ] "Springerkraft" can supervise multiple rooms simultaneously
- [ ] Session continues even if all supervisors removed (per requirements)

**Implementation Tasks**:
1. [ ] Add validation for minimum supervisors
2. [ ] Allow staff to be assigned to multiple active sessions
3. [ ] Document session lifecycle behavior

**Test Scenarios**:
- [ ] One staff supervising multiple rooms simultaneously
- [ ] Remove all supervisors (session continues)
- [ ] Query sessions by supervisor (returns multiple)

**Testing Checklist**:
- [ ] Create Bruno test `dev/device-session-edge-cases.bru`
- [ ] Test concurrent session scenarios
- [ ] Verify business rules enforced

---

## Implementation Timeline

### Phase 1: Core Implementation (Current)
- [x] Global PIN authentication
- [ ] Verify teacher list endpoint
- [ ] Implement multi-supervisor session start
- [ ] Ensure backward compatibility

### Phase 2: Dynamic Management
- [ ] Add supervisor endpoint
- [ ] Remove supervisor endpoint
- [ ] Edge case handling

### Phase 3: Integration & Testing
- [ ] Complete test suite
- [ ] Performance testing
- [ ] Documentation update

---

## Testing Matrix

| Scenario | O1 | O2 | O3 | O4 | O5 | O6 | O7 |
|----------|----|----|----|----|----|----|----|
| Device auth with global PIN | ✅ | | | | | | |
| List all teachers | | ✅ | | | | | |
| Start with 1 supervisor | | | ✅ | | | | |
| Start with 3 supervisors | | | ✅ | | | | |
| Old API still works | | | | ⏳ | | | |
| Add supervisor to session | | | | | ⏳ | | |
| Remove supervisor | | | | | | ⏳ | |
| Multiple rooms per person | | | | | | | ⏳ |

Legend: ✅ Complete | ⏳ Pending | ❌ Failed

---

## Code Locations

### Backend Files Modified:
- `backend/auth/device/device_auth.go` - Global PIN authentication
- `backend/api/iot/api.go` - Device endpoints, multi-supervisor handling
- `backend/services/active/interface.go` - Added multi-supervisor service methods
- `backend/services/active/active_service.go` - Implemented multi-supervisor logic

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

---

## Notes & Decisions

1. **Global PIN Choice**: Decided to use environment variable `OGS_DEVICE_PIN` for simplicity
2. **No Staff Tracking**: Device actions no longer tracked to specific staff members
3. **Session Persistence**: Sessions continue even without supervisors (per requirements)
4. **Springerkraft Support**: One person can supervise multiple rooms simultaneously

---

## Next Steps

1. Test backward compatibility (Objective 4)
2. Implement dynamic supervisor add/remove endpoints (Objectives 5 & 6)
3. Implement business rules for session management (Objective 7)
4. Complete integration testing

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

### 📝 Next Steps:
1. Implement dynamic supervisor add/remove endpoints
2. Add business rules for session management
3. Handle "Springerkraft" (staff supervising multiple rooms)
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
