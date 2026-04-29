// app/api/active-supervision-dashboard/route.ts
// BFF (Backend-for-Frontend) endpoint for Active Supervisions Dashboard
// Consolidates 8+ API calls into 1 to eliminate redundant auth() overhead
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import { auth } from "~/server/auth";
import { isAdmin } from "~/lib/auth-utils";

// Backend response types for supervised/active groups
interface BackendActiveGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
  end_time?: string;
}

// Backend response for unclaimed groups
interface BackendUnclaimedGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
}

// Backend response for staff
interface BackendStaff {
  id: number;
  person_id: number;
  role?: string;
  person?: {
    first_name: string;
    last_name: string;
  };
}

// Backend response for educational groups
interface BackendEducationalGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
}

// Backend response for room
interface BackendRoom {
  id: number;
  name: string;
  building?: string;
  floor?: number;
}

// Backend response for visits with display data
interface BackendVisitDisplay {
  id: number;
  student_id: number;
  active_group_id: number;
  check_in_time: string;
  check_out_time?: string;
  actual_arrival_time?: string;
  actual_pickup_time?: string;
  student_name?: string;
  school_class?: string;
  group_name?: string;
  sick?: boolean;
  sick_since?: string;
  excused?: boolean;
  excused_since?: string;
  is_active: boolean;
}

// Backend response for Schulhof status
interface BackendSchulhofSupervisor {
  id: number;
  staff_id: number;
  name: string;
  is_current_user: boolean;
}

interface BackendSchulhofStatus {
  exists: boolean;
  room_id?: number;
  room_name: string;
  activity_group_id?: number;
  active_group_id?: number;
  is_user_supervising: boolean;
  supervision_id?: number;
  supervisor_count: number;
  student_count: number;
  supervisors: BackendSchulhofSupervisor[];
}

// Combined dashboard response type
interface ActiveSupervisionDashboardResponse {
  // User's supervised active groups (with room info pre-loaded)
  supervisedGroups: Array<{
    id: string;
    name: string;
    room_id?: string;
    room?: { id: string; name: string };
  }>;

  // Unclaimed groups available to claim
  unclaimedGroups: Array<{
    id: string;
    name: string;
    room?: { name: string };
  }>;

  // Current staff info
  currentStaff: {
    id: string;
  } | null;

  // Educational groups for permission checking
  educationalGroups: Array<{
    id: string;
    name: string;
    room?: { name: string };
  }>;

  // Visits for first supervised room (pre-loaded)
  firstRoomVisits: Array<{
    studentId: string;
    studentName: string;
    schoolClass: string;
    groupName: string;
    activeGroupId: string;
    checkInTime: string;
    actualArrivalTime?: string;
    actualPickupTime?: string;
    isActive: boolean;
    sick?: boolean;
    sickSince?: string;
    excused?: boolean;
    excusedSince?: string;
  }>;

  // ID of first room (for state initialization)
  firstRoomId: string | null;

  // Schulhof (Schoolyard) status - always included for permanent tab
  schulhofStatus: {
    exists: boolean;
    roomId: string | null;
    roomName: string;
    activityGroupId: string | null;
    activeGroupId: string | null;
    isUserSupervising: boolean;
    supervisionId: string | null;
    supervisorCount: number;
    studentCount: number;
    supervisors: Array<{
      id: string;
      staffId: string;
      name: string;
      isCurrentUser: boolean;
    }>;
  } | null;
}

/**
 * GET /api/active-supervision-dashboard
 *
 * BFF endpoint that fetches all data needed for the Active Supervisions page in a single request.
 * This eliminates 8+ separate auth() calls (each ~300ms) by making one auth() call
 * and then fetching data in parallel from the Go backend.
 *
 * Performance improvement: ~2500-4000ms → ~400-500ms (80% faster)
 */
export const GET = createGetHandler<ActiveSupervisionDashboardResponse>(
  async (_request: NextRequest, token: string) => {
    // Resolve the caller's role once so we can pick the right supervised-groups
    // endpoint. Admins (including dual-role teacher-admins) go through
    // /supervisors/all first; non-admins skip that call entirely and use
    // /me/groups/supervised. Routing by role avoids an unnecessary 403 +
    // fallback round-trip on every non-admin dashboard load.
    const session = await auth();
    const shouldUseAdminEndpoint = isAdmin(session);

    // Step 1: Fetch all initial data in parallel (including Schulhof status)
    const [
      supervisedResult,
      unclaimedResult,
      staffResult,
      groupsResult,
      schulhofResult,
    ] = await Promise.all([
      // User's supervised active groups. For admin-only users, try the admin
      // overview endpoint first. If it returns 403 (setting disabled) fall
      // back to the caregiver endpoint so the user keeps their own rooms.
      // Other errors (5xx, network) are not swallowed — they propagate so
      // the frontend can surface them instead of silently rendering empty.
      shouldUseAdminEndpoint
        ? apiGet<{ data: BackendActiveGroup[] | null }>(
            "/api/active/supervisors/all",
            token,
          ).catch((err: unknown) => {
            // Only fall back on a forbidden response (setting disabled or
            // insufficient role). Any other error (5xx, network) propagates
            // so the caller sees a real failure instead of a silent empty.
            const msg = err instanceof Error ? err.message : String(err);
            if (msg.includes("(403)") || msg.includes(" 403 ")) {
              return apiGet<{ data: BackendActiveGroup[] | null }>(
                "/api/me/groups/supervised",
                token,
              );
            }
            throw err;
          })
        : apiGet<{ data: BackendActiveGroup[] | null }>(
            "/api/me/groups/supervised",
            token,
          ),

      // Unclaimed groups available to claim
      apiGet<{ data: BackendUnclaimedGroup[] | null }>(
        "/api/active/groups/unclaimed",
        token,
      ).catch(() => ({ data: [] as BackendUnclaimedGroup[] })),

      // Current staff info
      apiGet<{ data: BackendStaff }>("/api/me/staff", token).catch(() => ({
        data: null as BackendStaff | null,
      })),

      // Educational groups for permission checking
      apiGet<{ data: BackendEducationalGroup[] | null }>(
        "/api/me/groups",
        token,
      ).catch(() => ({ data: [] as BackendEducationalGroup[] })),

      // Schulhof status for permanent tab
      apiGet<{ data: BackendSchulhofStatus }>(
        "/api/active/schulhof/status",
        token,
      ).catch(() => ({ data: null as BackendSchulhofStatus | null })),
    ]);

    // Extract data with null safety, sorted by room name for deterministic order
    const supervisedGroups = (
      Array.isArray(supervisedResult.data) ? supervisedResult.data : []
    ).sort((a, b) =>
      (a.room?.name ?? a.name ?? "").localeCompare(
        b.room?.name ?? b.name ?? "",
        "de",
      ),
    );
    const unclaimedGroups = Array.isArray(unclaimedResult.data)
      ? unclaimedResult.data
      : [];
    const currentStaff = staffResult.data;
    const educationalGroups = Array.isArray(groupsResult.data)
      ? groupsResult.data
      : [];
    const schulhofData = schulhofResult.data;

    // Transform Schulhof status to frontend format
    const schulhofStatus = schulhofData
      ? {
          exists: schulhofData.exists,
          roomId: schulhofData.room_id?.toString() ?? null,
          roomName: schulhofData.room_name,
          activityGroupId: schulhofData.activity_group_id?.toString() ?? null,
          activeGroupId: schulhofData.active_group_id?.toString() ?? null,
          isUserSupervising: schulhofData.is_user_supervising,
          supervisionId: schulhofData.supervision_id?.toString() ?? null,
          supervisorCount: schulhofData.supervisor_count,
          studentCount: schulhofData.student_count,
          supervisors: (schulhofData.supervisors ?? []).map((s) => ({
            id: s.id.toString(),
            staffId: s.staff_id.toString(),
            name: s.name,
            isCurrentUser: s.is_current_user,
          })),
        }
      : null;

    // If no supervised groups, return early with just unclaimed groups data
    if (supervisedGroups.length === 0) {
      return {
        supervisedGroups: [],
        unclaimedGroups: unclaimedGroups.map((g) => ({
          id: g.id.toString(),
          name: g.name,
          room: g.room ? { name: g.room.name } : undefined,
        })),
        currentStaff: currentStaff ? { id: currentStaff.id.toString() } : null,
        educationalGroups: educationalGroups.map((g) => ({
          id: g.id.toString(),
          name: g.name,
          room: g.room ? { name: g.room.name } : undefined,
        })),
        firstRoomVisits: [],
        firstRoomId: null,
        schulhofStatus,
      };
    }

    // Step 2: Enrich supervised groups with room info and fetch visits for first room
    const firstGroup = supervisedGroups[0];
    const firstGroupId = firstGroup ? firstGroup.id.toString() : null;

    // Prepare parallel requests for room info (for groups missing room data)
    const enrichedGroups = await Promise.all(
      supervisedGroups.map(async (group) => {
        // If room info already present, use it
        if (group.room?.name) {
          return {
            id: group.id.toString(),
            name: group.name,
            room_id: group.room_id?.toString(),
            room: { id: group.room.id.toString(), name: group.room.name },
          };
        }

        // Otherwise fetch room info if room_id exists
        if (group.room_id) {
          try {
            const roomResponse = await apiGet<{ data: BackendRoom }>(
              `/api/rooms/${group.room_id}`,
              token,
            );
            return {
              id: group.id.toString(),
              name: group.name,
              room_id: group.room_id.toString(),
              room: roomResponse.data
                ? {
                    id: roomResponse.data.id.toString(),
                    name: roomResponse.data.name,
                  }
                : undefined,
            };
          } catch {
            return {
              id: group.id.toString(),
              name: group.name,
              room_id: group.room_id.toString(),
              room: undefined,
            };
          }
        }

        return {
          id: group.id.toString(),
          name: group.name,
          room_id: undefined,
          room: undefined,
        };
      }),
    );

    // Step 3: Fetch visits for first room (pre-load for immediate display)
    let firstRoomVisits: ActiveSupervisionDashboardResponse["firstRoomVisits"] =
      [];

    if (firstGroupId) {
      try {
        const visitsResponse = await apiGet<{ data: BackendVisitDisplay[] }>(
          `/api/active/groups/${firstGroupId}/visits/display`,
          token,
        );

        firstRoomVisits = (visitsResponse.data ?? [])
          .filter((v) => v.is_active)
          .map((v) => ({
            studentId: v.student_id.toString(),
            studentName: v.student_name ?? "",
            schoolClass: v.school_class ?? "",
            groupName: v.group_name ?? "",
            activeGroupId: v.active_group_id.toString(),
            checkInTime: v.check_in_time,
            actualArrivalTime: v.actual_arrival_time,
            actualPickupTime: v.actual_pickup_time,
            isActive: v.is_active,
            sick: v.sick,
            sickSince: v.sick_since,
            excused: v.excused,
            excusedSince: v.excused_since,
          }));
      } catch {
        firstRoomVisits = [];
      }
    }

    return {
      supervisedGroups: enrichedGroups,
      unclaimedGroups: unclaimedGroups.map((g) => ({
        id: g.id.toString(),
        name: g.name,
        room: g.room ? { name: g.room.name } : undefined,
      })),
      currentStaff: currentStaff ? { id: currentStaff.id.toString() } : null,
      educationalGroups: educationalGroups.map((g) => ({
        id: g.id.toString(),
        name: g.name,
        room: g.room ? { name: g.room.name } : undefined,
      })),
      firstRoomVisits,
      firstRoomId: firstGroupId,
      schulhofStatus,
    };
  },
);
