# Refactor MyRoom to Activity-Based Logic

## Overview

Refactor the MyRoom page from group-based supervision to activity-based supervision. Currently, MyRoom shows only OGS groups where the user is the designated group supervisor, causing 403 errors when trying to view standalone activities (e.g., "Zeichnen" in "Mensa" with children from multiple groups). The correct behavior is to show ALL activities the user is actively supervising, regardless of whether they're group sessions or standalone activities.

## Problem Statement

### Current (Incorrect) Behavior

**What MyRoom does now:**
1. Fetches `getMyActiveGroups()` - Returns only groups where user is THE group supervisor
2. Shows tabs for each group's room
3. Attempts to load visits using group-based API
4. **FAILS with 403** when user supervises a standalone activity with children from groups they don't own

**Example Failure Scenario:**
```
User: Andreas supervisiert "Zeichnen" in Raum "Mensa"
Teilnehmer:
  - Emma (Gruppe 1A)
  - Ben (Gruppe 2A)
  - Mia (Gruppe 1B - Andreas ist NICHT Supervisor dieser Gruppe)

MyRoom zeigt "Mensa"-Tab
User klickt auf "Mensa" →
API-Call: GET /api/active/groups/18/visits/display (Gruppe 1B)
Backend: 403 Forbidden - "not authorized to view this group"
Result: Zeigt "Keine Berechtigung" + leere Liste ❌
```

### Expected (Correct) Behavior

**What MyRoom should do:**
1. Fetch all activities where user is ANY supervisor (not just group owner)
2. Show tabs for each activity's room
3. Load visits for activities the user supervises
4. **NO 403 errors** - user sees all children in their supervised activities

**Corrected Scenario:**
```
User: Andreas supervisiert "Zeichnen" in Raum "Mensa"

MyRoom zeigt "Mensa"-Tab
User klickt auf "Mensa" →
API-Call: GET /api/active/supervisors/staff/{andreasId}/active
          GET /api/active/groups/{zeichnenId}/visits
Backend: 200 OK - Returns all children in "Zeichnen" activity
Result: Zeigt Emma, Ben, Mia ✅
Badge: Zeigt Gruppennamen (Emma "1A", Ben "2A", Mia "1B")
```

## Goals

1. **Change data source from group-based to activity-based:**
   - Replace `getMyActiveGroups()` with supervision-based API
   - Fetch activities where user is ANY supervisor (main supervisor OR assistant)
   - Include both OGS group sessions AND standalone activities

2. **Fix 403 Forbidden errors:**
   - Stop querying group visits when user doesn't own the group
   - Query activity visits where user IS a supervisor
   - Proper permission checks based on supervision role

3. **Show children from multiple groups:**
   - Display all children participating in an activity
   - Show group badges to indicate which group each child is from
   - Support multi-group activities (e.g., "Zeichnen" with kids from 1A, 2A, 1B)

4. **Maintain existing features:**
   - Room selection for 5+ supervisions
   - SSE real-time updates
   - Student filtering and search
   - Check-out functionality

## Success Criteria

- [ ] MyRoom loads without 403 errors
- [ ] Shows ALL activities user supervises (group sessions + standalone activities)
- [ ] Tabs show room names where activities take place
- [ ] Student lists include children from ALL participating groups
- [ ] Group badges show which group each child belongs to
- [ ] Color coding: 🟢 Green = activity in user's own group room, 🔵 Blue = external room
- [ ] SSE updates work correctly for all supervised activities
- [ ] No permission errors when switching between activity tabs
- [ ] `npm run check` passes with 0 warnings

## Non-Goals

- Creating new backend API endpoints (use existing supervision endpoints)
- Changing group supervision logic (only MyRoom changes)
- Modifying OGS Groups page (already works correctly)
- Backend permission model changes (work within existing permissions)

## Constraints

- **Use existing API endpoints:** Leverage `/api/active/supervisors/staff/{id}/active`
- **Maintain backwards compatibility:** Don't break existing features
- **Zero warnings policy:** All changes must pass `npm run check`
- **Performance:** Minimize API calls, use bulk endpoints where possible
- **Type safety:** Maintain strict TypeScript types throughout

## Dependencies

### Frontend Dependencies
- ✅ `usercontext-api.ts` - Already has `getMySupervisedGroups()` function
- ✅ No session changes needed - JWT token is sufficient
- ✅ No new API routes needed

### Backend Dependencies (VERIFIED - Already Working!)
- ✅ `GET /api/me/groups/supervised` - Returns ALL supervisions (auto-detects user from JWT)
  - Backend: `services/usercontext/usercontext_service.go:438-473`
  - Logic: `GetCurrentStaff(JWT) → FindActiveByStaffID() → Returns active groups`
  - **Works for users WITHOUT OGS groups** (returns empty array, no error)
  - **Includes standalone activities** (both stored in active.groups table)

### Key Backend Code Analysis

**From `usercontext_service.go` lines 438-473:**
```go
func (s *userContextService) GetMySupervisedGroups(ctx context.Context) ([]*active.Group, error) {
    // Extracts staff from JWT - NO staff_id parameter needed!
    staff, err := s.GetCurrentStaff(ctx)

    // Finds ALL supervisions (groups + standalone activities)
    supervisions, err := s.supervisorRepo.FindActiveByStaffID(ctx, staff.ID)

    // Returns active groups for these supervisions
    // Includes: OGS groups, standalone activities, combined groups
    return supervisedGroups, nil
}
```

**This means:**
- ✅ NO circular dependency (doesn't need staff_id as input)
- ✅ Works for users with ONLY standalone activities
- ✅ Works for users with ONLY OGS groups
- ✅ Works for mixed scenarios
- ✅ Returns empty array (not error) if no supervisions

## Timeline Estimate

**UPDATE: Much simpler than originally estimated!**

After analyzing backend code, this is primarily a **1-line change**:
- **Implementation:** 15 minutes (change function call)
- **Testing:** 30 minutes (verify all scenarios)
- **Documentation:** 15 minutes
- **Total:** ~1 hour (down from original 3-4 hours estimate)

**Original estimate was for creating new APIs - not needed!**

## Impact Assessment

**User Experience:**
- ✅ Fixes 403 errors - major UX improvement
- ✅ Shows complete supervision scope
- ✅ Multi-group activities now visible
- ⚠️ UI slightly different (shows activities instead of groups)

**Development:**
- ⚠️ Significant refactoring of MyRoom page logic
- ✅ No breaking changes to other pages
- ✅ Uses existing API endpoints
- ✅ Maintains existing features (search, filters, SSE)

**Maintenance:**
- ✅ Aligns with actual business logic (activity supervision)
- ✅ More maintainable (correct abstraction)
- ✅ Reduces confusion between group vs activity supervision

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Staff ID not available in session | High | Fetch from profile API if needed |
| Supervision API returns unexpected data | High | Test API endpoint before implementing |
| Performance degradation (multiple API calls) | Medium | Use bulk endpoints, implement caching |
| Breaking SSE updates | Medium | Update event handlers to use activity IDs |
| Confusion between group and activity concepts | Low | Clear naming, documentation |

## Open Questions

~~1. **Staff ID retrieval:**~~ ✅ **RESOLVED**
   - Backend extracts from JWT token automatically
   - Frontend uses `/api/me/groups/supervised` (no parameters needed)

~~2. **Supervision API response structure:**~~ ✅ **RESOLVED**
   - Returns `ActiveGroup[]` with room information
   - Uses existing `BackendActiveGroup` type (active-helpers.ts)

~~3. **Visit API permissions:**~~ ✅ **RESOLVED**
   - Current implementation already uses visits API correctly
   - Just need to switch data source from "active" to "supervised"

~~4. **SSE event structure:**~~ ✅ **RESOLVED**
   - Events use `active_group_id`
   - No changes needed to event handling
   - Works for both group sessions and standalone activities

**All questions resolved through backend code analysis!**

## References

- Current MyRoom implementation: `frontend/src/app/myroom/page.tsx`
- Active service functions: `frontend/src/lib/active-service.ts`
- Supervision context: `frontend/src/lib/supervision-context.tsx`
- Backend supervision endpoints: `backend/api/active/` (assumed)

## Related Work

- OGS Groups page (already activity-aware, works correctly)
- Student detail page (shows correct location badges)
- SSE integration (needs update for activity-based events)
