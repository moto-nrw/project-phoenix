// app/api/active-supervision-dashboard/route.ts
// BFF (Backend-for-Frontend) endpoint for Active Supervisions Dashboard
// Consolidates 8+ API calls into 1 to eliminate redundant auth() overhead
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import type { CareDayStatus } from "~/lib/timetable-types";

// Backend response types for supervised/active groups
interface BackendActiveGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
    color?: string | null;
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
  color?: string | null;
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
  // Authenticated /api/students/{id}/photo/{filename} URL — backend
  // rewrites the raw /uploads path before returning.
  photo_url?: string;
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

interface BackendPlannedTimetableInstance {
  id: number;
  title: string;
  date: string;
  start_time: string;
  end_time: string;
  room_id: number;
  room_name?: string | null;
  status: "planned" | "active" | "completed" | "cancelled";
  is_overdue: boolean;
  minutes_until_start: number;
  expected_students_count: number;
  present_students_count: number;
  not_scheduled_students_count?: number;
  assigned_staff_ids: number[];
  is_assigned?: boolean;
  is_primary?: boolean;
  is_substitute?: boolean;
  is_absent?: boolean;
  can_start?: boolean;
  start_available_at?: string;
  start_expires_at?: string;
  roster_preview?: BackendTimetableRosterRow[];
}

interface BackendTimetableRosterRow {
  student_id: number;
  student_name: string;
  school_class: string;
  group_name: string;
  planned: boolean;
  is_unplanned: boolean;
  currently_present: boolean;
  visit_id?: number | null;
  status: "expected" | "present" | "absent";
  substatus?: "late" | "excused" | "sick" | "field_trip" | "other" | null;
  note?: string | null;
  checked_in_at?: string | null;
  visit_entry_time?: string | null;
  warnings?: BackendTimetableRosterWarning[];
  care_day_status?: CareDayStatus;
}

interface BackendTimetableRosterWarning {
  kind: string;
  message: string;
  expected_arrival?: string | null;
  slot_start?: string | null;
  expected_group_id?: number | null;
  expected_group_name?: string | null;
  current_education_group_id?: number | null;
}

interface BackendTimetableOperationCapabilities {
  web_spontaneous_activities_enabled?: boolean;
}

// Backend response for today's running timetable sessions (#2265)
interface BackendActiveTimetableSession {
  active_group_id: number;
  instance_id: number;
  title: string;
  start_time: string;
  end_time: string;
}

// Combined dashboard response type
interface ActiveSupervisionDashboardResponse {
  // User's supervised active groups (with room info pre-loaded)
  supervisedGroups: Array<{
    id: string;
    name: string;
    room_id?: string;
    room?: { id: string; name: string; color?: string | null };
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
    photoUrl?: string;
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
  capabilities?: {
    webSpontaneousActivitiesEnabled: boolean;
  };
  // Plan windows of today's running sessions, keyed by active group, so tab
  // labels can show "Aktivitätsname · Planzeit" (#2265)
  activeSessions: Array<{
    activeGroupId: string;
    instanceId: string;
    title: string;
    startTime: string;
    endTime: string;
  }>;
  plannedNow: Array<{
    id: string;
    title: string;
    date: string;
    startTime: string;
    endTime: string;
    roomId: string;
    roomName: string | null;
    status: "planned" | "active" | "completed" | "cancelled";
    isOverdue: boolean;
    minutesUntilStart: number;
    expectedStudentsCount: number;
    presentStudentsCount: number;
    notScheduledStudentsCount: number;
    assignedStaffIds: string[];
    isAssigned: boolean;
    isPrimary: boolean;
    isSubstitute: boolean;
    isAbsent: boolean;
    canStart: boolean;
    startAvailableAt: string;
    startExpiresAt: string;
    rosterPreview: Array<{
      studentId: string;
      studentName: string;
      schoolClass: string;
      groupName: string;
      planned: boolean;
      isUnplanned: boolean;
      currentlyPresent: boolean;
      visitId: string | null;
      status: "expected" | "present" | "absent";
      substatus: "late" | "excused" | "sick" | "field_trip" | "other" | null;
      note: string | null;
      checkedInAt: string | null;
      visitEntryTime: string | null;
    }>;
  }>;
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
    // Step 1: Fetch all initial data in parallel (including Schulhof status)
    const [
      supervisedResult,
      unclaimedResult,
      staffResult,
      groupsResult,
      schulhofResult,
      plannedNowResult,
      activeSessionsResult,
    ] = await Promise.all([
      // Try the all-groups operational endpoint first. It is available to
      // configured admins and to permission-bearing staff in open-care mode.
      // Fixed-group staff receive 403 and fall back to their own rooms.
      // Other errors (5xx, network) are not swallowed — they propagate so
      // the frontend can surface them instead of silently rendering empty.
      apiGet<{ data: BackendActiveGroup[] | null }>(
        "/api/active/supervisors/all",
        token,
      ).catch((err: unknown) => {
        const msg = err instanceof Error ? err.message : String(err);
        if (msg.includes("(403)") || msg.includes(" 403 ")) {
          return apiGet<{ data: BackendActiveGroup[] | null }>(
            "/api/me/groups/supervised",
            token,
          );
        }
        throw err;
      }),

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
      apiGet<{ data: { instances: BackendPlannedTimetableInstance[] } }>(
        "/api/timetable/operations/planned-now?horizon_minutes=480&limit=5&include_roster=true",
        token,
      ).catch(() => ({
        data: { instances: [] as BackendPlannedTimetableInstance[] },
      })),

      // Plan windows of today's running sessions for tab labels (#2265).
      // Older backends without the endpoint degrade to name-only labels.
      apiGet<{ data: { sessions: BackendActiveTimetableSession[] } }>(
        "/api/timetable/operations/active-sessions",
        token,
      ).catch(() => ({
        data: { sessions: [] as BackendActiveTimetableSession[] },
      })),
    ]);
    const activeSessions = (activeSessionsResult.data?.sessions ?? []).map(
      (s) => ({
        activeGroupId: s.active_group_id.toString(),
        instanceId: s.instance_id.toString(),
        title: s.title,
        startTime: s.start_time,
        endTime: s.end_time,
      }),
    );

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
    const plannedNow = (plannedNowResult.data?.instances ?? []).map((i) => ({
      id: i.id.toString(),
      title: i.title,
      date: i.date,
      startTime: i.start_time,
      endTime: i.end_time,
      roomId: i.room_id.toString(),
      roomName: i.room_name ?? null,
      status: i.status,
      isOverdue: i.is_overdue,
      minutesUntilStart: i.minutes_until_start,
      expectedStudentsCount: i.expected_students_count,
      presentStudentsCount: i.present_students_count,
      notScheduledStudentsCount: i.not_scheduled_students_count ?? 0,
      assignedStaffIds: i.assigned_staff_ids.map(String),
      isAssigned: i.is_assigned ?? false,
      isPrimary: i.is_primary ?? false,
      isSubstitute: i.is_substitute ?? false,
      isAbsent: i.is_absent ?? false,
      canStart: i.can_start ?? false,
      startAvailableAt: i.start_available_at ?? "",
      startExpiresAt: i.start_expires_at ?? "",
      rosterPreview: (i.roster_preview ?? []).map((row) => ({
        studentId: row.student_id.toString(),
        studentName: row.student_name,
        schoolClass: row.school_class,
        groupName: row.group_name,
        planned: row.planned,
        isUnplanned: row.is_unplanned,
        currentlyPresent: row.currently_present,
        visitId: row.visit_id?.toString() ?? null,
        status: row.status,
        substatus: row.substatus ?? null,
        note: row.note ?? null,
        checkedInAt: row.checked_in_at ?? null,
        visitEntryTime: row.visit_entry_time ?? null,
        careDayStatus: row.care_day_status ?? "unknown",
        warnings: (row.warnings ?? []).map((warning) => ({
          kind: warning.kind,
          message: warning.message,
          expectedArrival: warning.expected_arrival ?? null,
          slotStart: warning.slot_start ?? null,
          expectedGroupId: warning.expected_group_id?.toString() ?? null,
          expectedGroupName: warning.expected_group_name ?? null,
          currentEducationGroupId:
            warning.current_education_group_id?.toString() ?? null,
        })),
      })),
    }));

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

    const fetchCapabilities = async () => {
      const result = await Promise.resolve(
        apiGet<{ data: BackendTimetableOperationCapabilities }>(
          "/api/timetable/operations/capabilities",
          token,
        ),
      ).catch(() => ({
        data: {
          web_spontaneous_activities_enabled: false,
        } satisfies BackendTimetableOperationCapabilities,
      }));
      return {
        webSpontaneousActivitiesEnabled:
          result?.data?.web_spontaneous_activities_enabled === true,
      };
    };

    // If no supervised groups, return early with just unclaimed groups data
    if (supervisedGroups.length === 0) {
      const capabilities = await fetchCapabilities();
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
        capabilities,
        activeSessions,
        plannedNow,
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
            room: {
              id: group.room.id.toString(),
              name: group.room.name,
              color: group.room.color ?? null,
            },
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
                    color: roomResponse.data.color ?? null,
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
            photoUrl: v.photo_url,
          }));
      } catch {
        firstRoomVisits = [];
      }
    }

    const capabilities = await fetchCapabilities();

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
      capabilities,
      activeSessions,
      plannedNow,
    };
  },
);
