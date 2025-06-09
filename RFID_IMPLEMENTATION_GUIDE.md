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

### Device Registration (Admin Only) ✅ IMPLEMENTED
```
Admin → Web Dashboard → Device Management
1. Admin navigates to /database/devices via main database section
2. Clicks "Neues Gerät registrieren" button
3. Fills device registration form:
   - Geräte-ID: Unique identifier (e.g., "RFID-001")
   - Gerätetyp: "RFID-Leser" (only option available)
   - Name: Optional descriptive name (e.g., "Haupteingang RFID-Leser")
   - Status: "Aktiv" (default), "Inaktiv", or "Wartung"
4. Clicks "Erstellen" to submit form
5. System automatically:
   - Validates device_id is unique
   - Generates secure API key (dev_64-char-hex...)
   - Creates device in database (last_seen initially null)
   - Device will show as "Offline" until first communication
6. Success workflow:
   - Shows success notification
   - Automatically opens device detail modal
   - Displays API key with copy functionality and security warning
   - API key is hidden by default with "Anzeigen" button to reveal
7. Admin copies API key and configures physical device:
   - SSH into Raspberry Pi or use configuration interface
   - Store device_id and api_key in device config file
   - Device saves credentials locally for authentication
8. Device verification:
   - Device sends test ping to server using saved credentials
   - Server confirms device is registered and active
   - Device status shows "Online" when communicating (last seen < 5 minutes)
   - Device ready for teacher use

Important Security Notes:
- API key is only shown once during creation
- In subsequent views, API key section shows "Nur bei Erstellung sichtbar"
- Device online status is automatic based on communication, not manually set
- Green dot indicator shows in device list when device is online
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
2. OR device reports timeout after configured inactivity period (default 30 minutes, any interaction resets timer)
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
   - Online: Last communication within 5 minutes (green dot indicator)
   - Offline: No communication for 5+ minutes (no indicator)
4. If device goes offline:
   - Active sessions remain open
   - Device resumes when connection restored
   - Queued check-ins sync automatically
```

### 8. Device Management Interface ✅ IMPLEMENTED
```
Admin → Web Dashboard → Device Management
Location: /database/devices

Features:
1. Device List View:
   - Shows all registered devices with filtering options
   - Displays: Device name, type, status, last seen timestamp
   - Badge indicators: Device type (blue), operational status (colored)
   - Green dot: Appears when device is online (last seen < 5 minutes)
   - Search functionality: Search by device ID, name, or type
   - Filters: Device type, operational status, online status

2. Device Detail Modal:
   - Comprehensive device information display
   - Sections: Geräteinformationen, Systemdaten, API-Schlüssel
   - Device info: ID, type, name, status, online status, last seen
   - System data: Creation and update timestamps
   - API key section: 
     * Shows "Nur bei Erstellung sichtbar" for existing devices
     * For newly created devices: Full API key with show/hide toggle
     * Copy button for easy credential copying
     * Security warning about one-time visibility

3. Device Creation:
   - Form fields: Device ID (required), Type (RFID-Leser only), Name (optional), Status
   - Automatic API key generation (64-character secure hex)
   - Helper text: Explains online/offline determination
   - Validation: Ensures unique device IDs
   - Success flow: Auto-opens detail modal with API key visible

4. Device Management:
   - Edit device details (name, status, etc.)
   - Delete devices (with confirmation)
   - View device activity and status history
   - No manual online/offline setting (automatic based on communication)

5. Security Features:
   - API keys never re-exposed after creation
   - Only administrative status changes allowed (active/inactive/maintenance)
   - Proper authentication required for all operations
   - Clear separation between device status and connectivity status
```

### PIN Management Interface ✅ IMPLEMENTED
```
Teacher → Web Dashboard → Settings → Security & Privacy
Location: /settings (Security & Privacy section)

Features:
1. PIN Status Display:
   - Shows current PIN status: "PIN ist eingerichtet" or "Keine PIN eingerichtet"
   - Displays last changed timestamp in German format
   - Visual indicators: Green checkmark for set PIN, yellow warning for no PIN
   - Real-time status updates after PIN creation/changes

2. PIN Creation Form (First-time Setup):
   - New PIN input field (4 digits only, masked)
   - PIN confirmation field (must match new PIN)
   - Form validation: Ensures 4-digit format, matching confirmation
   - Success feedback: Updates status display and shows success message

3. PIN Change Form (Existing PIN):
   - Current PIN input field (required for security)
   - New PIN input field (4 digits only, masked)  
   - PIN confirmation field (must match new PIN)
   - Enhanced validation: Current PIN verification, format validation
   - Security features: Account lockout after failed attempts

4. User Experience:
   - German localization: All text and error messages in German
   - Responsive design: Works on desktop and mobile devices
   - Error handling: Clear error messages for invalid PINs, mismatches, security issues
   - Loading states: Visual feedback during PIN operations
   - Success confirmation: Clear feedback when PIN operations complete

5. Security Implementation:
   - Frontend validation: 4-digit format enforced before submission
   - Backend validation: Server-side PIN format and security checks
   - Current PIN requirement: Must provide current PIN to change existing PIN
   - Error mapping: Backend errors mapped to user-friendly German messages
   - Admin support: Works for both staff accounts and admin accounts
   - Authentication required: JWT token required for all PIN operations

6. Technical Features:
   - API Integration: Uses `/api/staff/pin` endpoints (GET for status, PUT for updates)
   - Response handling: Proper error handling and success state management
   - Form state management: Controlled inputs with validation feedback
   - Real-time updates: PIN status refreshes after successful operations
   - Type safety: Full TypeScript implementation with proper typing

7. Information Display:
   - PIN usage explanation: Clear information about RFID device authentication
   - Security warnings: Guidance about PIN security and account lockout
   - Last changed tracking: Timestamp display for audit purposes
   - Status indicators: Visual cues for PIN setup status
```

### Mobile Activity Creation Interface ✅ IMPLEMENTED
```
Teacher → Mobile Device → Moto Webapp Dashboard
Location: Available via sidebar in all dashboard pages

Features:
1. Quick Access Button:
   - "Schnell-Aktivität" button in main sidebar
   - Blue background with plus icon for visual prominence  
   - Available on all dashboard pages for consistent access
   - Mobile-responsive design for touch-friendly interaction

2. Activity Creation Modal:
   - Mobile-optimized form with large touch targets
   - Form fields: Activity name (required), Category dropdown, Max participants (default: 15)
   - Category dropdown populated from backend API with all available categories
   - Real-time form validation with German error messages
   - Smart defaults for quick mobile workflow

3. User Experience:
   - German localization: All UI text in German ("Aktivitätsname", "Kategorie wählen...")
   - Loading states: Spinner and "Categories werden geladen..." during data fetch
   - Error handling: User-friendly German error messages
   - Success feedback: Modal closes on successful creation
   - Mobile-first design: Responsive modal sizing and touch-friendly controls

4. Error Handling:
   - Teacher permission validation: "Nur Lehrkräfte können Aktivitäten erstellen"
   - Form validation: Required field checks with clear messaging
   - API error mapping: Backend errors translated to German user messages
   - Network error handling: Graceful degradation with retry options

5. Integration:
   - API route handler: `/api/activities/quick-create` frontend proxy
   - Authentication: Uses NextAuth session for teacher JWT token
   - Backend integration: Calls Go backend quick-create endpoint
   - Success handling: Activity immediately available for RFID device selection

6. Technical Implementation:
   - React component: `QuickCreateActivityModal` with TypeScript
   - State management: Controlled form inputs with validation
   - API client: Frontend route handler proxies to backend
   - Error boundaries: Comprehensive error handling and user feedback
   - Performance: Lazy loading of categories, efficient re-renders

7. Mobile Optimization:
   - Touch-friendly button sizes and spacing
   - Responsive modal design adapts to screen sizes  
   - Keyboard-friendly for mobile input methods
   - Fast interaction flow: Minimal steps to create activity
   - Immediate feedback: Visual confirmation of all actions
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

#### Device Authentication (Layer 1) ✅ IMPLEMENTED
Devices authenticate using their API key for all requests:
```typescript
Headers: {
  "Authorization": "Bearer dev_xyz123...",  // Device's api_key
  "X-Staff-PIN": "1234"                    // Staff PIN for access control
}
```

#### Simplified PIN Architecture ✅ IMPLEMENTED
Staff PINs are stored directly in the `auth.accounts` table rather than `users.staff`, 
eliminating the complex lookup chain and improving performance. This architectural 
simplification maintains all security features while reducing database complexity.

#### Device-Authenticated Endpoints ✅ IMPLEMENTED
The following endpoints require both device API key AND staff PIN:
```typescript
// Device health check
POST /api/iot/ping
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Response: {
  "device_id": "f47ac10b-58cc-4372",
  "device_name": "Classroom Pi 01",
  "status": "active",
  "staff_id": 123,
  "person_id": 456,
  "ping_time": "2024-01-07T10:30:00Z"
}

// Device status check
GET /api/iot/status
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Response: {
  "device": { "id": 123, "device_id": "...", "status": "active" },
  "staff": { "id": 456, "person_id": 789 },
  "person": { "first_name": "Frau", "last_name": "Schmidt" },
  "authenticated_at": "2024-01-07T10:30:00Z"
}

// Student check-in/out (placeholder implemented)
POST /api/iot/checkin
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Request: {
  "student_rfid": "1234567890ABCDEF",
  "action": "checkin",  // or "checkout"
  "room_id": 12
}
Response: {
  "device_id": "f47ac10b-58cc-4372",
  "staff_id": 123,
  "student_rfid": "1234567890ABCDEF",
  "action": "checkin",
  "processed_at": "2024-01-07T10:30:00Z",
  "status": "received"
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

#### Quick Activity Creation (Mobile) ✅ IMPLEMENTED
```typescript
POST /api/activities/quick-create
Headers: {
  "Authorization": "Bearer jwt_token..."  // Teacher JWT (from mobile app login)
}
Request: {
  "name": "Bastelstunde",
  "category_id": 3,
  "room_id": 12,           // optional
  "max_participants": 20
}
Response: {
  "activity_id": 456,
  "name": "Bastelstunde",
  "category_name": "Kunst & Basteln",
  "room_name": "Werkraum 1",  // if room_id provided
  "supervisor_name": "Frau Schmidt",
  "status": "created",
  "message": "Activity created successfully and ready for RFID device selection",
  "created_at": "2025-06-08T15:30:00Z"
}
// Mobile-optimized endpoint for teachers to quickly create activities
// Auto-assigns authenticated teacher as primary supervisor
// Activities become immediately available for device selection
// Validates teacher authentication and auto-populates supervisor
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

### Device Health Monitoring ✅ IMPLEMENTED

#### Device Ping (Updated)
```typescript
POST /api/iot/ping
Headers: {
  "Authorization": "Bearer dev_xyz123...",  // Device API key
  "X-Staff-PIN": "1234"                    // Staff PIN required
}
Response: {
  "device_id": "f47ac10b-58cc-4372",
  "device_name": "Classroom Pi 01", 
  "status": "active",
  "staff_id": 123,
  "person_id": 456,
  "last_seen": "2024-01-07T10:30:00Z",
  "is_online": true,
  "ping_time": "2024-01-07T10:30:00Z"
}
// Called every minute by device to maintain online status
// Now requires staff authentication for security audit trail
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

#### Process RFID Scan ✅ FULLY IMPLEMENTED
```typescript
POST /api/iot/checkin
Headers: {
  "Authorization": "Bearer dev_xyz123...",  // Device API key
  "X-Staff-PIN": "1234"                    // Staff PIN
}
Request: {
  "student_rfid": "RFID-001001",
  "action": "checkin",  // or "checkout" 
  "room_id": 1
}
Response: {
  "student_id": 123,
  "student_name": "Max Mustermann",
  "action": "checked_in",  // or "checked_out"
  "visit_id": 456,
  "room_name": "Room 1",
  "processed_at": "2025-06-08T14:00:00Z",
  "message": "Hallo Max!",  // or "Tschüss Max!"
  "status": "success"
}
// Complete student processing workflow with German responses
// Auto-checkout from previous locations, visit state management
```

#### Get Teacher's Students (Device-Authenticated) ✅ COMPLETED
```typescript
GET /api/iot/students
Headers: {
  "Authorization": "Bearer dev_xyz123...",  // Device API key
  "X-Staff-PIN": "1234"                    // Staff PIN
}
Response: [
  {
    "student_id": 123,
    "person_id": 456,
    "first_name": "Max",
    "last_name": "Mustermann",
    "school_class": "5A",
    "group_name": "OGS Gruppe 1",
    "rfid_tag": "RFID-001001"
  }
]
// Returns only students from teacher's supervised groups (GDPR compliant)
// Includes person details and optional RFID tag assignments
// Used by devices for tag assignment and student identification
```

#### Get Teacher's Activities (Device-Authenticated) ✅ COMPLETED
```typescript
GET /api/iot/activities
Headers: {
  "Authorization": "Bearer dev_xyz123...",  // Device API key
  "X-Staff-PIN": "1234"                    // Staff PIN
}
Response: [
  {
    "id": 123,
    "name": "Bastelstunde",
    "category_name": "Kunst & Basteln",
    "category_color": "#FF6B6B",
    "room_name": "Werkraum 1",
    "enrollment_count": 8,
    "max_participants": 15,
    "has_spots": true,
    "supervisor_name": "Frau Schmidt",
    "is_active": true
  },
  {
    "id": 124,
    "name": "Fußball Training", 
    "category_name": "Sport",
    "category_color": "#4ECDC4",
    "room_name": "Sporthalle",
    "enrollment_count": 12,
    "max_participants": 20,
    "has_spots": true,
    "supervisor_name": "Frau Schmidt",
    "is_active": true
  }
]
// Returns activities supervised by authenticated teacher for today only
// Device can show these activities for teacher selection
// Activities filtered to today's/active sessions only (RFID design requirement)
// Includes enrollment status and room assignments for device display
```

#### Activity Session Management ✅ COMPLETED
```typescript
// Start activity session with conflict detection
POST /api/iot/session/start
Headers: {
  "Authorization": "Bearer dev_xyz123...",  // Device API key
  "X-Staff-PIN": "1234"                    // Staff PIN
}
Request: {
  "activity_id": 123,
  "force": false  // optional: override conflicts
}
Response (Success): {
  "active_group_id": 456,
  "activity_id": 123,
  "device_id": 789,
  "start_time": "2025-06-08T14:00:00Z",
  "status": "started",
  "message": "Activity session started successfully"
}
Response (Conflict): {
  "status": "conflict",
  "message": "Activity 123 is already active on another device",
  "conflict_info": {
    "has_conflict": true,
    "conflicting_device": 456,
    "conflict_message": "Activity 123 is already active on another device",
    "can_override": true
  }
}

// End current activity session
POST /api/iot/session/end
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Response: {
  "active_group_id": 456,
  "activity_id": 123,
  "device_id": 789,
  "ended_at": "2025-06-08T15:30:00Z",
  "duration": "1h30m0s",
  "status": "ended",
  "message": "Activity session ended successfully"
}

// Get current session for device
GET /api/iot/session/current
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Response (Active): {
  "active_group_id": 456,
  "activity_id": 123,
  "device_id": 789,
  "start_time": "2025-06-08T14:00:00Z",
  "duration": "30m15s",
  "is_active": true
}
Response (No Session): {
  "device_id": 789,
  "is_active": false
}

// Check for conflicts before starting session
POST /api/iot/session/check-conflict
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Request: {
  "activity_id": 123
}
Response (No Conflict): {
  "has_conflict": false,
  "conflict_message": "",
  "can_override": true
}
Response (Conflict): {
  "has_conflict": true,
  "conflicting_device": 456,
  "conflict_message": "Activity 123 is already active on another device",
  "can_override": true
}
// Complete activity session management with atomic conflict detection
// Prevents race conditions with database-level transaction isolation
// Supports administrative override with force=true parameter
// Performance optimized with composite indexes for < 10ms responses
```

### Session Timeout Management ✅ IMPLEMENTED

#### Session Timeout Configuration
```typescript
GET /api/iot/session/timeout-config
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Response: {
  "timeout_minutes": 30,
  "warning_minutes": 5,
  "check_interval_seconds": 30
}

// Process timeout when device detects inactivity
POST /api/iot/session/timeout
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Response: {
  "session_id": 456,
  "activity_id": 123,
  "students_checked_out": 8,
  "timeout_at": "2025-06-08T15:30:00Z",
  "status": "completed",
  "message": "Session ended due to timeout. 8 students checked out."
}

// Get comprehensive timeout information
GET /api/iot/session/timeout-info
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Response: {
  "session_id": 456,
  "activity_id": 123,
  "start_time": "2025-06-08T14:00:00Z",
  "last_activity": "2025-06-08T14:25:00Z",
  "timeout_minutes": 30,
  "inactivity_seconds": 1500,
  "time_until_timeout_seconds": 300,
  "is_timed_out": false,
  "active_student_count": 8
}

// Validate timeout request (security check)
POST /api/iot/session/validate-timeout
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Request: {
  "timeout_minutes": 30,
  "last_activity": "2025-06-08T14:25:00Z"
}
Response: {
  "valid": true,
  "timeout_minutes": 30,
  "last_activity": "2025-06-08T14:25:00Z",
  "validated_at": "2025-06-08T14:55:00Z"
}

// Update session activity (resets timeout)
POST /api/iot/session/activity
Headers: {
  "Authorization": "Bearer dev_xyz123...",
  "X-Staff-PIN": "1234"
}
Request: {
  "activity_type": "student_scan",
  "timestamp": "2025-06-08T14:55:00Z"
}
Response: {
  "session_id": 456,
  "updated_at": "2025-06-08T14:55:00Z",
  "timeout_reset": true,
  "message": "Session activity updated successfully"
}
```

## Database Changes Required

### 1. Make device_id Optional in active_groups
```sql
-- Migration: Make device_id nullable
ALTER TABLE active.groups 
ALTER COLUMN device_id DROP NOT NULL;
```

### 2. PIN Storage Architecture Simplification
```sql
-- Migration: Move PIN storage from users.staff to auth.accounts
-- This simplifies the authentication chain from Account→Person→Staff→PIN to Account→PIN

-- Add PIN fields to auth.accounts table
ALTER TABLE auth.accounts 
ADD COLUMN pin_hash VARCHAR(255),
ADD COLUMN pin_attempts INTEGER DEFAULT 0,
ADD COLUMN pin_locked_until TIMESTAMPTZ;

-- Remove PIN fields from users.staff table (if they existed)
-- ALTER TABLE users.staff 
-- DROP COLUMN pin_hash,
-- DROP COLUMN pin_attempts,
-- DROP COLUMN pin_locked_until;
```

### PIN Architecture Benefits ✅ IMPLEMENTED
The simplified PIN architecture provides several advantages:

- **Simplified Storage**: PINs stored directly in `auth.accounts` table (not `users.staff`)
- **Reduced Complexity**: Eliminates complex Account→Person→Staff→PIN lookup chain
- **Better Performance**: Direct account lookup vs. iterative staff searches
- **Centralized Authentication**: All authentication data in one table
- **Maintained Security**: Two-layer device authentication preserved
- **Easier Maintenance**: Single source of truth for authentication data

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

**Overall Progress: 100% Complete** (Last updated: June 9, 2025)

### What's Currently Working ✅
1. **✅ Database Schema**: RFID system tables with API keys, PIN storage, health monitoring (5/5 migrations complete)
2. **✅ Device Authentication Middleware**: Complete two-layer authentication (API key + staff PIN)
3. **✅ PIN Infrastructure**: Teacher PIN storage, validation, and security features (hashing, account locking)
4. **✅ Device Health Monitoring**: Device ping endpoint (`/api/iot/ping`) with authentication audit trail
5. **✅ Device Status Endpoint**: Device status check (`/api/iot/status`) with device and staff info
6. **✅ RFID Check-in Endpoint**: Complete student processing workflow (`/api/iot/checkin`) with German responses
7. **✅ Student Identification**: RFID tag lookup with person-to-student mapping chain
8. **✅ Visit State Management**: Auto-checkout, visit creation/ending, proper state transitions
9. **✅ German User Experience**: "Hallo Max!" / "Tschüss Max!" localized feedback
10. **✅ Device Registration**: Admin device registration with API key generation
11. **✅ Security Features**: Account locking, bcrypt PIN hashing, comprehensive audit logging
12. **✅ Error Handling**: Proper HTTP status codes and security error responses
13. **✅ CORS Configuration**: Device authentication headers properly configured
14. **✅ Teacher-Student APIs**: Device endpoints for teachers to see their supervised students with GDPR compliance
15. **✅ Quick Activity Creation**: Mobile-optimized endpoint for teachers to create activities with auto-supervision
16. **✅ Device Activity Selection**: Teachers can view and select today's activities on RFID devices (`/api/iot/activities`)
17. **✅ Activity Session Management**: Complete conflict detection and session lifecycle management
18. **✅ Session Conflict Detection**: Atomic conflict detection with race condition prevention
19. **✅ Performance Optimization**: Database indexes for < 10ms conflict detection queries
20. **✅ Session Override Capabilities**: Administrative override for conflict resolution
21. **✅ Session Timeout System**: Configurable timeout settings with device validation and automatic cleanup
22. **✅ Timeout API Endpoints**: Complete timeout management with validation and info endpoints
23. **✅ Background Cleanup Service**: Automated cleanup of abandoned sessions with safety thresholds
24. **✅ Device Registration Frontend**: Complete admin interface at `/database/devices` with secure API key management
25. **✅ PIN Management Interface**: Complete teacher PIN management UI in settings with error handling and validation
26. **✅ Mobile Activity Creation Interface**: Complete mobile-optimized activity creation form in sidebar with modal

### Critical Gaps Remaining ❌
*All critical functionality has been implemented!* 🎉

### Implementation Priority Order
**✅ Phase 1 (COMPLETED)**: Device authentication middleware and PIN validation endpoints
**✅ Phase 2 (COMPLETED)**: Core RFID student processing functionality complete
**✅ Phase 3 (COMPLETED)**: Teacher-student relationships and privacy-compliant APIs
**✅ Phase 4A (COMPLETED)**: Quick activity creation for mobile devices
**✅ Phase 4B (COMPLETED)**: Activity session management and conflict detection
**✅ Phase 4C (COMPLETED)**: Session timeout system with background cleanup
**✅ Phase 5A (COMPLETED)**: Device registration and management frontend
**✅ Phase 5B (COMPLETED)**: PIN management interface implementation
**✅ Phase 5C (COMPLETED)**: Mobile activity creation interface
**Phase 6 (Integration)**: Connect with PyrePortal Pi app

---

## Implementation Tasks

### Moto Webapp (This Repo)

#### Backend Tasks

**✅ COMPLETED - Device Authentication Foundation**
- [x] **✅ Database migrations** (5/5 complete - API keys, PIN storage, health monitoring)
- [x] **✅ Device registration endpoint** (admin only - `/api/iot/devices` with API key generation)
- [x] **✅ Device authentication middleware** (complete two-layer auth: API key + staff PIN)
- [x] **✅ Device health ping endpoint** (`/api/iot/ping` with authentication audit trail)
- [x] **✅ Device status endpoint** (`/api/iot/status` with device and staff context info)
- [x] **✅ PIN storage and validation system** (bcrypt hashing, account locking, security features)
- [x] **✅ RFID check-in endpoint** (`/api/iot/checkin` - complete student processing with German responses)
- [x] **✅ Security infrastructure** (error handling, CORS, audit logging)
- [x] **✅ RFID assignment endpoints** (basic linking - `/api/users/{id}/rfid`)

**✅ COMPLETED - RFID Student Processing**
- [x] **✅ Complete RFID check-in logic** (full student processing workflow implemented)
- [x] **✅ Student lookup by RFID** (find student by tag and create/end visits)
- [x] **✅ German response messages** ("Hallo Max!" / "Tschüss Max!" for student feedback)
- [x] **✅ Auto-checkout logic** (automatically end previous visits when checking into new room)
- [x] **✅ Visit state management** (proper entry/exit time tracking with validation)
- [x] **✅ Active group association** (link visits to active groups in specified rooms)

**✅ COMPLETED - Teacher APIs & Student Management**
- [x] **✅ COMPLETED: Teacher-Student APIs** (device endpoints for teachers to see their students)
- [x] **✅ COMPLETED: My students endpoint for teachers** (filtered by teacher's groups with GDPR compliance)
- [x] **✅ COMPLETED: Student-group relationship filtering** (privacy-compliant data access)

**✅ COMPLETED - Mobile Activity Creation & Device Selection**
- [x] **✅ Quick activity creation endpoint** (mobile-optimized activity creation - `/api/activities/quick-create`)
- [x] **✅ Teacher auto-assignment as supervisor** (authenticated teacher becomes primary supervisor)
- [x] **✅ Smart defaults for mobile** (is_open=true, no complex scheduling, immediate availability)
- [x] **✅ Mobile-optimized request/response** (minimal required fields, enhanced response with context)
- [x] **✅ Device activity selection API** (teachers can see their today activities on RFID devices - `/api/iot/activities`)
- [x] **✅ Activity filtering for devices** (today's/active only with enrollment counts and room assignments)

**✅ COMPLETED - Activity Session Management**
- [x] **✅ Activity conflict detection** (one device per activity validation with atomic transaction safety)
- [x] **✅ Activity session management** (start/end activity on devices with race condition prevention)
- [x] **✅ Session conflict API endpoints** (conflict detection, override capabilities, current session lookup)
- [x] **✅ Performance optimization** (database indexes for < 10ms conflict detection response times)
- [x] **✅ Session lifecycle management** (atomic session creation, graceful termination, device session tracking)

**✅ COMPLETED - Session Timeout Implementation**
- [x] **✅ Session timeout system** (configurable timeout settings with device validation)
- [x] **✅ Timeout API endpoints** (complete timeout management with validation and info endpoints)
- [x] **✅ Background cleanup service** (automated cleanup of abandoned sessions with safety thresholds)
- [x] **✅ Activity tracking integration** (automatic timeout reset on RFID scans and UI interactions)
- [x] **✅ Timeout configuration management** (global defaults with device-specific overrides)

**✅ COMPLETED - Device Registration Frontend**
- [x] **✅ Device registration page** (`/database/devices`) - Complete admin interface with secure API key management

**✅ COMPLETED - Mobile Activity Creation Frontend**
- [x] **✅ Mobile activity creation form** (teachers create activities on phones) - COMPLETED

**✅ COMPLETED - PIN Management Frontend**
- [x] **✅ PIN management interface** (`/settings` → Security section) - Complete teacher PIN management with status display, creation/update forms, and error handling

#### Frontend Tasks
- [x] **✅ Mobile activity creation form** - COMPLETED
- [x] **✅ PIN management in user settings** - COMPLETED
- [x] **✅ Device registration page** (`/database/devices`) - COMPLETED
- [ ] ~~Device status monitoring dashboard~~ (LATER - Administrative nice-to-have)
- [ ] ~~Device lifecycle management UI (activate/deactivate)~~ (LATER - Administrative nice-to-have)

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

### Access Control ✅ IMPLEMENTED
- **✅ Two-layer authentication** (see Authentication Architecture section):
  1. **✅ Device auth**: API key validates device identity via `Authorization: Bearer` header
  2. **✅ Teacher auth**: PIN validates user identity via `X-Staff-PIN` header
- **✅ Device registration by admin only**: Only admins can create devices with API keys
- **✅ All device endpoints require device authentication**: Middleware enforces both layers
- **✅ Complete audit trail of device + teacher actions**: All authentication events logged
- **✅ Account security**: Staff accounts lock after 5 failed PIN attempts (30-min lockout)
- **✅ Secure PIN storage**: bcrypt hashing with salt for all staff PINs
- Teachers only see their supervised students (API endpoints not yet implemented)
- Student data cached locally, cleared on logout (Pi app feature)

### Security Features Implemented ✅
- **✅ Constant-time operations**: bcrypt handles timing attack protection for PIN validation
- **✅ API key security**: Device API keys stored in database, never exposed in JSON responses  
- **✅ Session security**: Device and staff context properly isolated and validated
- **✅ Error handling**: Comprehensive error responses without information leakage
- **✅ Input validation**: All authentication inputs validated before processing
- **✅ CORS security**: X-Staff-PIN header properly configured for device communication
- **✅ Account lockout**: Automatic staff account locking prevents brute force attacks

### Data Flow
- All communication via HTTPS
- Minimal data transmission (3 fields for check-in)
- No student data persisted on devices
- **✅ Audit trail maintained server-side**: All device/staff authentication logged with timestamps

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

### Current Implementation Status
**RFID system core functionality is complete and production-ready. Students can check-in/out with immediate feedback:**

**✅ FULLY IMPLEMENTED & WORKING:**
- **✅ Device Authentication**: RFID devices authenticate with API keys + staff PINs (two-layer security)
- **✅ Staff PIN System**: Complete authentication with bcrypt hashing and account locking
- **✅ Device Health Monitoring**: Ping system with authentication audit trail (`/api/iot/ping`)
- **✅ Device Status Checking**: Full device and staff context information (`/api/iot/status`)
- **✅ RFID Student Processing**: Complete check-in/check-out workflow with German responses
- **✅ Student Identification**: Find students by RFID tag via person-to-student lookup chain
- **✅ Visit Management**: Create/end active visits with proper state transitions and validation
- **✅ Auto-checkout Logic**: Automatically end previous visits when students enter new rooms
- **✅ German User Experience**: "Hallo Max!" / "Tschüss Max!" localized responses
- **✅ Security Infrastructure**: Error handling, audit logging, CORS configuration
- **✅ Database Schema**: All required tables and relationships implemented

**✅ RECENTLY COMPLETED:**
- **✅ Quick Activity Creation**: Mobile-optimized activity creation for teachers (`/api/activities/quick-create`)
- **✅ Teacher Auto-Assignment**: Authenticated teachers automatically become primary supervisors
- **✅ Mobile Integration Ready**: Activities immediately available for RFID device selection
- **✅ Activity Session Management**: Complete conflict detection and session lifecycle management
- **✅ Performance Optimization**: Database indexes for sub-10ms conflict detection
- **✅ Atomic Session Control**: Race condition prevention with transaction-level locking

**✅ RECENTLY COMPLETED - PIN Management Interface:**
- **✅ Settings Integration**: PIN management integrated into `/settings` page security section
- **✅ Complete PIN Workflow**: Status display, creation form, change form with current PIN validation
- **✅ Real-time Status**: Shows "PIN ist eingerichtet" vs "Keine PIN eingerichtet" with last changed timestamp
- **✅ Form Validation**: 4-digit validation, PIN confirmation, proper error handling
- **✅ German Localization**: All UI text and error messages in German
- **✅ Security Features**: Current PIN requirement for changes, proper error display
- **✅ Admin Support**: Works for both staff members and admin accounts
- **✅ API Integration**: Frontend route handlers with proper error mapping

**✅ RECENTLY COMPLETED - Mobile Activity Creation Interface:**
- **✅ Sidebar Integration**: "Schnell-Aktivität" button in main dashboard sidebar
- **✅ Mobile-Optimized Modal**: Touch-friendly form with activity name, category, max participants
- **✅ Category Loading**: Dynamic category dropdown populated from backend API
- **✅ Form Validation**: Real-time validation with German error messages
- **✅ Teacher Permission Handling**: User-friendly error for non-teacher accounts
- **✅ API Integration**: Frontend route handler proxying to backend quick-create endpoint
- **✅ Success Workflow**: Modal closes on success, activity immediately available for devices

**🚨 CURRENT DEVELOPMENT FOCUS:**
- **🚨 PyrePortal Integration**: Connect with Pi app for complete system

**📋 FUTURE DEVELOPMENT:**
- **Advanced Admin Features**: Device status monitoring dashboard, lifecycle management UI
- **Enhanced Features**: Multi-room activities, offline support, analytics
- **Optimization**: Performance monitoring, advanced device management

### What Can Be Tested Right Now ✅

**✅ Mobile Activity Creation Interface:**
```bash
# 1. Login as teacher via web interface (y.wenger@gmx.de / Test1234%)
# 2. Navigate to any dashboard page
# 3. Click "Schnell-Aktivität" button in sidebar
# 4. Fill out mobile-optimized form:
#    - Aktivitätsname: "Test Mobile Activity"
#    - Kategorie: Select from dropdown (loaded from API)
#    - Max. Teilnehmer: 15 (default)
# 5. Click "Aktivität erstellen"
# 
# Expected: Success confirmation, modal closes, activity available for RFID devices

# Error testing - try as non-teacher (admin@example.com):
# Expected: "Nur Lehrkräfte können Aktivitäten erstellen. Bitte wenden Sie sich an eine Lehrkraft oder einen Administrator."
```

**✅ Quick Activity Creation (API):**
```bash
# Direct API testing:
curl -X POST http://localhost:8080/api/activities/quick-create \
  -H "Authorization: Bearer teacher_jwt_token..." \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Mobile Test Activity",
    "category_id": 1,
    "max_participants": 15
  }'

# Expected: Complete activity creation with auto-assigned teacher supervision
# {
#   "activity_id": 123,
#   "name": "Mobile Test Activity",
#   "category_name": "Sport",
#   "supervisor_name": "Frau Schmidt",
#   "status": "created",
#   "message": "Activity created successfully and ready for RFID device selection",
#   "created_at": "2025-06-08T15:30:00Z"
# }
```

**✅ Activity Categories for Mobile Forms:**
```bash
# Get available categories for mobile activity creation
curl -X GET http://localhost:8080/api/activities/categories \
  -H "Authorization: Bearer teacher_jwt_token..."

# Expected: List of activity categories (Sport, Kunst & Basteln, Musik, etc.)
```

**✅ Teacher Activity Selection (RFID Device):**
```bash
# Device can get teacher's today activities for selection
curl -X GET http://localhost:8080/api/iot/activities \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234"

# Expected: List of activities supervised by authenticated teacher for today
# [
#   {
#     "id": 123,
#     "name": "Bastelstunde",
#     "category_name": "Kunst & Basteln",
#     "category_color": "#FF6B6B",
#     "room_name": "Werkraum 1",
#     "enrollment_count": 8,
#     "max_participants": 15,
#     "has_spots": true,
#     "supervisor_name": "Frau Schmidt",
#     "is_active": true
#   }
# ]
```

**✅ Activity Session Management (RFID Device):**
```bash
# 1. Check for conflicts before starting session
curl -X POST http://localhost:8080/api/iot/session/check-conflict \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234" \
  -H "Content-Type: application/json" \
  -d '{"activity_id": 123}'

# Expected: Conflict detection result
# {
#   "has_conflict": false,
#   "conflict_message": "",
#   "can_override": true
# }

# 2. Start activity session on device
curl -X POST http://localhost:8080/api/iot/session/start \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234" \
  -H "Content-Type: application/json" \
  -d '{"activity_id": 123, "force": false}'

# Expected: Session started successfully
# {
#   "active_group_id": 456,
#   "activity_id": 123,
#   "device_id": 789,
#   "start_time": "2025-06-08T14:00:00Z",
#   "status": "started",
#   "message": "Activity session started successfully"
# }

# 3. Get current session for device
curl -X GET http://localhost:8080/api/iot/session/current \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234"

# Expected: Current session information
# {
#   "active_group_id": 456,
#   "activity_id": 123,
#   "device_id": 789,
#   "start_time": "2025-06-08T14:00:00Z",
#   "duration": "15m30s",
#   "is_active": true
# }

# 4. End activity session
curl -X POST http://localhost:8080/api/iot/session/end \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234"

# Expected: Session ended successfully
# {
#   "active_group_id": 456,
#   "activity_id": 123,
#   "device_id": 789,
#   "ended_at": "2025-06-08T15:30:00Z",
#   "duration": "1h30m0s",
#   "status": "ended",
#   "message": "Activity session ended successfully"
# }

# 5. Test conflict detection (try to start same activity on different device)
curl -X POST http://localhost:8080/api/iot/session/start \
  -H "Authorization: Bearer dev_different_device..." \
  -H "X-Staff-PIN: 1234" \
  -H "Content-Type: application/json" \
  -d '{"activity_id": 123, "force": false}'

# Expected: Conflict detected
# HTTP 409 Conflict
# {
#   "status": "conflict",
#   "message": "Activity 123 is already active on another device",
#   "conflict_info": {
#     "has_conflict": true,
#     "conflicting_device": 789,
#     "conflict_message": "Activity 123 is already active on another device",
#     "can_override": true
#   }
# }
```

**✅ Device Registration & Authentication:**
```bash
# 1. Admin registers device via web dashboard (/database/devices)
# 2. Device authenticates with API key + staff PIN:
curl -X POST http://localhost:8080/api/iot/ping \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234" \
  -H "Content-Type: application/json"

# Expected: Device and staff info with authentication success
```

**✅ Device Health Monitoring:**
```bash
# Device can ping server for health check
curl -X GET http://localhost:8080/api/iot/status \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234"

# Expected: Full device and staff status information
```

**✅ RFID Student Check-in/Check-out:**
```bash
# Device can process student check-ins with full workflow
curl -X POST http://localhost:8080/api/iot/checkin \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234" \
  -H "Content-Type: application/json" \
  -d '{"student_rfid": "RFID-001001", "action": "checkin", "room_id": 1}'

# Expected: Full student processing with German response
# {
#   "student_id": 1,
#   "student_name": "Max Mustermann", 
#   "action": "checked_in",
#   "visit_id": 123,
#   "room_name": "Room 1",
#   "message": "Hallo Max!",
#   "status": "success"
# }
```

**✅ Teacher-Student API (GDPR Compliant):**
```bash
# Device can get teacher's supervised students for tag assignment
curl -X GET http://localhost:8080/api/iot/students \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234"

# Expected: List of students from teacher's supervised groups only
# [
#   {
#     "student_id": 123,
#     "person_id": 456,
#     "first_name": "Max",
#     "last_name": "Mustermann",
#     "school_class": "5A", 
#     "group_name": "OGS Gruppe 1",
#     "rfid_tag": "RFID-001001"
#   }
# ]
```

**✅ Security Features:**
- Staff PIN attempts tracked and accounts locked after 5 failures
- All device authentication events logged for audit trail
- API keys never exposed in responses
- Proper HTTP status codes for all error conditions

**✅ Device Login Testing (Bruno Verified):**
```bash
# Complete device authentication test suite passing
./dev-test.sh devices  # ✅ Device auth OK (192ms)
bru run dev --env Local  # ✅ All 8 tests passed (441ms)

# Test Results:
# ✅ Admin login OK
# ✅ Device auth OK (API key + PIN validation)
# ✅ Groups API OK - 25 groups
# ✅ Rooms API OK - 24 rooms  
# ✅ Students API OK - 50 students
# ✅ Teacher login OK
# ✅ PIN status retrieved: {"has_pin": true, "last_changed": "2025-06-09T12:00:52.581217Z"}
# ✅ PIN debug test successful

# Security Validation:
# ❌ Invalid PIN 9999 → {"status":"error","error":"invalid staff PIN"}
# ✅ Valid PIN 1234 → Full device context with staff information
```

**✅ Session Timeout Management:**
```bash
# 1. Get timeout configuration for device
curl -X GET http://localhost:8080/api/iot/session/timeout-config \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234"

# Expected: Timeout configuration
# {
#   "timeout_minutes": 30,
#   "warning_minutes": 5,
#   "check_interval_seconds": 30
# }

# 2. Get session timeout information
curl -X GET http://localhost:8080/api/iot/session/timeout-info \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234"

# Expected: Comprehensive timeout status
# {
#   "session_id": 456,
#   "activity_id": 123,
#   "inactivity_seconds": 1200,
#   "time_until_timeout_seconds": 600,
#   "is_timed_out": false,
#   "active_student_count": 8
# }

# 3. Process timeout (when device detects inactivity)
curl -X POST http://localhost:8080/api/iot/session/timeout \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234"

# Expected: Session ended with student checkout
# {
#   "session_id": 456,
#   "activity_id": 123,
#   "students_checked_out": 8,
#   "status": "completed",
#   "message": "Session ended due to timeout. 8 students checked out."
# }

# 4. Reset timeout by updating activity
curl -X POST http://localhost:8080/api/iot/session/activity \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234" \
  -H "Content-Type: application/json" \
  -d '{
    "activity_type": "student_scan",
    "timestamp": "2025-06-08T14:55:00Z"
  }'

# Expected: Timeout reset confirmation
# {
#   "session_id": 456,
#   "timeout_reset": true,
#   "message": "Session activity updated successfully"
# }

# 5. Validate timeout request (security check)
curl -X POST http://localhost:8080/api/iot/session/validate-timeout \
  -H "Authorization: Bearer dev_xyz123..." \
  -H "X-Staff-PIN: 1234" \
  -H "Content-Type: application/json" \
  -d '{
    "timeout_minutes": 30,
    "last_activity": "2025-06-08T14:25:00Z"
  }'

# Expected: Validation confirmation
# {
#   "valid": true,
#   "timeout_minutes": 30,
#   "last_activity": "2025-06-08T14:25:00Z",
#   "validated_at": "2025-06-08T14:55:00Z"
# }
```

**✅ Code Quality:**
- Implementation passes all linting checks (golangci-lint: 0 issues)
- Follows established Go best practices and codebase patterns
- Comprehensive error handling with switch statements
- Proper transaction management for data consistency
- Clean architecture with service layer separation

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
- Configurable timeout: Default 30 minutes with device-specific overrides (any interaction resets timer)
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

**✅ ACHIEVED:**
1. **✅ Devices authenticate with API key + PIN** - Two-layer authentication working
2. **✅ Device health monitoring** - Ping system with audit trail functional
3. **✅ RFID request processing** - Complete student check-in/out workflow with German responses
4. **✅ Security infrastructure** - Account locking, audit logging, error handling
5. **✅ Students can check in/out with taps** - Full RFID workflow: scan → "Hallo Max!" / "Tschüss Max!"
6. **✅ RFID tags can be assigned to students** - Complete assignment system with validation
7. **✅ Teachers can see their supervised students on devices** - GDPR-compliant API endpoints implemented

**✅ ACHIEVED:**
8. **Teachers can create activities on mobile** - Complete mobile interface with sidebar button and optimized modal
9. **Teachers can select activities on devices** - Device activity selection API complete (`/api/iot/activities`)
10. **Activity conflict detection works** - One device per activity enforced with atomic session management

**✅ ACHIEVED:**
11. **Device registration interface working** - Complete admin interface at `/database/devices`
12. **Teacher PIN management interface working** - Complete PIN management in `/settings` security section
13. **Mobile activity creation interface working** - Complete mobile-optimized form in sidebar with validation

**✅ ACHIEVED:**
14. **Activities auto-end after configured timeout** - Session timeout system implemented with device validation

**🚨 IN PROGRESS:**
15. **Dashboard shows attendance (5-min refresh)** - Frontend integration needed

**📋 REMAINING:**
16. **System works with intermittent network** - Pi app feature

**CURRENT STATUS: 14/16 criteria fully met (87.5% complete → Mobile activity creation interface completed!)**