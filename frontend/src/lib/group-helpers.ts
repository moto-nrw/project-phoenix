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

export interface BackendCombinedGroup {
  id: number;
  name: string;
  is_active: boolean;
  created_at: string;
  valid_until?: string;
  access_policy: string;
  specific_group_id?: number;
  specific_group?: BackendGroup;
  groups?: BackendGroup[];
  access_specialists?: Array<{ id: number; name: string }>;
  is_expired?: boolean;
  group_count?: number;
  specialist_count?: number;
  time_until_expiration?: string;
}

// Frontend types
// Define a compatible StudentForGroup interface that matches the required properties of Student in api.ts
export interface StudentForGroup {
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

export interface CombinedGroup {
  id: string;
  name: string;
  is_active: boolean;
  created_at?: string;
  valid_until?: string;
  access_policy: "all" | "first" | "specific" | "manual";
  specific_group_id?: string;
  specific_group?: Group;
  groups?: Group[];
  access_specialists?: Array<{ id: string; name: string }>;
  is_expired?: boolean;
  group_count?: number;
  specialist_count?: number;
  time_until_expiration?: string;
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

export function mapCombinedGroupResponse(
  backendGroup: BackendCombinedGroup,
): CombinedGroup {
  return {
    id: String(backendGroup.id),
    name: backendGroup.name,
    is_active: backendGroup.is_active,
    created_at: backendGroup.created_at,
    valid_until: backendGroup.valid_until,
    access_policy: backendGroup.access_policy as
      | "all"
      | "first"
      | "specific"
      | "manual",
    specific_group_id: backendGroup.specific_group_id
      ? String(backendGroup.specific_group_id)
      : undefined,
    specific_group: backendGroup.specific_group
      ? mapGroupResponse(backendGroup.specific_group)
      : undefined,
    groups: backendGroup.groups
      ? backendGroup.groups.map(mapGroupResponse)
      : undefined,
    access_specialists: backendGroup.access_specialists
      ? backendGroup.access_specialists.map((spec) => ({
          id: String(spec.id),
          name: spec.name,
        }))
      : undefined,
    is_expired: backendGroup.is_expired,
    group_count: backendGroup.group_count,
    specialist_count: backendGroup.specialist_count,
    time_until_expiration: backendGroup.time_until_expiration,
  };
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

export function mapCombinedGroupsResponse(
  backendGroups: BackendCombinedGroup[],
): CombinedGroup[] {
  return backendGroups.map(mapCombinedGroupResponse);
}

export function mapSingleGroupResponse(response: {
  data: BackendGroup;
}): Group {
  return mapGroupResponse(response.data);
}

export function mapSingleCombinedGroupResponse(response: {
  data: BackendCombinedGroup;
}): CombinedGroup {
  return mapCombinedGroupResponse(response.data);
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

export function prepareCombinedGroupForBackend(
  group: Partial<CombinedGroup>,
): Partial<BackendCombinedGroup> {
  return {
    id: group.id ? Number.parseInt(group.id, 10) : undefined,
    name: group.name,
    is_active: group.is_active,
    valid_until: group.valid_until,
    access_policy: group.access_policy,
    specific_group_id: group.specific_group_id
      ? Number.parseInt(group.specific_group_id, 10)
      : undefined,
    // Create a complete BackendGroup for each group, not just the ID
    groups: group.groups?.map(
      (g) =>
        ({
          id: Number.parseInt(g.id, 10),
          name: g.name,
          created_at: g.created_at ?? new Date().toISOString(),
          updated_at: g.updated_at ?? new Date().toISOString(),
        }) as BackendGroup,
    ),
    access_specialists: group.access_specialists?.map((s) => ({
      id: Number.parseInt(s.id, 10),
      name: s.name,
    })),
  };
}

// Request/Response types
export interface CreateGroupRequest {
  name: string;
  room_id?: number;
  representative_id?: number;
}

export interface UpdateGroupRequest {
  name: string;
  room_id?: number;
  representative_id?: number;
}

export interface CreateCombinedGroupRequest {
  name: string;
  is_active: boolean;
  valid_until?: string;
  access_policy: string;
  specific_group_id?: number;
  group_ids?: number[];
  specialist_ids?: number[];
}

export interface UpdateCombinedGroupRequest {
  name: string;
  is_active: boolean;
  valid_until?: string;
  access_policy: string;
  specific_group_id?: number;
  group_ids?: number[];
  specialist_ids?: number[];
}

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

export function formatCombinedGroupStatus(group: CombinedGroup): string {
  if (!group.is_active) return "Inactive";
  if (group.is_expired) return "Expired";
  return "Active";
}

export function formatCombinedGroupValidity(group: CombinedGroup): string {
  if (!group.valid_until) return "No expiration";
  return group.valid_until;
}

export function getAccessPolicyName(policy: string): string {
  const policies: Record<string, string> = {
    all: "All Specialists",
    first: "First Specialist",
    specific: "Specific Specialist",
    manual: "Manual Assignment",
  };

  return policies[policy] ?? policy;
}
