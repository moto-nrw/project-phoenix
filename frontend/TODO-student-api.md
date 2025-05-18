# TODO: Student API Integration

## Status
The Groups functionality is complete and working. The Groups detail page currently has a placeholder for students.

## What needs to be done

1. **Frontend: Create the missing API route**
   - Create `/src/app/api/groups/[id]/students/route.ts`
   - This should proxy to the backend endpoint `/api/groups/{id}/students`
   - The backend endpoint already exists and is functional

2. **Update the Groups detail page**
   - Remove the TODO placeholder in `/src/app/database/groups/[id]/page.tsx`
   - Re-enable the `groupService.getGroupStudents(groupId)` call in the useEffect

## Code Template for the missing route

```typescript
// src/app/api/groups/[id]/students/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import type { BackendStudent } from "~/lib/student-helpers";

/**
 * Handler for GET /api/groups/[id]/students
 * Returns all students in a specific group
 */
export const GET = createGetHandler(async (request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  const endpoint = `/api/groups/${id}/students`;
  
  // Fetch students from the API
  return await apiGet<BackendStudent[]>(endpoint, token);
});
```

## Integration Points
- Backend endpoint exists: `GET /api/groups/{id}/students` (see backend/api/groups/api.go:49)
- Frontend service exists: `groupService.getGroupStudents()` (see frontend/src/lib/api.ts:889)
- Component is ready: Groups detail page just needs the TODO removed

## Current State
- Groups list: ✅ Working
- Group details: ✅ Working
- Group create/edit/delete: ✅ Working
- Students in groups: ⏳ Placeholder (awaiting student API completion)