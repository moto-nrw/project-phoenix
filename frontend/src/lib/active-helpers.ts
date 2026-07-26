// lib/active-helpers.ts
// Type definitions for active entities

// Tracking indicators response from POST /api/active/tracking-indicators
export interface TrackingIndicatorsResponse {
  labels: string[];
  results: Record<string, boolean[]>; // student_id → [matched1, matched2, ...]
}

// Backend types (from Go structs).
// group_id: number | null — WP-B6 made active.groups.group_id nullable so
// spontaneous activity instances can run without a parent template. The
// backend serializes `null` explicitly (no omitempty), so clients MUST
// handle both shapes.
export interface BackendActiveGroup {
  id: number;
  group_id: number | null;
  room_id: number;
  start_time: string;
  end_time?: string;
  is_active: boolean;
  notes?: string;
  visit_count?: number;
  supervisor_count?: number;
  room?: {
    id: number;
    name: string;
    category?: string;
  };
  actual_group?: {
    id: number;
    name: string;
  };
  created_at: string;
  updated_at: string;
}

export interface BackendVisit {
  id: number;
  student_id: number;
  active_group_id: number;
  check_in_time: string;
  check_out_time?: string;
  actual_arrival_time?: string;
  actual_pickup_time?: string;
  is_active: boolean;
  notes?: string;
  student_name?: string;
  // Optional display fields available on /active/groups/{id}/visits/display
  school_class?: string;
  group_name?: string; // Student's OGS group (not the active group)
  active_group_name?: string;
  // Status flags surfaced by the visits/display endpoint
  sick?: boolean;
  sick_since?: string;
  excused?: boolean;
  excused_since?: string;
  // Authenticated proxy URL — backend rewrites the raw /uploads path
  // to /api/students/{id}/photo/{filename} before sending it down.
  photo_url?: string;
  created_at: string;
  updated_at: string;
}

export interface BackendSupervisor {
  id: number;
  staff_id: number;
  active_group_id: number;
  start_time: string;
  end_time?: string;
  is_active: boolean;
  notes?: string;
  staff_name?: string;
  active_group_name?: string;
  created_at: string;
  updated_at: string;
}

// Frontend types
// groupId: string | null — mirrors the nullable backend contract (WP-B6).
// A null value means the session is spontaneous (no parent template).
export interface ActiveGroup {
  id: string;
  groupId: string | null;
  roomId: string;
  startTime: Date;
  endTime?: Date;
  isActive: boolean;
  notes?: string;
  visitCount?: number;
  supervisorCount?: number;
  room?: {
    id: number;
    name: string;
    category?: string;
  };
  actualGroup?: {
    id: number;
    name: string;
  };
  createdAt: Date;
  updatedAt: Date;
}

export interface Visit {
  id: string;
  studentId: string;
  activeGroupId: string;
  checkInTime: Date;
  checkOutTime?: Date;
  actualArrivalTime?: string;
  actualPickupTime?: string;
  isActive: boolean;
  notes?: string;
  studentName?: string;
  // Optional display fields propagated when available
  schoolClass?: string;
  groupName?: string;
  activeGroupName?: string;
  // Status flags (populated by the visits/display endpoint)
  sick?: boolean;
  sickSince?: string;
  excused?: boolean;
  excusedSince?: string;
  // Authenticated photo URL (forwarded as-is from the BFF/proxy).
  photoUrl?: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface Supervisor {
  id: string;
  staffId: string;
  activeGroupId: string;
  startTime: Date;
  endTime?: Date;
  isActive: boolean;
  notes?: string;
  staffName?: string;
  activeGroupName?: string;
  createdAt: Date;
  updatedAt: Date;
}

// Transformation functions
export function mapActiveGroupResponse(
  backendActiveGroup: BackendActiveGroup,
): ActiveGroup {
  return {
    id: String(backendActiveGroup.id),
    // Preserve null when the backend sends `group_id: null` (spontaneous
    // session, WP-B6). Do NOT do `String(null)` — that produces the literal
    // string "null" which would silently break any equality check.
    groupId:
      backendActiveGroup.group_id === null
        ? null
        : String(backendActiveGroup.group_id),
    roomId: String(backendActiveGroup.room_id),
    startTime: new Date(backendActiveGroup.start_time),
    endTime: backendActiveGroup.end_time
      ? new Date(backendActiveGroup.end_time)
      : undefined,
    isActive: backendActiveGroup.is_active,
    notes: backendActiveGroup.notes,
    visitCount: backendActiveGroup.visit_count,
    supervisorCount: backendActiveGroup.supervisor_count,
    room: backendActiveGroup.room,
    actualGroup: backendActiveGroup.actual_group,
    createdAt: new Date(backendActiveGroup.created_at),
    updatedAt: new Date(backendActiveGroup.updated_at),
  };
}

export function mapVisitResponse(backendVisit: BackendVisit): Visit {
  return {
    id: String(backendVisit.id),
    studentId: String(backendVisit.student_id),
    activeGroupId: String(backendVisit.active_group_id),
    checkInTime: new Date(backendVisit.check_in_time),
    checkOutTime: backendVisit.check_out_time
      ? new Date(backendVisit.check_out_time)
      : undefined,
    actualArrivalTime: backendVisit.actual_arrival_time,
    actualPickupTime: backendVisit.actual_pickup_time,
    isActive: backendVisit.is_active,
    notes: backendVisit.notes,
    studentName: backendVisit.student_name,
    schoolClass: backendVisit.school_class,
    groupName: backendVisit.group_name,
    activeGroupName: backendVisit.active_group_name,
    sick: backendVisit.sick,
    sickSince: backendVisit.sick_since,
    excused: backendVisit.excused,
    excusedSince: backendVisit.excused_since,
    photoUrl: backendVisit.photo_url,
    createdAt: new Date(backendVisit.created_at),
    updatedAt: new Date(backendVisit.updated_at),
  };
}

export function mapSupervisorResponse(
  backendSupervisor: BackendSupervisor,
): Supervisor {
  return {
    id: String(backendSupervisor.id),
    staffId: String(backendSupervisor.staff_id),
    activeGroupId: String(backendSupervisor.active_group_id),
    startTime: new Date(backendSupervisor.start_time),
    endTime: backendSupervisor.end_time
      ? new Date(backendSupervisor.end_time)
      : undefined,
    isActive: backendSupervisor.is_active,
    notes: backendSupervisor.notes,
    staffName: backendSupervisor.staff_name,
    activeGroupName: backendSupervisor.active_group_name,
    createdAt: new Date(backendSupervisor.created_at),
    updatedAt: new Date(backendSupervisor.updated_at),
  };
}

// Request/Response types
interface CreateActiveGroupRequest {
  group_id: number;
  room_id: number;
  start_time: string;
  end_time?: string;
  notes?: string;
}

interface CreateVisitRequest {
  student_id: number;
  active_group_id: number;
  check_in_time: string;
  check_out_time?: string;
  notes?: string;
}

interface CreateSupervisorRequest {
  staff_id: number;
  active_group_id: number;
  start_time: string;
  end_time?: string;
  notes?: string;
}

// Utility functions to prepare data for backend.
//
// NOTE (WP-B6): this helper targets the admin CRUD endpoint
// (POST/PUT /api/active/groups) which REQUIRES a positive template id. A
// spontaneous session (groupId === null) cannot be created through this
// path — spontaneous creation goes through a separate handler (WP-B9+).
// The truthy guard below is therefore intentional: if groupId is null, we
// omit the field, and the backend's request validator will reject the
// request with a 400. That is the desired behaviour for this endpoint.
export function prepareActiveGroupForBackend(
  activeGroup: Partial<ActiveGroup>,
): Partial<CreateActiveGroupRequest> {
  const request: Partial<CreateActiveGroupRequest> = {};

  if (activeGroup.groupId)
    request.group_id = Number.parseInt(activeGroup.groupId);
  if (activeGroup.roomId) request.room_id = Number.parseInt(activeGroup.roomId);
  if (activeGroup.startTime)
    request.start_time = activeGroup.startTime.toISOString();
  if (activeGroup.endTime) request.end_time = activeGroup.endTime.toISOString();
  if (activeGroup.notes !== undefined) request.notes = activeGroup.notes;

  return request;
}

export function prepareVisitForBackend(
  visit: Partial<Visit>,
): Partial<CreateVisitRequest> {
  const request: Partial<CreateVisitRequest> = {};

  if (visit.studentId) request.student_id = Number.parseInt(visit.studentId);
  if (visit.activeGroupId)
    request.active_group_id = Number.parseInt(visit.activeGroupId);
  if (visit.checkInTime)
    request.check_in_time = visit.checkInTime.toISOString();
  if (visit.checkOutTime)
    request.check_out_time = visit.checkOutTime.toISOString();
  if (visit.notes !== undefined) request.notes = visit.notes;

  return request;
}

export function prepareSupervisorForBackend(
  supervisor: Partial<Supervisor>,
): Partial<CreateSupervisorRequest> {
  const request: Partial<CreateSupervisorRequest> = {};

  if (supervisor.staffId)
    request.staff_id = Number.parseInt(supervisor.staffId);
  if (supervisor.activeGroupId)
    request.active_group_id = Number.parseInt(supervisor.activeGroupId);
  if (supervisor.startTime)
    request.start_time = supervisor.startTime.toISOString();
  if (supervisor.endTime) request.end_time = supervisor.endTime.toISOString();
  if (supervisor.notes !== undefined) request.notes = supervisor.notes;

  return request;
}

// Input types for create operations (omitting auto-generated fields)
export type CreateActiveGroupInput = Omit<
  ActiveGroup,
  "id" | "isActive" | "createdAt" | "updatedAt"
>;

export type CreateVisitInput = Omit<
  Visit,
  "id" | "isActive" | "createdAt" | "updatedAt"
>;

export type CreateSupervisorInput = Omit<
  Supervisor,
  "id" | "isActive" | "createdAt" | "updatedAt"
>;

// UnclaimedGroup interface for deviceless room claiming

// Schulhof (schoolyard) types for permanent tab functionality
export interface BackendSchulhofStatus {
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

interface BackendSchulhofSupervisor {
  id: number;
  staff_id: number;
  name: string;
  is_current_user: boolean;
}

export interface BackendToggleSupervisionResponse {
  action: string;
  supervision_id?: number;
  active_group_id: number;
}

export interface SchulhofStatus {
  exists: boolean;
  roomId: string | null;
  roomName: string;
  activityGroupId: string | null;
  activeGroupId: string | null;
  isUserSupervising: boolean;
  supervisionId: string | null;
  supervisorCount: number;
  studentCount: number;
  supervisors: SchulhofSupervisor[];
}

interface SchulhofSupervisor {
  id: string;
  staffId: string;
  name: string;
  isCurrentUser: boolean;
}

export interface ToggleSupervisionResponse {
  action: "started" | "stopped";
  supervisionId: string | null;
  activeGroupId: string;
}

export function mapSchulhofStatusResponse(
  backend: BackendSchulhofStatus,
): SchulhofStatus {
  return {
    exists: backend.exists,
    roomId: backend.room_id ? String(backend.room_id) : null,
    roomName: backend.room_name,
    activityGroupId: backend.activity_group_id
      ? String(backend.activity_group_id)
      : null,
    activeGroupId: backend.active_group_id
      ? String(backend.active_group_id)
      : null,
    isUserSupervising: backend.is_user_supervising,
    supervisionId: backend.supervision_id
      ? String(backend.supervision_id)
      : null,
    supervisorCount: backend.supervisor_count,
    studentCount: backend.student_count,
    supervisors: backend.supervisors.map((sup) => ({
      id: String(sup.id),
      staffId: String(sup.staff_id),
      name: sup.name,
      isCurrentUser: sup.is_current_user,
    })),
  };
}

export function mapToggleSupervisionResponse(
  backend: BackendToggleSupervisionResponse,
): ToggleSupervisionResponse {
  return {
    action: backend.action as "started" | "stopped",
    supervisionId: backend.supervision_id
      ? String(backend.supervision_id)
      : null,
    activeGroupId: String(backend.active_group_id),
  };
}
