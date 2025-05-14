# User Context API Endpoints

This document describes the User Context API endpoints that provide access to the current user's context.

## Authentication

All `/me/*` endpoints require a valid JWT token in the Authorization header:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

## Base URL

All user context endpoints are under the `/api/me` base path.

## Endpoints

### Get Current User Account

```
GET /api/me
```

Returns the current authenticated user account.

#### Response

```json
{
  "data": {
    "id": 1,
    "username": "john.doe",
    "email": "john.doe@example.com",
    "roles": ["staff", "admin"],
    "created_at": "2023-01-02T15:04:05Z",
    "updated_at": "2023-01-02T15:04:05Z"
  },
  "message": "Current user retrieved successfully",
  "status": "success"
}
```

### Get Current Person Profile

```
GET /api/me/profile
```

Returns the person profile linked to the current authenticated user account.

#### Response

```json
{
  "data": {
    "id": 10,
    "first_name": "John",
    "last_name": "Doe",
    "birthdate": "1990-01-01",
    "account_id": 1,
    "tag_id": "ABC123",
    "created_at": "2023-01-02T15:04:05Z",
    "updated_at": "2023-01-02T15:04:05Z"
  },
  "message": "Current profile retrieved successfully",
  "status": "success"
}
```

### Get Current Staff Profile

```
GET /api/me/staff
```

Returns the staff profile linked to the current authenticated user (if applicable).

#### Response

```json
{
  "data": {
    "id": 5,
    "person_id": 10,
    "staff_id": "S12345",
    "position": "Teacher",
    "department": "Science",
    "created_at": "2023-01-02T15:04:05Z",
    "updated_at": "2023-01-02T15:04:05Z"
  },
  "message": "Current staff profile retrieved successfully",
  "status": "success"
}
```

### Get Current Teacher Profile

```
GET /api/me/teacher
```

Returns the teacher profile linked to the current authenticated user (if applicable).

#### Response

```json
{
  "data": {
    "id": 3,
    "staff_id": 5,
    "specialization": "Physics",
    "created_at": "2023-01-02T15:04:05Z",
    "updated_at": "2023-01-02T15:04:05Z"
  },
  "message": "Current teacher profile retrieved successfully",
  "status": "success"
}
```

## Group-Related Endpoints

These endpoints require the `staff:read` permission.

### Get My Educational Groups

```
GET /api/me/groups
```

Returns educational groups associated with the current user.

#### Response

```json
{
  "data": [
    {
      "id": 1,
      "name": "Class 10A",
      "grade": 10,
      "subject": "Physics",
      "created_at": "2023-01-02T15:04:05Z",
      "updated_at": "2023-01-02T15:04:05Z"
    },
    {
      "id": 2,
      "name": "Class 11B",
      "grade": 11,
      "subject": "Chemistry",
      "created_at": "2023-01-02T15:04:05Z",
      "updated_at": "2023-01-02T15:04:05Z"
    }
  ],
  "message": "Educational groups retrieved successfully",
  "status": "success"
}
```

### Get My Activity Groups

```
GET /api/me/groups/activity
```

Returns activity groups associated with the current user.

#### Response

```json
{
  "data": [
    {
      "id": 10,
      "name": "Chess Club",
      "category_id": 1,
      "description": "Weekly chess club meetings",
      "created_at": "2023-01-02T15:04:05Z",
      "updated_at": "2023-01-02T15:04:05Z"
    }
  ],
  "message": "Activity groups retrieved successfully",
  "status": "success"
}
```

### Get My Active Groups

```
GET /api/me/groups/active
```

Returns currently active groups associated with the current user.

#### Response

```json
{
  "data": [
    {
      "id": 5,
      "source_id": 1,
      "source_type": "education_group",
      "name": "Class 10A - Active",
      "room_id": 3,
      "started_at": "2023-05-10T09:00:00Z",
      "ended_at": null
    }
  ],
  "message": "Active groups retrieved successfully",
  "status": "success"
}
```

### Get My Supervised Groups

```
GET /api/me/groups/supervised
```

Returns active groups currently being supervised by the current user.

#### Response

```json
{
  "data": [
    {
      "id": 7,
      "source_id": 2,
      "source_type": "education_group",
      "name": "Class 11B - Active",
      "room_id": 5,
      "started_at": "2023-05-10T11:00:00Z",
      "ended_at": null
    }
  ],
  "message": "Supervised groups retrieved successfully",
  "status": "success"
}
```

### Get Group Students

```
GET /api/me/groups/{groupID}/students
```

Returns students in a specific group where the current user has access.

#### Parameters

- `groupID`: ID of the group to get students for

#### Response

```json
{
  "data": [
    {
      "id": 101,
      "person_id": 50,
      "student_id": "ST12345",
      "grade": 10,
      "class": "A",
      "created_at": "2023-01-02T15:04:05Z",
      "updated_at": "2023-01-02T15:04:05Z",
      "person": {
        "id": 50,
        "first_name": "Jane",
        "last_name": "Smith",
        "birthdate": "2008-05-15"
      }
    },
    {
      "id": 102,
      "person_id": 51,
      "student_id": "ST12346",
      "grade": 10,
      "class": "A",
      "created_at": "2023-01-02T15:04:05Z",
      "updated_at": "2023-01-02T15:04:05Z",
      "person": {
        "id": 51,
        "first_name": "David",
        "last_name": "Jones",
        "birthdate": "2008-08-20"
      }
    }
  ],
  "message": "Group students retrieved successfully",
  "status": "success"
}
```

### Get Group Visits

```
GET /api/me/groups/{groupID}/visits
```

Returns active visits for a specific group where the current user has access.

#### Parameters

- `groupID`: ID of the group to get visits for

#### Response

```json
{
  "data": [
    {
      "id": 201,
      "group_id": 5,
      "student_id": 101,
      "started_at": "2023-05-10T09:00:00Z",
      "ended_at": null,
      "student": {
        "id": 101,
        "person_id": 50,
        "student_id": "ST12345"
      }
    },
    {
      "id": 202,
      "group_id": 5,
      "student_id": 102,
      "started_at": "2023-05-10T09:02:00Z",
      "ended_at": null,
      "student": {
        "id": 102,
        "person_id": 51,
        "student_id": "ST12346"
      }
    }
  ],
  "message": "Group visits retrieved successfully",
  "status": "success"
}
```

## Error Responses

### Unauthorized (401)

Returned when the request does not include a valid JWT token:

```json
{
  "status": "error",
  "message": "Unauthorized",
  "error": "user not authenticated"
}
```

### Forbidden (403)

Returned when the user does not have the required permissions:

```json
{
  "status": "error",
  "message": "Forbidden",
  "error": "user not authorized"
}
```

### Not Found (404)

Returned when the requested resource does not exist:

```json
{
  "status": "error",
  "message": "Not Found",
  "error": "group not found"
}
```

or

```json
{
  "status": "error",
  "message": "Not Found",
  "error": "user not linked to a staff member"
}
```