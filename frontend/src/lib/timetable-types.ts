/**
 * Domain types for the timetable feature.
 *
 * Mirrors the Go DTOs in backend/api/timetable/. Backend int64 IDs become
 * frontend strings. Backend snake_case becomes frontend camelCase via the
 * mappers in timetable-helpers.ts.
 */

export type InstanceStatus = "planned" | "active" | "completed" | "cancelled";

export type ActivityType = "care" | "activity" | "external";

type ConflictKind = "room" | "staff" | "student";

/**
 * Soft warning surfaced by the backend when an instance has overlapping
 * rooms, double-booked staff, or double-booked students. Conflicts never
 * block an action; they are advisory and can always be overridden.
 */
interface ConflictWarning {
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
  absenceReason?: string;
}

type InstanceAttendanceStatus = "expected" | "present" | "absent";
type InstanceAttendanceSubstatus =
  "late" | "excused" | "sick" | "field_trip" | "other";

export interface InstanceStudentSummary {
  studentId: string;
  status: InstanceAttendanceStatus;
  substatus?: InstanceAttendanceSubstatus | null;
  note?: string | null;
  checkedInAt?: string | null;
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
  /** Per-occurrence Tagesnotiz (schedule.activity_instances.notes). */
  notes?: string;
  /**
   * Durable Wochennotiz inherited from the series template
   * (activities.groups.notes). Read-only on the occurrence; edited via the
   * Regeltermin. Shown on every occurrence, survives Re-Plan/Split.
   */
  seriesNotes?: string;
  status: InstanceStatus;
  isSpontaneous: boolean;
  isLive: boolean;
  activityGroupId?: string;
  activityType: ActivityType;
  roomId: string;
  roomName: string;
  staff: InstanceStaffSummary[];
  studentIds: string[];
  students: InstanceStudentSummary[];
  staffCount: number;
  absentStaffCount: number;
  /**
   * #1840: admin deliberately accepts this block running with zero staff.
   * Optional so existing instance fixtures need no change; the mapper always
   * populates it (defaults false).
   */
  understaffedAck?: boolean;
  understaffedNote?: string;
  cancelReason?: string;
  expectedStudentsCount: number;
  presentStudentsCount: number;
  /**
   * Betreuungsplan capacity indicator (issue #1838): requiredStaffCount is
   * ceil(children / Betreuungsschlüssel); assignedStaffCount is the
   * non-absent staff count (staffCount - absentStaffCount, computed
   * backend-side). Understaffed = assignedStaffCount < requiredStaffCount.
   */
  requiredStaffCount: number;
  assignedStaffCount: number;
  /**
   * Raw manual Personalbedarf override (issue #1839), undefined when the block
   * derives its requirement from the Betreuungsschlüssel. requiredStaffCount
   * above already folds the override in; this is the raw value the edit form
   * needs to distinguish "derive" from a set number.
   */
  requiredStaffOverride?: number;
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
interface BackendInstanceStaffSummary {
  staff_id: number;
  is_primary: boolean;
  is_absent: boolean;
  is_substitute: boolean;
  absence_reason?: string | null;
}

interface BackendInstanceStudentSummary {
  student_id: number;
  status: InstanceAttendanceStatus;
  substatus?: InstanceAttendanceSubstatus | null;
  note?: string | null;
  checked_in_at?: string | null;
}

export interface BackendEnrichedInstance {
  id: number;
  date: string;
  start_time: string;
  end_time: string;
  title: string;
  description?: string;
  notes?: string;
  series_notes?: string;
  status: InstanceStatus;
  is_spontaneous: boolean;
  is_live: boolean;
  activity_group_id?: number;
  activity_type: ActivityType;
  room_id: number;
  room_name: string;
  staff: BackendInstanceStaffSummary[];
  student_ids?: number[];
  students?: BackendInstanceStudentSummary[];
  staff_count: number;
  absent_staff_count: number;
  understaffed_ack?: boolean;
  understaffed_note?: string | null;
  cancel_reason?: string | null;
  expected_students_count: number;
  present_students_count: number;
  required_staff_count: number;
  assigned_staff_count: number;
  required_staff_override?: number | null;
  conflict_warnings?: Array<{
    kind: ConflictKind;
    resource_id: number;
    message: string;
    can_override: boolean;
  }>;
}

export interface BackendWeeklyInstancesResponse {
  from: string;
  to: string;
  instances: BackendEnrichedInstance[];
}

export interface GapInstance {
  instanceId: string;
  date: string;
  title: string;
  startTime: string;
  endTime: string;
  roomId: string;
  status: InstanceStatus;
  assignedStaffCount: number;
  absentStaffCount: number;
  /** Non-absent count: planned people still there plus any covering substitute. */
  presentStaffCount?: number;
  /** Base-plan positions (non-substitute rows). A partial shortfall has 0 < present < planned. */
  plannedStaffCount?: number;
  /** Present only on acknowledged gaps (#1840): the deliberately-unstaffed reason. */
  understaffedNote?: string;
}

export interface GapsResponse {
  from: string;
  to: string;
  gaps: GapInstance[];
  /** #1840: zero-staff blocks an admin deliberately left open — still a shortfall, no longer nagging. */
  acknowledged: GapInstance[];
}

export interface BackendGapInstance {
  instance_id: number;
  date: string;
  title: string;
  start_time: string;
  end_time: string;
  room_id: number;
  status: InstanceStatus;
  assigned_staff_count: number;
  absent_staff_count: number;
  present_staff_count?: number;
  planned_staff_count?: number;
  understaffed_note?: string | null;
}

export interface BackendGapsResponse {
  from: string;
  to: string;
  gaps: BackendGapInstance[];
  acknowledged?: BackendGapInstance[];
}

type ExceptionConflictKind =
  | "cancelled_instance_with_scheduled_arrivals"
  | "modified_instance_time_mismatch";

export interface ExceptionConflict {
  kind: ExceptionConflictKind;
  date: string;
  activityGroupId: string;
  instanceId: string;
  activityTitle: string;
  studentId: string;
  expectedArrival?: string;
  arrivalSource: string;
  cancellationReason?: string;
  originalStartTime?: string;
  modifiedStartTime?: string;
}

export interface ExceptionConflictsResponse {
  from: string;
  to: string;
  conflicts: ExceptionConflict[];
}

interface BackendExceptionConflict {
  kind: ExceptionConflictKind;
  date: string;
  activity_group_id: number;
  instance_id: number;
  activity_title: string;
  student_id: number;
  expected_arrival?: string;
  arrival_source: string;
  cancellation_reason?: string;
  original_start_time?: string;
  modified_start_time?: string;
}

export interface BackendExceptionConflictsResponse {
  from: string;
  to: string;
  conflicts: BackendExceptionConflict[];
}

interface TemplateSchedule {
  id: string;
  weekday: number;
  startTime: string;
  endTime: string;
  weekPattern: number;
  calendarPeriodId?: string;
  validUntil?: string;
}

/**
 * Zielgruppe (target group) type for a Betreuungsplan block (issue #1838).
 * "gruppe" reuses educationGroupId/educationGroupName above rather than a
 * separate value field.
 */
export type TargetGroupType =
  "jahrgang" | "klasse" | "gruppe" | "angebot" | "none";

export interface TimetableTemplate {
  id: string;
  name: string;
  type: ActivityType;
  categoryId: string;
  categoryName: string;
  roomId?: string;
  roomName?: string;
  educationGroupId?: string;
  educationGroupName?: string;
  isOpen: boolean;
  maxParticipants: number;
  /** Durable Wochennotiz for the series (activities.groups.notes, #1837). */
  notes?: string;
  /** Category's mapped Dienstplan-Schichtart (#1836/#1837); empty = unmapped. */
  shiftTypeName?: string;
  shiftTypeColor?: string;
  /** The template's own calendar-period pin (distinct from each schedule's). */
  calendarPeriodId?: string;
  targetGroupType: TargetGroupType;
  targetGradeLevel?: number;
  targetSchoolClass?: string;
  enrollmentCount: number;
  supervisorCount: number;
  /** Betreuungsplan capacity indicator (issue #1838) — see EnrichedInstance. */
  requiredStaffCount: number;
  assignedStaffCount: number;
  /** Raw manual Personalbedarf override (issue #1839); undefined = derive. */
  requiredStaffOverride?: number;
  studentIds: string[];
  staffIds: string[];
  primaryStaffId?: string;
  schedules: TemplateSchedule[];
}

export interface TemplatesResponse {
  templates: TimetableTemplate[];
}

interface BackendTemplateSchedule {
  id: number;
  weekday: number;
  start_time: string;
  end_time: string;
  week_pattern: number;
  calendar_period_id?: number;
  valid_until?: string;
}

export interface BackendTimetableTemplate {
  id: number;
  name: string;
  type: ActivityType;
  category_id: number;
  category_name: string;
  room_id?: number;
  room_name?: string;
  education_group_id?: number;
  education_group_name?: string;
  is_open: boolean;
  max_participants: number;
  notes?: string;
  shift_type_name?: string;
  shift_type_color?: string;
  calendar_period_id?: number;
  target_group_type: TargetGroupType;
  target_grade_level?: number;
  target_school_class?: string;
  enrollment_count: number;
  supervisor_count: number;
  required_staff_count: number;
  assigned_staff_count: number;
  required_staff_override?: number | null;
  student_ids?: number[];
  staff_ids?: number[];
  primary_staff_id?: number;
  schedules: BackendTemplateSchedule[];
}

export interface BackendTemplatesResponse {
  templates: BackendTimetableTemplate[];
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

export interface ReplanWeekResult {
  from: string;
  to: string;
  deletedInstances: number;
  candidatesSkippedExisting: number;
  instancesCreated: number;
  instanceStudentsCreated: number;
  instanceStaffCreated: number;
  warnings: MaterializeWarning[];
  durationMs: number;
}

export interface BackendReplanWeekResult {
  from: string;
  to: string;
  deleted_instances: number;
  candidates_skipped_existing: number;
  instances_created: number;
  instance_students_created: number;
  instance_staff_created: number;
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

export interface AttendancePatchBody {
  status?: InstanceAttendanceStatus;
  substatus?: InstanceAttendanceSubstatus | null;
  note?: string | null;
}

export interface AttendanceResponse {
  id: string;
  instanceId: string;
  studentId: string;
  status: InstanceAttendanceStatus;
  substatus?: InstanceAttendanceSubstatus | null;
  note?: string | null;
  checkedInAt?: string | null;
}

export interface BackendAttendanceResponse {
  id: number;
  instance_id: number;
  student_id: number;
  status: InstanceAttendanceStatus;
  substatus?: InstanceAttendanceSubstatus | null;
  note?: string | null;
  checked_in_at?: string | null;
}

interface SubstituteTimeConflict {
  instanceId: string;
  title: string;
  date: string;
  startTime: string;
  endTime: string;
}

type SubstituteAction =
  | "substituted"
  | "already_substituted"
  | "already_on_instance"
  | "marked_absent"
  | "already_absent"
  // Returned by POST /instances/{id}/deviations when a saved day-wide absence is
  // restored (staff marked present again) — see affectedInstanceOf (#1840).
  | "marked_present";

interface SubstituteAffectedInstance {
  instanceId: string;
  title: string;
  startTime: string;
  action: SubstituteAction;
}

export interface SubstituteResponse {
  absentStaffId: string;
  substituteStaffId: string;
  date: string;
  affectedInstances: SubstituteAffectedInstance[];
  warnings: SubstituteTimeConflict[];
}

interface BackendSubstituteTimeConflict {
  instance_id: number;
  title: string;
  date: string;
  start_time: string;
  end_time: string;
}

interface BackendSubstituteAffectedInstance {
  instance_id: number;
  title: string;
  start_time: string;
  action: SubstituteAction;
}

export interface BackendSubstituteResponse {
  absent_staff_id: number;
  substitute_staff_id: number;
  date: string;
  affected_instances: BackendSubstituteAffectedInstance[];
  warnings: BackendSubstituteTimeConflict[];
}

/** #1840: POST /instances/{id}/acknowledge-understaffed result. */
export interface AcknowledgeUnderstaffedResponse {
  instanceId: string;
  status: InstanceStatus;
  understaffedAck: boolean;
  understaffedNote?: string;
}

export interface BackendAcknowledgeUnderstaffedResponse {
  instance_id: number;
  status: InstanceStatus;
  understaffed_ack: boolean;
  understaffed_note?: string | null;
}

/**
 * #1840: the whole Vertretungsplan slide-over save applied atomically via
 * POST /instances/{id}/deviations. `cancel` is exclusive (other fields are
 * ignored); `understaffedAck` undefined means "no change". IDs are frontend
 * strings; the client converts them to numbers for the backend.
 */
export interface ApplyDeviationsInput {
  cancel?: boolean;
  cancelReason?: string;
  understaffedAck?: boolean;
  understaffedNote?: string;
  absences?: Array<{ staffId: string; reason?: string }>;
  substitutions?: Array<{
    absentStaffId: string;
    substituteStaffId: string;
    reason?: string;
  }>;
  /** Staff to mark present again — clears a persisted day-wide absence (#1840). */
  presences?: string[];
}

export interface ApplyDeviationsResponse {
  instanceId: string;
  cancelled: boolean;
  understaffedAck: boolean;
  affectedInstances: SubstituteAffectedInstance[];
  warnings: SubstituteTimeConflict[];
}

export interface BackendApplyDeviationsResponse {
  instance_id: number;
  cancelled: boolean;
  understaffed_ack: boolean;
  affected_instances: BackendSubstituteAffectedInstance[];
  warnings: BackendSubstituteTimeConflict[];
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
  student_ids?: number[];
  /** Manual Personalbedarf override (#1839); null/omitted = derive. */
  required_staff?: number | null;
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
  /** Durable Wochennotiz for the series (#1837 follow-up); omitted = none. */
  notes?: string;
  education_group_id?: number;
  max_participants?: number;
  /** Manual Personalbedarf override (#1839); null/omitted = derive. */
  required_staff?: number | null;
  week_pattern?: number;
  calendar_period_id?: number;
  target_group_type?: TargetGroupType;
  target_grade_level?: number;
  target_school_class?: string;
  materialize_from?: string;
  materialize_to?: string;
  student_ids?: number[];
  staff_ids?: number[];
  primary_staff_id?: number;
}

export type UpdateTemplateBody = Omit<
  CreateTemplateBody,
  "materialize_from" | "materialize_to"
>;

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

/**
 * Body for POST /api/timetable/templates/{id}/split — the "ab Datum
 * ändern" flow. Carries every field of the regular update-template body
 * plus the required effective_date (YYYY-MM-DD) from which the new
 * template version applies. The optional materialize window triggers an
 * immediate materialization run for the new template, mirroring the
 * create-template behaviour.
 */
export interface SplitTemplateBody extends UpdateTemplateBody {
  effective_date: string; // YYYY-MM-DD
  materialize_from?: string; // YYYY-MM-DD
  materialize_to?: string; // YYYY-MM-DD
}

export interface SplitTemplateResult {
  oldTemplateId: string;
  newTemplateId: string;
  scheduleIds: string[];
  deletedInstances: number;
  instancesCreated: number;
}

export interface BackendSplitTemplateResult {
  old_template_id: number;
  new_template_id: number;
  schedule_ids: number[];
  deleted_instances: number;
  instances_created: number;
}

export interface EndTemplateBody {
  effective_date: string; // YYYY-MM-DD
}

export interface EndTemplateResult {
  templateId: string;
  effectiveDate: string;
  deletedInstances: number;
}

export interface BackendEndTemplateResult {
  template_id: number;
  effective_date: string;
  deleted_instances: number;
}

/**
 * Params for GET /api/timetable/conflict-check. Optional resource params
 * are omitted from the query string when empty — the backend treats a
 * missing param as "do not check this resource kind".
 */
export interface ConflictCheckParams {
  date: string; // YYYY-MM-DD
  startTime: string; // HH:MM
  endTime: string; // HH:MM
  roomId?: string;
  staffIds?: string[];
  studentIds?: string[];
  excludeInstanceId?: string;
}

/**
 * One advisory conflict from the pre-save check. Never blocking — the
 * caller surfaces it as a hint and the user can always proceed.
 */
export interface ConflictWarningItem {
  kind: ConflictKind;
  resourceId: string;
  message: string;
  conflictingInstanceId: string;
  conflictingTitle: string;
}

/**
 * Advisory Dienstplan warning for one uncovered part of a staff assignment.
 * The warning never blocks saving a Betreuungsplan block.
 */
export interface ShiftCoverageWarningItem {
  staffId: string;
  staffName: string;
  date: string;
  startTime: string;
  endTime: string;
  uncoveredStartTime: string;
  uncoveredEndTime: string;
  message: string;
}

export interface ConflictCheckResult {
  date: string;
  startTime: string;
  endTime: string;
  warnings: ConflictWarningItem[];
}

/**
 * Batched, read-only Dienstplan probe. Dates are concrete candidates; the
 * backend applies the selected calendar period's existing A/B-week rule.
 */
export interface ShiftCoverageCheckParams {
  dates: string[];
  startTime: string;
  endTime: string;
  staffIds: string[];
  excludeInstanceId?: string;
  /** Date whose concrete roster belongs to excludeInstanceId in a series probe. */
  concreteInstanceDate?: string;
  /** Existing series whose concrete #1871 deviations survive this edit. */
  replanActivityGroupId?: string;
  calendarPeriodId?: string;
  weekPattern?: number;
}

export interface ShiftCoverageCheckResult {
  coverageWarnings: ShiftCoverageWarningItem[];
  coverageWarningCount: number;
}

export interface BackendShiftCoverageCheckResult {
  coverage_warning_count?: number;
  coverage_warnings?: Array<{
    staff_id: number;
    staff_name: string;
    date: string;
    start_time: string;
    end_time: string;
    uncovered_start_time: string;
    uncovered_end_time: string;
    message: string;
  }>;
}

export interface BackendConflictCheckResult {
  date: string;
  start_time: string;
  end_time: string;
  warnings?: Array<{
    kind: ConflictKind;
    resource_id: number;
    message: string;
    conflicting_instance_id: number;
    conflicting_title: string;
  }>;
}
