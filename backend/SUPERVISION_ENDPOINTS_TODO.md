# Backend Supervision Endpoints TODO

## Overview
The frontend menu filtering system requires backend endpoints to check user supervision status. These endpoints need to be implemented to complete the dynamic menu functionality.

## Required Endpoints

### 1. User Context Supervision Status
**Endpoint**: `GET /api/usercontext/supervision`

**Purpose**: Check if the current authenticated user is supervising any active session

**Response**:
```json
{
  "is_supervising": true,
  "room_id": 123,
  "room_name": "Room 101",
  "group_id": 456,
  "group_name": "3A"
}
```

**Implementation**:
- Add to `backend/api/usercontext/api.go`
- Use the UserContextService to get current staff/teacher
- Query active.supervisors table for active supervisions
- Join with rooms and groups tables for names

### 2. Alternative: Extend Existing Endpoints

If adding a new endpoint is not preferred, consider extending existing endpoints:

#### Option A: Add to user profile response
- Extend `GET /api/usercontext/` to include supervision status
- Add `current_supervision` field to the response

#### Option B: Use existing supervision endpoints
- The frontend could call `/api/active/supervisors/staff/{staffId}/active`
- But this requires the frontend to know the staff ID first

## Implementation Example

```go
// In api/usercontext/api.go

// Add route
r.router.Get("/supervision", r.getCurrentSupervision)

// Add handler
func (res *Resource) getCurrentSupervision(w http.ResponseWriter, r *http.Request) {
    supervision, err := res.service.GetCurrentSupervision(r.Context())
    if err != nil {
        // Return empty supervision status, not an error
        render.JSON(w, r, map[string]interface{}{
            "is_supervising": false,
        })
        return
    }
    
    render.JSON(w, r, supervision)
}
```

## Service Method

```go
// In services/usercontext/usercontext_service.go

func (s *Service) GetCurrentSupervision(ctx context.Context) (*SupervisionStatus, error) {
    // Get current user's staff ID
    staff, err := s.GetCurrentStaff(ctx)
    if err != nil {
        return nil, err
    }
    
    // Query active supervisions
    supervision, err := s.supervisorRepo.GetActiveByStaffID(ctx, staff.ID)
    if err != nil {
        return nil, err
    }
    
    // Return formatted response
    return &SupervisionStatus{
        IsSupervising: true,
        RoomID:        supervision.ActiveGroup.RoomID,
        RoomName:      supervision.ActiveGroup.Room.Name,
        GroupID:       supervision.ActiveGroup.GroupID,
        GroupName:     supervision.ActiveGroup.Group.Name,
    }, nil
}
```

## Notes
- The supervision status changes frequently, so caching should be minimal
- Consider WebSocket support for real-time updates in the future
- The frontend polls every 60 seconds, which should be sufficient for most use cases