# Attendance Tracking Implementation Guide

## Overview

This guide provides step-by-step instructions for implementing the attendance tracking system in Project Phoenix. The system allows teachers to mark student attendance using IoT devices (RFID readers).

### Key Requirements
- Teachers use IoT devices to mark student attendance
- Students can check in/out multiple times per day (e.g., leave for appointment and return)
- Only teachers with group access can mark attendance for those students
- Two-step process: scan RFID → confirm action
- Server is single source of truth

### Assumptions
1. **Timezone**: All dates/times use server timezone
2. **Multiple Records**: Students can have multiple attendance records per day
3. **Partial Records**: Check-out time can be null (student checked in but not yet out)
4. **Device Tracking**: We track which device was used for each attendance action
5. **Manual Process**: No automatic checkout at end of day
6. **Permission Model**: Teachers can only mark attendance for students in their groups

### API Design
- `GET /api/iot/attendance/status/{rfid}` - Check student's current attendance status
- `POST /api/iot/attendance/toggle` - Toggle attendance state (check-in or check-out)

### API Documentation

#### Authentication
All attendance endpoints require **three headers** for two-layer device authentication:

```http
Authorization: Bearer <device_api_key>
X-Staff-PIN: <4_digit_pin>
X-Staff-ID: <staff_id>
```

**Example Headers:**
```http
Authorization: Bearer dev_cb1b1c9aae51924661797c4bfef08c5378fbd91bf5dc090109a54d392529c575
X-Staff-PIN: 1234
X-Staff-ID: 1
```

#### 1. GET /api/iot/attendance/status/{rfid}

**Description**: Check student's current attendance status

**URL Parameters:**
- `rfid` (string, required): Normalized RFID tag (e.g., "1047722B7A0C88")

**Success Response (200 OK):**
```json
{
  "status": "success",
  "data": {
    "student": {
      "id": 1,
      "first_name": "Test",
      "last_name": "Test",
      "group": {
        "id": 1,
        "name": "Test"
      }
    },
    "attendance": {
      "status": "checked_in|checked_out|not_checked_in",
      "date": "2025-06-22T00:00:00Z",
      "check_in_time": "2025-06-22T22:33:04.515977Z",
      "check_out_time": "2025-06-22T22:37:19.413323Z",
      "checked_in_by": "Yannick Wenger",
      "checked_out_by": "Yannick Wenger"
    }
  },
  "message": "Student attendance status retrieved successfully"
}
```

**Status Values:**
- `"not_checked_in"`: No attendance record for today OR latest record has check-out time
- `"checked_in"`: Latest record has check-in time but no check-out time
- `"checked_out"`: Latest record has both check-in and check-out times

**Error Responses:**
- `404 Not Found`: Student with RFID not found
  ```json
  {
    "status": "error",
    "error": "RFID tag not found"
  }
  ```
- `401 Unauthorized`: Missing or invalid authentication headers
- `403 Forbidden`: Staff member doesn't have access to this student

#### 2. POST /api/iot/attendance/toggle

**Description**: Toggle attendance state (check-in/check-out or cancel)

**Request Body:**
```json
{
  "rfid": "1047722B7A0C88",
  "action": "confirm|cancel"
}
```

**Action Values:**
- `"confirm"`: Execute the attendance toggle (check-in or check-out)
- `"cancel"`: Cancel the operation (returns success without making changes)

**Success Response - Confirm (200 OK):**
```json
{
  "status": "success",
  "data": {
    "action": "checked_in|checked_out",
    "student": {
      "id": 1,
      "first_name": "Test",
      "last_name": "Test",
      "group": {
        "id": 1,
        "name": "Test"
      }
    },
    "attendance": {
      "status": "checked_in",
      "date": "2025-06-22T00:00:00Z",
      "check_in_time": "2025-06-22T22:45:47.870325Z",
      "check_out_time": null,
      "checked_in_by": "Yannick Wenger",
      "checked_out_by": ""
    },
    "message": "Hallo Test!"
  },
  "message": "Student checked_in successfully"
}
```

**Success Response - Cancel (200 OK):**
```json
{
  "status": "success",
  "data": {
    "action": "cancelled",
    "student": {
      "id": 0,
      "first_name": "",
      "last_name": ""
    },
    "attendance": {
      "status": "",
      "date": "0001-01-01T00:00:00Z",
      "check_in_time": null,
      "check_out_time": null,
      "checked_in_by": "",
      "checked_out_by": ""
    },
    "message": "Attendance tracking cancelled"
  },
  "message": "Attendance tracking cancelled"
}
```

**Toggle Logic:**
- If student status is `"not_checked_in"` or `"checked_out"` → **check-in** (create new record)
- If student status is `"checked_in"` → **check-out** (update existing record)

**Error Responses:**
- `404 Not Found`: Student with RFID not found
  ```json
  {
    "status": "error",
    "error": "RFID tag not found"
  }
  ```
- `401 Unauthorized`: Missing or invalid authentication headers  
- `403 Forbidden`: Staff member doesn't have access to this student
- `400 Bad Request`: Invalid request body or RFID format

#### RFID Tag Format
- **Input**: Normalized uppercase hex string without separators
- **Examples**: "1047722B7A0C88", "04D69482", "ABCD1234"
- **Invalid**: "04:D6:94:82", "04-d6-94-82", "04d69482" (lowercase)

#### Business Rules
1. **Multiple Check-ins**: Students can check-in/out multiple times per day
2. **Partial Records**: Check-out time can be null (student checked in but not out)
3. **Latest Record**: Status determined by most recent record for today
4. **Permission Check**: Staff can only mark attendance for students in their assigned groups
5. **Device Tracking**: Each attendance action records which device was used

---

## Implementation Layers

## 1. Database Layer

### 1.1 Migration File
**File**: `backend/database/migrations/001006005_create_attendance_table.go`

**Table Structure**:
```
active.attendance
- id (BIGSERIAL PRIMARY KEY)
- student_id (BIGINT NOT NULL, FK to users.students)
- date (DATE NOT NULL)
- check_in_time (TIMESTAMPTZ NOT NULL)
- check_out_time (TIMESTAMPTZ NULLABLE)
- checked_in_by (BIGINT NOT NULL, FK to users.staff)
- checked_out_by (BIGINT NULLABLE, FK to users.staff)
- device_id (BIGINT NOT NULL, FK to iot.devices)
- created_at (TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)
- updated_at (TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)

Indexes:
- idx_attendance_student_date ON (student_id, date)
- idx_attendance_date ON (date)
- idx_attendance_device ON (device_id)

Trigger:
- update_attendance_updated_at (reuse existing function)
```

**Dependencies**: users_students, users_staff, iot_devices migrations

**Status**: ✅ Complete

### 1.2 Model File
**File**: `backend/models/active/attendance.go`

**Pseudo-code Structure**:
```
type Attendance struct {
    // Standard BUN model setup with table alias
    // All fields from database table
    // Timestamps
}

// BeforeAppendModel hook for BUN
// - Commented out (follows active schema pattern)
// - Database handles timestamps via defaults and triggers
// - Would only set table expressions if uncommented

// Helper method: IsCheckedIn() bool
// - Returns true if CheckOutTime is nil

// Repository interface:
// - Standard CRUD from base.Repository
// - FindByStudentAndDate(studentID, date) ([]*Attendance, error)
// - FindLatestByStudent(studentID) (*Attendance, error)
// - GetStudentCurrentStatus(studentID) (*Attendance, error)
```

**Status**: ✅ Complete

---

## 2. Repository Layer

### 2.1 Repository Implementation
**File**: `backend/database/repositories/active/attendance_repository.go`

**Pseudo-code Implementation**:
```
// Constructor follows factory pattern

FindByStudentAndDate:
- Extract date only (no time)
- Query with student_id and date
- Order by check_in_time ASC
- Return all records

FindLatestByStudent:
- Query by student_id
- Order by date DESC, check_in_time DESC
- Limit 1

GetStudentCurrentStatus:
- Get today's date
- Query latest record for student today
- Order by check_in_time DESC
- Limit 1
```

**Status**: ✅ Complete

### 2.2 Update Repository Factory
**File**: `backend/database/repositories/factory.go` (UPDATE)

- Add attendanceRepo field
- Initialize in NewFactory
- Add NewAttendanceRepository() method

**Status**: ✅ Complete

---

## 3. Service Layer

### 3.1 Update Active Service Interface
**File**: `backend/services/active/interface.go` (UPDATE)

**New Methods**:
```
GetStudentAttendanceStatus(ctx, studentID) (*AttendanceStatus, error)
ToggleStudentAttendance(ctx, studentID, staffID, deviceID) (*AttendanceResult, error)
CheckTeacherStudentAccess(ctx, teacherID, studentID) (bool, error)
```

**New Types**:
```
AttendanceStatus:
- StudentID, Status (not_checked_in/checked_in/checked_out)
- Date, CheckInTime, CheckOutTime
- CheckedInBy, CheckedOutBy (names)

AttendanceResult:
- Action (checked_in/checked_out)
- AttendanceID, StudentID, Timestamp
```

**Status**: ✅ Complete

### 3.2 Update Active Service Implementation
**File**: `backend/services/active/active_service.go` (UPDATE)

**Add Dependencies**:
- educationService (for group lookups)
- usersService (for person lookups)
- teacherRepo (for teacher access checks)

**Pseudo-code for Methods**:

```
GetStudentAttendanceStatus:
- Get today's latest attendance record
- If no record: return status "not_checked_in"
- If CheckOutTime is null: return status "checked_in"
- Else: return status "checked_out"
- Load staff names:
  - Use staffRepo.Get(staffID) to get staff record
  - Use usersService.Get(staff.PersonID) to get person details
  - Format as "FirstName LastName"
- Return with formatted names for checked_in_by/checked_out_by

ToggleStudentAttendance:
- Check teacher has access via CheckTeacherStudentAccess
- Get current status
- If not_checked_in or checked_out:
  - Create new attendance record with check_in_time
  - Return "checked_in" result
- If checked_in:
  - Update record with check_out_time and checked_out_by
  - Return "checked_out" result

CheckTeacherStudentAccess:
- Get teacher from staff ID
- Get teacher's groups via educationService
- Get student info
- Check if student.GroupID is in teacher's groups
- Return true/false
```

**Status**: ✅ Complete

### 3.3 Update Service Factory
**File**: `backend/services/factory.go` (UPDATE)

Update NewActiveService to include:
- attendanceRepo
- educationService
- staffRepo
- teacherRepo
- usersService

**Status**: ✅ Complete

---

## 4. API Layer

### 4.1 Add Attendance Response Types
**File**: `backend/api/iot/attendance_types.go` (NEW)

**Types Structure**:
```
AttendanceStatusResponse:
- Student (id, first_name, last_name, group)
- Attendance (status, date, times, checked_by)

AttendanceToggleRequest:
- RFID
- Action (confirm/cancel)

AttendanceToggleResponse:
- Action (checked_in/checked_out)
- Student info
- Attendance info

AttendanceGroupInfo:
- ID (from education.groups)
- Name (actual group name, NOT student.SchoolClass)
```

**Note**: Group information comes from the `education.groups` table via `educationService.GetGroup()`, not from the student's SchoolClass field.

**Status**: ✅ Complete

### 4.2 Add Attendance Handlers
**File**: `backend/api/iot/api.go` (UPDATE)

**Router Addition**:
```
// In device-authenticated section:
r.Get("/attendance/status/{rfid}", rs.getAttendanceStatus)
r.Post("/attendance/toggle", rs.toggleAttendance)
```

**Handler Pseudo-code**:

```
getAttendanceStatus:
- Get device/staff from context
- Get RFID from URL, normalize it
- Find person by RFID tag using usersService.FindByTagID
- Get student from person using studentRepo.FindByPersonID
- Check teacher has access to student
- Get attendance status from service
- Load student's group info:
  - If student.GroupID exists, use educationService.GetGroup(*student.GroupID)
  - Include group.ID and group.Name in response (not SchoolClass)
- Build and return response

toggleAttendance:
- Get device/staff from context
- Parse request body
- If action is "cancel": return cancelled response
- Find person by RFID tag using usersService.FindByTagID
- Get student from person using studentRepo.FindByPersonID
- Call ToggleStudentAttendance service
- Get updated status
- Load student's group info (same pattern as status endpoint):
  - If student.GroupID exists, use educationService.GetGroup(*student.GroupID)
  - Include actual group name, not SchoolClass
- Build and return response
```

**Status**: ✅ Complete

---

## 5. Testing

### 5.1 Repository Tests
**File**: `backend/database/repositories/active/attendance_repository_test.go` (NEW)

**Test Coverage (✅ Complete)**:

**`TestAttendanceRepository_Create`** (4 test cases):
- Create valid attendance record with all required fields
- Create with check-out time and verify optional fields  
- Validation test (nil attendance should fail)
- Verify `IsCheckedIn()` helper method behavior

**`TestAttendanceRepository_FindByStudentAndDate`** (6 test cases):
- Single record for student on specific date
- Multiple records ordered by check_in_time ASC
- No records found for empty date
- Date filtering ignores time component
- Different students on same date isolation
- Different dates for same student filtering

**`TestAttendanceRepository_FindLatestByStudent`** (6 test cases):
- Latest record across multiple dates
- Latest record same day with multiple check-ins
- No records returns proper error
- Single record for student
- Complex scenario with mixed dates and times
- Different students do not interfere

**`TestAttendanceRepository_GetStudentCurrentStatus`** (7 test cases):
- No records today (not checked in)
- Student checked in (no check-out time)
- Student checked out (has check-out time)
- Multiple records today - returns latest by check-in time
- Historical records exist but none today
- Different students on same day
- Timezone handling and today calculation

**Implementation Features**:
- Real PostgreSQL connection using existing test DB pattern
- Foreign key relationships (creates test students, staff, devices, persons)
- Schema-qualified queries with proper `ModelTableExpr` and quoted aliases
- BUN ORM integration following existing repository patterns
- Proper cleanup removing test data in reverse dependency order
- Error handling testing both success and error scenarios
- Attendance business logic verification (multiple check-ins, date filtering, status determination)

**Status**: ✅ Complete

### 5.2 Service Tests
**File**: `backend/services/active/attendance_service_test.go` (NEW)

**Test Coverage (🔄 In Progress)**:

**Implemented Tests**:
- ✅ Get status: not checked in (with mock repository)
- ✅ IsCheckedIn helper method on Attendance model
- ✅ Mock repository behavior testing
- ✅ Service testing pattern documentation

**Remaining Tests**:
- ⬜ Get status: checked in, checked out (requires comprehensive mocking)
- ⬜ Toggle: check-in flow, check-out flow (requires service dependency mocks)
- ⬜ Multiple check-ins per day scenarios
- ⬜ Permission checks (CheckTeacherStudentAccess with educationService/usersService mocks)

**Implementation Notes**:
- Establishes foundation with MockAttendanceRepository using testify/mock
- Demonstrates testing patterns for future comprehensive implementation
- Requires mocking of educationService, usersService, staffRepo, teacherRepo, studentRepo for complete coverage
- Two demonstration tests skipped to show dependency requirements

**Status**: 🔄 In Progress

### 5.3 API Tests
**File**: `backend/api/iot/attendance_handlers_test.go` (NEW)

Test cases:
- Status endpoint with valid/invalid RFID
- Toggle endpoint with confirm/cancel
- Authentication failures
- Permission denied scenarios

**Status**: ⬜ To Do

### 5.4 Bruno API Tests
**Files**: 
- `bruno/dev/attendance.bru` - Status endpoint test
- `bruno/dev/attendance-toggle.bru` - Toggle confirm test  
- `bruno/dev/attendance-toggle-cancel.bru` - Toggle cancel test

**Test Coverage**:
- ✅ Get attendance status with valid RFID
- ✅ Toggle attendance (confirm action)
- ✅ Toggle attendance (cancel action)
- ✅ Authentication with device API key, staff PIN, and staff ID
- ✅ Real RFID tag from seed data (1047722B7A0C88)
- ✅ Complete attendance workflow testing

**Execution**:
```bash
./dev-test.sh attendance-rfid    # Run all 3 attendance tests (~365ms)
./dev-test.sh attendance         # Run both web and RFID tests
```

**Status**: ✅ Complete

---

## 6. Progress Tracking Checklist

### Database Layer
- ✅ Create migration file
- ✅ Create model file
- ✅ Run migration

### Repository Layer
- ✅ Create repository implementation
- ✅ Update repository factory
- ✅ Write repository tests

### Service Layer
- ✅ Update service interface
- ✅ Implement service methods
- ✅ Update service factory
- 🔄 Write service tests

### API Layer
- ✅ Create response types
- ✅ Add routes
- ✅ Implement handlers
- ⬜ Write API tests

### Integration Testing
- ✅ Create Bruno tests
- ✅ Test with seed data
- ✅ Verify permissions
- ✅ Test edge cases

### Documentation
- ✅ Update API docs
- ⬜ Update RFID guide
- ✅ Add examples

---

## Notes for Implementation

### Current Work Tracking
Mark completed items with ✅ and in-progress with 🔄.

### Potential Changes/Backtracking
1. **Permission Model**: If teacher-student access pattern changes, update `CheckTeacherStudentAccess`
2. **Date Handling**: Currently using server timezone - may need timezone support later
3. **API Response**: May need to add more student/group details based on UI needs
4. **Validation**: May need to add business rules (e.g., max check-ins per day)

### Dependencies to Watch
- Existing IoT authentication middleware
- Person/Student/Staff relationships (staff→person via PersonID)
- Group-Teacher relationships (via education.groups)
- RFID tag normalization logic (normalizeTagID function)
- Group lookups (education.groups table, not student.SchoolClass)

### Error Handling Patterns
Follow existing patterns:
- ErrorNotFound for missing resources
- ErrorForbidden for permission issues
- ErrorInvalidRequest for validation errors
- ErrorInternalServer for unexpected errors
