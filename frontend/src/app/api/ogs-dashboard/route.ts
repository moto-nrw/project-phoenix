// app/api/ogs-dashboard/route.ts
// BFF (Backend-for-Frontend) endpoint for OGS Dashboard
// Consolidates 4 API calls into 1 to eliminate redundant auth() overhead
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

// Backend response types
interface BackendEducationalGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
  via_substitution?: boolean;
}

interface BackendStudent {
  id: number;
  first_name: string;
  second_name: string;
  name?: string;
  school_class?: string;
  current_location?: string;
  sick_since?: string;
  sick_until?: string;
  location_since?: string;
}

interface BackendRoomStatus {
  group_has_room: boolean;
  group_room_id?: number;
  student_room_status: Record<
    string,
    {
      in_group_room: boolean;
      current_room_id?: number;
      first_name?: string;
      last_name?: string;
      reason?: string;
    }
  >;
}

interface BackendSubstitution {
  id: number;
  group_id: number;
  regular_staff_id: number | null;
  substitute_staff_id: number;
  substitute_staff?: {
    person?: {
      first_name: string;
      last_name: string;
    };
  };
  start_date: string;
  end_date: string;
}

// Combined dashboard response type
interface OGSDashboardResponse {
  groups: BackendEducationalGroup[];
  students: BackendStudent[];
  roomStatus: BackendRoomStatus | null;
  substitutions: BackendSubstitution[];
  firstGroupId: string | null;
}

/**
 * GET /api/ogs-dashboard
 *
 * BFF endpoint that fetches all data needed for the OGS groups page in a single request.
 * This eliminates 4 separate auth() calls (each ~300ms) by making one auth() call
 * and then fetching data in parallel from the Go backend.
 *
 * Performance improvement: ~1200ms → ~400ms (70% faster)
 */
export const GET = createGetHandler<OGSDashboardResponse>(
  async (_request: NextRequest, token: string) => {
    const startTime = Date.now();
    console.log("⏱️ [BFF] Starting OGS dashboard data fetch...");

    // Step 1: Fetch user's educational groups first (we need the first group ID)
    const groupsStart = Date.now();
    const groupsResponse = await apiGet<{ data: BackendEducationalGroup[] }>(
      "/api/me/groups",
      token,
    );
    const groups = groupsResponse.data ?? [];
    console.log(
      `⏱️ [BFF] Groups fetch: ${Date.now() - groupsStart}ms (${groups.length} groups)`,
    );

    // If no groups, return early with empty data
    if (groups.length === 0) {
      console.log(`⏱️ [BFF] No groups found, returning early`);
      return {
        groups: [],
        students: [],
        roomStatus: null,
        substitutions: [],
        firstGroupId: null,
      };
    }

    const firstGroup = groups[0];
    if (!firstGroup) {
      // This shouldn't happen since we checked groups.length > 0, but TypeScript needs the guard
      return {
        groups: [],
        students: [],
        roomStatus: null,
        substitutions: [],
        firstGroupId: null,
      };
    }
    const firstGroupId = firstGroup.id.toString();

    // Step 2: Fetch students, room status, and substitutions in parallel
    const parallelStart = Date.now();
    const [studentsResult, roomStatusResult, substitutionsResult] =
      await Promise.all([
        // Fetch students for first group
        apiGet<{ data: BackendStudent[] }>(
          `/api/students?group_id=${firstGroupId}`,
          token,
        ).catch((err) => {
          console.error("[BFF] Students fetch error:", err);
          return { data: [] as BackendStudent[] };
        }),

        // Fetch room status for first group
        apiGet<{ data: BackendRoomStatus }>(
          `/api/groups/${firstGroupId}/students/room-status`,
          token,
        ).catch((err) => {
          console.error("[BFF] Room status fetch error:", err);
          return { data: null as BackendRoomStatus | null };
        }),

        // Fetch substitutions for first group
        apiGet<{ data: BackendSubstitution[] }>(
          `/api/groups/${firstGroupId}/substitutions`,
          token,
        ).catch((err) => {
          console.error("[BFF] Substitutions fetch error:", err);
          return { data: [] as BackendSubstitution[] };
        }),
      ]);

    console.log(
      `⏱️ [BFF] Parallel fetches: ${Date.now() - parallelStart}ms (students, roomStatus, substitutions)`,
    );
    console.log(`⏱️ [BFF] ✅ Total: ${Date.now() - startTime}ms`);

    return {
      groups,
      students: studentsResult.data ?? [],
      roomStatus: roomStatusResult.data,
      substitutions: substitutionsResult.data ?? [],
      firstGroupId,
    };
  },
);
