# Room Page Implementation TODO

## Overview
The room supervision page (`/room`) should be implemented when a teacher is supervising an active session in a room. The menu item is now conditionally displayed based on the `isSupervising` state from the supervision context.

## Implementation Requirements

### 1. Create Room Page Component
- Location: `/src/app/room/page.tsx`
- Should display the room the teacher is currently supervising
- Use the supervision context to get `supervisedRoomId` and `supervisedRoomName`

### 2. Features to Include
- **Room Information**: Display room name, number, and capacity
- **Active Students**: List of students currently checked into the room
- **Check-in/Check-out**: Interface for manual student management
- **Quick Actions**: 
  - End supervision
  - Transfer supervision to another teacher
  - View room statistics

### 3. API Endpoints Needed
- `GET /api/active/rooms/{roomId}` - Get active room details
- `GET /api/active/rooms/{roomId}/students` - Get students in room
- `POST /api/active/rooms/{roomId}/checkin` - Manual check-in
- `POST /api/active/rooms/{roomId}/checkout` - Manual check-out
- `DELETE /api/active/rooms/{roomId}/supervision` - End supervision

### 4. Real-time Updates
- Consider using polling or WebSocket for real-time student updates
- Refresh student list every 30 seconds
- Show notifications for new check-ins/check-outs

### 5. Access Control
- Page should redirect to dashboard if user is not supervising
- Use the supervision context to verify access
- Show loading state while checking supervision status

### 6. Mobile Optimization
- Ensure the page works well on mobile devices
- Large touch targets for check-in/check-out actions
- Responsive layout for student list

## Example Implementation Structure

```typescript
// src/app/room/page.tsx
"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useSupervision } from "~/lib/supervision-context";

export default function RoomPage() {
  const router = useRouter();
  const { isSupervising, supervisedRoomId, supervisedRoomName, isLoadingSupervision } = useSupervision();

  useEffect(() => {
    if (!isLoadingSupervision && !isSupervising) {
      router.push("/dashboard");
    }
  }, [isSupervising, isLoadingSupervision, router]);

  if (isLoadingSupervision) {
    return <div>Loading...</div>;
  }

  if (!isSupervising || !supervisedRoomId) {
    return null; // Will redirect
  }

  return (
    <div>
      <h1>Room Supervision: {supervisedRoomName}</h1>
      {/* Add room supervision UI here */}
    </div>
  );
}
```

## Notes
- The supervision context already provides the necessary state
- The menu item is automatically shown/hidden based on supervision status
- Consider adding a badge to the menu item showing student count