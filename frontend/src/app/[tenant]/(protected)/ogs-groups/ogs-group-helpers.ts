import type { Student } from "~/lib/api";
import {
  isHomeLocation,
  isSchoolyardLocation,
  isTransitLocation,
  parseLocation,
} from "~/lib/location-helper";

// Define OGSGroup type based on EducationalGroup with additional fields
export interface OGSGroup {
  id: string;
  name: string;
  room_name?: string;
  room_id?: string;
  student_count?: number;
  supervisor_name?: string;
  students?: Student[];
  viaSubstitution?: boolean;
}

// Pickup urgency constants and helper
const PICKUP_URGENCY_SOON_MINUTES = 30;

export type PickupUrgency = "overdue" | "soon" | "normal" | "none";

export function getPickupUrgency(
  pickupTimeStr: string | undefined,
  now: Date,
): PickupUrgency {
  if (!pickupTimeStr) return "none";

  const [hours, minutes] = pickupTimeStr.split(":").map(Number);
  const pickupDate = new Date(now);
  pickupDate.setHours(hours ?? 0, minutes ?? 0, 0, 0);

  const diffMs = pickupDate.getTime() - now.getTime();
  const diffMinutes = diffMs / 60000;

  if (diffMinutes < 0) return "overdue";
  if (diffMinutes <= PICKUP_URGENCY_SOON_MINUTES) return "soon";
  return "normal";
}

export function isStudentInGroupRoom(
  student: Student,
  currentGroup?: OGSGroup | null,
): boolean {
  if (!student?.current_location || !currentGroup?.room_name) {
    return false;
  }

  const parsed = parseLocation(student.current_location);
  if (parsed.room) {
    const normalizedStudentRoom = parsed.room.trim().toLowerCase();
    const normalizedGroupRoom = currentGroup.room_name.trim().toLowerCase();
    if (normalizedStudentRoom === normalizedGroupRoom) {
      return true;
    }
  }

  if (currentGroup.room_id) {
    const normalizedLocation = student.current_location.toLowerCase();
    return normalizedLocation.includes(currentGroup.room_id.toString());
  }

  return false;
}

// Helper functions for student filtering

export function matchesSearchFilter(
  student: Student,
  searchTerm: string,
): boolean {
  if (!searchTerm) return true;

  const searchLower = searchTerm.toLowerCase();
  return (
    (student.name?.toLowerCase().includes(searchLower) ?? false) ||
    (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
    (student.second_name?.toLowerCase().includes(searchLower) ?? false) ||
    (student.school_class?.toLowerCase().includes(searchLower) ?? false)
  );
}

export function matchesAttendanceFilter(
  student: Student,
  attendanceFilter: string,
  roomStatus: Record<
    string,
    { in_group_room?: boolean; current_room_id?: number }
  >,
): boolean {
  if (attendanceFilter === "all") return true;

  const studentRoomStatus = roomStatus[student.id.toString()];

  switch (attendanceFilter) {
    case "in_room":
      return studentRoomStatus?.in_group_room ?? false;
    case "foreign_room":
      return matchesForeignRoomFilter(studentRoomStatus);
    case "transit":
      return isTransitLocation(student.current_location);
    case "schoolyard":
      return isSchoolyardLocation(student.current_location);
    case "at_home":
      return isHomeLocation(student.current_location);
    default:
      return true;
  }
}

export function matchesForeignRoomFilter(studentRoomStatus?: {
  in_group_room?: boolean;
  current_room_id?: number;
}): boolean {
  return (
    !!studentRoomStatus?.current_room_id &&
    studentRoomStatus.in_group_room === false
  );
}
