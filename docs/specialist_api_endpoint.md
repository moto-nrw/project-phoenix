# Pedagogical Specialist API Endpoints Overview

## Specialist Management

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| / | GET | List all specialists | role, search |
| / | POST | Create a new specialist | - |
| /available | GET | List specialists not assigned to any group | - |
| /{id} | GET | Get a specific specialist | - |
| /{id} | PUT | Update a specific specialist | - |
| /{id} | DELETE | Delete a specific specialist | - |

## Group Supervision

| Endpoint | Method | Description | Path Parameters |
|----------|--------|-------------|----------------|
| /{id}/groups | GET | Get all groups a specialist is assigned to | id (specialist_id) |
| /{id}/groups/{groupId} | POST | Assign a specialist to a group | id (specialist_id), groupId |
| /{id}/groups/{groupId} | DELETE | Remove a specialist from a group | id (specialist_id), groupId |

## Request Body Examples

### Specialist Creation
```json
{
  "role": "Teacher",
  "first_name": "John",
  "second_name": "Doe",
  "tag_id": "TEACHER001",
  "account_id": 123
}
```

### Specialist Update
```json
{
  "id": 1,
  "role": "Principal",
  "custom_user": {
    "id": 5
  },
  "first_name": "John",
  "second_name": "Smith",
  "tag_id": "PRINCIPAL001"
}
```

## Response Examples

### Specialist Response
```json
{
  "id": 1,
  "role": "Teacher",
  "user_id": 5,
  "custom_user": {
    "id": 5,
    "first_name": "John",
    "second_name": "Doe",
    "tag_id": "TEACHER001",
    "created_at": "2025-04-30T09:00:00Z",
    "modified_at": "2025-04-30T09:00:00Z"
  },
  "created_at": "2025-04-30T09:00:00Z",
  "modified_at": "2025-04-30T09:00:00Z"
}
```

### Group Assignment Response
```json
{
  "success": true,
  "specialist_id": 1,
  "group_id": 2,
  "assigned_at": "2025-05-04T10:30:15Z"
}
```

### Specialist's Assigned Groups Response
```json
[
  {
    "id": 1,
    "name": "Group A",
    "room": {
      "id": 101,
      "room_name": "Room 101"
    }
  },
  {
    "id": 2,
    "name": "Group B",
    "room": {
      "id": 102,
      "room_name": "Room 102"
    }
  }
]
```

### Available Specialists Response
```json
[
  {
    "id": 3,
    "role": "Teacher",
    "custom_user": {
      "id": 7,
      "first_name": "Jane",
      "second_name": "Smith"
    }
  },
  {
    "id": 4,
    "role": "Assistant",
    "custom_user": {
      "id": 8,
      "first_name": "Robert",
      "second_name": "Johnson"
    }
  }
]
```

## Implementation Notes

- All endpoints are protected by JWT authentication
- Filtering by role allows clients to find specialists with specific roles
- The `/available` endpoint returns specialists not currently assigned to any group supervision
- When creating a specialist, you can either:
    - Provide an existing `account_id` to link to an existing account
    - Provide `first_name` and `second_name` to create a new user
- The optional `tag_id` parameter can be used for RFID tag association
- When updating a specialist, you can update both the specialist record and the associated user information in a single request
