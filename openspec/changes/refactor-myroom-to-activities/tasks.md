# Implementation Tasks - MyRoom Activity-Based Refactoring

## ⚡ SIMPLIFIED IMPLEMENTATION

**UPDATE:** After backend code analysis, this is a **1-line change** instead of full refactoring!

**Key Finding:** `getMySupervisedGroups()` already returns ALL supervisions including standalone activities.

### ✅ Validation: Works WITHOUT OGS Groups

**User Question:** *"Funktioniert das auch wenn User KEINE OGS-Gruppe hat, aber standalone Aktivitäten supervised?"*

**Answer: JA! ✅**

**Backend Logic (usercontext_service.go:438-473):**
```go
func GetMySupervisedGroups(ctx) ([]*active.Group, error) {
    staff, err := s.GetCurrentStaff(ctx)
    if err != nil {
        return []*active.Group{}, nil  // ← Returns EMPTY array, not error!
    }

    // Finds ALL supervisions (via supervisor table)
    supervisions, err := s.supervisorRepo.FindActiveByStaffID(ctx, staff.ID)

    // Returns active groups for these supervisions
    // Works for:
    // ✅ User with only OGS groups
    // ✅ User with only standalone activities
    // ✅ User with both
    // ✅ User with neither (returns [])
}
```

**What "Active Group" really means:**
- Table: `active.groups` (stores BOTH OGS sessions AND standalone activities)
- OGS Group Session: `{ group_id: 5, room_id: 3, name: "Gruppe 1A" }`
- Standalone Activity: `{ group_id: null?, room_id: 7, name: "Zeichnen" }`
- **All are "groups" in the active table!**

**The Fix Works Because:**
```
Teacher "Max" (NO OGS group, supervises "Malen"):
1. Backend: FindActiveByStaffID(max_id)
   → Findet: Supervision für "Malen" Aktivität
2. Backend: FindByID(malen_group_id)
   → Findet: Active Group "Malen" (is_active = true)
3. Return: [{ id: malen_id, name: "Malen", room: "Mensa" }]
4. Frontend: getMySupervisedGroups()
   → Empfängt: "Malen" Aktivität ✅
5. MyRoom zeigt: Tab "Mensa" mit allen Kindern ✅
```

---

## Task Breakdown

### Phase 1: ~~API Analysis & Validation~~ ✅ COMPLETED (Via Code Analysis)

#### 1.1 Backend Code Analysis ✅ COMPLETED

**Findings from `backend/services/usercontext/usercontext_service.go`:**

- ✅ `GetMySupervisedGroups()` exists (lines 438-473)
- ✅ Extracts user from JWT automatically via `GetCurrentStaff(ctx)`
- ✅ Finds supervisions via `FindActiveByStaffID(ctx, staff.ID)`
- ✅ Returns `[]*active.Group` (includes OGS groups + standalone activities)
- ✅ Works even if user has NO OGS groups (returns `[]`, not error)

**API Endpoint:**
```
GET /api/me/groups/supervised
→ Frontend proxy: userContextService.getMySupervisedGroups()
→ Backend: GetMySupervisedGroups(ctx)
→ Returns: All active groups where user is a supervisor
```

**Response Structure (from active-helpers.ts):**
```tsx
interface BackendActiveGroup {
  id: number;
  group_id: number;
  room_id: number;
  is_active: boolean;
  room?: { id: number; name: string };
  actual_group?: { id: number; name: string };
  // ... other fields
}
```

**Validation:** ✅ All API requirements met with existing endpoints

#### 1.2 Staff ID Retrieval ✅ RESOLVED

**Solution:** NOT NEEDED!

Backend auto-detects staff from JWT token in `/api/me/groups/supervised`.

**Validation:** ✅ No staff_id parameter required

#### 1.3 Permissions Validation ✅ VERIFIED

**From backend logic:**
- `FindActiveByStaffID()` finds all supervisions for the staff member
- Includes supervisions where user is `supervisor` OR `assistant` role
- Returns active groups for these supervisions
- No 403 errors because user IS a supervisor of these groups

**Validation:** ✅ Permissions work correctly

---

### Phase 2: ~~Data Model & Interfaces~~ ✅ NOT NEEDED

**UPDATE:** Existing `ActiveGroup` interface already supports both OGS groups and standalone activities. No new interfaces required!

#### 2.1 Create New Interfaces
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Define `SupervisedActivity` interface:
  ```tsx
  interface SupervisedActivity {
    id: string;
    name: string;
    type: "group_session" | "standalone_activity";
    room: { id: string; name: string; building?: string; floor?: number } | null;
    studentCount: number;
    supervisorRole: "supervisor" | "assistant";
    isGroupRoom: boolean;
    students?: StudentInActivity[];
  }
  ```
- [ ] Define `StudentInActivity` interface:
  ```tsx
  interface StudentInActivity {
    id: string;
    name: string;
    firstName: string;
    lastName: string;
    schoolClass: string;
    groupId: string;
    groupName: string;
    checkInTime: string;
    isActive: boolean;
  }
  ```
- [ ] Add JSDoc comments explaining the interfaces

**Validation:** TypeScript compiles, interfaces well-documented

#### 2.2 Create Mapper Functions
**Files:** `frontend/src/lib/active-helpers.ts` (or create if doesn't exist)

- [ ] Create `mapSupervisionToActivity()`:
  ```tsx
  export function mapSupervisionToActivity(
    supervision: Supervisor,
    activityDetails: ActiveGroup,
    room: Room | null
  ): SupervisedActivity {
    return {
      id: supervision.activeGroupId,
      name: activityDetails.name,
      type: activityDetails.is_combined ? "standalone_activity" : "group_session",
      room: room ? {
        id: room.id,
        name: room.name,
        building: room.building,
        floor: room.floor
      } : null,
      studentCount: 0, // Will be updated when visits load
      supervisorRole: supervision.role,
      isGroupRoom: false, // Will be calculated
      students: []
    };
  }
  ```
- [ ] Create `mapVisitToStudentInActivity()`:
  ```tsx
  export function mapVisitToStudentInActivity(visit: Visit): StudentInActivity {
    // Map visit data to student structure
  }
  ```
- [ ] Add unit tests for mappers (if testing infrastructure exists)

**Validation:** Mapper functions work correctly, handle edge cases

#### 2.3 Update State Variables
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Rename state variables:
  ```tsx
  // OLD
  const [allRooms, setAllRooms] = useState<ActiveRoom[]>([]);
  const [selectedRoomIndex, setSelectedRoomIndex] = useState(0);

  // NEW
  const [supervisedActivities, setSupervisedActivities] = useState<SupervisedActivity[]>([]);
  const [selectedActivityIndex, setSelectedActivityIndex] = useState(0);
  ```
- [ ] Update all references to old variable names
- [ ] Update computed values:
  ```tsx
  const currentActivity = supervisedActivities[selectedActivityIndex] ?? null;
  ```

**Validation:** TypeScript compiles, no undefined variable errors

---

### Phase 3: Core Implementation (15 min) ⭐ MAIN TASK

#### 3.1 Change API Call (THE Main Fix!) 🎯
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Find line ~188: `userContextService.getMyActiveGroups()`
- [ ] Replace with:
  ```tsx
  // OLD (line 188):
  const myActiveGroups = await userContextService.getMyActiveGroups();

  // NEW:
  const myActiveGroups = await userContextService.getMySupervisedGroups();
  //                                               ^^^^^^^^^^^^^^^^^^^
  //                                               Only change!
  ```
- [ ] No other changes needed in this function!

**Why this fixes everything:**
- ✅ `getMyActiveGroups()` = Only groups I OWN
- ✅ `getMySupervisedGroups()` = ALL groups I SUPERVISE (owner + assigned supervisor)
- ✅ Includes standalone activities (e.g., "Zeichnen" in "Mensa")
- ✅ No 403 errors (I'm a supervisor of all returned groups)

**Validation:** Code compiles, no TypeScript errors

#### 3.2 ~~Implement fetchMySupervisedActivities()~~ ✅ NOT NEEDED

**UPDATE:** Existing code structure works perfectly with `getMySupervisedGroups()`.

No refactoring required!
  ```tsx
  const fetchMySupervisedActivities = async () => {
    try {
      setIsLoading(true);

      // 1. Get staff ID
      const staffId = await getStaffId();

      // 2. Fetch all supervisions
      const supervisions = await activeService.getStaffActiveSupervisions(staffId);

      if (supervisions.length === 0) {
        setHasAccess(false);
        return;
      }

      setHasAccess(true);

      // 3. Fetch details for each supervision
      const activities = await Promise.all(
        supervisions.map(async (supervision) => {
          try {
            // Get activity details
            const activityDetails = await activeService.getActiveGroup(supervision.activeGroupId);

            // Get room details if room_id exists
            let room = null;
            if (activityDetails.room_id) {
              try {
                const roomResponse = await fetch(`/api/rooms/${activityDetails.room_id}`);
                if (roomResponse.ok) {
                  const roomData = await roomResponse.json();
                  room = roomData.data ?? roomData;
                }
              } catch (e) {
                console.warn(`Could not fetch room ${activityDetails.room_id}:`, e);
              }
            }

            // Map to SupervisedActivity
            return mapSupervisionToActivity(supervision, activityDetails, room);

          } catch (err) {
            console.error(`Error fetching activity ${supervision.activeGroupId}:`, err);
            return null; // Skip problematic activities
          }
        })
      );

      // Filter out nulls (failed fetches)
      const validActivities = activities.filter(a => a !== null);

      setSupervisedActivities(validActivities);

      // Load students for first activity
      if (validActivities[0]) {
        await loadActivityStudents(validActivities[0].id, 0);
      }

    } catch (err) {
      if (err instanceof Error && err.message.includes("403")) {
        setHasAccess(false);
      } else {
        setError("Fehler beim Laden der Supervisions.");
      }
    } finally {
      setIsLoading(false);
    }
  };
  ```
- [ ] Add to useEffect dependency array
- [ ] Test with different user roles

**Validation:** Supervisions load correctly, no 403 errors

#### 3.3 Implement loadActivityStudents()
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Replace `loadRoomVisits()` with:
  ```tsx
  const loadActivityStudents = async (
    activityId: string,
    activityIndex: number
  ): Promise<void> => {
    try {
      // Fetch visits for this activity
      const visits = await activeService.getActiveGroupVisits(activityId);

      // Filter active visits
      const activeVisits = visits.filter(v => v.isActive);

      // Fetch full student data for each visit
      const students = await Promise.all(
        activeVisits.map(async (visit) => {
          try {
            const studentData = await fetchStudent(visit.studentId);
            return {
              ...studentData,
              groupName: studentData.group_name,
              checkInTime: visit.checkInTime,
              isActive: visit.isActive
            } as StudentInActivity;
          } catch (err) {
            // Fallback if student fetch fails
            return {
              id: visit.studentId,
              name: visit.studentName,
              firstName: visit.studentName?.split(' ')[0] ?? '',
              lastName: visit.studentName?.split(' ').slice(1).join(' ') ?? '',
              schoolClass: '',
              groupId: '',
              groupName: 'Unbekannt',
              checkInTime: visit.checkInTime,
              isActive: true
            } as StudentInActivity;
          }
        })
      );

      // Update activity with students
      setSupervisedActivities(prev =>
        prev.map((activity, idx) =>
          idx === activityIndex
            ? { ...activity, students, studentCount: students.length }
            : activity
        )
      );

    } catch (err) {
      console.error(`Error loading students for activity ${activityId}:`, err);
      throw err;
    }
  };
  ```
- [ ] Update function signature and references
- [ ] Remove old `loadRoomVisits()` function

**Validation:** Students load correctly for supervised activities

#### 3.4 Update switchToActivity()
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Rename and refactor `switchToRoom()`:
  ```tsx
  const switchToActivity = async (activityIndex: number) => {
    if (activityIndex === selectedActivityIndex) return;
    if (!supervisedActivities[activityIndex]) return;

    setSelectedActivityIndex(activityIndex);
    setIsLoading(true);
    setStudents([]); // Clear current students

    try {
      const activity = supervisedActivities[activityIndex];
      await loadActivityStudents(activity.id, activityIndex);
      setError(null);
    } catch (err) {
      setError("Fehler beim Wechseln der Aktivität.");
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };
  ```
- [ ] Update all references from `switchToRoom` to `switchToActivity`
- [ ] Update tab onChange handler

**Validation:** Tab switching works without errors

---

### Phase 4: UI Updates (30 min)

#### 4.1 Update PageHeaderWithSearch
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Update tabs configuration:
  ```tsx
  tabs={
    supervisedActivities.length > 1
      ? {
          items: supervisedActivities.map(activity => ({
            id: activity.id,
            label: activity.room?.name ?? activity.name,
            count: activity.studentCount
          })),
          activeTab: currentActivity?.id ?? "",
          onTabChange: (activityId) => {
            const index = supervisedActivities.findIndex(a => a.id === activityId);
            if (index !== -1) void switchToActivity(index);
          }
        }
      : undefined
  }
  ```
- [ ] Update badge to show current activity name:
  ```tsx
  badge={{
    count: currentActivity?.studentCount ?? 0,
    label: currentActivity?.type === "standalone_activity" ? "Aktivität" : "Gruppe"
  }}
  ```

**Validation:** Tabs show correct labels, counts update

#### 4.2 Add Group Badges to Student Cards
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Update student card rendering:
  ```tsx
  <div className="student-card">
    <div className="student-info">
      <h3>{student.name}</h3>

      {/* NEW: Show which group student is from */}
      <span className="text-xs px-2 py-1 bg-gray-100 rounded-full">
        {student.groupName}
      </span>
    </div>

    {/* Existing status badge */}
    <StatusBadge
      location={getStudentLocation(student)}
      isGroupRoom={isInTheirGroupRoom(student, currentActivity)}
    />
  </div>
  ```
- [ ] Add helper function `isInTheirGroupRoom()`:
  ```tsx
  const isInTheirGroupRoom = (
    student: StudentInActivity,
    activity: SupervisedActivity
  ): boolean => {
    // If activity is in a room
    if (!activity.room) return false;

    // Check if student's group room matches activity room
    // This requires knowing the student's group room ID
    // For now, return false for standalone activities
    return activity.type === "group_session" &&
           student.groupId === activity.id.split('_')[0]; // Simplified logic
  };
  ```

**Validation:** Group badges visible, badge colors correct

#### 4.3 Update Empty States
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Update "no access" message:
  ```tsx
  if (hasAccess === false) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-600 mb-4">
          Sie haben aktuell keine aktiven Supervisions.
        </p>
        <p className="text-sm text-gray-500">
          Starten Sie eine Aktivität oder übernehmen Sie eine Gruppe.
        </p>
      </div>
    );
  }
  ```
- [ ] Update "no students" state:
  ```tsx
  {filteredStudents.length === 0 && (
    <div className="text-center py-8">
      <p className="text-gray-600">
        Keine Kinder in dieser Aktivität.
      </p>
    </div>
  )}
  ```

**Validation:** Empty states show appropriate messages

---

### Phase 5: SSE Integration Update (30 min)

#### 5.1 Update SSE Event Handler
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Update `handleSSEEvent()` to check all supervised activities:
  ```tsx
  const handleSSEEvent = useCallback((event: SSEEvent) => {
    console.log("SSE event:", event.type, event.active_group_id);

    // Check if event is for any of our supervised activities
    const affectedActivityIndex = supervisedActivities.findIndex(
      activity => activity.id === event.active_group_id
    );

    if (affectedActivityIndex !== -1) {
      console.log(`Event for supervised activity ${affectedActivityIndex}`);

      // If it's the currently selected activity, reload
      if (affectedActivityIndex === selectedActivityIndex) {
        const activity = supervisedActivities[affectedActivityIndex];
        void loadActivityStudents(activity.id, affectedActivityIndex);
      }
      // Otherwise, just mark it as needing refresh (optional enhancement)
    }
  }, [supervisedActivities, selectedActivityIndex, loadActivityStudents]);
  ```
- [ ] Verify SSE subscription still works
- [ ] Test that check-in/check-out events trigger updates

**Validation:** SSE updates refresh correct activity

#### 5.2 Update SSE Connection Logic
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Verify SSE connects when user has supervisions
- [ ] Verify SSE disconnects when navigating away
- [ ] Check SSE status indicator still works

**Validation:** SSE connection stable, status indicator accurate

---

### Phase 6: Testing & Validation (30 min)

#### 6.1 Manual Testing - Single Group Supervision
**User Story:** User supervises only their OGS group

- [ ] Login as user with one OGS group supervision
- [ ] Navigate to /myroom
- [ ] Verify:
  - Shows 1 tab with group's room name
  - Lists children from that group only
  - No 403 errors
  - Badge colors correct (🟢 green for own room)

**Validation:** Single group scenario works

#### 6.2 Manual Testing - Standalone Activity
**User Story:** User supervises multi-group activity

- [ ] Login as user supervising standalone activity (e.g., "Zeichnen")
- [ ] Navigate to /myroom
- [ ] Verify:
  - Shows 1 tab with activity's room name ("Mensa")
  - Lists children from ALL participating groups
  - Group badges show correct group names ("1A", "2A", "1B")
  - No 403 errors
  - Badge colors correct (🔵 blue for external room)

**Validation:** Multi-group activity works

#### 6.3 Manual Testing - Multiple Activities
**User Story:** User supervises both group and standalone activity

- [ ] Login as user with multiple supervisions
- [ ] Navigate to /myroom
- [ ] Verify:
  - Shows multiple tabs (one per activity)
  - Can switch between tabs without errors
  - Student lists update correctly
  - SSE updates work for all activities
  - Badge colors correct for each context

**Validation:** Multiple activities work, no conflicts

#### 6.4 Manual Testing - Edge Cases
**Edge case testing**

- [ ] User with 5+ supervisions (room selection screen)
- [ ] Activity without room assignment
- [ ] Activity with no students (empty list)
- [ ] Student check-in/out during browsing (SSE update)
- [ ] Network error during load
- [ ] Switching tabs rapidly (race conditions)

**Validation:** Edge cases handled gracefully

#### 6.5 Code Quality Checks
**Automated testing**

- [ ] Run `npm run check` (lint + typecheck)
- [ ] Fix any TypeScript errors
- [ ] Fix any ESLint warnings
- [ ] Verify no console errors in browser
- [ ] Check no memory leaks (SSE cleanup)

**Validation:** All quality checks pass

---

### Phase 7: Documentation & Cleanup (30 min)

#### 7.1 Code Documentation
**Files:** `frontend/src/app/myroom/page.tsx`

- [ ] Add JSDoc comments to new functions
- [ ] Update component-level comments
- [ ] Document data flow in comments
- [ ] Add TODO comments for future enhancements

**Validation:** Code is self-documenting

#### 7.2 Update README/CLAUDE.md (If Needed)
**Files:** `frontend/CLAUDE.md`, `CLAUDE.md`

- [ ] Document MyRoom business logic change
- [ ] Update any architecture diagrams
- [ ] Note the activity-based approach

**Validation:** Documentation up to date

#### 7.3 Git Commit
**Commit strategy**

- [ ] Review all changes: `git diff`
- [ ] Create atomic commits:
  1. `refactor: update MyRoom data model for activity-based supervision`
  2. `feat: implement activity-based student loading in MyRoom`
  3. `fix: resolve 403 errors in MyRoom for multi-group activities`
  4. `chore: update MyRoom documentation and comments`
- [ ] Conventional commit format
- [ ] No large files, no sensitive data

**Validation:** Clean git history

---

## Task Dependencies

```
Phase 1 (API Analysis)
  ├─ 1.1 Test Supervision API (no dependencies)
  ├─ 1.2 Staff ID Retrieval (no dependencies)
  └─ 1.3 Test Visits API (after 1.1)

Phase 2 (Data Model) - Depends on Phase 1
  ├─ 2.1 Interfaces (after 1.1, 1.3)
  ├─ 2.2 Mappers (after 2.1)
  └─ 2.3 State Variables (after 2.1)

Phase 3 (API Integration) - Depends on Phase 2
  ├─ 3.1 Staff ID (after 1.2, 2.3)
  ├─ 3.2 Fetch Supervisions (after 3.1, 2.2)
  ├─ 3.3 Load Students (after 3.2)
  └─ 3.4 Switch Logic (after 3.3)

Phase 4 (UI) - Depends on Phase 3
  ├─ 4.1 Tabs (after 3.2)
  ├─ 4.2 Group Badges (after 3.3)
  └─ 4.3 Empty States (after 3.2)

Phase 5 (SSE) - Can parallel with Phase 4
  ├─ 5.1 Event Handler (after 3.2)
  └─ 5.2 Connection Logic (after 5.1)

Phase 6 (Testing) - After Phases 3-5
  ├─ 6.1 Single Group (after Phase 3-5)
  ├─ 6.2 Standalone Activity (after 6.1)
  ├─ 6.3 Multiple Activities (after 6.2)
  ├─ 6.4 Edge Cases (after 6.1-6.3)
  └─ 6.5 Quality Checks (after 6.1-6.4)

Phase 7 (Documentation) - After Phase 6
  ├─ 7.1 Code Docs (no dependencies)
  ├─ 7.2 README (after 7.1)
  └─ 7.3 Git Commit (after 7.1-7.2)
```

---

## Parallel Work Opportunities

**Can work in parallel:**
- Phase 2.2 (Mappers) + Phase 2.3 (State Variables)
- Phase 4 (UI) + Phase 5 (SSE)
- Phase 6.1, 6.2, 6.3 (Different test scenarios)

**Must be sequential:**
- Phase 1 must complete before Phase 2 (need API structure)
- Phase 2 must complete before Phase 3 (need data model)
- Phase 3 must complete before Phase 4/5 (need working logic)
- Phase 6 must complete before Phase 7 (validate before documenting)

---

## Success Metrics

### Functional Metrics
- [ ] Zero 403 errors in MyRoom
- [ ] All supervised activities visible
- [ ] Correct student counts in tabs
- [ ] Badge colors match rules (green/blue)
- [ ] SSE updates work for all activities

### Performance Metrics
- [ ] Page load time < 2 seconds (with 3 activities)
- [ ] Tab switch time < 500ms
- [ ] No memory leaks (SSE cleanup verified)
- [ ] No unnecessary re-renders

### Code Quality Metrics
- [ ] `npm run check` passes (0 warnings)
- [ ] TypeScript strict mode satisfied
- [ ] No console errors in production mode
- [ ] Code coverage maintained (if tests exist)

---

## Rollback Plan

**If critical issues arise:**

```bash
# Immediate rollback
git revert HEAD

# Or restore specific file
git checkout HEAD~1 -- frontend/src/app/myroom/page.tsx
```

**Risk:** Low - Changes isolated to MyRoom page only

**Recovery:** < 5 minutes to rollback via Git
