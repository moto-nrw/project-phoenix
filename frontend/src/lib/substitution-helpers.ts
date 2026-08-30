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
  running_supervisions: BackendRunningSupervision[];
  targets: Array<{ id: number; full_name: string }>;
}

interface BackendRunningSupervision {
  id: number;
  type: "additional_supervision";
  name: string;
  room_name?: string;
  supervisors: Array<{ id: number; full_name: string }>;
  available_targets: Array<{ id: number; full_name: string }>;
  is_current_user_supervising: boolean;
  can_assign: boolean;
}

export interface BackendAdditionalSupervisionResult {
  id: number;
  type: "additional_supervision";
  active_group_id: number;
  target: { id: number; full_name: string };
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

export interface RunningSupervision {
  id: string;
  name: string;
  roomName?: string;
  supervisors: Array<{ id: string; fullName: string }>;
  availableTargets: Array<{ id: string; fullName: string }>;
  isCurrentUserSupervising: boolean;
  canAssign: boolean;
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

export interface AddSupervisorRequest {
  type: "additional_supervision";
  additional_supervision: {
    active_group_id: number;
    target_staff_id: number;
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
