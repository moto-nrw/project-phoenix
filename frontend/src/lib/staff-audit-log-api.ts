// Cross-staff audit feed (#1417): merged change trail over session edits,
// absence transitions, Stundenkonto bookings, month close/reopen, and
// deletion tombstones. Requires time_tracking:manage — callers must gate on
// the permission (like the time-accounts overview) so no request fires
// without it.

import { sessionFetch } from "./session-cache";

// ─── Wire shapes (backend snake_case) ────────────────────────────────────────

export type AuditLogSource =
  | "session_edit"
  | "absence"
  | "adjustment"
  | "vacation_opening"
  | "absence_type_allowance"
  | "month_close"
  | "month_reopen"
  | "deletion"
  | "personnel_number";

export interface BackendAuditLogEvent {
  occurred_at: string;
  source: AuditLogSource;
  entry_id: number;
  staff?: { id: number; name: string };
  actor: {
    staff_id?: number;
    name: string;
    is_system: boolean;
    is_self: boolean;
  };
  reason: string;
  /** Source-typed payload; see the render switch in staff-audit-log.tsx. */
  detail: Record<string, unknown>;
}

export interface BackendAuditLogPage {
  events: BackendAuditLogEvent[];
  next_cursor?: string;
  /** Oldest day session edits can still exist for (retention setting). */
  retention_cutoff: string;
}

// ─── Frontend shapes ─────────────────────────────────────────────────────────

export interface AuditLogEvent {
  occurredAt: string;
  source: AuditLogSource;
  entryId: string;
  staffId: string | null;
  staffName: string;
  actorStaffId: string | null;
  actorName: string;
  actorIsSystem: boolean;
  actorIsSelf: boolean;
  reason: string;
  detail: Record<string, unknown>;
}

export interface AuditLogPage {
  events: AuditLogEvent[];
  nextCursor: string | null;
  retentionCutoff: string;
}

export const auditLogSourceLabels: Record<AuditLogSource, string> = {
  session_edit: "Zeiterfassung",
  absence: "Abwesenheit",
  adjustment: "Stundenkonto",
  vacation_opening: "Urlaubs-Übernahme",
  absence_type_allowance: "Abwesenheitskontingent",
  month_close: "Monatsabschluss",
  month_reopen: "Monat geöffnet",
  deletion: "Löschung",
  personnel_number: "Personalnummer",
};

export function mapAuditLogEvent(data: BackendAuditLogEvent): AuditLogEvent {
  return {
    occurredAt: data.occurred_at,
    source: data.source,
    entryId: data.entry_id.toString(),
    staffId: data.staff ? data.staff.id.toString() : null,
    staffName: data.staff?.name ?? "",
    actorStaffId:
      data.actor.staff_id !== undefined ? data.actor.staff_id.toString() : null,
    actorName: data.actor.name,
    actorIsSystem: data.actor.is_system,
    actorIsSelf: data.actor.is_self,
    reason: data.reason,
    detail: data.detail,
  };
}

export interface AuditLogQuery {
  /** YYYY-MM-DD, inclusive. */
  from?: string;
  to?: string;
  staffId?: string;
  actorId?: string;
  sources?: AuditLogSource[];
  cursor?: string;
  limit?: number;
}

export function buildAuditLogQuery(query: AuditLogQuery): string {
  const params = new URLSearchParams();
  if (query.from) params.set("from", query.from);
  if (query.to) params.set("to", query.to);
  if (query.staffId) params.set("staff_id", query.staffId);
  if (query.actorId) params.set("actor_id", query.actorId);
  if (query.sources && query.sources.length > 0)
    params.set("sources", query.sources.join(","));
  if (query.cursor) params.set("cursor", query.cursor);
  if (query.limit !== undefined) params.set("limit", String(query.limit));
  return params.toString();
}

export const staffAuditLogService = {
  /** Requires time_tracking:manage. */
  async getAuditLog(query: AuditLogQuery = {}): Promise<AuditLogPage> {
    const suffix = buildAuditLogQuery(query);
    const response = await sessionFetch(
      `/api/staff/time-tracking/audit-log${suffix ? `?${suffix}` : ""}`,
    );
    if (!response.ok) {
      throw new Error("Änderungsprotokoll konnte nicht geladen werden");
    }
    const json = (await response.json()) as { data: BackendAuditLogPage };
    return {
      events: json.data.events.map(mapAuditLogEvent),
      nextCursor: json.data.next_cursor ?? null,
      retentionCutoff: json.data.retention_cutoff,
    };
  },
};
