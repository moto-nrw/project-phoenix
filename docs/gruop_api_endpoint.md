# Group API Endpoints Overview

## Group Management

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| /groups | GET | List all groups | supervisor_id, search |
| /groups | POST | Create a new group | - |
| /groups/{id} | GET | Get a specific group | - |
| /groups/{id} | PUT | Update a specific group | - |
| /groups/{id} | DELETE | Delete a specific group | - |
| /groups/public | GET | Get a public list of all groups | - |

## Group Relationships

| Endpoint | Method | Description | Request Body |
|----------|--------|-------------|-------------|
| /groups/{id}/supervisors | POST | Update supervisors for a group | supervisor_ids [] |
| /groups/{id}/representative | POST | Set a pedagogical specialist as representative | specialist_id |
| /groups/{id}/representative | DELETE | Remove the representative from a group | - |
| /groups/{id}/students | GET | Get all students belonging to a group | - |
| /groups/room/{roomId}/group | GET | Get the group associated with a room | - |

## Combined Groups

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| /groups/combined | GET | List all combined groups | active |
| /groups/combined | POST | Create a new combined group | - |
| /groups/combined/{id} | GET | Get a specific combined group | - |
| /groups/combined/{id} | PUT | Update a specific combined group | - |
| /groups/combined/{id} | DELETE | Deactivate a combined group | - |

## Combined Group Management

| Endpoint | Method | Description | Request Body |
|----------|--------|-------------|-------------|
| /groups/combined/{id}/groups | POST | Add groups to a combined group | group_ids [] |
| /groups/combined/{id}/groups/{groupId} | DELETE | Remove a group from a combined group | - |
| /groups/combined/{id}/specialists | POST | Update specialists with access to a combined group | supervisor_ids [] |
| /groups/merge-rooms | POST | Merge two rooms and create a combined group | source_room_id, target_room_id |

## Request Body Examples

### Group Creation/Update
```json
{
  "name": "Group Name",
  "room_id": 101,
  "supervisor_ids": [1, 2, 3]
}
```

### Set Representative
```json
{
  "specialist_id": 1
}
```

### Update Supervisors
```json
{
  "supervisor_ids": [1, 2, 3]
}
```

### Combined Group Creation
```json
{
  "name": "Combined Group Name",
  "is_active": true,
  "access_policy": "all",
  "valid_until": "2025-05-30T15:00:00Z",
  "group_ids": [1, 2],
  "specialist_ids": [1, 3]
}
```

### Add Groups to Combined Group
```json
{
  "group_ids": [3, 4]
}
```

### Merge Rooms
```json
{
  "source_room_id": 101,
  "target_room_id": 102
}
```

## Access Policy Types for Combined Groups

- **all**: All groups in the combined group have access
- **first**: Only the first group has access
- **specific**: Only the specific group has access
- **manual**: Access is determined manually

## Response Examples

### Group Response
```json
{
  "id": 1,
  "name": "Group Name",
  "room_id": 101,
  "room": {
    "id": 101,
    "room_name": "Room 101"
  },
  "representative_id": 5,
  "representative": {
    "id": 5,
    "role": "Teacher",
    "custom_user": {
      "id": 10,
      "first_name": "John",
      "second_name": "Smith"
    }
  },
  "supervisors": [
    {
      "id": 1,
      "role": "Teacher",
      "custom_user": {
        "id": 5,
        "first_name": "Jane",
        "second_name": "Doe"
      }
    }
  ],
  "created_at": "2025-04-30T09:00:00Z",
  "updated_at": "2025-04-30T10:00:00Z"
}
```

### Combined Group Response
```json
{
  "id": 1,
  "name": "Combined Group A+B",
  "is_active": true,
  "access_policy": "all",
  "valid_until": "2025-05-30T15:00:00Z",
  "groups": [
    {
      "id": 1,
      "name": "Group A"
    },
    {
      "id": 2,
      "name": "Group B"
    }
  ],
  "access_specialists": [
    {
      "id": 1,
      "role": "Teacher",
      "custom_user": {
        "id": 5,
        "first_name": "Jane",
        "second_name": "Doe"
      }
    }
  ],
  "created_at": "2025-04-30T09:00:00Z"
}
```

### Public Group Response
```json
[
  {
    "id": 1,
    "name": "Group A"
  },
  {
    "id": 2,
    "name": "Group B"
  }
]
```
