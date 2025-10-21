# MyRoom Activity-Based Refactoring - Design Document

## Business Logic Overview

### Current Understanding (Group-Based - WRONG)

```
MyRoom = "My supervised OGS groups"
├─ User is group supervisor
├─ Shows group's assigned room
└─ Shows children from that single group
```

**Problem:** Misses standalone activities where user supervises children from multiple groups.

### Correct Understanding (Activity-Based)

```
MyRoom = "My actively supervised rooms"
├─ User supervises an activity (group session OR standalone)
├─ Activity happens in a room
├─ Activity may include children from multiple groups
└─ Shows ALL children in that activity/room
```

**Examples:**

**Scenario A: OGS Group Session**
```
Activity: "Gruppe 1A Betreuung"
Type: Group session
Room: "1A"
Supervisor: Andreas (group supervisor)
Children: All from Gruppe 1A
→ MyRoom shows: Tab "1A" with children from Gruppe 1A
```

**Scenario B: Standalone Activity**
```
Activity: "Zeichnen"
Type: Standalone activity
Room: "Mensa"
Supervisor: Andreas (activity supervisor, NOT group supervisor)
Children: Emma (1A), Ben (2A), Mia (1B)
→ MyRoom shows: Tab "Mensa" with children from ALL groups
```

**Scenario C: Multiple Supervisions**
```
User Andreas supervises:
1. "Gruppe 1A Betreuung" in Room "1A"
2. "Zeichnen" in Room "Mensa"

MyRoom shows 2 tabs:
├─ "1A" - Children from Gruppe 1A
└─ "Mensa" - Children from multiple groups in Zeichnen
```

---

## Data Model Changes

### Current Data Model (Group-Based)

```tsx
interface ActiveRoom {
  id: string;              // Actually a GROUP ID
  name: string;            // Room name
  room_id?: string;        // Physical room ID
  student_count?: number;
  supervisor_name?: string;
  students?: Student[];
}

// Fetched via:
getMyActiveGroups() → Returns groups where user is GROUP supervisor
```

**Issues:**
- `id` is group ID, not room ID (confusing naming)
- Only returns groups where user is THE group supervisor
- Missing standalone activities

### New Data Model (Activity-Based)

```tsx
interface SupervisedActivity {
  id: string;                    // Activity ID (could be group session or standalone)
  name: string;                  // Activity name
  type: "group_session" | "standalone_activity";
  room: {
    id: string;
    name: string;
    building?: string;
    floor?: number;
  } | null;
  studentCount: number;
  supervisorRole: "supervisor" | "assistant";
  isGroupRoom: boolean;          // True if activity in user's own group room
  students?: StudentInActivity[];
}

interface StudentInActivity {
  id: string;
  name: string;
  firstName: string;
  lastName: string;
  schoolClass: string;
  groupId: string;
  groupName: string;            // Shows which group child is from
  checkInTime: string;
  isActive: boolean;
  isInGroupRoom: boolean;       // For badge color (green vs blue)
}

// Fetched via:
getStaffActiveSupervisions(staffId) → Returns ALL supervisions (groups + activities)
```

**Improvements:**
- Clear naming: `SupervisedActivity` instead of `ActiveRoom`
- Distinguishes between group sessions and standalone activities
- Includes `groupName` for each student (shows origin)
- Proper `isGroupRoom` flag for badge colors

---

## API Integration Strategy

### Option A: Use Existing Supervision API (Recommended)

**Endpoints Required:**
```
1. GET /api/active/supervisors/staff/{staffId}/active
   → Returns: Supervisor[] (all active supervisions)

2. GET /api/active/groups/{activityId}
   → Returns: Activity/Group details (name, room_id, type)

3. GET /api/active/groups/{activityId}/visits
   → Returns: Visit[] (all children in activity)
   → Permission: User must be supervisor of this activity

4. GET /api/rooms/{roomId}
   → Returns: Room details (name, building, floor)
```

**Data Flow:**
```
Step 1: Fetch supervisions
  getStaffActiveSupervisions(staffId)
  ↓
  Returns: [
    { id: "sup1", activeGroupId: "15", role: "supervisor" },
    { id: "sup2", activeGroupId: "18", role: "assistant" }
  ]

Step 2: For each supervision, fetch activity details
  getActiveGroup(activeGroupId)
  ↓
  Returns: {
    id: "15",
    name: "Gruppe 1A",
    room_id: "5",
    is_combined: false
  }

Step 3: Fetch room details
  fetchRoom(room_id)
  ↓
  Returns: { id: "5", name: "1A", floor: 1 }

Step 4: Fetch children in activity
  getActiveGroupVisits(activeGroupId)
  ↓
  Returns: [
    { studentId: "101", studentName: "Emma Friedrich", isActive: true },
    { studentId: "102", studentName: "Ben Neumann", isActive: true }
  ]
```

**Advantages:**
- ✅ Uses existing, tested API endpoints
- ✅ Proper permission checks (backend verifies supervision)
- ✅ No new backend code required
- ✅ Aligns with actual business logic

**Trade-offs:**
- ⚠️ Multiple API calls per activity (could be optimized later)
- ⚠️ Need to fetch staff ID (might not be in session)

### Option B: New Room-Based API (Not Recommended)

Create new endpoint: `GET /api/active/rooms/{roomId}/visits`

**Why NOT recommended:**
- Requires backend changes
- Doesn't align with business logic (supervision is activity-based, not room-based)
- Would still need to know which rooms user supervises
- More complex permission logic

---

## Component Architecture Changes

### Current Architecture (Broken)

```tsx
MyRoom Component
  ├─ useEffect: fetchMyRooms()
  │   └─ getMyActiveGroups() ← WRONG
  ├─ State: allRooms[] (actually groups)
  ├─ loadRoomVisits(roomId)
  │   └─ getActiveGroupVisitsWithDisplay(roomId) ← Treats roomId as groupId
  └─ switchToRoom(index) ← 403 errors
```

### New Architecture (Correct)

```tsx
MyRoom Component
  ├─ useEffect: fetchMySupervisedActivities()
  │   ├─ getStaffId() ← Get from session or API
  │   ├─ getStaffActiveSupervisions(staffId)
  │   ├─ For each supervision:
  │   │   ├─ getActiveGroup(activityId)
  │   │   ├─ fetchRoom(room_id)
  │   │   └─ getActiveGroupVisits(activityId)
  │   └─ Map to SupervisedActivity[]
  ├─ State: supervisedActivities[]
  ├─ loadActivityStudents(activityId)
  │   └─ getActiveGroupVisits(activityId) ← Correct!
  └─ switchToActivity(index) ← No 403 errors
```

---

## State Management Changes

### Before (Group-Based)

```tsx
const [allRooms, setAllRooms] = useState<ActiveRoom[]>([]);
const [selectedRoomIndex, setSelectedRoomIndex] = useState(0);
const currentRoom = allRooms[selectedRoomIndex];

// Confusing: "room" is actually a group
```

### After (Activity-Based)

```tsx
const [supervisedActivities, setSupervisedActivities] = useState<SupervisedActivity[]>([]);
const [selectedActivityIndex, setSelectedActivityIndex] = useState(0);
const currentActivity = supervisedActivities[selectedActivityIndex];

// Clear: activity can be group session or standalone activity
```

---

## Staff ID Retrieval Strategy

### Problem

The supervision API requires `staffId`, but it might not be in the session object.

### Solution Options

**Option 1: Add to session (Backend change)**
```tsx
// In session.user object
interface User {
  id: string;
  staffId?: string;  // ← Add this
  email: string;
  roles: string[];
}
```

**Option 2: Fetch from profile API (No backend changes)**
```tsx
// Frontend fetches staff ID
const profile = await fetch('/api/profile');
const staffId = profile.data.staffId;

// Use in supervision query
const supervisions = await getStaffActiveSupervisions(staffId);
```

**Option 3: Backend infers from token (Best)**
```tsx
// New endpoint that auto-detects staff ID from JWT
GET /api/active/supervisors/me/active
→ Backend extracts user ID from token
→ Looks up staff ID
→ Returns supervisions
```

**Recommendation:** Option 3 if backend can implement, otherwise Option 2.

---

## SSE Event Handling Changes

### Current Event Handling

```tsx
const handleSSEEvent = (event: SSEEvent) => {
  console.log("SSE event:", event.type, event.active_group_id);

  // Refetch if event is for current room's group
  if (event.active_group_id === currentRoom?.id) {
    loadRoomVisits(currentRoom.id);
  }
};
```

### New Event Handling

```tsx
const handleSSEEvent = (event: SSEEvent) => {
  console.log("SSE event:", event.type, event.active_group_id);

  // Refetch if event is for ANY of user's supervised activities
  const affectedActivity = supervisedActivities.find(
    activity => activity.id === event.active_group_id
  );

  if (affectedActivity) {
    loadActivityStudents(affectedActivity.id);
  }
};
```

**Changes:**
- Check against ALL supervised activities, not just current one
- Use activity ID instead of "room" ID
- More accurate event matching

---

## Badge Color Logic

### For "Anwesend" Status

**Determine badge color based on room comparison:**

```tsx
// Activity's room vs Student's group's room
const activity = supervisedActivities[currentIndex];
const activityRoomId = activity.room?.id;

students.map(student => {
  // Get student's group room
  const studentGroupRoomId = student.groupRoomId; // Need to fetch this

  // Compare
  const isInOwnGroupRoom = activityRoomId === studentGroupRoomId;

  return {
    ...student,
    badgeColor: isInOwnGroupRoom ? "#83CD2D" : "#5080D8"
  };
});
```

**Color Rules:**
- 🟢 **Green (#83CD2D):** Student is in their OWN group's room
  - Example: Child from "1A" in Room "1A"
- 🔵 **Blue (#5080D8):** Student is in EXTERNAL room
  - Example: Child from "1A" in Room "Mensa"

---

## Implementation Phases

### Phase 1: API Foundation (30 min)
1. Test `/api/active/supervisors/staff/{staffId}/active` endpoint
2. Document response structure
3. Verify permissions work correctly
4. Determine staff ID retrieval method

### Phase 2: Data Model (1 hour)
1. Define `SupervisedActivity` interface
2. Define `StudentInActivity` interface
3. Create mapper functions in `active-helpers.ts`
4. Update MyRoom component state variables

### Phase 3: Core Logic (1 hour)
1. Replace `getMyActiveGroups()` with supervision-based fetch
2. Update `loadRoomVisits()` to `loadActivityStudents()`
3. Update `switchToRoom()` to `switchToActivity()`
4. Fix API calls to use activity IDs correctly

### Phase 4: UI Updates (30 min)
1. Update tab labels (room names)
2. Add group badges to student cards
3. Update empty states
4. Fix error messages

### Phase 5: Testing (30 min)
1. Test with group session only
2. Test with standalone activity only
3. Test with multiple activities
4. Verify SSE updates
5. Check badge colors

---

## Rollback Plan

If issues arise:

1. **Immediate rollback:**
   ```bash
   git checkout HEAD -- frontend/src/app/myroom/page.tsx
   ```

2. **Data preserved:**
   - No database changes
   - No backend API changes
   - Only frontend logic changes

3. **Low risk:**
   - Isolated to MyRoom page
   - Other pages unaffected
   - Can rollback without data loss

---

## Testing Strategy

### Unit Testing (If Applicable)
- Mapper functions (supervision → SupervisedActivity)
- Badge color logic
- Student filtering

### Integration Testing
- API calls return expected data
- Multiple activities load correctly
- Tab switching works
- SSE events trigger correct updates

### Manual Testing Scenarios

**Scenario 1: User with only group supervision**
```
Given: User supervises "Gruppe 1A"
When: Navigate to /myroom
Then:
  - Shows 1 tab: "1A"
  - Lists children from Gruppe 1A
  - Badge colors: 🟢 Green (all in own room)
```

**Scenario 2: User with standalone activity**
```
Given: User supervises "Zeichnen" in "Mensa"
       Children: Emma (1A), Ben (2A), Mia (1B)
When: Navigate to /myroom
Then:
  - Shows 1 tab: "Mensa"
  - Lists Emma, Ben, Mia
  - Group badges show: "1A", "2A", "1B"
  - Badge colors: 🔵 Blue (all in external room)
```

**Scenario 3: Multiple activities**
```
Given: User supervises both:
  - "Gruppe 1A" in "1A"
  - "Zeichnen" in "Mensa"
When: Navigate to /myroom
Then:
  - Shows 2 tabs: "1A" | "Mensa"
  - Both tabs load correctly
  - No 403 errors
  - SSE updates work for both
```

---

## Migration Checklist

Before starting implementation:
- [ ] Test supervision API endpoint manually
- [ ] Document staff ID retrieval method
- [ ] Verify response structure matches expectations
- [ ] Confirm permissions allow activity visit access

During implementation:
- [ ] Update interfaces in separate commit
- [ ] Refactor data fetching logic
- [ ] Update UI rendering
- [ ] Test each phase before proceeding

After implementation:
- [ ] Test all scenarios manually
- [ ] Verify no console errors
- [ ] Check performance (page load time)
- [ ] Validate with different user roles
- [ ] Update documentation

---

## Performance Considerations

### API Call Optimization

**Initial Load:**
```
1 call: getStaffActiveSupervisions(staffId)
  ↓
N calls: getActiveGroup(activityId) for each supervision
  ↓
N calls: fetchRoom(roomId) for each unique room
  ↓
N calls: getActiveGroupVisits(activityId) for each activity

Total: 1 + 3N calls (N = number of supervisions)
```

**Optimizations:**
1. **Batch room fetches:** Deduplicate room IDs, fetch once
2. **Parallel API calls:** Use `Promise.all()` for concurrent requests
3. **Cache room data:** Rooms don't change frequently
4. **Lazy load students:** Only load for selected activity initially

**Target:** Page load < 2 seconds with 3 activities

### Memory Optimization

- Store only active students (filter `isActive: true`)
- Limit student data to required fields
- Clear inactive activity data on unmount

---

## Error Handling Strategy

### Permission Errors (403)

**Current:**
```tsx
catch (err) {
  if (err.message.includes("403")) {
    setError("Keine Berechtigung");
    return []; // Empty list
  }
}
```

**Should not happen after refactor** because:
- User only fetches activities they supervise
- Backend grants access to supervised activities
- No cross-group permission issues

**If 403 still occurs:**
- Log detailed error for debugging
- Show user-friendly message
- Filter out problematic activity from list

### Network Errors

```tsx
catch (err) {
  if (err.message.includes("network") || err.message.includes("fetch")) {
    setError("Netzwerkfehler. Bitte versuchen Sie es erneut.");
    // Keep existing data, allow retry
  }
}
```

### Missing Data

```tsx
if (!activity.room) {
  // Activity without room assignment
  console.warn(`Activity ${activity.id} has no room assigned`);
  // Show in list but with warning badge
}
```

---

## Backwards Compatibility

### Breaking Changes
- Tab IDs change from group IDs to activity IDs
- State variable names change
- API call patterns change

### Non-Breaking
- UI appearance stays similar
- Tab switching behavior unchanged
- Student filtering logic preserved
- SSE integration maintained (just adjusted)

### Migration Path
- No database migration needed
- No user data migration needed
- Pure frontend logic change
- Can deploy immediately

---

## Success Metrics

**Before Refactor:**
- 403 errors when accessing multi-group activities
- Incomplete activity list (missing standalone activities)
- Confusing "room" vs "group" terminology

**After Refactor:**
- ✅ Zero 403 errors
- ✅ Complete activity list (all supervisions)
- ✅ Clear activity-based terminology
- ✅ Correct badge colors (green/blue)
- ✅ Group badges show student origins

---

## Open Technical Questions

1. **Staff ID Retrieval:**
   - Where is staff_id stored?
   - Session? Profile API? Separate endpoint?

2. **Supervision Response Structure:**
   - Does `Supervisor` object include activity name?
   - Or do we need separate API call for each?

3. **Visit Permissions:**
   - Can assistant supervisors fetch visits?
   - Or only main supervisor?

4. **SSE Event Format:**
   - Does event include `supervisor_id`?
   - Or only `active_group_id`?

**Resolution:** These will be answered in Phase 1 (API Analysis)

---

## Future Enhancements (Out of Scope)

- Bulk API endpoint: `GET /api/active/supervisors/{staffId}/activities/with-visits`
- Real-time student count in tabs (via SSE)
- Activity type indicators (group session vs standalone)
- Room capacity warnings
- Quick actions (check out all, move to different room)

These can be added after core refactoring is stable.
