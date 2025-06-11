# RFID Implementation Guide

This guide details the complete implementation for RFID-based authentication and room management in Project Phoenix.

## Overview

The RFID system enables:
1. Device authentication for RFID readers
2. Staff authentication via RFID + PIN
3. Student check-in/check-out tracking
4. Room occupancy management
5. Automatic session handling

## Database Changes Required

### 1. Make device_id Optional in active.groups
```sql
-- Migration: Make device_id nullable
ALTER TABLE active.groups ALTER COLUMN device_id DROP NOT NULL;
```
**Rationale**: Allows creation of active groups through the web interface without requiring a device.

### 2. Add PIN Storage to Staff Table
```sql
-- Migration: Add PIN fields to users.staff
ALTER TABLE users.staff ADD COLUMN pin_hash VARCHAR(255);
ALTER TABLE users.staff ADD COLUMN pin_attempts INTEGER DEFAULT 0;
ALTER TABLE users.staff ADD COLUMN pin_locked_until TIMESTAMP WITH TIME ZONE;

-- Create index for PIN lookups
CREATE INDEX idx_staff_pin_locked ON users.staff(pin_locked_until) WHERE pin_locked_until IS NOT NULL;
```

### 3. Add Device Authentication to iot.devices
```sql
-- Migration: Add API key to devices
ALTER TABLE iot.devices ADD COLUMN api_key VARCHAR(64) UNIQUE;
ALTER TABLE iot.devices ADD COLUMN last_seen_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE iot.devices ADD COLUMN is_active BOOLEAN DEFAULT true;

-- Generate API keys for existing devices
UPDATE iot.devices SET api_key = encode(gen_random_bytes(32), 'hex') WHERE api_key IS NULL;

-- Make api_key NOT NULL after population
ALTER TABLE iot.devices ALTER COLUMN api_key SET NOT NULL;
```

### 4. Device Session Tracking (Optional for MVP)
```sql
-- Migration: Create device sessions table (can be deferred)
CREATE TABLE iot.device_sessions (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL REFERENCES iot.devices(id) ON DELETE CASCADE,
    session_token VARCHAR(64) UNIQUE NOT NULL,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_device_sessions_device ON iot.device_sessions(device_id);
CREATE INDEX idx_device_sessions_token ON iot.device_sessions(session_token);
CREATE INDEX idx_device_sessions_expires ON iot.device_sessions(expires_at);
```

### 5. Default PIN Migration for Existing Teachers
```sql
-- Migration: Set default PIN "0000" for all existing staff
UPDATE users.staff 
SET pin_hash = '$2a$10$' || encode(digest('0000' || 'default_salt', 'sha256'), 'base64')
WHERE pin_hash IS NULL;

-- Note: In production, use proper bcrypt hashing
-- This is a simplified example - actual implementation should use bcrypt
```

## Implementation Tasks

### Backend (Go) Implementation

#### 1. Device Authentication Endpoint
```go
// POST /api/iot/device/auth
type DeviceAuthRequest struct {
    DeviceID string `json:"device_id"`
    APIKey   string `json:"api_key"`
}

type DeviceAuthResponse struct {
    Token     string `json:"token"`
    ExpiresIn int    `json:"expires_in"`
}
```

#### 2. Staff Authentication Endpoint
```go
// POST /api/iot/staff/auth
type StaffAuthRequest struct {
    RFIDCard string `json:"rfid_card"`
    PIN      string `json:"pin"`
}

type StaffAuthResponse struct {
    StaffID   int64  `json:"staff_id"`
    Name      string `json:"name"`
    Token     string `json:"token"`
    ExpiresIn int    `json:"expires_in"`
}
```

#### 3. Student Check-in/out Endpoint
```go
// POST /api/iot/student/checkin
type StudentCheckinRequest struct {
    RFIDCard string `json:"rfid_card"`
    RoomID   int64  `json:"room_id"`
    // Staff authentication included in JWT
}

type StudentCheckinResponse struct {
    VisitID    int64  `json:"visit_id"`
    StudentID  int64  `json:"student_id"`
    Name       string `json:"name"`
    GroupID    int64  `json:"group_id"`
    GroupName  string `json:"group_name"`
    Action     string `json:"action"` // "checked_in" or "checked_out"
}
```

#### 4. PIN Management Endpoints
```go
// PUT /api/staff/{id}/pin (requires admin or self)
type ChangePINRequest struct {
    CurrentPIN string `json:"current_pin,omitempty"` // Required for self
    NewPIN     string `json:"new_pin"`
}
```

#### 5. Update Models and Services
- Add PIN fields to Staff model
- Add API key to Device model
- Create DeviceAuthService
- Update StaffService with PIN validation
- Add rate limiting for PIN attempts

### Frontend (Next.js) Implementation

#### 1. Device Management UI
- Add API key display/regeneration in device details
- Show last seen timestamp
- Add active/inactive toggle

#### 2. Staff PIN Management
- Add PIN change form in staff profile
- Show PIN lock status for admins
- Add "Reset PIN" admin action

#### 3. RFID Testing Interface (Admin Only)
- Simulate RFID card reads
- Test authentication flows
- View device logs

## Security Implementation

### Two-Layer Authentication
1. **Device Layer**: API key validates the RFID reader
2. **User Layer**: Staff JWT for operations requiring authorization

### Request Flow
```
RFID Reader → Device Auth → Device JWT → Staff Auth → Staff JWT → API Operations
```

### Security Headers Required
```
X-Device-Token: {device_jwt}
Authorization: Bearer {staff_jwt}
```

### PIN Security
- BCrypt hashing with cost factor 10
- Lock after 5 failed attempts
- 15-minute lockout period
- Admins can reset PINs and unlock accounts

### HTTPS Enforcement
- All device communication must use HTTPS
- Certificate pinning recommended for production

## Error Handling Approach (MVP)

### Happy Path Focus
For MVP, implement basic error responses without complex retry logic:

```go
// Simple error responses
400 Bad Request - Invalid input
401 Unauthorized - Authentication failed
403 Forbidden - Insufficient permissions
404 Not Found - Resource not found
429 Too Many Requests - Rate limit exceeded
500 Internal Server Error - Server error
```

### Device-Specific Errors
```json
{
  "error": "device_not_found",
  "message": "Device ID not recognized"
}

{
  "error": "pin_locked",
  "message": "PIN locked due to failed attempts",
  "locked_until": "2024-01-20T15:30:00Z"
}
```

## What's NOT Included (MVP Scope)

### 1. WebSocket Connections
- No real-time updates
- Devices must poll for status changes

### 2. Email Templates
- No PIN reset emails
- No device registration notifications

### 3. Complex Session Management
- No device session persistence
- No multi-device staff sessions

### 4. Advanced Features
- No offline mode
- No device grouping/zones
- No custom PIN policies
- No biometric support

### 5. Monitoring
- Basic logging only
- No device health dashboards
- No usage analytics

## Operational Details

### Device Registration Process
1. Admin creates device in web UI
2. System generates API key
3. API key displayed once (must be saved)
4. Configure RFID reader with device ID and API key

### PIN Reset Process
1. Staff requests PIN reset from admin
2. Admin resets PIN to default "0000"
3. Staff must change PIN on next login

### Session Timeouts
- Device JWT: 24 hours
- Staff JWT: 1 hour
- No automatic renewal in MVP

### Rate Limits
- Device auth: 10 attempts per hour
- PIN attempts: 5 per 15 minutes
- API calls: 100 per minute per device

### Default Configuration
- Default staff PIN: "0000"
- PIN minimum length: 4 digits
- Session timeout: 1 hour
- Device timeout: 24 hours

## Testing Strategy

### Manual Testing Checklist
1. Device authentication with valid/invalid API keys
2. Staff PIN validation and lockout
3. Student check-in/out flow
4. Multiple staff using same device
5. Session expiration handling

### Integration Points
1. Existing JWT authentication system
2. Active groups and visits tables
3. Authorization middleware
4. Rate limiting middleware

## Migration Rollback Plan

Each migration should include rollback SQL:

```sql
-- Rollback device_id optional
ALTER TABLE active.groups ALTER COLUMN device_id SET NOT NULL;

-- Rollback PIN fields
ALTER TABLE users.staff DROP COLUMN pin_hash;
ALTER TABLE users.staff DROP COLUMN pin_attempts;
ALTER TABLE users.staff DROP COLUMN pin_locked_until;

-- Rollback device auth
ALTER TABLE iot.devices DROP COLUMN api_key;
ALTER TABLE iot.devices DROP COLUMN last_seen_at;
ALTER TABLE iot.devices DROP COLUMN is_active;

-- Rollback device sessions
DROP TABLE IF EXISTS iot.device_sessions;
```

## Implementation Priority

1. **Phase 1 - Core Authentication**
   - Device API key field and authentication
   - Staff PIN storage and validation
   - Basic auth endpoints

2. **Phase 2 - Check-in/out**
   - Student check-in endpoint
   - Visit tracking
   - Room updates

3. **Phase 3 - Management UI**
   - Device management interface
   - PIN reset functionality
   - Basic monitoring

4. **Phase 4 - Future Enhancements**
   - Device sessions table
   - Advanced monitoring
   - WebSocket support

## Notes for Developers

- Keep device operations atomic and fast
- Log all authentication attempts for security
- Use database transactions for check-in/out
- Validate room capacity constraints
- Consider device clock synchronization
- Plan for offline device scenarios (future)