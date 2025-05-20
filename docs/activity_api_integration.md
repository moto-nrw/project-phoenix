# Activity API Integration

This document provides an overview of the Activity API integration between the frontend and backend systems of Project Phoenix.

## Backend API Endpoints

The backend API has the following endpoints. All endpoints are under the base path `/activities`.

### Activity CRUD Operations

| Endpoint | Method | Description | Required Permission |
|----------|--------|-------------|---------------------|
| `/` | GET | Get all activities with optional filtering | `activities:read` |
| `/{id}` | GET | Get a single activity by ID | `activities:read` |
| `/` | POST | Create a new activity | `activities:create` |
| `/{id}` | PUT | Update an existing activity | `activities:update` |
| `/{id}` | DELETE | Delete an activity | `activities:delete` |

### Student Enrollment

| Endpoint | Method | Description | Required Permission |
|----------|--------|-------------|---------------------|
| `/{id}/enroll/{studentId}` | POST | Enroll a student in an activity | `activities:enroll` |
| `/{id}/students` | GET | Get students enrolled in an activity | `activities:read` |

### Categories and Timeframes

| Endpoint | Method | Description | Required Permission |
|----------|--------|-------------|---------------------|
| `/categories` | GET | Get all activity categories | `activities:read` |
| `/timespans` | GET | Get all time spans for activities | `activities:read` |

## Frontend API Client

The frontend API client (`/frontend/src/lib/activity-api.ts`) provides JavaScript functions to interact with the backend API. These functions handle both browser-side and server-side API calls with appropriate authentication:

### Activity CRUD

```typescript
// Get all activities with optional filtering
fetchActivities(filters?: ActivityFilter): Promise<Activity[]>

// Get a single activity by ID
getActivity(id: string): Promise<Activity>

// Create a new activity
createActivity(data: CreateActivityRequest): Promise<Activity>

// Update an activity
updateActivity(id: string, data: UpdateActivityRequest): Promise<Activity>

// Delete an activity
deleteActivity(id: string): Promise<void>
```

### Student Enrollment

```typescript
// Get enrolled students
getEnrolledStudents(activityId: string): Promise<ActivityStudent[]>

// Enroll a student
enrollStudent(activityId: string, studentData: { studentId: string }): Promise<{ success: boolean }>

// Unenroll a student
unenrollStudent(activityId: string, studentId: string): Promise<void>
```

### Categories and Supervisors

```typescript
// Get all categories
getCategories(): Promise<ActivityCategory[]>

// Get all supervisors
getSupervisors(): Promise<Array<{ id: string; name: string }>>
```

## Frontend API Routes

The frontend implements Next.js API routes that proxy requests to the backend:

| Next.js API Route | Method | Description |
|------------------|--------|-------------|
| `/api/activities` | GET | Get all activities |
| `/api/activities/{id}` | GET | Get a single activity |
| `/api/activities` | POST | Create a new activity |
| `/api/activities/{id}` | PUT | Update an activity |
| `/api/activities/{id}` | DELETE | Delete an activity |
| `/api/activities/{id}/students` | GET | Get students enrolled in an activity |
| `/api/activities/{id}/enroll/{studentId}` | POST | Enroll a student in an activity |
| `/api/activities/{id}/students/{studentId}` | DELETE | Unenroll a student from an activity |
| `/api/activities/categories` | GET | Get all activity categories |
| `/api/activities/supervisors` | GET | Get all supervisors |

## Data Models

### Frontend Types

```typescript
// Activity data structure
interface Activity {
    id: string;
    name: string;
    max_participant: number;
    is_open_ags: boolean;
    supervisor_id: string;
    supervisor_name?: string;
    ag_category_id: string;
    category_name?: string;
    created_at: Date;
    updated_at: Date;
    participant_count?: number;
    times?: ActivityTime[];
    students?: ActivityStudent[];
}

// Activity category
interface ActivityCategory {
    id: string;
    name: string;
    description?: string;
    created_at: Date;
    updated_at: Date;
}

// Activity student enrollment
interface ActivityStudent {
    id: string;
    activity_id: string;
    student_id: string;
    name?: string;
    school_class?: string;
    in_house: boolean;
    created_at: Date;
    updated_at: Date;
}
```

### Backend Types

```go
// Activity response structure
type ActivityResponse struct {
    ID              int64              `json:"id"`
    Name            string             `json:"name"`
    MaxParticipants int                `json:"max_participants"`
    IsOpen          bool               `json:"is_open"`
    CategoryID      int64              `json:"category_id"`
    PlannedRoomID   *int64             `json:"planned_room_id,omitempty"`
    Category        *CategoryResponse  `json:"category,omitempty"`
    Schedules       []ScheduleResponse `json:"schedules,omitempty"`
    EnrollmentCount int                `json:"enrollment_count,omitempty"`
    CreatedAt       time.Time          `json:"created_at"`
    UpdatedAt       time.Time          `json:"updated_at"`
}

// Category response structure
type CategoryResponse struct {
    ID          int64     `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description,omitempty"`
    Color       string    `json:"color,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

## Request/Response Examples

### Creating an Activity

**Request:**
```json
POST /api/activities
{
  "name": "Fußball AG",
  "max_participants": 20,
  "category_id": 1,
  "supervisor_ids": [1, 2]
}
```

**Response:**
```json
{
  "success": true,
  "message": "Activity created successfully",
  "data": {
    "id": "123",
    "name": "Fußball AG",
    "max_participants": 20,
    "is_open": false,
    "category_id": 1,
    "created_at": "2023-05-20T10:30:00Z",
    "updated_at": "2023-05-20T10:30:00Z",
    "enrollment_count": 0,
    "schedules": []
  }
}
```

### Enrolling a Student

**Request:**
```
POST /api/activities/123/enroll/456
```

**Response:**
```json
{
  "success": true,
  "message": "Student enrolled successfully"
}
```

## Implementation Notes

1. **Frontend-Backend Mapping**: The frontend uses string IDs while the backend uses numeric IDs. All IDs are converted between string and number formats automatically.

2. **Response Formats**: The frontend handles two API response formats:
   - Direct data objects (e.g., `Activity` or `BackendActivity`)
   - Wrapped responses (e.g., `{ status: "success", data: Activity }`)

3. **Enrollment Special Case**: Unlike other endpoints, the student enrollment endpoint uses URL parameters rather than a request body. The URL structure `/activities/{id}/enroll/{studentId}` must be used.

4. **Data Type Differences**: Note these key field name differences between frontend and backend:
   - `max_participant` (frontend) vs `max_participants` (backend)
   - `ag_category_id` (frontend) vs `category_id` (backend)
   - `supervisor_id` (frontend) vs `supervisor_ids` (backend array)

## Testing

Use the test endpoint at `/api/activities/test/enrollment?activityId=1&studentId=2` to verify the enrollment functionality is working as expected. This endpoint:

1. Attempts to enroll the specified student in the activity
2. Returns the result of the operation
3. Provides error details if the operation fails

## Troubleshooting

Common issues and their solutions:

1. **404 Not Found for enrollment**: Verify the correct endpoint path format. The backend expects `/activities/{id}/enroll/{studentId}` for enrollment.

2. **Authorization errors**: Ensure the user has the required permission (`activities:enroll`) for managing enrollments.

3. **ID format mismatches**: Remember that frontend uses string IDs, while backend expects numeric IDs. The conversion is handled automatically by the API client functions.

4. **Response format issues**: The API is designed to handle both direct data and wrapped responses. If you're getting unexpected formats, check the API client implementation.

5. **Student already enrolled**: The backend will return an error if you try to enroll a student who is already enrolled in the activity.