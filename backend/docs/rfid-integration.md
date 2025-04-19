# RFID Integration Documentation

This document provides a comprehensive guide to integrating with the MOTO RFID tracking system. The system offers endpoints for tracking student locations, room occupancy, and synchronizing data from various sources including RFID readers and desktop applications.

## Authentication

Most RFID endpoints require authentication using an API key. API keys are specific to registered devices and should be kept secure.

### API Key Authentication

To authenticate with API key:

1. Include the API key in the `Authorization` header as a Bearer token
2. Format: `Authorization: Bearer YOUR_API_KEY`

Example:
```shell
curl -X POST \
  "http://your-server/api/rfid/tag" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "tag_id": "abc123456",
    "reader_id": "reader-01"
  }'
```

## Device Registration

Before using the RFID API, a device needs to be registered to obtain an API key.

### Register a New Device

**Endpoint:** `POST /rfid/devices`

**Request Body:**
```json
{
  "device_id": "unique-device-id",
  "name": "RFID Reader 1",
  "description": "RFID reader at main entrance"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Device registered successfully",
  "device": {
    "id": 1,
    "device_id": "unique-device-id",
    "name": "RFID Reader 1",
    "description": "RFID reader at main entrance",
    "status": "active",
    "created_at": "2023-10-15T14:30:00Z",
    "updated_at": "2023-10-15T14:30:00Z"
  },
  "api_key": "YOUR_API_KEY",
  "device_id": "unique-device-id"
}
```

**Important:** The API key is returned only once during registration. Store it securely.

## RFID Tag Management

### Record Tag Read

**Endpoint:** `POST /rfid/tag`

**Auth Required:** Yes

**Request Body:**
```json
{
  "tag_id": "abc123456",
  "reader_id": "reader-01"
}
```

**Response:**
```json
{
  "id": 1,
  "tag_id": "abc123456",
  "reader_id": "reader-01",
  "created_at": "2023-10-15T14:35:00Z"
}
```

### Get All Tag Reads

**Endpoint:** `GET /rfid/tags`

**Auth Required:** Yes

**Response:**
```json
[
  {
    "id": 1,
    "tag_id": "abc123456",
    "reader_id": "reader-01",
    "created_at": "2023-10-15T14:35:00Z"
  },
  {
    "id": 2,
    "tag_id": "def789012",
    "reader_id": "reader-02",
    "created_at": "2023-10-15T14:40:00Z"
  }
]
```

## Student Tracking

### Track Student Location

**Endpoint:** `POST /rfid/track-student`

**Auth Required:** Yes

**Request Body:**
```json
{
  "tag_id": "abc123456",
  "reader_id": "reader-01",
  "location_type": "entry"
}
```

Location types:
- `entry` - Student has entered the building
- `wc` - Student is in a bathroom
- `schoolyard` - Student is in the schoolyard
- `exit` - Student has left the building

**Response:**
```json
{
  "success": true,
  "message": "Location tracking recorded",
  "student_id": 42,
  "name": "John Doe",
  "location": "in-house"
}
```

## Room Occupancy Management

### Record Room Entry

**Endpoint:** `POST /rfid/room-entry`

**Auth Required:** Yes

**Request Body:**
```json
{
  "tag_id": "abc123456",
  "reader_id": "reader-01",
  "room_id": 123
}
```

**Response:**
```json
{
  "success": true,
  "message": "Student entered room successfully",
  "student_id": 42,
  "room_id": 123,
  "student_count": 15
}
```

### Record Room Exit

**Endpoint:** `POST /rfid/room-exit`

**Auth Required:** Yes

**Request Body:**
```json
{
  "tag_id": "abc123456",
  "reader_id": "reader-01",
  "room_id": 123
}
```

**Response:**
```json
{
  "success": true,
  "message": "Student exited room successfully",
  "student_id": 42,
  "room_id": 123,
  "student_count": 14
}
```

### Get Room Occupancy

**Endpoint:** `GET /rfid/room-occupancy`

**Auth Required:** Yes

**Query Parameters:**
- `room_id` (optional) - Specific room ID

**Response (specific room):**
```json
{
  "room_id": 123,
  "room_name": "Room 101",
  "student_count": 14,
  "students": [
    {
      "student_id": 42,
      "name": "John Doe"
    },
    {
      "student_id": 43,
      "name": "Jane Smith"
    }
  ]
}
```

**Response (all rooms):**
```json
[
  {
    "room_id": 123,
    "room_name": "Room 101",
    "student_count": 14
  },
  {
    "room_id": 124,
    "room_name": "Room 102",
    "student_count": 8
  }
]
```

## Visit Records

### Get Student Visits

**Endpoint:** `GET /rfid/student/{id}/visits`

**Auth Required:** Yes

**Path Parameters:**
- `id` - Student ID

**Query Parameters:**
- `date` (optional) - Filter by date (YYYY-MM-DD)

**Response:**
```json
[
  {
    "id": 1,
    "day": "2023-10-15",
    "student": {
      "id": 42,
      "name": "John Doe",
      "school_class": "4B",
      "group_name": "Group A",
      "in_house": true
    },
    "room": {
      "id": 123,
      "room_name": "Room 101",
      "building": "Building A",
      "floor": 1,
      "capacity": 25,
      "occupied": true,
      "color": "#FF0000",
      "category": "Classroom",
      "activity": "Learning"
    },
    "timespan": {
      "id": 1,
      "starttime": "09:15:00",
      "endtime": "10:30:00"
    }
  }
]
```

### Get Room Visits

**Endpoint:** `GET /rfid/room/{id}/visits`

**Auth Required:** Yes

**Path Parameters:**
- `id` - Room ID

**Query Parameters:**
- `date` (optional) - Filter by date (YYYY-MM-DD)
- `active` (optional) - Filter for active visits only (true/false)

**Response:**
```json
[
  {
    "id": 1,
    "day": "2023-10-15",
    "student": {
      "id": 42,
      "name": "John Doe",
      "school_class": "4B",
      "group_name": "Group A",
      "in_house": true
    },
    "room": {
      "id": 123,
      "room_name": "Room 101",
      "building": "Building A",
      "floor": 1,
      "capacity": 25,
      "occupied": true,
      "color": "#FF0000",
      "category": "Classroom",
      "activity": "Learning"
    },
    "timespan": {
      "id": 1,
      "starttime": "09:15:00",
      "endtime": "10:30:00"
    }
  }
]
```

### Get Today's Visits

**Endpoint:** `GET /rfid/visits/today`

**Auth Required:** Yes

**Response:**
```json
[
  {
    "id": 1,
    "day": "2023-10-15",
    "student": {
      "id": 42,
      "name": "John Doe",
      "school_class": "4B",
      "group_name": "Group A",
      "in_house": true
    },
    "room": {
      "id": 123,
      "room_name": "Room 101",
      "building": "Building A",
      "floor": 1,
      "capacity": 25,
      "occupied": true,
      "color": "#FF0000",
      "category": "Classroom",
      "activity": "Learning"
    },
    "timespan": {
      "id": 1,
      "starttime": "09:15:00",
      "endtime": "10:30:00"
    }
  }
]
```

## Tauri App Integration

### Sync Tauri App Data

**Endpoint:** `POST /rfid/app/sync`

**Auth Required:** Yes (API key in Authorization header)

**Request Body:**
```json
{
  "device_id": "tauri-app-123",
  "app_version": "1.0.0",
  "data": [
    {
      "tag_id": "abc123456",
      "reader_id": "tauri-app-123",
      "timestamp": "2023-10-15T14:35:00Z"
    },
    {
      "tag_id": "def789012",
      "reader_id": "tauri-app-123",
      "timestamp": "2023-10-15T14:40:00Z"
    }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "message": "Successfully synced 2 tags, processed 2 student locations"
}
```

### Get Tauri App System Status

**Endpoint:** `GET /rfid/app/status`

**Auth Required:** Optional (provides more details with authentication)

**Query Parameters:**
- `api_key` (optional) - API key for authenticated status

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2023-10-15T15:00:00Z",
  "version": "1.0.0",
  "stats": {
    "tag_count": 1250,
    "students_in_house": 156,
    "students_in_wc": 4,
    "students_in_school_yard": 28
  }
}
```

## Device Management

### List All Devices

**Endpoint:** `GET /rfid/devices`

**Auth Required:** Yes

**Response:**
```json
[
  {
    "id": 1,
    "device_id": "tauri-app-123",
    "name": "Tauri App 1",
    "description": "Reception desk Tauri app",
    "last_sync_at": "2023-10-15T14:55:00Z",
    "last_ip": "192.168.1.100",
    "status": "active",
    "created_at": "2023-10-01T10:00:00Z",
    "updated_at": "2023-10-15T14:55:00Z"
  },
  {
    "id": 2,
    "device_id": "rfid-reader-45",
    "name": "RFID Reader 45",
    "description": "Main entrance RFID reader",
    "last_sync_at": "2023-10-15T14:50:00Z",
    "last_ip": "192.168.1.45",
    "status": "active",
    "created_at": "2023-10-01T10:30:00Z",
    "updated_at": "2023-10-15T14:50:00Z"
  }
]
```

### Get Device Details

**Endpoint:** `GET /rfid/devices/{device_id}`

**Auth Required:** Yes

**Path Parameters:**
- `device_id` - Device ID (string)

**Response:**
```json
{
  "id": 1,
  "device_id": "tauri-app-123",
  "name": "Tauri App 1",
  "description": "Reception desk Tauri app",
  "last_sync_at": "2023-10-15T14:55:00Z",
  "last_ip": "192.168.1.100",
  "status": "active",
  "created_at": "2023-10-01T10:00:00Z",
  "updated_at": "2023-10-15T14:55:00Z"
}
```

### Update Device

**Endpoint:** `PUT /rfid/devices/{device_id}`

**Auth Required:** Yes

**Path Parameters:**
- `device_id` - Device ID (string)

**Request Body:**
```json
{
  "name": "Tauri App 1 Updated",
  "description": "Updated description",
  "status": "inactive"
}
```

**Response:**
```json
{
  "id": 1,
  "device_id": "tauri-app-123",
  "name": "Tauri App 1 Updated",
  "description": "Updated description",
  "last_sync_at": "2023-10-15T14:55:00Z",
  "last_ip": "192.168.1.100",
  "status": "inactive",
  "created_at": "2023-10-01T10:00:00Z",
  "updated_at": "2023-10-15T15:05:00Z"
}
```

### Get Device Sync History

**Endpoint:** `GET /rfid/devices/{device_id}/sync-history`

**Auth Required:** Yes

**Path Parameters:**
- `device_id` - Device ID (string)

**Query Parameters:**
- `limit` (optional) - Maximum number of records to return (default 50)

**Response:**
```json
[
  {
    "id": 1,
    "device_id": "tauri-app-123",
    "ip_address": "192.168.1.100",
    "app_version": "1.0.0",
    "tags_count": 15,
    "created_at": "2023-10-15T14:55:00Z"
  },
  {
    "id": 2,
    "device_id": "tauri-app-123",
    "ip_address": "192.168.1.100",
    "app_version": "1.0.0",
    "tags_count": 8,
    "created_at": "2023-10-15T13:30:00Z"
  }
]
```

## Error Responses

All endpoints return standardized error responses when something goes wrong.

### Error Structure

```json
{
  "status": "Invalid request.",
  "code": 0,
  "error": "Detailed error message"
}
```

### Common HTTP Status Codes

- `200` - Success
- `201` - Created (for POST requests that create resources)
- `400` - Bad Request (client error)
- `401` - Unauthorized (missing or invalid API key)
- `404` - Not Found
- `422` - Unprocessable Entity (malformed request)
- `500` - Internal Server Error

## Security Considerations

1. API Keys should be kept confidential and never exposed in client-side code
2. Regenerate API keys if they are compromised
3. Use HTTPS for all API communications
4. Restrict API key access to needed endpoints only
5. Regularly audit device access and remove inactive devices

## Rate Limiting

The API implements rate limiting to prevent abuse. Limits are:

- RFID Tag endpoints: 10 requests per second per device
- Student tracking endpoints: 5 requests per second per device
- Room occupancy endpoints: 5 requests per second per device
- Tauri app endpoints: 2 requests per second per device

## Support

For assistance with integration, contact the support team at support@moto-system.example.com