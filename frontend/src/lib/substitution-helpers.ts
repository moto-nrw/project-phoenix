// lib/substitution-helpers.ts
// Type definitions and helper functions for substitutions

import { toISODate } from "~/lib/date-helpers";

export interface BackendGroupHandover {
  id: number;
  type: "group_handover";
  group: { id: number; name: string };
  target: {
    id: number;
    full_name: string;
  };
  period: { start_date: string; end_date: string };
  can_end: boolean;
}

export interface BackendSubstitutionOverview {
  group_handovers: BackendGroupHandover[];
  targets: Array<{ id: number; full_name: string }>;
  schedule_appointments?: BackendScheduleAppointment[];
  schedule_targets?: Array<{ id: number; full_name: string }>;
}

interface BackendScheduleAppointment {
  id: number;
  type: "schedule_substitution";
  date: string;
  start_time: string;
  end_time: string;
  title: string;
  status: string;
  staff: Array<{
    assignment_id: number;
    staff: { id: number; full_name: string };
    is_absent: boolean;
    is_substitute: boolean;
    can_end: boolean;
  }>;
}

export interface ScheduleSubstitutionOverview {
  appointments: Array<{
    id: string;
    date: string;
    startTime: string;
    endTime: string;
    title: string;
    status: string;
    staff: Array<{
      assignmentId: string;
      id: string;
      name: string;
      isAbsent: boolean;
      isSubstitute: boolean;
      canEnd: boolean;
    }>;
  }>;
  staff: Array<{ id: string; name: string }>;
}

export interface SubstitutionProxyEnvelope<T> {
  data?: T;
}

export function unwrapSubstitutionProxyEnvelope<T>(
  envelope: SubstitutionProxyEnvelope<T>,
): T {
  if (envelope.data === undefined) {
    throw new Error("Ungültige Antwort für Gruppenübergaben.");
  }
  return envelope.data;
}

// Frontend types
export interface Substitution {
  id: string;
  type: "group_handover";
  groupId: string;
  groupName: string;
  substituteStaffId: string;
  substituteStaffName: string;
  startDate: string;
  endDate: string;
  canEnd: boolean;
}

export interface TeacherAvailability {
  id: string;
  firstName: string;
  lastName: string;
  regularGroup?: string;
  role?: string;
  inSubstitution: boolean;
  substitutionCount: number;
  teacherId?: string;
  specialization?: string;
}

// Mapping functions
export function mapSubstitutionResponse(
  backend: BackendGroupHandover,
): Substitution {
  return {
    id: String(backend.id),
    type: backend.type,
    groupId: String(backend.group.id),
    groupName: backend.group.name,
    substituteStaffId: String(backend.target.id),
    substituteStaffName: backend.target.full_name,
    startDate: backend.period.start_date,
    endDate: backend.period.end_date,
    canEnd: backend.can_end,
  };
}

export function mapSubstitutionsResponse(
  backendSubstitutions: BackendGroupHandover[],
): Substitution[] {
  if (!Array.isArray(backendSubstitutions)) {
    throw new Error("Ungültige Antwort für Gruppenübergaben.");
  }
  return backendSubstitutions.map(mapSubstitutionResponse);
}

export function mapScheduleSubstitutionOverview(
  backend: BackendSubstitutionOverview,
): ScheduleSubstitutionOverview {
  return {
    appointments: (backend.schedule_appointments ?? []).map((appointment) => ({
      id: String(appointment.id),
      date: appointment.date,
      startTime: appointment.start_time,
      endTime: appointment.end_time,
      title: appointment.title,
      status: appointment.status,
      staff: appointment.staff.map((row) => ({
        assignmentId: String(row.assignment_id),
        id: String(row.staff.id),
        name: row.staff.full_name,
        isAbsent: row.is_absent,
        isSubstitute: row.is_substitute,
        canEnd: row.can_end,
      })),
    })),
    staff: (backend.schedule_targets ?? []).map((member) => ({
      id: String(member.id),
      name: member.full_name,
    })),
  };
}

// Prepare frontend types for backend
export interface CreateSubstitutionRequest {
  type: "group_handover";
  group_handover: {
    group_id: number;
    target_staff_id: number;
    start_date: string;
    end_date: string;
  };
}

export function prepareSubstitutionForBackend(
  groupId: string,
  substituteStaffId: string,
  startDate: string,
  endDate: string,
): CreateSubstitutionRequest {
  return {
    type: "group_handover",
    group_handover: {
      group_id: Number.parseInt(groupId, 10),
      target_staff_id: Number.parseInt(substituteStaffId, 10),
      start_date: startDate,
      end_date: endDate,
    },
  };
}

// Helper functions
export function formatDateForBackend(date: Date): string {
  return toISODate(date); // YYYY-MM-DD format
}

export function formatTeacherName(teacher: TeacherAvailability): string {
  return `${teacher.firstName} ${teacher.lastName}`.trim();
}
