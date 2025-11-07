# PR #258 Restoration Plan

## Overview
PR #258 (Oct 27, 2025) implemented a unified location badge system that was overwritten by PR #257 (Nov 4, 2025). This document outlines the restoration strategy.

## PR #258: What Was Lost

### Files Added by PR #258 (now deleted)
1. `backend/api/common/student_locations.go` - Centralized student location logic
2. `backend/services/active/context.go` - Active context helpers
3. `backend/database/repositories/active/student_location_repository_test.go` - Test suite
4. `frontend/src/components/ui/location-badge.tsx` - Unified location badge component
5. `frontend/src/lib/location-helper.ts` - Location status helper functions

### Files Deleted by PR #258 (restored by PR #257)
These OLD components should be removed again:
1. `frontend/src/components/simple/student/ModernStatusBadge.tsx`
2. `frontend/src/components/simple/student/ModernStudentProfile.tsx`
3. `frontend/src/components/simple/student/StatusBadge.tsx`
4. `frontend/src/components/simple/student/StudentProfileCard.tsx`

### Files Modified by PR #258 (overwritten by PR #257)
These need their PR #258 versions restored:

**Backend:**
- `backend/api/active/checkout.go`
- `backend/api/base.go`
- `backend/api/groups/api.go`
- `backend/api/sse/api.go`
- `backend/api/sse/resource.go`
- `backend/api/students/api.go`
- `backend/auth/authorize/policies/student_visits_test.go`
- `backend/cmd/test_sql_syntax.sql`
- `backend/database/migrations/001003005_users_students.go`
- `backend/database/repositories/active/attendance_repository.go` (includes GetTodayByStudentIDs)
- `backend/database/repositories/active/group.go`
- `backend/database/repositories/active/visits.go`
- `backend/database/repositories/activities/student_enrollment.go`
- `backend/database/repositories/users/student.go`
- `backend/models/active/attendance.go`
- `backend/models/active/repository.go`
- `backend/models/users/repository.go`
- `backend/models/users/student.go`
- `backend/openspec/project.md`
- `backend/seed/fixed/students.go`
- `backend/seed/runtime/seeder.go`
- `backend/services/active/active_service.go` (340 lines changed)
- `backend/services/active/attendance_service_test.go`
- `backend/services/active/interface.go`
- `backend/services/active/timeout_simple_test.go`

**Frontend:**
- `frontend/package-lock.json`
- `frontend/src/app/api/students/[id]/current-location/route.ts`
- `frontend/src/app/api/students/[id]/route.ts`
- `frontend/src/app/api/students/route.ts`
- `frontend/src/app/database/students/csv-import/page.tsx`
- `frontend/src/app/myroom/page.tsx`
- `frontend/src/app/ogs_groups/page.tsx`
- `frontend/src/app/students/[id]/feedback_history/page.tsx`
- `frontend/src/app/students/[id]/mensa_history/page.tsx`
- `frontend/src/app/students/[id]/page.tsx` (major changes)
- `frontend/src/app/students/[id]/room_history/page.tsx`
- `frontend/src/app/students/search/page.tsx`
- `frontend/src/components/dashboard/responsive-layout.tsx`
- `frontend/src/components/simple/student/index.ts`
- `frontend/src/components/students/student-detail-view.tsx`
- `frontend/src/components/students/student-form.tsx`
- `frontend/src/components/students/student-list-item.tsx`
- `frontend/src/components/students/student-list.tsx`
- `frontend/src/lib/activity-helpers.ts`
- `frontend/src/lib/database/configs/students.config.tsx`
- `frontend/src/lib/group-helpers.ts`
- `frontend/src/lib/student-api.ts`
- `frontend/src/lib/student-helpers.ts`
- `frontend/src/lib/usercontext-api.ts`

## Conflict with PR #257

PR #257 made changes to the same files that PR #258 modified. The challenge is that we need to:

1. **Restore PR #258's unified badge system** (the new approach)
2. **Keep PR #257's guardian management features** (new functionality)
3. **Remove PR #257's badge system restoration** (duplicate/old approach)

## Restoration Strategy

### Phase 1: Restore New Files
Cherry-pick or extract these files from PR #258:
- `backend/api/common/student_locations.go`
- `backend/services/active/context.go`
- `backend/database/repositories/active/student_location_repository_test.go`
- `frontend/src/components/ui/location-badge.tsx`
- `frontend/src/lib/location-helper.ts`

### Phase 2: Remove Old Files
Delete these files that were restored by PR #257:
- `frontend/src/components/simple/student/ModernStatusBadge.tsx`
- `frontend/src/components/simple/student/ModernStudentProfile.tsx`
- `frontend/src/components/simple/student/StatusBadge.tsx`
- `frontend/src/components/simple/student/StudentProfileCard.tsx`

### Phase 3: Merge Modified Files
For files modified by both PRs, we need to:
1. Start with current development (which has PR #257's changes)
2. Apply PR #258's changes carefully, preserving PR #257's guardian features
3. Focus on location/badge related changes from PR #258
4. Keep all guardian-related code from PR #257

## Risk Areas

1. **backend/services/active/active_service.go** - Heavily modified by both PRs
2. **frontend/src/app/students/[id]/page.tsx** - Student detail page with major changes
3. **backend/database/repositories/active/attendance_repository.go** - GetTodayByStudentIDs method
4. **Student-related components** - May have dependencies on both systems

## Testing Requirements

After restoration:
1. Backend lint: `cd backend && golangci-lint run --timeout 10m`
2. Frontend lint: `cd frontend && npm run lint`
3. Frontend typecheck: `cd frontend && npm run typecheck`
4. Manual testing: Student location badges display correctly
5. Manual testing: Guardian management still works
6. API testing: Run Bruno tests for students and active sessions

## Git Commands

```bash
# Restore new files from PR #258
git checkout e961ea5 -- backend/api/common/student_locations.go
git checkout e961ea5 -- backend/services/active/context.go
git checkout e961ea5 -- backend/database/repositories/active/student_location_repository_test.go
git checkout e961ea5 -- frontend/src/components/ui/location-badge.tsx
git checkout e961ea5 -- frontend/src/lib/location-helper.ts

# Remove old badge components
git rm frontend/src/components/simple/student/ModernStatusBadge.tsx
git rm frontend/src/components/simple/student/ModernStudentProfile.tsx
git rm frontend/src/components/simple/student/StatusBadge.tsx
git rm frontend/src/components/simple/student/StudentProfileCard.tsx
```

## Notes

- PR #257's guardian management is a valuable feature and must be preserved
- PR #258's unified badge system is the correct architectural approach
- The old badge components are redundant and should be removed
- Some manual merging will be required for files modified by both PRs
