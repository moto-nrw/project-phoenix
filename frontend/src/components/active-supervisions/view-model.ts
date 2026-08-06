import type { Student } from "~/lib/student-helpers";

export const SCHULHOF_ROOM_NAME = "Schulhof";
export const SCHULHOF_TAB_ID = "schulhof";

export interface MinimalActiveGroup {
  id: string;
  room?: { name?: string };
}

export interface ActiveSupervisionStudent extends Student {
  activeGroupId: string;
  checkInTime: Date;
}

export interface ActiveSupervisionRoom {
  id: string;
  name: string;
  room_name?: string;
  room_id?: string;
  room_color?: string;
  student_count?: number;
  supervisor_name?: string;
  students?: ActiveSupervisionStudent[];
}

export interface SchulhofStatusResponse {
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
}

export function activeSupervisionRosterKey(options: {
  readonly selectedTimetableInstanceId: string | null;
  readonly currentRoomId: string | null | undefined;
  readonly missingRosterActiveGroupIds: ReadonlySet<string>;
}): string | null {
  if (options.selectedTimetableInstanceId) {
    return `timetable-roster-${options.selectedTimetableInstanceId}`;
  }
  if (
    !options.currentRoomId ||
    options.missingRosterActiveGroupIds.has(options.currentRoomId)
  ) {
    return null;
  }
  return `timetable-roster-active-group-${options.currentRoomId}`;
}

export function roomsOutsideSchulhofStatus(
  rooms: readonly ActiveSupervisionRoom[],
  options: {
    readonly schulhofTabEnabled: boolean;
    readonly statusActiveGroupId: string | null | undefined;
  },
): ActiveSupervisionRoom[] {
  if (!options.schulhofTabEnabled || !options.statusActiveGroupId) {
    return [...rooms];
  }

  return rooms.filter((room) => room.id !== options.statusActiveGroupId);
}

interface VisitDisplayLike {
  studentId: string;
  studentName?: string;
  schoolClass?: string;
  groupName?: string;
  activeGroupId: string;
  checkInTime: string | Date;
  actualArrivalTime?: string;
  actualPickupTime?: string;
  isActive: boolean;
  sick?: boolean;
  sickSince?: string;
  excused?: boolean;
  excusedSince?: string;
  photoUrl?: string;
}

interface SupervisedGroupLike {
  id: string;
  name: string;
  room_id?: string;
  room?: { id: string; name: string; color?: string | null };
}

interface EducationalGroupLike {
  id: string;
  name: string;
  room?: { name: string };
}

export function buildGroupNameToIdMap(
  groups: readonly EducationalGroupLike[],
): Map<string, string> {
  const nameToIdMap = new Map<string, string>();
  groups.forEach((group) => {
    if (group.name) {
      nameToIdMap.set(group.name, group.id);
    }
  });
  return nameToIdMap;
}

export function mapSupervisedGroupsToRooms(
  groups: readonly SupervisedGroupLike[],
): ActiveSupervisionRoom[] {
  return groups
    .map((group) => ({
      id: group.id,
      name: group.name,
      room_name: group.room?.name,
      room_id: group.room_id,
      room_color: group.room?.color ?? undefined,
      student_count: undefined,
      supervisor_name: undefined,
    }))
    .sort((a, b) =>
      (a.room_name ?? a.name).localeCompare(b.room_name ?? b.name, "de"),
    );
}

function mapVisitToSupervisionStudent(
  visit: VisitDisplayLike,
  options: {
    roomName?: string;
    roomColor?: string | null;
    groupNameToId?: Map<string, string>;
  },
): ActiveSupervisionStudent {
  const nameParts = visit.studentName?.split(" ") ?? ["", ""];
  const firstName = nameParts[0] ?? "";
  const lastName = nameParts.slice(1).join(" ") ?? "";
  const location = options.roomName
    ? `Anwesend - ${options.roomName}`
    : "Anwesend";
  const groupId = visit.groupName
    ? options.groupNameToId?.get(visit.groupName)
    : undefined;

  return {
    id: visit.studentId,
    name: visit.studentName ?? "",
    first_name: firstName,
    second_name: lastName,
    school_class: visit.schoolClass ?? "",
    current_location: location,
    current_room_color: options.roomColor ?? null,
    group_name: visit.groupName,
    group_id: groupId,
    sick: visit.sick,
    sick_since: visit.sickSince,
    excused: visit.excused,
    excused_since: visit.excusedSince,
    actual_arrival_time: visit.actualArrivalTime,
    actual_pickup_time: visit.actualPickupTime,
    activeGroupId: visit.activeGroupId,
    checkInTime:
      visit.checkInTime instanceof Date
        ? visit.checkInTime
        : new Date(visit.checkInTime),
    photo_url: visit.photoUrl,
  } as ActiveSupervisionStudent;
}

export function mapVisitsToSupervisionStudents(
  visits: readonly VisitDisplayLike[],
  options: {
    roomName?: string;
    roomColor?: string | null;
    groupNameToId?: Map<string, string>;
  },
): ActiveSupervisionStudent[] {
  return visits
    .filter((visit) => visit.isActive)
    .map((visit) => mapVisitToSupervisionStudent(visit, options));
}

export function withActiveSupervisionPresence(
  student: ActiveSupervisionStudent,
): ActiveSupervisionStudent {
  return {
    ...student,
    sick: false,
    sick_since: undefined,
    excused: false,
    excused_since: undefined,
    class_trip: false,
    class_trip_since: undefined,
    day_planning_status:
      student.day_planning_status === "not_coming_today"
        ? undefined
        : student.day_planning_status,
    day_planning_reason: undefined,
    day_planning_label: undefined,
  };
}
