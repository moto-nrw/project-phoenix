// lib/group-helpers.ts
// Type definitions and helper functions for groups

import { normalizeLocation } from "./location-helper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GroupHelpers" });

// Backend types (from Go structs)
// Define a simple backend student structure
interface BackendGroupStudent {
  id: number;
  name?: string;
  school_class?: string;
  current_location?: string | null;
}

export interface BackendGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    // Room is a nested object
    id: number;
    name: string;
  };
  representative_id?: number;
  representative?: {
    // Representative is also a nested object
    id: number;
    staff_id: number;
    first_name: string;
    last_name: string;
    full_name: string;
    specialization: string | null;
    role?: string;
  };
  teachers?: Array<{
    // Array of teachers
    id: number;
    staff_id: number;
    first_name: string;
    last_name: string;
    full_name: string;
    specialization: string | null;
    role?: string;
  }>;
  student_count?: number; // These fields might come from the backend
  supervisor_count?: number;
  created_at: string;
  updated_at: string;
  students?: BackendGroupStudent[];
  teacher_ids?: number[];
}

// Frontend types
// Define a compatible StudentForGroup interface that matches the required properties of Student in api.ts
interface StudentForGroup {
  id: string;
  name: string;
  school_class: string;
  current_location: string;
}

export interface Group {
  id: string;
  name: string;
  room_id?: string;
  room_name?: string;
  representative_id?: string;
  representative_name?: string;
  representative?: {
    id: string;
    staffId: string;
    firstName: string;
    lastName: string;
    fullName: string;
  };
  student_count?: number;
  supervisor_count?: number;
  created_at?: string;
  updated_at?: string;
  students?: StudentForGroup[];
  supervisors?: Array<{ id: string; name: string }>;
  teacher_ids?: string[];
}

// Mapping functions
export function mapGroupResponse(backendGroup: BackendGroup): Group {
  // Create the basic group properties
  const group: Group = {
    id: String(backendGroup.id),
    name: backendGroup.name,
    // Use room_id from the direct field first, then fallback to nested room object
    room_id: backendGroup.room_id ? String(backendGroup.room_id) : undefined,
    // Extract room name from nested room object if available
    room_name: backendGroup.room?.name ?? undefined,
    representative_id: backendGroup.representative_id
      ? String(backendGroup.representative_id)
      : undefined,
    representative_name: backendGroup.representative?.full_name ?? undefined,
    student_count: backendGroup.student_count,
    supervisor_count: backendGroup.supervisor_count,
    created_at: backendGroup.created_at,
    updated_at: backendGroup.updated_at,
  };

  // Map the representative if available
  if (backendGroup.representative) {
    group.representative = {
      id: String(backendGroup.representative.id),
      staffId: String(backendGroup.representative.staff_id),
      firstName: backendGroup.representative.first_name,
      lastName: backendGroup.representative.last_name,
      fullName: backendGroup.representative.full_name,
    };
  }

  // If the backend group has students, map them to StudentForGroup objects
  if (Array.isArray(backendGroup.students)) {
    group.students = backendGroup.students.map((student) => ({
      id: String(student.id),
      name: student.name ?? "Unnamed Student",
      school_class: student.school_class ?? "",
      current_location: normalizeLocation(student.current_location),
    }));
  }

  // If the backend group has teachers, map them to supervisors
  if (Array.isArray(backendGroup.teachers)) {
    group.supervisors = backendGroup.teachers.map((teacher) => ({
      id: String(teacher.id),
      name: teacher.full_name,
    }));

    // Extract teacher IDs for form population
    group.teacher_ids = backendGroup.teachers.map((teacher) =>
      String(teacher.id),
    );
  }

  return group;
}

export function mapGroupsResponse(backendGroups: BackendGroup[]): Group[] {
  // Safety check to ensure we have an array
  if (!Array.isArray(backendGroups)) {
    logger.error("expected array for backendGroups", {
      received: typeof backendGroups,
    });
    return [];
  }
  return backendGroups.map(mapGroupResponse);
}

export function mapSingleGroupResponse(response: {
  data: BackendGroup;
}): Group {
  return mapGroupResponse(response.data);
}

// Prepare frontend types for backend
export function prepareGroupForBackend(
  group: Partial<Group>,
): Partial<BackendGroup> {
  return {
    id: group.id ? Number.parseInt(group.id, 10) : undefined,
    name: group.name,
    room_id: group.room_id ? Number.parseInt(group.room_id, 10) : undefined,
    // Note: room_name is not sent to backend, only room_id
    representative_id: group.representative_id
      ? Number.parseInt(group.representative_id, 10)
      : undefined,
    // Note: representative_name is also not sent to backend, only representative_id
    teacher_ids: group.teacher_ids?.map((id) => Number.parseInt(id, 10)),
  };
}

// Request/Response types

// Helper functions
export function formatGroupName(group: Group): string {
  return group.name || "Unnamed Group";
}

export function formatGroupLocation(group: Group): string {
  return group.room_name ?? "No Room Assigned";
}

export function formatGroupRepresentative(group: Group): string {
  return group.representative_name ?? "No Representative";
}
