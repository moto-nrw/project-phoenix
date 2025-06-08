# RFID Check-in System Implementation Guide

## Overview
This document outlines the complete implementation plan for the RFID-based student attendance tracking system, integrating the Moto webapp (this repo) with the PyrePortal Pi app.

## System Architecture

### Components
1. **Moto Webapp** (this repo) - Backend API and web dashboard
2. **PyrePortal** (Pi app) - Tauri-based RFID scanner application
3. **Raspberry Pi Devices** - Physical devices with RFID readers
4. **Mobile Interface** - Teacher activity creation on phones

### Key Design Decisions
- **Device Mobility**: Devices are not tied to rooms, teachers carry them
- **PIN Authentication**: For access control only, not user tracking
- **Activity-Centric**: Devices track activities, not teachers
- **Offline-First**: Pi app caches data locally for reliability
- **Teacher Filtering**: Teachers only see their supervised students
- **One Device per Activity**: No concurrent activity usage on same device
- **Teacher Self-Coordination**: Manual coordination between teachers
- **Universal RFID**: All students must have RFID tags (no manual entry)
- **Tag Override**: New tag assignments automatically replace old ones
- **Two-Layer Auth**: Device API key + Teacher PIN for security
- **Activity Availability**: Activities shown are today's/active only
- **Immediate Availability**: Activities available immediately after creation
- **Let It Crash**: Simple error handling with resume option after crashes
- **Mandatory PIN**: Every teacher must have a PIN set
- **Substitute Teachers**: Create new activities (no activity sharing)
- **Local Student Caching**: Device caches student names locally
- **Auto-Logout**: Automatic logout when entering different room

## Complete Workflow

### Device Registration (Admin Only)
```
Admin → Web Dashboard → Device Management
1. Admin navigates to /database/devices
2. Clicks "Register New Device"
3. Enters:
   - Device Name (e.g., "Classroom Pi 01")
   - Device Type: "rfid_reader"
4. System generates:
   - Unique device_id (UUID)
   - API key (dev_xyz123...)
5. Admin configures device with credentials:
   - SSH into device or use config UI
   - Store device_id and api_key in device config
   - Device saves credentials locally
6. Device verifies connection:
   - Sends test ping to server
   - Server confirms device is registered
   - Device ready for teacher use
```

### 1. Teacher Activity Creation (Mobile)
```
Teacher → Mobile Phone → Moto Webapp
1. Opens mobile-optimized activity creation form
2. Enters: Activity name, Category, Room, Max students (optional)
3. System auto-assigns teacher as supervisor
4. Activity created and available for selection
```

### 2. Device Setup and Login
```
Teacher → Pi Device
1. Powers on device (auto-updates via git pull)
2. Device authenticates with API using its api_key
3. Teacher sees dropdown list (fetched using device auth)
4. Teacher selects their name
5. Teacher enters 4-digit PIN
6. Device unlocked for that teacher (receives JWT token)
```

### 3. Activity Selection
```
Teacher → Pi Device → Moto API
1. Device shows teacher's activities (today's/active only)
2. Teacher selects activity to supervise
3. Creates active session (links device → activity → room)
4. Device displays: Activity name, Room, Teacher, Student count
5. Optional: Refresh button to sync latest activities
```

### 4. RFID Tag Assignment
```
Teacher → Pi Device → Student
1. Teacher selects "Assign Tags" mode
2. Scans RFID tag (assigned or unassigned)
3. Device queries API for current assignment
4. If tag already assigned:
   - Shows current student name
   - Asks "Reassign to different student?"
5. Shows dropdown of teacher's students (20-40 max)
6. Teacher selects student
7. Tag assigned to new student:
   - If student had previous tag, old tag is unlinked
   - If tag was assigned to another student, that link is removed
   - New tag-student link created
8. Confirmation: "Tag assigned to [Student Name]"
```

### 5. Student Check-in/Check-out
```
Student → RFID Tag → Pi Device → Moto API
1. Student taps RFID tag on device
2. Device sends: device_id, rfid_tag, timestamp
3. Server determines:
   - Student identity (from RFID)
   - Current activity (from device session)
   - Check-in or check-out (toggle logic)
   - Auto-logout from other rooms
4. Returns: Student name, action, updated count
5. Device shows popup: "Hallo Max!" or "Tschüss Max!"
```

### 6. Activity End
```
Teacher → Pi Device
1. Teacher clicks "End Activity" button
2. OR 30 minutes of no interaction triggers auto-end (any button/scan resets timer)
3. All students marked as checked out
4. Device returns to activity selection
5. Next teacher can use device
```

### 7. Device Health Monitoring (Automatic)
```
Pi Device → Server (Background Process)
1. Device sends ping every 60 seconds:
   - Includes device_id and timestamp
   - Uses device API key authentication
2. Server updates last_activity timestamp
3. Dashboard shows device status:
   - Green: Online (ping within 2 minutes)
   - Yellow: Warning (ping 2-5 minutes old)
   - Red: Offline (no ping for 5+ minutes)
4. If device goes offline:
   - Active sessions remain open
   - Device resumes when connection restored
   - Queued check-ins sync automatically
```

## API Specifications

## Authentication Architecture

### Why Two-Layer Authentication?

The system uses two distinct authentication layers for security and audit purposes:

1. **Device Authentication (API Key)**
   - Identifies which physical device is making requests
   - Ensures only registered devices can access the API
   - Allows tracking device location and usage patterns
   - Can be revoked if device is lost or compromised

2. **Teacher Authentication (PIN)**
   - Identifies which teacher is using the device
   - Ensures proper access control to student data
   - Creates audit trail of who did what
   - Allows multiple teachers to share devices safely

This separation means:
- Lost devices can be disabled without affecting teacher accounts
- Teacher PINs work on any registered device
- Complete audit trail shows both device AND user
- Compromised teacher PIN doesn't compromise device security

### Authentication Endpoints

#### Device Authentication (Layer 1)
Devices authenticate using their API key for all requests:
```typescript
Headers: {
  "Authorization": "Device dev_xyz123..."  // Device's api_key
}
```

#### Teacher PIN Login (Layer 2)
```typescript
POST /api/auth/device-pin
Headers: {
  "Authorization": "Device dev_xyz123..."  // Device auth required
}
Request: {
  "teacher_id": 123,  // Selected from dropdown
  "pin": "1234"
}
Response: {
  "success": true,
  "token": "jwt...",  // Teacher-specific JWT token
  "teacher": {
    "id": 123,
    "name": "Frau Schmidt"
  }
}
```

#### Teacher PIN Management
```typescript
GET /api/staff/pin
Headers: {
  "Authorization": "Bearer jwt_token..."  // Teacher's JWT required
}
Response: {
  "has_pin": true,
  "last_changed": "2024-01-07T10:00:00Z"
}

PUT /api/staff/pin
Headers: {
  "Authorization": "Bearer jwt_token..."  // Teacher's JWT required
}
Request: {
  "current_pin": "1234",  // null on first set
  "new_pin": "5678"
}
Response: {
  "success": true,
  "message": "PIN updated successfully"
}
```

### Device Management

#### Device Registration (Admin Only)
```typescript
POST /api/iot/devices/register
Headers: {
  "Authorization": "Bearer jwt_token..."  // Admin JWT required
}
Request: {
  "device_id": "f47ac10b-58cc-4372",
  "name": "Classroom Pi 01",
  "device_type": "rfid_reader"
}
Response: {
  "id": 123,
  "device_id": "f47ac10b-58cc-4372",
  "api_key": "dev_xyz123...",
  "status": "active"
}
```

#### Device-Authenticated Teacher List
```typescript
GET /api/teachers/device-list
Headers: {
  "Authorization": "Device dev_xyz123..."  // Device auth required
}
Response: {
  "teachers": [
    { "id": 1, "name": "Frau Schmidt" },  // Only teachers with PINs set
    { "id": 2, "name": "Herr Müller" }
  ]
}
```

### Activity Management

#### Quick Activity Creation (Mobile)
```typescript
POST /api/activities/quick-create
Headers: {
  "Authorization": "Bearer jwt_token..."  // Regular user JWT (from mobile app)
}
Request: {
  "name": "Bastelstunde",
  "category_id": 3,
  "room_id": 12,
  "max_participants": 20  // optional
}
Response: {
  "id": 456,
  "name": "Bastelstunde",
  "supervisor": { "id": 1, "name": "Frau Schmidt" },
  "room": { "id": 12, "name": "Werkraum 1" }
}
```

#### Start Active Session
```typescript
POST /api/active/quick-start
Headers: {
  "Authorization": "Bearer jwt_token..."  // Teacher's JWT from PIN login
}
Request: {
  "activity_id": 456,
  "device_id": "f47ac10b-58cc-4372"
}
Response (Success): {
  "active_session_id": 789,
  "activity": "Bastelstunde",
  "room": "Werkraum 1",
  "supervisor": "Frau Schmidt",
  "student_count": 0
}
Response (Conflict): {
  "error": "Activity already active on device Pi-03",
  "device_name": "Classroom Pi 03",
  "teacher": "Herr Müller",
  "can_override": true
}
// To override: POST /api/active/quick-start?force=true
```

### Device Health Monitoring

#### Device Ping
```typescript
POST /api/iot/{deviceId}/ping
Headers: {
  "Authorization": "Device dev_xyz123..."  // Device auth required
}
Response: {
  "status": "ok",
  "timestamp": "2024-01-07T10:30:00Z"
}
// Called every minute by device to maintain online status
```

### RFID Operations

#### Check Tag Assignment
```typescript
GET /api/rfid-cards/{tagId}
Headers: {
  "Authorization": "Bearer jwt_token..."  // Teacher's JWT from PIN login
}
Response (Assigned): {
  "assigned": true,
  "student": {
    "id": 123,
    "name": "Max Mustermann",
    "group": "Klasse 3A"
  }
}
Response (Unassigned): {
  "assigned": false
}
```

#### Assign Tag to Student
```typescript
POST /api/students/{studentId}/rfid
Headers: {
  "Authorization": "Bearer jwt_token..."  // Teacher's JWT from PIN login
}
Request: {
  "rfid_tag": "1234567890ABCDEF"
}
Response: {
  "success": true,
  "previous_tag": "9876543210FEDCBA"  // if replaced
}
```

#### Get Teacher's Students
```typescript
GET /api/students/my-students
Headers: {
  "Authorization": "Bearer jwt_token..."  // Teacher's JWT from PIN login
}
Response: {
  "students": [
    { "id": 123, "name": "Max Mustermann", "group": "3A" },
    { "id": 124, "name": "Anna Schmidt", "group": "3A" }
  ]
}
```

#### Process RFID Scan
```typescript
POST /api/iot/checkin
Headers: {
  "Authorization": "Bearer jwt_token..."  // Teacher's JWT token
}
Request: {
  "device_id": "f47ac10b-58cc-4372",
  "rfid_tag": "1234567890ABCDEF",
  "timestamp": "2024-01-07T10:30:00Z"
}
Response: {
  "student": {
    "id": 123,
    "name": "Max Mustermann"
  },
  "action": "checked_in",  // or "checked_out"
  "visit_id": 999,
  "student_count": 15,
  "message": "Hallo Max!"  // or "Tschüss Max!"
}
```

## Database Changes Required

### 1. Make device_id Optional in active_groups
```sql
-- Migration: Make device_id nullable
ALTER TABLE active.groups 
ALTER COLUMN device_id DROP NOT NULL;
```

### 2. Add PIN Storage to Staff
```sql
-- Migration: Add PIN field
ALTER TABLE users.staff 
ADD COLUMN pin_hash VARCHAR(255),
ADD COLUMN pin_attempts INTEGER DEFAULT 0,
ADD COLUMN pin_locked_until TIMESTAMP;
```

### 3. Add Device Authentication
```sql
-- Migration: Add API key to devices
ALTER TABLE iot.devices
ADD COLUMN api_key VARCHAR(255) UNIQUE,
ADD COLUMN last_activity TIMESTAMP;
```

### 4. Update Device Table for Health Monitoring
```sql
-- Already included in migration 3 above:
-- ADD COLUMN last_activity TIMESTAMP;
```

### 5. Add Device Session Tracking (Optional for MVP)
```sql
-- Migration: Track device usage sessions
CREATE TABLE device_sessions (
    id SERIAL PRIMARY KEY,
    device_id VARCHAR(255) REFERENCES iot.devices(device_id),
    teacher_id INTEGER REFERENCES users.teachers(id),
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMP,
    active_session_id INTEGER REFERENCES active.groups(id)
);
```

## Implementation Progress Status

**Overall Progress: ~15% Complete** (Last updated: January 2025)

### What's Currently Working ✅
1. **Database Schema**: RFID system tables with API keys, PIN storage, health monitoring (4/5 migrations complete)
2. **PIN Infrastructure**: Teacher PIN storage and validation functions (no API endpoints yet)
3. **Device Management**: Basic device registration and health ping endpoints (JWT auth only)
4. **RFID Card Linking**: Basic RFID card to person linking (not device-optimized)

### Critical Gaps Remaining ❌
1. **🚨 BLOCKING: Device Authentication**: No API key middleware or device authentication system
2. **🚨 BLOCKING: PIN API Endpoints**: No endpoints for device-to-teacher PIN validation
3. **🚨 BLOCKING: Teacher-Student APIs**: No endpoints for teachers to see their students
4. **Activity Management**: No quick activity creation or conflict detection
5. **Frontend UI**: Complete absence of device management interfaces
6. **Session Management**: No 30-minute timeouts or active session handling
7. **Mobile Interface**: No mobile-optimized activity creation

### Implementation Priority Order
**Phase 1 (BLOCKING)**: Device authentication middleware and PIN validation endpoints
**Phase 2 (Core APIs)**: Teacher-student relationships and activity management
**Phase 3 (Session Logic)**: Add timeout handling and conflict detection  
**Phase 4 (Frontend)**: Build device management and mobile interfaces
**Phase 5 (Integration)**: Connect with PyrePortal Pi app

---

## Implementation Tasks

### Moto Webapp (This Repo)

#### Backend Tasks
- [x] **Database migrations** (4/5 complete - API keys, PIN storage, health monitoring implemented)
- [x] **Device registration endpoint** (admin only - `/api/iot/devices`)
- [x] **Device health ping endpoint** (`/api/iot/{deviceId}/ping`)
- [x] **PIN storage and validation functions** (database layer only)
- [x] **RFID assignment endpoints** (basic linking - `/api/users/{id}/rfid`)
- [ ] **🚨 BLOCKING: Device authentication middleware** (no API key validation exists)
- [ ] **🚨 BLOCKING: PIN authentication endpoints** (no `/api/auth/device-pin` exists)
- [ ] **🚨 BLOCKING: Public teacher list endpoint** (no `/api/teachers/device-list` exists)
- [ ] **RFID check-in endpoint** (partial implementation exists, needs workflow completion)
- [ ] **Quick activity creation endpoint** (mobile-optimized activity creation)
- [ ] **My students endpoint for teachers** (filtered by teacher's groups)
- [ ] **30-minute activity timeout logic** (automatic session ending)
- [ ] **Activity conflict detection and override** (one device per activity)
- [ ] **Default PIN migration for existing teachers** (set default PINs)

#### Frontend Tasks
- [ ] Mobile activity creation form
- [ ] PIN management in user settings
- [ ] Device registration page (`/database/devices`)
- [ ] Device status monitoring dashboard
- [ ] Device lifecycle management UI (activate/deactivate)

### PyrePortal (Pi App)

#### Core Features
- [ ] PIN login screen with teacher dropdown
- [ ] Activity selection interface (today's/active only)
- [ ] RFID tag assignment UI
- [ ] Check-in/out popup displays
- [ ] 30-minute inactivity detection (resets on any interaction)
- [ ] Local student name caching
- [ ] Offline queue with sync
- [ ] Auto-update script on boot
- [ ] Device crash recovery (resume active sessions)
- [ ] Activity refresh button
- [ ] Device health ping every minute
- [ ] Auto-logout when entering different room

#### UI Components
- [ ] Teacher selection dropdown
- [ ] PIN input pad
- [ ] Activity list view
- [ ] Student dropdown (for tag assignment)
- [ ] Success/error popups
- [ ] Activity end confirmation

## Security & Privacy

### Access Control
- Two-layer authentication (see Authentication Architecture section):
  1. Device auth: API key validates device identity
  2. Teacher auth: PIN validates user identity
- Device registration by admin only
- All device endpoints require device authentication
- Teachers only see their supervised students
- Student data cached locally, cleared on logout
- Complete audit trail of device + teacher actions

### Data Flow
- All communication via HTTPS
- Minimal data transmission (3 fields for check-in)
- No student data persisted on devices
- Audit trail maintained server-side

## Error Handling (MVP)

### Simplified Approach ("Let It Crash")
- Assume happy path for pilot
- Fix issues as they arise
- No complex error recovery
- Basic retry for network failures
- Crash recovery: Resume active sessions on restart

### Device Lifecycle
1. **Registration**: Admin creates device record, gets credentials
2. **Configuration**: Device stores API key locally
3. **Active**: Device pings server, available for use
4. **Offline**: No pings received, shown as offline in dashboard
5. **Deactivated**: Admin disables device, API key rejected
6. **Reactivation**: Admin can re-enable deactivated devices

### Current Implementation Limitations
**System is NOT yet functional for production use. BLOCKING issues prevent basic operation:**

- **🚨 BLOCKING: No device authentication**: RFID devices cannot authenticate with API keys
- **🚨 BLOCKING: No PIN validation API**: No endpoints for teacher PIN authentication
- **🚨 BLOCKING: No teacher-student API**: Teachers cannot see their supervised students
- **No working RFID check-in flow**: Endpoint exists but incomplete workflow
- **No activity creation for devices**: Teachers cannot create activities via mobile
- **No session management**: No 30-minute timeouts or activity ending
- **No frontend interfaces**: Admin cannot manage devices via web UI
- **No conflict detection**: Multiple teachers can interfere with same activity
- **No default PIN setup**: Existing teachers have no PINs set

### Additional Known Limitations (Design)
- No offline tag assignment
- No PIN recovery mechanism
- **No concurrent device handling**: One device per activity at a time
- **Manual coordination between teachers**: Teachers coordinate themselves
- **Substitute teachers must create new activities**: No activity sharing
- 3-second delay between scans handled by Pi app
- **No manual student entry**: All students must use RFID tags

## Not Included (Future Enhancements)

- WebSocket real-time updates
- Activity templates
- Complex device permissions
- Keyboard/search for students
- Multi-room activities
- Guest/visitor tracking
- Accessibility features
- Multi-language support

## Operational Details

### PIN Management
- **Mandatory**: Every teacher must have a PIN set
- Migration: Existing teachers get default PIN "0000"
- First login forces PIN change
- PINs are 4 digits only
- PIN is for access control only, not tracking

### Activity Lifecycle
- Activities shown: Today's and currently active only
- Immediate availability: Activities available right after creation
- Conflict handling: One device per activity, error message with override option
- 30-minute timeout: Any interaction resets timer
- Crash recovery: On restart, option to resume active sessions
- Auto-logout: Students automatically logged out when entering different room

### Device Health
- Devices ping every minute to show online status
- Dashboard shows device online/offline status
- Offline after 5 minutes without ping

### RFID Assignment Flow
1. New student created → assigned to group
2. Teacher assigns RFID tag via device
3. Only sees students from their supervised groups
4. Tag assignment behavior:
   - New assignment: Creates tag-student link
   - Student has existing tag: Old tag unlinked, new tag linked
   - Tag already assigned: Previous student unlinked, new student linked
   - One student = one tag at a time
   - One tag = one student at a time
5. **No Manual Entry**: All students must have RFID tags

### Troubleshooting Guide

#### Device Won't Connect
1. Check device has internet connection
2. Verify API key is correctly configured
3. Check server logs for authentication errors
4. Ensure device is not deactivated in admin panel

#### Teacher PIN Not Working
1. Verify teacher has PIN set (not null in database)
2. Check PIN hasn't been changed recently
3. Ensure teacher account is active
4. Try resetting PIN via web interface

#### RFID Tag Not Scanning
1. Check RFID reader hardware connection
2. Verify tag is compatible (NFC/RFID type)
3. Test with known working tag
4. Check device logs for scan errors

#### Device Shows Offline
1. Normal if no activity for 5+ minutes
2. Check network connectivity
3. Restart device to resume pinging
4. Check server logs for ping failures

## Pilot Success Criteria

1. Teachers can create activities on mobile
2. Devices authenticate with PIN
3. RFID tags can be assigned to students
4. Students can check in/out with taps
5. Dashboard shows attendance (5-min refresh)
6. Activities auto-end after 30 minutes
7. System works with intermittent network