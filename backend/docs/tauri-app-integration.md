# Tauri App Integration

This document describes how to integrate the Tauri desktop application with the RFID server.

## Device Registration and Management

### Register a new device

Before a Tauri app can communicate with the server, it needs to be registered and issued an API key.

**Endpoint:** `POST /rfid/devices`

**Request Body:**
```json
{
  "device_id": "unique-device-identifier",
  "name": "Classroom 101 RFID Reader",
  "description": "Optional description of the device"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Device registered successfully",
  "device": {
    "id": 1,
    "device_id": "unique-device-identifier",
    "name": "Classroom 101 RFID Reader",
    "description": "Optional description of the device",
    "status": "active",
    "created_at": "2023-05-15T10:30:00Z",
    "updated_at": "2023-05-15T10:30:00Z"
  },
  "api_key": "generated-api-key",
  "device_id": "unique-device-identifier"
}
```

**Important:** The API key is only returned once during registration. Store it securely.

### List all registered devices

**Endpoint:** `GET /rfid/devices`

**Response:**
```json
[
  {
    "id": 1,
    "device_id": "unique-device-identifier",
    "name": "Classroom 101 RFID Reader",
    "description": "Optional description of the device",
    "last_sync_at": "2023-05-15T11:30:00Z",
    "last_ip": "192.168.1.100",
    "status": "active",
    "created_at": "2023-05-15T10:30:00Z",
    "updated_at": "2023-05-15T10:30:00Z"
  }
]
```

### Get device details

**Endpoint:** `GET /rfid/devices/{device_id}`

**Response:**
```json
{
  "id": 1,
  "device_id": "unique-device-identifier",
  "name": "Classroom 101 RFID Reader",
  "description": "Optional description of the device",
  "last_sync_at": "2023-05-15T11:30:00Z",
  "last_ip": "192.168.1.100",
  "status": "active",
  "created_at": "2023-05-15T10:30:00Z",
  "updated_at": "2023-05-15T10:30:00Z"
}
```

### Update device information

**Endpoint:** `PUT /rfid/devices/{device_id}`

**Request Body:**
```json
{
  "name": "Updated Device Name",
  "description": "Updated description",
  "status": "inactive"
}
```

**Response:**
```json
{
  "id": 1,
  "device_id": "unique-device-identifier",
  "name": "Updated Device Name",
  "description": "Updated description",
  "last_sync_at": "2023-05-15T11:30:00Z",
  "last_ip": "192.168.1.100",
  "status": "inactive",
  "created_at": "2023-05-15T10:30:00Z",
  "updated_at": "2023-05-15T12:30:00Z"
}
```

### Get device sync history

**Endpoint:** `GET /rfid/devices/{device_id}/sync-history?limit=10`

**Response:**
```json
[
  {
    "id": 1,
    "device_id": "unique-device-identifier",
    "sync_at": "2023-05-15T11:30:00Z",
    "ip_address": "192.168.1.100",
    "tags_count": 15,
    "app_version": "1.0.0",
    "created_at": "2023-05-15T11:30:00Z"
  }
]
```

## Data Synchronization

### Sync RFID tag data

**Endpoint:** `POST /rfid/app/sync`

**Headers:**
```
Authorization: Bearer {api_key}
```

**Request Body:**
```json
{
  "device_id": "unique-device-identifier",
  "app_version": "1.0.0",
  "data": [
    {
      "tag_id": "ABCDEF123456",
      "reader_id": "ENTRANCE_READER",
      "local_read_at": "2023-05-15T11:25:00Z"
    },
    {
      "tag_id": "GHIJKL789012",
      "reader_id": "ENTRANCE_READER",
      "local_read_at": "2023-05-15T11:26:00Z"
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

### Get system status

**Endpoint:** `GET /rfid/app/status`

**Headers (optional):**
```
Authorization: Bearer {api_key}
```

**Query Parameters (optional):**
```
api_key={api_key}
```

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2023-05-15T12:00:00Z",
  "stats": {
    "tag_count": 1500,
    "students_in_house": 45,
    "students_in_wc": 3,
    "students_in_school_yard": 12
  },
  "version": "1.0.0"
}
```

## Integration in Tauri App

1. **Registration**:
   - Generate a unique device ID for each installation (can be done at first run)
   - Register the device with the server to get an API key
   - Store the API key securely on the device

2. **Authentication**:
   - Include the API key in the `Authorization` header for all API requests
   - Format: `Authorization: Bearer {api_key}`

3. **Regular Syncing**:
   - Collect RFID tag reads locally
   - Periodically sync with the server (e.g., every 5 minutes or when a batch of readings is available)
   - Include app version information with each sync

4. **Status Checking**:
   - Poll status endpoint to check server connectivity
   - Display student location statistics in the app UI

5. **Handling Network Issues**:
   - Implement local caching of tag reads if server is unreachable
   - Retry syncing on network recovery
   - Show sync status in the app

## Error Handling

Common error response codes:

- `400 Bad Request`: Invalid or malformed request
- `401 Unauthorized`: Missing or invalid API key
- `422 Unprocessable Entity`: Request validation failed
- `500 Internal Server Error`: Server-side error

Example error response:
```json
{
  "status": "Unauthorized.",
  "error": "API key required"
}
```