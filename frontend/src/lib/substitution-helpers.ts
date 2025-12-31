// lib/substitution-helpers.ts
// Type definitions and helper functions for substitutions

// Backend types (from Go structs)
export interface BackendPerson {
  id: number;
  first_name: string;
  last_name: string;
  full_name?: string;
}

export interface BackendStaff {
  id: number;
  person_id: number;
  person?: BackendPerson;
  staff_notes?: string;
}

export interface BackendGroup {
  id: number;
  name: string;
  room_id?: number;
  representative_id?: number;
}

export interface BackendSubstitution {
  id: number;
  group_id: number;
  group?: BackendGroup;
  regular_staff_id?: number; // Now optional
  regular_staff?: BackendStaff;
  substitute_staff_id: number;
  substitute_staff?: BackendStaff;
  start_date: string;
  end_date: string;
  reason?: string;
  notes?: string;
  created_at: string;
  updated_at: string;
}

// Backend substitution info (for multiple substitutions per staff)
export interface BackendSubstitutionInfo {
  id: number;
  group_id: number;
  group_name?: string;
  is_transfer: boolean; // true if duration is 1 day (Tagesübergabe)
  start_date: string;
  end_date: string;
  group?: BackendGroup;
}

export interface BackendStaffWithSubstitutionStatus {
  // Staff fields
  id: number;
  person_id: number;
  person?: BackendPerson;
  staff_notes?: string;
  // Substitution status (updated to support multiple)
  is_substituting: boolean;
  substitution_count: number;
  substitutions?: BackendSubstitutionInfo[];
  current_group?: BackendGroup;
  regular_group?: BackendGroup;
  // Legacy field for backward compatibility
  substitution?: BackendSubstitution;
  // Teacher fields
  teacher_id?: number;
  specialization?: string | null;
  role?: string;
  qualifications?: string;
}

// Frontend types
export interface Substitution {
  id: string;
  groupId: string;
  groupName?: string;
  substituteStaffId: string;
  substituteStaffName?: string;
  startDate: Date;
  endDate: Date;
  reason?: string;
  notes?: string;
  isTransfer: boolean; // true if duration is 1 day (Tagesübergabe)
}

// Frontend substitution info (for multiple substitutions per teacher)
export interface SubstitutionInfo {
  id: string;
  groupId: string;
  groupName: string;
  isTransfer: boolean; // true if this is a day transfer
  startDate: Date;
  endDate: Date;
}

export interface TeacherAvailability {
  id: string;
  firstName: string;
  lastName: string;
  regularGroup?: string;
  role?: string;
  inSubstitution: boolean; // kept for backward compatibility
  substitutionCount: number;
  substitutions: SubstitutionInfo[];
  currentGroup?: string;
  teacherId?: string;
  specialization?: string;
}

// Mapping functions
export function mapSubstitutionResponse(
  backend: BackendSubstitution,
): Substitution {
  return {
    id: String(backend.id),
    groupId: String(backend.group_id),
    groupName: backend.group?.name,
    substituteStaffId: String(backend.substitute_staff_id),
    substituteStaffName:
      backend.substitute_staff?.person?.full_name ??
      (backend.substitute_staff?.person
        ? `${backend.substitute_staff.person.first_name} ${backend.substitute_staff.person.last_name}`
        : undefined),
    startDate: new Date(backend.start_date),
    endDate: new Date(backend.end_date),
    reason: backend.reason,
    notes: backend.notes,
    isTransfer: backend.start_date === backend.end_date, // Transfer if duration is 1 day (Tagesübergabe)
  };
}

export function mapTeacherAvailabilityResponse(
  backend: BackendStaffWithSubstitutionStatus,
): TeacherAvailability {
  // Map substitutions array
  const substitutions: SubstitutionInfo[] = (backend.substitutions ?? []).map(
    (sub) => ({
      id: String(sub.id),
      groupId: String(sub.group_id),
      groupName: sub.group_name ?? sub.group?.name ?? "Unbekannt",
      isTransfer: sub.is_transfer,
      startDate: new Date(sub.start_date),
      endDate: new Date(sub.end_date),
    }),
  );

  return {
    id: String(backend.id),
    firstName: backend.person?.first_name ?? "",
    lastName: backend.person?.last_name ?? "",
    regularGroup: backend.regular_group?.name,
    role: backend.role,
    inSubstitution: backend.is_substituting,
    substitutionCount: backend.substitution_count ?? 0,
    substitutions,
    currentGroup: backend.current_group?.name,
    teacherId: backend.teacher_id ? String(backend.teacher_id) : undefined,
    specialization: backend.specialization ?? undefined,
  };
}

export function mapSubstitutionsResponse(
  backendSubstitutions: BackendSubstitution[],
): Substitution[] {
  if (!Array.isArray(backendSubstitutions)) {
    console.error(
      "Expected array for backendSubstitutions, got:",
      backendSubstitutions,
    );
    return [];
  }
  return backendSubstitutions.map(mapSubstitutionResponse);
}

export function mapTeacherAvailabilityResponses(
  backendStaff: BackendStaffWithSubstitutionStatus[],
): TeacherAvailability[] {
  if (!Array.isArray(backendStaff)) {
    console.error("Expected array for backendStaff, got:", backendStaff);
    return [];
  }
  return backendStaff.map(mapTeacherAvailabilityResponse);
}

// Prepare frontend types for backend
export interface CreateSubstitutionRequest {
  group_id: number;
  regular_staff_id?: number; // Now optional - only needed for specific replacements
  substitute_staff_id: number;
  start_date: string;
  end_date: string;
  reason?: string;
  notes?: string;
}

export function prepareSubstitutionForBackend(
  groupId: string,
  regularStaffId: string | null, // Now optional
  substituteStaffId: string,
  startDate: Date,
  endDate: Date,
  reason?: string,
  notes?: string,
): CreateSubstitutionRequest {
  const startDateParts = startDate.toISOString().split("T");
  const endDateParts = endDate.toISOString().split("T");

  return {
    group_id: Number.parseInt(groupId, 10),
    regular_staff_id: regularStaffId ? Number.parseInt(regularStaffId, 10) : undefined,
    substitute_staff_id: Number.parseInt(substituteStaffId, 10),
    start_date: startDateParts[0] ?? "", // YYYY-MM-DD format
    end_date: endDateParts[0] ?? "",
    reason,
    notes,
  };
}

// Helper functions
export function formatDateForBackend(date: Date): string {
  const dateString = date.toISOString().split("T");
  return dateString[0] ?? ""; // YYYY-MM-DD format
}

export function formatTeacherName(teacher: TeacherAvailability): string {
  return `${teacher.firstName} ${teacher.lastName}`.trim();
}

export function getTeacherStatus(teacher: TeacherAvailability): string {
  if (teacher.inSubstitution && teacher.substitutionCount > 0) {
    if (teacher.substitutionCount === 1) {
      const sub = teacher.substitutions[0];
      const type = sub?.isTransfer ? "Übergabe" : "Vertretung";
      return `${type}: ${sub?.groupName ?? teacher.currentGroup ?? "Gruppe"}`;
    }
    return `${teacher.substitutionCount} Zuweisungen aktiv`;
  }
  return "Verfügbar";
}

// Get counts for transfers vs substitutions
export function getSubstitutionCounts(teacher: TeacherAvailability): {
  transfers: number;
  substitutions: number;
} {
  const transfers = teacher.substitutions.filter((s) => s.isTransfer).length;
  const substitutions = teacher.substitutions.filter(
    (s) => !s.isTransfer,
  ).length;
  return { transfers, substitutions };
}
