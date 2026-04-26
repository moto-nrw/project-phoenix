/**
 * Domain types for the timetable feature.
 *
 * Mirrors the Go DTOs in backend/api/timetable/. Backend int64 IDs become
 * frontend strings. Backend snake_case becomes frontend camelCase via the
 * mappers in timetable-helpers.ts.
 */

export type InstanceStatus = "planned" | "active" | "completed" | "cancelled";

export type ActivityType = "care" | "activity" | "external";

export type ConflictKind = "room" | "staff" | "student";

/**
 * Soft warning surfaced by the backend when an instance has overlapping
 * rooms, double-booked staff, or double-booked students. Conflicts never
 * block an action; they are advisory and can always be overridden.
 */
export interface ConflictWarning {
  kind: ConflictKind;
  resourceId: string;
  message: string;
  canOverride: boolean;
}

/**
 * Lightweight staff assignment as it appears in the weekly list. The slide-
 * over detail view fetches person names separately to keep the list payload
 * compact.
 */
export interface InstanceStaffSummary {
  staffId: string;
  isPrimary: boolean;
  isAbsent: boolean;
  isSubstitute: boolean;
}

/**
 * One activity instance enriched with everything the weekly grid needs:
 * room name, activity-group type (drives the colour key), staff list, and
 * expected/present student counts. is_live is true when the instance has an
 * active.group bridge — drives the LÄUFT badge.
 */
export interface EnrichedInstance {
  id: string;
  date: string; // YYYY-MM-DD
  startTime: string; // HH:MM
  endTime: string; // HH:MM
  title: string;
  description?: string;
  notes?: string;
  status: InstanceStatus;
  isSpontaneous: boolean;
  isLive: boolean;
  activityGroupId?: string;
  activityType: ActivityType;
  roomId: string;
  roomName: string;
  staff: InstanceStaffSummary[];
  staffCount: number;
  absentStaffCount: number;
  expectedStudentsCount: number;
  presentStudentsCount: number;
  conflictWarnings: ConflictWarning[];
}

export interface WeeklyInstancesResponse {
  from: string;
  to: string;
  instances: EnrichedInstance[];
}

/**
 * Backend GET /api/timetable/instances response shape (snake_case).
 * Used internally by the mapper; consumers should rely on
 * WeeklyInstancesResponse.
 */
export interface BackendInstanceStaffSummary {
  staff_id: number;
  is_primary: boolean;
  is_absent: boolean;
  is_substitute: boolean;
}

export interface BackendEnrichedInstance {
  id: number;
  date: string;
  start_time: string;
  end_time: string;
  title: string;
  description?: string;
  notes?: string;
  status: InstanceStatus;
  is_spontaneous: boolean;
  is_live: boolean;
  activity_group_id?: number;
  activity_type: ActivityType;
  room_id: number;
  room_name: string;
  staff: BackendInstanceStaffSummary[];
  staff_count: number;
  absent_staff_count: number;
  expected_students_count: number;
  present_students_count: number;
}

export interface BackendWeeklyInstancesResponse {
  from: string;
  to: string;
  instances: BackendEnrichedInstance[];
}

/**
 * Result of a manual materialization run — POST /api/timetable/materialize.
 * Exposed in the UI as a German toast: "Plan aktualisiert: X Aktivitäten
 * angelegt". When `warnings` is non-empty the run hit a precondition that
 * caused a no-op (e.g. tenant has no active calendar period); the toast
 * surfaces the warning text instead of the success message.
 */
export interface MaterializeResult {
  from: string;
  to: string;
  instancesCreated: number;
  candidatesSkippedExisting: number;
  warnings: MaterializeWarning[];
  durationMs: number;
}

/**
 * Typed soft-precondition warning — keep `code` in sync with the backend
 * MaterializationWarning constants (services/schedule/materialization_service.go).
 */
export interface MaterializeWarning {
  code: "no_active_period" | "no_templates" | (string & {});
  message: string;
}

export interface BackendMaterializeResult {
  from: string;
  to: string;
  instances_created: number;
  candidates_skipped_existing: number;
  warnings?: { code: string; message: string }[];
  duration_ms: number;
}

/**
 * Result of the lifecycle endpoints. Start returns warnings; complete and
 * cancel return only the new status.
 */
export interface StartInstanceResult {
  instanceId: string;
  status: InstanceStatus;
  activeGroupId: string;
  startedAt: string;
  warnings: ConflictWarning[];
}

export interface InstanceStatusResult {
  instanceId: string;
  status: InstanceStatus;
  completedAt?: string;
}

export interface BackendStartInstanceResult {
  instance_id: number;
  status: InstanceStatus;
  active_group_id: number;
  started_at: string;
  warnings: Array<{
    kind: ConflictKind;
    resource_id: number;
    message: string;
    can_override: boolean;
  }>;
}

export interface BackendInstanceStatusResult {
  instance_id: number;
  status: InstanceStatus;
  completed_at?: string;
}

/**
 * Body for POST /api/timetable/instances. Mirrors the Go
 * createInstanceRequest shape; the caller passes ISO date and HH:MM
 * times (Berlin local), the backend handles normalisation.
 */
export interface CreateInstanceBody {
  date: string; // YYYY-MM-DD
  start_time: string; // HH:MM
  end_time: string; // HH:MM
  title: string;
  room_id: number;
  description?: string;
  notes?: string;
  activity_group_id?: number;
  staff_ids?: number[];
}

/**
 * Body for POST /api/timetable/templates. weekdays use ISO 8601
 * (Mo=1 … Fr=5). week_pattern: 0=every week, 1=A, 2=B.
 *
 * materialize_from / materialize_to are optional — when present the
 * backend triggers a materialization run for that window after the
 * template lands so fresh instances appear on the grid immediately.
 */
export interface CreateTemplateBody {
  name: string;
  type: ActivityType;
  weekdays: number[];
  start_time: string; // HH:MM
  end_time: string; // HH:MM
  room_id: number;
  category_id: number;
  max_participants?: number;
  week_pattern?: number;
  calendar_period_id?: number;
  materialize_from?: string;
  materialize_to?: string;
}

export interface CreateTemplateResult {
  templateId: string;
  timeframeId: string;
  scheduleIds: string[];
  instancesCreated?: number;
  materializedFrom?: string;
  materializedTo?: string;
}

export interface BackendCreateTemplateResult {
  template_id: number;
  timeframe_id: number;
  schedule_ids: number[];
  instances_created?: number;
  materialized_from?: string;
  materialized_to?: string;
}
