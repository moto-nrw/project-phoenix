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

### 🚧 In Progress
- [ ] Session creation with multiple supervisors

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

### Objective 3: Start Session with Multiple Supervisors
**Status**: PENDING  
**Target**: Implement core multi-supervisor session creation

**API Design**:
```json
POST /api/iot/session/start
{
  "activity_id": 123,
  "room_id": 456,              // optional
  "supervisor_ids": [1, 2, 3], // NEW: array of staff IDs
  "force": false
}
```

**Acceptance Criteria**:
- [ ] Add `supervisor_ids` field to `SessionStartRequest`
- [ ] Validate minimum 1 supervisor required
- [ ] Validate all supervisor IDs exist as staff
- [ ] Create `active.groups` record
- [ ] Create `active.group_supervisors` records for each supervisor
- [ ] Set role = "supervisor" for all records
- [ ] Return active_group_id and supervisor list

**Implementation Tasks**:
1. [ ] Update `SessionStartRequest` struct in `api/iot/api.go`
2. [ ] Add validation for supervisor_ids array
3. [ ] Create new service method for multi-supervisor session start
4. [ ] Implement transaction for atomic group + supervisors creation
5. [ ] Update response structure to include supervisors

**Test Scenarios**:
- [ ] Valid: Single supervisor `[1]`
- [ ] Valid: Multiple supervisors `[1, 2, 3]`
- [ ] Invalid: Empty array `[]` → Error
- [ ] Invalid: Non-existent staff `[999]` → Error
- [ ] Edge: Duplicate IDs `[1, 1, 2]` → Handle gracefully

**Testing Checklist**:
- [ ] Create Bruno test `dev/device-session-start-multi.bru`
- [ ] Test all scenarios above
- [ ] Verify database records created correctly

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
| Start with 1 supervisor | | | ⏳ | | | | |
| Start with 3 supervisors | | | ⏳ | | | | |
| Old API still works | | | | ⏳ | | | |
| Add supervisor to session | | | | | ⏳ | | |
| Remove supervisor | | | | | | ⏳ | |
| Multiple rooms per person | | | | | | | ⏳ |

Legend: ✅ Complete | ⏳ Pending | ❌ Failed

---

## Code Locations

### Backend Files Modified:
- `backend/auth/device/device_auth.go` - Global PIN authentication
- `backend/api/iot/api.go` - Device endpoints
- `backend/services/active/interface.go` - Service interfaces
- `backend/services/active/active_service.go` - Business logic

### Environment Files:
- `.env.example` - Added OGS_DEVICE_PIN
- `backend/dev.env.example` - Added OGS_DEVICE_PIN
- `docker-compose.yml` - Added OGS_DEVICE_PIN to server environment
- `docker-compose.example.yml` - Added OGS_DEVICE_PIN

### Database Tables (from PR #219):
- `active.groups` - Activity sessions
- `active.group_supervisors` - Supervisor assignments (many-to-many)

---

## Notes & Decisions

1. **Global PIN Choice**: Decided to use environment variable `OGS_DEVICE_PIN` for simplicity
2. **No Staff Tracking**: Device actions no longer tracked to specific staff members
3. **Session Persistence**: Sessions continue even without supervisors (per requirements)
4. **Springerkraft Support**: One person can supervise multiple rooms simultaneously

---

## Next Steps

1. Test Objective 2 (teacher list endpoint)
2. Implement Objective 3 (multi-supervisor session start)
3. Create Bruno test suite for new functionality

---

Last Updated: 2025-06-30