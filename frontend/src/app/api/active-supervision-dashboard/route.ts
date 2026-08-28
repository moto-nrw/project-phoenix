// app/api/active-supervision-dashboard/route.ts
// Proxy for the aggregated supervision dashboard projection (#2096).
// One backend request returns supervised sessions (rooms bulk-loaded),
// unclaimed groups, staff id, educational groups, Schulhof status,
// capabilities, active sessions, planned instances, and the selected
// session's visits, tracking indicators, and pickup/arrival times —
// replacing the former BFF fan-out of ~11 backend calls. Errors are
// forwarded as-is: the backend fails the whole request instead of
// degrading sections to empty arrays.
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import type { CareDayStatus } from "~/lib/timetable-types";

// ===== Wire types (Go: backend/services/supervisiondashboard/service.go) =====

interface WireGroup {
  id: string;
  name: string;
  room_id?: string;
  room_name?: string;
  room_color?: string | null;
}

interface WireUnclaimedGroup {
  id: string;
  room_name?: string;
}

interface WireEducationalGroup {
  id: string;
  name: string;
  room_name?: string;
}

interface WireSchulhofStatus {
  exists: boolean;
  room_id?: number;
  room_name: string;
  activity_group_id?: number;
  active_group_id?: number;
  is_user_supervising: boolean;
  supervision_id?: number;
  supervisor_count: number;
  student_count: number;
  supervisors: Array<{
    id: number;
    staff_id: number;
    name: string;
    is_current_user: boolean;
  }>;
}

interface WireActiveSession {
  active_group_id: number;
  instance_id: number;
  title: string;
  start_time: string;
  end_time: string;
}

interface WirePlannedInstance {
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
  pickup_times_loaded?: boolean;
  pickup_times_redacted?: boolean;
  roster_preview?: WireRosterRow[];
}

interface WireRosterRow {
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
  pickup_time?: string | null;
  warnings?: WireRosterWarning[];
  care_day_status?: CareDayStatus;
}

interface WireRosterWarning {
  kind: string;
  message: string;
  expected_arrival?: string | null;
  slot_start?: string | null;
  expected_group_id?: number | null;
  expected_group_name?: string | null;
  current_education_group_id?: number | null;
}

interface WireVisit {
  student_id: string;
  student_name: string;
  school_class: string;
  group_name: string;
  active_group_id: string;
  check_in_time: string;
  actual_arrival_time?: string;
  actual_pickup_time?: string;
  sick: boolean;
  sick_since?: string;
  excused: boolean;
  excused_since?: string;
  photo_url?: string;
}

interface WireDayNote {
  id: string;
  content: string;
}

interface WirePickupTime {
  student_id: string;
  date: string;
  weekday_name: string;
  pickup_time?: string;
  is_exception: boolean;
  notes?: string;
  day_notes?: WireDayNote[];
}

interface WireArrivalTime {
  student_id: string;
  date: string;
  weekday_name: string;
  expected_arrival?: string;
  is_exception: boolean;
  notes?: string;
  day_notes?: WireDayNote[];
}

interface WireDashboard {
  groups: WireGroup[];
  selected_group_id?: string;
  unclaimed_groups: WireUnclaimedGroup[];
  current_staff_id?: string;
  educational_groups: WireEducationalGroup[];
  schulhof_status: WireSchulhofStatus | null;
  capabilities: { web_spontaneous_activities_enabled: boolean };
  active_sessions: WireActiveSession[];
  planned_now: WirePlannedInstance[];
  visits: WireVisit[];
  tracking_indicators: { labels: string[]; results: Record<string, boolean[]> };
  pickup_times: WirePickupTime[];
  arrival_times: WireArrivalTime[];
}

// ===== Frontend response type (camelCase view the page consumes) =====

interface ActiveSupervisionDashboardResponse {
  supervisedGroups: Array<{
    id: string;
    name: string;
    room_id?: string;
    room?: { id: string; name: string; color?: string | null };
  }>;
  unclaimedGroups: Array<{
    id: string;
    name: string;
    room?: { name: string };
  }>;
  currentStaff: { id: string } | null;
  educationalGroups: Array<{
    id: string;
    name: string;
    room?: { name: string };
  }>;
  // Visits of the selected session (the backend resolves group_id, or the
  // first supervised session when the parameter is absent).
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
  firstRoomId: string | null;
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
    pickupTimesLoaded?: boolean;
    pickupTimesRedacted?: boolean;
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
      pickupTime?: string | null;
    }>;
  }>;
  // Folded-in sections of the selected session (#2096) — replace the former
  // separate supervision-visits / tracking / pickup / arrival fetches.
  selectedGroupId: string | null;
  trackingIndicators: { labels: string[]; results: Record<string, boolean[]> };
  pickupTimes: Array<{
    studentId: string;
    date: string;
    weekdayName: string;
    pickupTime: string | null;
    isException: boolean;
    notes: string;
    dayNotes: Array<{ id: string; content: string }>;
  }>;
  arrivalTimes: Array<{
    studentId: string;
    date: string;
    weekdayName: string;
    expectedArrival: string | null;
    isException: boolean;
    notes: string;
    dayNotes: Array<{ id: string; content: string }>;
  }>;
}

function mapDashboard(wire: WireDashboard): ActiveSupervisionDashboardResponse {
  return {
    supervisedGroups: (wire.groups ?? []).map((g) => ({
      id: g.id,
      name: g.name,
      room_id: g.room_id,
      room: g.room_id
        ? {
            id: g.room_id,
            name: g.room_name ?? "",
            color: g.room_color ?? null,
          }
        : undefined,
    })),
    unclaimedGroups: (wire.unclaimed_groups ?? []).map((g) => ({
      id: g.id,
      name: "",
      room: g.room_name ? { name: g.room_name } : undefined,
    })),
    currentStaff: wire.current_staff_id ? { id: wire.current_staff_id } : null,
    educationalGroups: (wire.educational_groups ?? []).map((g) => ({
      id: g.id,
      name: g.name,
      room: g.room_name ? { name: g.room_name } : undefined,
    })),
    firstRoomVisits: (wire.visits ?? []).map((v) => ({
      studentId: v.student_id,
      studentName: v.student_name,
      schoolClass: v.school_class,
      groupName: v.group_name,
      activeGroupId: v.active_group_id,
      checkInTime: v.check_in_time,
      actualArrivalTime: v.actual_arrival_time,
      actualPickupTime: v.actual_pickup_time,
      isActive: true,
      sick: v.sick,
      sickSince: v.sick_since,
      excused: v.excused,
      excusedSince: v.excused_since,
      photoUrl: v.photo_url,
    })),
    firstRoomId: wire.selected_group_id ?? null,
    schulhofStatus: wire.schulhof_status
      ? {
          exists: wire.schulhof_status.exists,
          roomId: wire.schulhof_status.room_id?.toString() ?? null,
          roomName: wire.schulhof_status.room_name,
          activityGroupId:
            wire.schulhof_status.activity_group_id?.toString() ?? null,
          activeGroupId:
            wire.schulhof_status.active_group_id?.toString() ?? null,
          isUserSupervising: wire.schulhof_status.is_user_supervising,
          supervisionId:
            wire.schulhof_status.supervision_id?.toString() ?? null,
          supervisorCount: wire.schulhof_status.supervisor_count,
          studentCount: wire.schulhof_status.student_count,
          supervisors: (wire.schulhof_status.supervisors ?? []).map((s) => ({
            id: s.id.toString(),
            staffId: s.staff_id.toString(),
            name: s.name,
            isCurrentUser: s.is_current_user,
          })),
        }
      : null,
    capabilities: {
      webSpontaneousActivitiesEnabled:
        wire.capabilities?.web_spontaneous_activities_enabled === true,
    },
    activeSessions: (wire.active_sessions ?? []).map((s) => ({
      activeGroupId: s.active_group_id.toString(),
      instanceId: s.instance_id.toString(),
      title: s.title,
      startTime: s.start_time,
      endTime: s.end_time,
    })),
    plannedNow: (wire.planned_now ?? []).map((i) => ({
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
      assignedStaffIds: (i.assigned_staff_ids ?? []).map(String),
      isAssigned: i.is_assigned ?? false,
      isPrimary: i.is_primary ?? false,
      isSubstitute: i.is_substitute ?? false,
      isAbsent: i.is_absent ?? false,
      canStart: i.can_start ?? false,
      startAvailableAt: i.start_available_at ?? "",
      startExpiresAt: i.start_expires_at ?? "",
      pickupTimesLoaded: i.pickup_times_loaded,
      pickupTimesRedacted: i.pickup_times_redacted,
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
        pickupTime: row.pickup_time,
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
    })),
    selectedGroupId: wire.selected_group_id ?? null,
    trackingIndicators: {
      labels: wire.tracking_indicators?.labels ?? [],
      results: wire.tracking_indicators?.results ?? {},
    },
    pickupTimes: (wire.pickup_times ?? []).map((p) => ({
      studentId: p.student_id,
      date: p.date,
      weekdayName: p.weekday_name,
      pickupTime: p.pickup_time ?? null,
      isException: p.is_exception,
      notes: p.notes ?? "",
      dayNotes: (p.day_notes ?? []).map((n) => ({
        id: n.id,
        content: n.content,
      })),
    })),
    arrivalTimes: (wire.arrival_times ?? []).map((a) => ({
      studentId: a.student_id,
      date: a.date,
      weekdayName: a.weekday_name,
      expectedArrival: a.expected_arrival ?? null,
      isException: a.is_exception,
      notes: a.notes ?? "",
      dayNotes: (a.day_notes ?? []).map((n) => ({
        id: n.id,
        content: n.content,
      })),
    })),
  };
}

/**
 * GET /api/active-supervision-dashboard?group_id={activeGroupId}
 *
 * Thin proxy over the aggregated Go projection. The backend owns scope
 * authorization, the room bulk load, and the strict error contract; an
 * unknown group_id is a backend 403 (retry without the parameter to let the
 * backend resolve the caller's first supervised session).
 */
export const GET = createGetHandler<ActiveSupervisionDashboardResponse>(
  async (request: NextRequest, token: string) => {
    const groupId = request.nextUrl.searchParams.get("group_id");
    const query =
      groupId && /^[1-9]\d{0,18}$/.test(groupId)
        ? `?group_id=${encodeURIComponent(groupId)}`
        : "";
    const response = await apiGet<{ data: WireDashboard }>(
      `/api/active/supervision-dashboard${query}`,
      token,
    );
    return mapDashboard(response.data);
  },
);
