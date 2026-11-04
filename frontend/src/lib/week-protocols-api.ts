// Wochenprotokolle (#4769): je OGS-Gruppe und Woche ein Protokoll aus den
// Abenden der Woche. Das Backend hält jeden Abend den Tag fest, macht die
// Woche ab Freitagabend zum Entwurf und schließt ihn nach der Folgewoche
// selbst ab. Wer welches Protokoll sieht, entscheidet das Backend.

import { apiErrorFromResponse } from "./api-error";
import { sessionFetch } from "./session-cache";

export type WeekProtocolStatus = "collecting" | "draft" | "closed";
type WeekProtocolDayKind = "care" | "holiday" | "closing";

interface WeekProtocolMissingChild {
  studentId: string;
  name: string;
  reason: string;
  label: string;
}

export interface WeekProtocolBlock {
  title: string;
  startTime: string;
  endTime: string;
  status: string;
  cancelReason: string;
}

interface WeekProtocolNotice {
  title: string;
  body: string;
}

export interface WeekProtocolDay {
  /** YYYY-MM-DD */
  date: string;
  /** False when the evening of the day was not recorded. */
  captured: boolean;
  kind: WeekProtocolDayKind | null;
  present: number;
  sick: number;
  excused: number;
  classTrip: number;
  absent: number;
  missing: WeekProtocolMissingChild[];
  blocks: WeekProtocolBlock[];
  notices: WeekProtocolNotice[];
  note: string;
}

export interface WeekProtocol {
  id: string;
  scope: "group" | "school";
  groupId: string | null;
  groupName: string;
  weekStart: string;
  weekEnd: string;
  status: WeekProtocolStatus;
  /** The Friday from which the week is a draft. */
  draftFrom: string;
  /** The Monday on which moto closes a draft nobody closed. */
  autoCloseOn: string;
  weekNote: string;
  closedAt: string | null;
  closedByName: string;
  closedAutomatically: boolean;
  /** Monday to Friday, and a weekend day only when it was recorded. */
  days: WeekProtocolDay[];
}

interface BackendDay {
  date: string;
  captured: boolean;
  kind?: WeekProtocolDayKind;
  present: number;
  sick: number;
  excused: number;
  class_trip: number;
  absent: number;
  missing: {
    student_id: string;
    name: string;
    reason: string;
    label: string;
  }[];
  blocks: {
    title: string;
    start_time: string;
    end_time: string;
    status: string;
    cancel_reason?: string;
  }[];
  notices: { title: string; body?: string }[];
  note: string;
}

interface BackendWeekProtocol {
  id: string;
  scope: "group" | "school";
  group_id?: string;
  group_name: string;
  week_start: string;
  week_end: string;
  status: WeekProtocolStatus;
  draft_from: string;
  auto_close_on: string;
  week_note: string;
  closed_at?: string;
  closed_by_name?: string;
  closed_automatically: boolean;
  days?: BackendDay[];
}

function mapDay(day: BackendDay): WeekProtocolDay {
  return {
    date: day.date,
    captured: day.captured,
    kind: day.kind ?? null,
    present: day.present,
    sick: day.sick,
    excused: day.excused,
    classTrip: day.class_trip,
    absent: day.absent,
    missing: day.missing.map((child) => ({
      studentId: child.student_id,
      name: child.name,
      reason: child.reason,
      label: child.label,
    })),
    blocks: day.blocks.map((block) => ({
      title: block.title,
      startTime: block.start_time,
      endTime: block.end_time,
      status: block.status,
      cancelReason: block.cancel_reason ?? "",
    })),
    notices: day.notices.map((notice) => ({
      title: notice.title,
      body: notice.body ?? "",
    })),
    note: day.note,
  };
}

function mapWeekProtocol(data: BackendWeekProtocol): WeekProtocol {
  return {
    id: data.id,
    scope: data.scope,
    groupId: data.group_id ?? null,
    groupName: data.group_name,
    weekStart: data.week_start,
    weekEnd: data.week_end,
    status: data.status,
    draftFrom: data.draft_from,
    autoCloseOn: data.auto_close_on,
    weekNote: data.week_note,
    closedAt: data.closed_at ?? null,
    closedByName: data.closed_by_name ?? "",
    closedAutomatically: data.closed_automatically,
    days: (data.days ?? []).map(mapDay),
  };
}

/** The notes of a draft: the week, and per day YYYY-MM-DD → text. */
export interface WeekProtocolNotes {
  weekNote: string;
  days: Record<string, string>;
}

async function protocolOf(
  response: Response,
  fallback: string,
): Promise<WeekProtocol> {
  if (!response.ok) {
    throw await apiErrorFromResponse(response, fallback);
  }
  const json = (await response.json()) as { data: BackendWeekProtocol };
  return mapWeekProtocol(json.data);
}

/** The page of the weekly protocols, optionally of one group. */
export function weekProtocolsPath(groupId?: string | null): string {
  return groupId
    ? `/wochenprotokolle?group=${encodeURIComponent(groupId)}`
    : "/wochenprotokolle";
}

/** The page of one weekly protocol. */
export function weekProtocolPath(protocolId: string): string {
  return `/wochenprotokolle/${protocolId}`;
}

class WeekProtocolsService {
  async list(groupId?: string | null): Promise<WeekProtocol[]> {
    const query = groupId ? `?group_id=${encodeURIComponent(groupId)}` : "";
    const response = await sessionFetch(`/api/week-protocols${query}`);
    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        "Die Wochenprotokolle konnten nicht geladen werden.",
      );
    }
    const json = (await response.json()) as { data: BackendWeekProtocol[] };
    return json.data.map(mapWeekProtocol);
  }

  async get(protocolId: string): Promise<WeekProtocol> {
    const response = await sessionFetch(`/api/week-protocols/${protocolId}`);
    return protocolOf(
      response,
      "Das Wochenprotokoll konnte nicht geladen werden.",
    );
  }

  async saveNotes(
    protocolId: string,
    notes: WeekProtocolNotes,
  ): Promise<WeekProtocol> {
    const response = await sessionFetch(
      `/api/week-protocols/${protocolId}/notes`,
      {
        method: "PUT",
        body: JSON.stringify({
          week_note: notes.weekNote,
          days: Object.entries(notes.days).map(([date, note]) => ({
            date,
            note,
          })),
        }),
      },
    );
    return protocolOf(
      response,
      "Die Ergänzungen konnten nicht gespeichert werden.",
    );
  }

  async close(protocolId: string): Promise<WeekProtocol> {
    const response = await sessionFetch(
      `/api/week-protocols/${protocolId}/close`,
      { method: "POST" },
    );
    return protocolOf(
      response,
      "Das Wochenprotokoll konnte nicht abgeschlossen werden.",
    );
  }

  /** Same-origin proxy URL of the closed protocol as PDF. */
  pdfUrl(protocolId: string): string {
    return `/api/week-protocols/${protocolId}/pdf`;
  }
}

export const weekProtocolsService = new WeekProtocolsService();
