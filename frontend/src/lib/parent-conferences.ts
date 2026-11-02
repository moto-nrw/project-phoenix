// Elternsprechtag (#4801): Die Lehrkraft bietet in moto schule einen Tag für
// eine ihrer Klassen an, die Eltern buchen in der Eltern-App ein freies
// Zeitfenster je Kind. Die Zeitfenster rechnet das Backend aus Beginn, Ende,
// Gesprächslänge und Pausen; diese Datei hält Typen, Aufrufe und die Prüfung
// des Formulars. Die Aufrufe der Eltern-App liegen in `parent-api.ts`.

import { authFetch } from "~/lib/api-helpers";

interface ProxyResponse<T> {
  readonly data: T;
}

export interface ParentConferenceWindow {
  readonly start_time: string;
  readonly end_time: string;
}

export interface ParentConferenceSlot extends ParentConferenceWindow {
  readonly booked: boolean;
  readonly student_id?: string;
  readonly student_name?: string;
}

/** Ein Sprechtag, wie ihn die Lehrkraft sieht: mit Belegung. */
export interface ParentConference {
  readonly id: string;
  readonly school_class: string;
  readonly date: string;
  readonly start_time: string;
  readonly end_time: string;
  readonly slot_minutes: number;
  readonly breaks: readonly ParentConferenceWindow[];
  readonly location?: string;
  readonly booked_count: number;
  readonly slots: readonly ParentConferenceSlot[];
}

export interface SchoolParentConferenceList {
  readonly conferences: readonly ParentConference[];
  readonly classes: readonly string[];
}

/** Ein Sprechtag, wie ihn die Eltern für ein Kind sehen. */
export interface ParentConferenceOffer {
  readonly id: string;
  readonly school_class: string;
  readonly teacher_name: string;
  readonly date: string;
  readonly location?: string;
  readonly free_slots: readonly ParentConferenceWindow[];
  readonly booking?: ParentConferenceWindow;
  readonly can_change: boolean;
}

export interface ParentConferenceRequest {
  readonly school_class: string;
  readonly date: string;
  readonly start_time: string;
  readonly end_time: string;
  readonly slot_minutes: number;
  readonly breaks: readonly ParentConferenceWindow[];
  readonly location?: string;
}

export const SCHOOL_PARENT_CONFERENCES_ROUTE = "/school/elternsprechtag";
const SCHOOL_BASE = "/api/school/calendar/parent-conferences";

export async function fetchSchoolParentConferences(): Promise<SchoolParentConferenceList> {
  const response =
    await authFetch<ProxyResponse<Partial<SchoolParentConferenceList>>>(
      SCHOOL_BASE,
    );
  return {
    conferences: response.data.conferences ?? [],
    classes: response.data.classes ?? [],
  };
}

export async function createSchoolParentConference(
  request: ParentConferenceRequest,
): Promise<ParentConference> {
  const response = await authFetch<ProxyResponse<ParentConference>>(
    SCHOOL_BASE,
    {
      method: "POST",
      body: request,
    },
  );
  return response.data;
}

export async function updateSchoolParentConference(
  id: string,
  request: ParentConferenceRequest,
): Promise<ParentConference> {
  const response = await authFetch<ProxyResponse<ParentConference>>(
    `${SCHOOL_BASE}/${encodeURIComponent(id)}`,
    { method: "PUT", body: request },
  );
  return response.data;
}

export async function deleteSchoolParentConference(id: string): Promise<void> {
  await authFetch<unknown>(`${SCHOOL_BASE}/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export interface ParentConferenceDraft {
  readonly schoolClass: string;
  readonly date: string;
  readonly startTime: string;
  readonly endTime: string;
  readonly slotMinutes: string;
  readonly breakStart: string;
  readonly breakEnd: string;
  // The form edits the first break; retain further API-created breaks on save.
  readonly additionalBreaks: readonly ParentConferenceWindow[];
  readonly location: string;
}

export const PARENT_CONFERENCE_SLOT_OPTIONS = ["10", "15", "20", "30"] as const;

export function emptyParentConferenceDraft(
  classes: readonly string[],
): ParentConferenceDraft {
  return {
    schoolClass: classes.length === 1 ? (classes[0] ?? "") : "",
    date: "",
    startTime: "14:00",
    endTime: "17:00",
    slotMinutes: "15",
    breakStart: "",
    breakEnd: "",
    additionalBreaks: [],
    location: "",
  };
}

export function parentConferenceDraftOf(
  conference: ParentConference,
): ParentConferenceDraft {
  const pause = conference.breaks[0];
  return {
    schoolClass: conference.school_class,
    date: conference.date,
    startTime: conference.start_time,
    endTime: conference.end_time,
    slotMinutes: String(conference.slot_minutes),
    breakStart: pause?.start_time ?? "",
    breakEnd: pause?.end_time ?? "",
    additionalBreaks: conference.breaks.slice(1),
    location: conference.location ?? "",
  };
}

/**
 * Die Fehler des Formulars je Feld, mit den Feldnamen des Backends. Die
 * Zeiten kommen aus `<input type="time">` als „HH:MM“ und lassen sich als
 * Text vergleichen.
 */
export function parentConferenceFieldErrors(
  draft: ParentConferenceDraft,
  today: string,
): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!draft.schoolClass) errors.school_class = "Bitte wählen Sie eine Klasse.";
  if (!draft.date) errors.date = "Bitte wählen Sie einen Tag.";
  else if (draft.date < today)
    errors.date = "Der Tag liegt in der Vergangenheit.";
  if (!draft.startTime) errors.start_time = "Bitte geben Sie den Beginn an.";
  if (!draft.endTime) errors.end_time = "Bitte geben Sie das Ende an.";
  else if (draft.startTime && draft.endTime <= draft.startTime)
    errors.end_time = "Das Ende muss nach dem Beginn liegen.";
  const hasBreak = draft.breakStart !== "" || draft.breakEnd !== "";
  if (hasBreak) {
    if (
      !draft.breakStart ||
      !draft.breakEnd ||
      draft.breakEnd <= draft.breakStart
    ) {
      errors.breaks = "Die Pause braucht einen Beginn und ein späteres Ende.";
    } else if (
      draft.breakStart <= draft.startTime ||
      draft.breakEnd >= draft.endTime
    ) {
      errors.breaks = "Die Pause muss zwischen Beginn und Ende liegen.";
    }
  }
  return errors;
}

export function parentConferenceRequest(
  draft: ParentConferenceDraft,
): ParentConferenceRequest {
  const location = draft.location.trim();
  return {
    school_class: draft.schoolClass,
    date: draft.date,
    start_time: draft.startTime,
    end_time: draft.endTime,
    slot_minutes: Number(draft.slotMinutes),
    breaks: [
      ...(draft.breakStart && draft.breakEnd
        ? [{ start_time: draft.breakStart, end_time: draft.breakEnd }]
        : []),
      ...draft.additionalBreaks,
    ],
    ...(location ? { location } : {}),
  };
}

/** „14:15–14:30 Uhr“ */
export function parentConferenceWindowLabel(
  window: ParentConferenceWindow,
): string {
  return `${window.start_time}–${window.end_time} Uhr`;
}
