/**
 * API client for the timetable feature.
 *
 * All calls go through Next.js proxy routes under `/api/timetable/*` which
 * inject the JWT server-side. The client mirrors the surface of the backend:
 * - getWeek(from, to)            → GET    /instances?from=&to=
 * - start(id), complete(id),     → POST   /instances/{id}/{action}
 *   cancel(id)
 * - materialize(from?, to?)      → POST   /materialize
 */

import { getSession } from "next-auth/react";
import { createLogger } from "./logger";
import type {
  BackendConflictCheckResult,
  BackendShiftCoverageCheckResult,
  BackendCreateTemplateResult,
  BackendAttendanceResponse,
  BackendEndTemplateResult,
  BackendEnrichedInstance,
  BackendGapsResponse,
  BackendDeviationHistoryResponse,
  BackendInstanceStatusResult,
  BackendMaterializeResult,
  BackendReplanWeekResult,
  BackendEditedInWindowResult,
  BackendSplitTemplateResult,
  BackendStartInstanceResult,
  BackendApplyDeviationsResponse,
  BackendMoveStaffResponse,
  BackendStaffPoolResponse,
  MoveStaffInput,
  MoveStaffResponse,
  StaffPoolResponse,
  BackendTimetableTemplate,
  BackendTemplatesResponse,
  BackendWeeklyInstancesResponse,
  AttendancePatchBody,
  AttendanceResponse,
  ConflictCheckParams,
  ConflictCheckResult,
  CreateInstanceBody,
  CreateTemplateBody,
  CreateTemplateResult,
  EndTemplateBody,
  EndTemplateResult,
  EnrichedInstance,
  GapsResponse,
  DeviationHistoryResponse,
  InstanceStatusResult,
  MaterializeResult,
  ReplanWeekResult,
  EditedInWindowResult,
  ShiftCoverageCheckParams,
  ShiftCoverageCheckResult,
  SplitTemplateBody,
  SplitTemplateResult,
  StartInstanceResult,
  ApplyDeviationsInput,
  ApplyDeviationsResponse,
  TemplatesResponse,
  TimetableTemplate,
  UpdateTemplateBody,
  WeeklyInstancesResponse,
} from "./timetable-types";
import {
  mapConflictCheckResult,
  mapShiftCoverageCheckResult,
  mapCreateTemplateResult,
  mapEndTemplateResult,
  mapAttendance,
  mapGaps,
  mapDeviationHistory,
  mapApplyDeviations,
  mapMoveStaff,
  mapStaffPool,
  mapInstance,
  mapInstanceStatusResult,
  mapMaterializeResult,
  mapReplanWeekResult,
  mapEditedInWindowResult,
  mapSplitTemplateResult,
  mapStartInstanceResult,
  mapTemplates,
  mapWeeklyInstances,
} from "./timetable-helpers";

const logger = createLogger({ component: "TimetableApiClient" });

interface ApiEnvelope<T> {
  status: string;
  message?: string;
  data: T;
}

class TimetableApiError extends Error {
  readonly httpStatus: number;
  readonly code?: string;

  constructor(message: string, httpStatus: number, code?: string) {
    super(message);
    this.name = "TimetableApiError";
    this.httpStatus = httpStatus;
    this.code = code;
  }
}

async function unwrap<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `Anfrage fehlgeschlagen (HTTP ${response.status})`;
    let code: string | undefined;
    try {
      const body = (await response.json()) as { error?: string; code?: string };
      if (body.error) message = body.error;
      code = body.code;
    } catch {
      // Body wasn't JSON; keep the generic message.
    }
    throw new TimetableApiError(message, response.status, code);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const envelope = (await response.json()) as ApiEnvelope<T>;
  return envelope.data;
}

class TimetableService {
  /**
   * GET /api/timetable/instances?from=YYYY-MM-DD&to=YYYY-MM-DD.
   * Returns the materialized instances in the requested window plus
   * lightweight enrichment (room name, type, staff list, counts).
   */
  async getWeek(from: string, to: string): Promise<WeeklyInstancesResponse> {
    const session = await getSession();
    if (!session?.user?.token) {
      throw new TimetableApiError("Nicht angemeldet", 401, "UNAUTHENTICATED");
    }

    const params = new URLSearchParams({ from, to });
    const response = await fetch(`/api/timetable/instances?${params}`, {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });

    const raw = await unwrap<BackendWeeklyInstancesResponse>(response);
    return mapWeeklyInstances(raw);
  }

  /**
   * POST /api/timetable/instances — creates a planned instance outside
   * the materialization flow. When activity_group_id is omitted the row
   * is purely spontaneous (is_spontaneous=true server-side). Returns the
   * enriched instance shape so the caller can splice it into the SWR
   * cache without a refetch.
   */
  async create(body: CreateInstanceBody): Promise<EnrichedInstance> {
    const response = await fetch("/api/timetable/instances", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    const raw = await unwrap<BackendEnrichedInstance>(response);
    logger.info("instance_created", {
      instance_id: raw.id,
      is_spontaneous: raw.is_spontaneous,
    });
    return mapInstance(raw);
  }

  /**
   * POST /api/timetable/templates — creates a recurring template
   * (activities.groups + schedules + timeframe in one shot) plus
   * optionally materializes the visible week so the new rows appear
   * on the planner grid immediately.
   */
  async createTemplate(
    body: CreateTemplateBody,
  ): Promise<CreateTemplateResult> {
    const response = await fetch("/api/timetable/templates", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    const raw = await unwrap<BackendCreateTemplateResult>(response);
    logger.info("template_created", {
      template_id: raw.template_id,
      schedules: raw.schedule_ids.length,
      instances_created: raw.instances_created,
    });
    return mapCreateTemplateResult(raw);
  }

  async getTemplates(periodId?: string | null): Promise<TemplatesResponse> {
    const params = new URLSearchParams();
    if (periodId) params.set("period_id", periodId);
    const response = await fetch(
      `/api/timetable/templates${params.toString() ? `?${params}` : ""}`,
      {
        method: "GET",
        headers: { Accept: "application/json" },
        credentials: "include",
      },
    );

    const raw = await unwrap<BackendTemplatesResponse>(response);
    return mapTemplates(raw);
  }

  /**
   * GET /api/timetable/templates/{id} — single template with schedules,
   * enrollment and staff assignments. Same enriched shape as the list.
   */
  async getTemplate(templateId: string): Promise<TimetableTemplate> {
    const response = await fetch(`/api/timetable/templates/${templateId}`, {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
    const raw = await unwrap<BackendTimetableTemplate>(response);
    return mapTemplates({ templates: [raw] }).templates[0]!;
  }

  /**
   * POST /api/timetable/templates/{id}/split — "ab Datum ändern". Ends
   * the old template version at effective_date and creates a new one with
   * the submitted fields from that date on. Future instances of the old
   * version are deleted; the optional materialize window re-creates them
   * from the new version immediately.
   */
  async splitTemplate(
    templateId: string,
    body: SplitTemplateBody,
  ): Promise<SplitTemplateResult> {
    const response = await fetch(
      `/api/timetable/templates/${templateId}/split`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        credentials: "include",
        body: JSON.stringify(body),
      },
    );
    const raw = await unwrap<BackendSplitTemplateResult>(response);
    logger.info("template_split", {
      old_template_id: raw.old_template_id,
      new_template_id: raw.new_template_id,
      deleted_instances: raw.deleted_instances,
      instances_created: raw.instances_created,
    });
    return mapSplitTemplateResult(raw);
  }

  /**
   * POST /api/timetable/templates/{id}/end — "dieser und alle folgenden
   * löschen". Ends the template at effective_date and removes still-planned
   * future instances of that template.
   */
  async endTemplate(
    templateId: string,
    body: EndTemplateBody,
  ): Promise<EndTemplateResult> {
    const response = await fetch(`/api/timetable/templates/${templateId}/end`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    const raw = await unwrap<BackendEndTemplateResult>(response);
    logger.info("template_ended", {
      template_id: raw.template_id,
      effective_date: raw.effective_date,
      deleted_instances: raw.deleted_instances,
    });
    return mapEndTemplateResult(raw);
  }

  /**
   * GET /api/timetable/conflict-check — advisory pre-save conflict check
   * for a date/time window. Optional resource params are omitted when
   * empty so the backend only checks what the caller asked about.
   */
  async checkConflicts(
    params: ConflictCheckParams,
  ): Promise<ConflictCheckResult> {
    const query = new URLSearchParams({
      date: params.date,
      start_time: params.startTime,
      end_time: params.endTime,
    });
    if (params.roomId) query.set("room_id", params.roomId);
    if (params.staffIds && params.staffIds.length > 0) {
      query.set("staff_ids", params.staffIds.join(","));
    }
    if (params.studentIds && params.studentIds.length > 0) {
      query.set("student_ids", params.studentIds.join(","));
    }
    if (params.excludeInstanceId) {
      query.set("exclude_instance_id", params.excludeInstanceId);
    }

    const response = await fetch(`/api/timetable/conflict-check?${query}`, {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
    const raw = await unwrap<BackendConflictCheckResult>(response);
    return mapConflictCheckResult(raw);
  }

  /**
   * POST /api/timetable/shift-coverage — batched, read-only comparison of
   * concrete candidate dates with the assigned staff members' shifts.
   */
  async checkShiftCoverage(
    params: ShiftCoverageCheckParams,
    options?: { signal?: AbortSignal },
  ): Promise<ShiftCoverageCheckResult> {
    const response = await fetch("/api/timetable/shift-coverage", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      signal: options?.signal,
      body: JSON.stringify({
        dates: params.dates,
        start_time: params.startTime,
        end_time: params.endTime,
        staff_ids: params.staffIds.map(Number),
        exclude_instance_id: params.excludeInstanceId
          ? Number(params.excludeInstanceId)
          : undefined,
        concrete_instance_date: params.concreteInstanceDate,
        replan_activity_group_id: params.replanActivityGroupId
          ? Number(params.replanActivityGroupId)
          : undefined,
        calendar_period_id: params.calendarPeriodId
          ? Number(params.calendarPeriodId)
          : undefined,
        week_pattern: params.weekPattern,
      }),
    });
    const raw = await unwrap<BackendShiftCoverageCheckResult>(response);
    return mapShiftCoverageCheckResult(raw);
  }

  async updateTemplate(
    templateId: string,
    body: UpdateTemplateBody,
  ): Promise<TimetableTemplate> {
    const response = await fetch(`/api/timetable/templates/${templateId}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    const raw = await unwrap<BackendTimetableTemplate>(response);
    return mapTemplates({ templates: [raw] }).templates[0]!;
  }

  async archiveTemplate(templateId: string): Promise<void> {
    const response = await fetch(`/api/timetable/templates/${templateId}`, {
      method: "DELETE",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
    await unwrap<unknown>(response);
  }

  async update(
    instanceId: string,
    body: CreateInstanceBody,
  ): Promise<EnrichedInstance> {
    const response = await fetch(`/api/timetable/instances/${instanceId}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    const raw = await unwrap<BackendEnrichedInstance>(response);
    return mapInstance(raw);
  }

  async deleteCancelled(instanceId: string): Promise<void> {
    const response = await fetch(`/api/timetable/instances/${instanceId}`, {
      method: "DELETE",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
    await unwrap<unknown>(response);
    logger.info("cancelled_instance_deleted", {
      instance_id: instanceId,
    });
  }

  /**
   * POST /api/timetable/instances/{id}/start.
   * Transitions a planned instance to active and creates the active.group
   * bridge. Returns soft warnings (room/staff/student conflicts) so the
   * caller can surface them.
   */
  async start(instanceId: string): Promise<StartInstanceResult> {
    return this.lifecycle<BackendStartInstanceResult, StartInstanceResult>(
      instanceId,
      "start",
      mapStartInstanceResult,
    );
  }

  /**
   * POST /api/timetable/instances/{id}/complete.
   * Transitions an active instance to completed. Closes the active.group,
   * marks remaining expected students as absent.
   */
  async complete(instanceId: string): Promise<InstanceStatusResult> {
    return this.lifecycle<BackendInstanceStatusResult, InstanceStatusResult>(
      instanceId,
      "complete",
      mapInstanceStatusResult,
    );
  }

  /**
   * POST /api/timetable/instances/{id}/cancel.
   * Cancels a planned or active instance. From active, ends the bridge
   * cleanly via active.EndActivitySession.
   */
  async cancel(
    instanceId: string,
    reason?: string,
  ): Promise<InstanceStatusResult> {
    return this.lifecycle<BackendInstanceStatusResult, InstanceStatusResult>(
      instanceId,
      "cancel",
      mapInstanceStatusResult,
      reason ? { reason } : undefined,
    );
  }

  private async lifecycle<TBackend, TFront>(
    instanceId: string,
    action: "start" | "complete" | "cancel",
    mapper: (raw: TBackend) => TFront,
    body: Record<string, unknown> = {},
  ): Promise<TFront> {
    const response = await fetch(
      `/api/timetable/instances/${instanceId}/${action}`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        credentials: "include",
        body: JSON.stringify(body),
      },
    );

    const raw = await unwrap<TBackend>(response);
    logger.info("instance_lifecycle_action", {
      instance_id: instanceId,
      action,
    });
    return mapper(raw);
  }

  /**
   * POST /api/timetable/materialize.
   * When called without arguments, the backend defaults to next Mon–Sun.
   * The planner UI passes the currently visible week so the user can re-run
   * materialization for that window.
   */
  async materialize(from?: string, to?: string): Promise<MaterializeResult> {
    const body: Record<string, string> = {};
    if (from) body.from_date = from;
    if (to) body.to_date = to;

    const response = await fetch("/api/timetable/materialize", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });

    const raw = await unwrap<BackendMaterializeResult>(response);
    logger.info("materialize_completed", {
      from: raw.from,
      to: raw.to,
      created: raw.instances_created,
    });
    return mapMaterializeResult(raw);
  }

  /**
   * POST /api/timetable/instances/re-plan-week.
   * Optional activityGroupId narrows the re-plan to one template's planned
   * instances (backend body field activity_group_id); the
   * re-materialization still covers the whole window (insert-only).
   */
  async replanWeek(
    from: string,
    to: string,
    activityGroupId?: string,
  ): Promise<ReplanWeekResult> {
    const body: Record<string, unknown> = { from_date: from, to_date: to };
    if (activityGroupId) {
      body.activity_group_id = Number(activityGroupId);
    }
    const response = await fetch("/api/timetable/instances/re-plan-week", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });

    const raw = await unwrap<BackendReplanWeekResult>(response);
    logger.info("week_replanned", {
      from: raw.from,
      to: raw.to,
      deleted: raw.deleted_instances,
      created: raw.instances_created,
    });
    return mapReplanWeekResult(raw);
  }

  /**
   * GET /api/timetable/instances/edited-in-window (#1875).
   * Which planned occurrences of one template were individually edited ("Nur
   * diesen Termin") in [from, to] — the changes a series re-plan would discard.
   * The planner calls this before an "Alle Termine" / "Dieser und folgende"
   * edit to warn and list the affected dates. Pass includeDeletions on the
   * "following" split path: a split resurrects individually-deleted occurrences
   * under the successor template (same-template re-plans preserve them).
   */
  async countEditedInWindow(
    activityGroupId: string,
    from: string,
    to: string,
    includeDeletions = false,
  ): Promise<EditedInWindowResult> {
    const params = new URLSearchParams({
      activity_group_id: activityGroupId,
      from,
      to,
    });
    if (includeDeletions) params.set("include_deletions", "true");
    const response = await fetch(
      `/api/timetable/instances/edited-in-window?${params}`,
      {
        method: "GET",
        headers: { Accept: "application/json" },
        credentials: "include",
      },
    );

    const raw = await unwrap<BackendEditedInWindowResult>(response);
    return mapEditedInWindowResult(raw);
  }

  async getGaps(from: string, to: string): Promise<GapsResponse> {
    const params = new URLSearchParams({ date: from, date_to: to });
    const response = await fetch(`/api/timetable/gaps?${params}`, {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });

    const raw = await unwrap<BackendGapsResponse>(response);
    return mapGaps(raw);
  }

  /** Änderungsprotokoll (#1886): Abweichungs-Verlauf im Datumsfenster,
   * optional auf einen Slot (Serie + Startzeit) eingegrenzt. */
  async getDeviationHistory(
    from: string,
    to: string,
    activityGroupId?: string,
    startTime?: string,
  ): Promise<DeviationHistoryResponse> {
    const params = new URLSearchParams({ date: from, date_to: to });
    if (activityGroupId) params.set("activity_group_id", activityGroupId);
    if (startTime) params.set("start_time", startTime);
    const response = await fetch(
      `/api/timetable/deviations/history?${params}`,
      {
        method: "GET",
        headers: { Accept: "application/json" },
        credentials: "include",
      },
    );

    const raw = await unwrap<BackendDeviationHistoryResponse>(response);
    return mapDeviationHistory(raw);
  }

  /**
   * POST /api/timetable/instances/{id}/deviations — applies an entire
   * Vertretungsplan slide-over save (absences, substitute, understaffed
   * acknowledgement, cancel) in ONE backend transaction (#1840). Either every
   * edit lands or none does; the frontend never sequences independent
   * mutations that could half-commit.
   */
  async applyDeviations(
    instanceId: string,
    input: ApplyDeviationsInput,
  ): Promise<ApplyDeviationsResponse> {
    const body: Record<string, unknown> = {};
    if (input.cancel) {
      body.cancel = true;
      if (input.cancelReason) body.cancel_reason = input.cancelReason;
    } else {
      if (input.understaffedAck !== undefined) {
        body.understaffed_ack = input.understaffedAck;
        if (input.understaffedAck && input.understaffedNote) {
          body.understaffed_note = input.understaffedNote;
        }
      }
      if (input.absences && input.absences.length > 0) {
        body.absences = input.absences.map((a) => ({
          staff_id: Number(a.staffId),
          reason: a.reason ?? undefined,
        }));
      }
      if (input.substitutions && input.substitutions.length > 0) {
        body.substitutions = input.substitutions.map((s) => ({
          absent_staff_id: Number(s.absentStaffId),
          substitute_staff_id: Number(s.substituteStaffId),
          reason: s.reason ?? undefined,
        }));
      }
      if (input.presences && input.presences.length > 0) {
        body.presences = input.presences.map((staffId) => Number(staffId));
      }
    }

    const response = await fetch(
      `/api/timetable/instances/${instanceId}/deviations`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        credentials: "include",
        body: JSON.stringify(body),
      },
    );

    const raw = await unwrap<BackendApplyDeviationsResponse>(response);
    logger.info("deviations_applied", {
      instance_id: instanceId,
      cancelled: raw.cancelled,
    });
    return mapApplyDeviations(raw);
  }

  /**
   * GET /api/timetable/instances/{id}/staff-pool — Personalpool für das
   * Zeitfenster des Blocks (#1884): jede Person kategorisiert gegen
   * Dienstplan (Schichten) und überlappende Blockzuordnungen.
   */
  async getStaffPool(instanceId: string): Promise<StaffPoolResponse> {
    const response = await fetch(
      `/api/timetable/instances/${instanceId}/staff-pool`,
      {
        method: "GET",
        headers: { Accept: "application/json" },
        credentials: "include",
      },
    );
    const raw = await unwrap<BackendStaffPoolResponse>(response);
    return mapStaffPool(raw);
  }

  /**
   * POST /api/timetable/instances/{id}/move-staff — atomarer Personal-Move
   * (#1884): Entnahme aus dem Quellblock und Zuordnung zum Zielblock in
   * EINEM Save; ohne sourceInstanceId wird eine freie Person zugewiesen.
   */
  async moveStaff(
    instanceId: string,
    input: MoveStaffInput,
  ): Promise<MoveStaffResponse> {
    const body: Record<string, unknown> = { staff_id: Number(input.staffId) };
    if (input.sourceInstanceId) {
      body.source_instance_id = Number(input.sourceInstanceId);
    }
    const response = await fetch(
      `/api/timetable/instances/${instanceId}/move-staff`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        credentials: "include",
        body: JSON.stringify(body),
      },
    );
    const raw = await unwrap<BackendMoveStaffResponse>(response);
    logger.info("staff_moved", {
      target_instance_id: instanceId,
      action: raw.action,
    });
    return mapMoveStaff(raw);
  }

  async patchAttendance(
    instanceId: string,
    studentId: string,
    body: AttendancePatchBody,
  ): Promise<AttendanceResponse> {
    const response = await fetch(
      `/api/timetable/instances/${instanceId}/students/${studentId}`,
      {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        credentials: "include",
        body: JSON.stringify(body),
      },
    );

    const raw = await unwrap<BackendAttendanceResponse>(response);
    return mapAttendance(raw);
  }
}

export const timetableService = new TimetableService();
