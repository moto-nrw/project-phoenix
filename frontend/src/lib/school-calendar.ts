// Kalender in moto schule (#4798): Schließtage und Ferien als ganztägige
// Einträge im Raster, das Formular eines Schultermins und seine Prüfung.

import type {
  CalendarEvent,
  SchoolAppointmentParents,
  SchoolAppointmentRequest,
  SchoolCalendarDays,
  SchoolCalendarResponse,
  SchoolEventType,
} from "~/lib/personal-calendar-api";

function daysAsEvents(
  days: readonly SchoolCalendarDays[],
  source: "closing_day" | "holiday",
): CalendarEvent[] {
  return days.map((day) => ({
    id: `${source}:${day.id}`,
    source,
    title: day.name,
    start_date: day.start_date,
    end_date: day.end_date,
    start_time: "00:00",
    end_time: "23:59",
    all_day: true,
    can_respond: false,
    can_edit: false,
  }));
}

/**
 * Alle Einträge des Rasters: die Termine, dazu Schließtage und Ferien der
 * Schule als ganztägige Einträge. So sieht die Lehrkraft auf einen Blick,
 * wann die OGS zu hat.
 */
export function schoolCalendarEvents(
  response: SchoolCalendarResponse | undefined,
): CalendarEvent[] {
  if (!response) return [];
  return [
    ...response.events,
    ...daysAsEvents(response.closing_days ?? [], "closing_day"),
    ...daysAsEvents(response.holidays ?? [], "holiday"),
  ];
}

export interface SchoolAppointmentDraft {
  readonly title: string;
  readonly eventType: SchoolEventType;
  readonly startDate: string;
  readonly endDate: string;
  readonly allDay: boolean;
  readonly startTime: string;
  readonly endTime: string;
  readonly location: string;
  readonly description: string;
  readonly parents: SchoolAppointmentParents;
  readonly classes: readonly string[];
}

export function emptySchoolAppointmentDraft(
  dateISO: string,
): SchoolAppointmentDraft {
  return {
    title: "",
    eventType: "ausflug",
    startDate: dateISO,
    endDate: dateISO,
    allDay: false,
    startTime: "08:00",
    endTime: "12:00",
    location: "",
    description: "",
    parents: "none",
    classes: [],
  };
}

/** Das Formular zum Ändern eines eigenen Schultermins. */
export function schoolAppointmentDraftFromEvent(
  event: CalendarEvent,
): SchoolAppointmentDraft {
  return {
    title: event.title,
    eventType: event.school_event_type ?? "sonstiges",
    startDate: event.start_date,
    endDate: event.end_date,
    allDay: event.all_day,
    startTime: event.all_day ? "08:00" : event.start_time,
    endTime: event.all_day ? "12:00" : event.end_time,
    location: event.location ?? "",
    description: event.description ?? "",
    // Beim Ändern liest das Backend die Eltern nicht; der Empfängerkreis
    // bleibt, wie er beim Anlegen gewählt wurde.
    parents: "none",
    classes: [],
  };
}

/** Prüfung vor dem Senden: Hinweis am Feld, Schlüssel wie im Backend. */
export function schoolAppointmentFieldErrors(
  draft: SchoolAppointmentDraft,
  creating: boolean,
): Record<string, string> {
  const fields: Record<string, string> = {};
  if (!draft.title.trim()) fields.title = "Bitte geben Sie einen Titel ein.";
  if (!draft.startDate) fields.start_date = "Bitte wählen Sie den ersten Tag.";
  if (!draft.endDate) fields.end_date = "Bitte wählen Sie den letzten Tag.";
  if (
    !draft.allDay &&
    draft.startDate === draft.endDate &&
    draft.startTime &&
    draft.endTime &&
    draft.endTime <= draft.startTime
  ) {
    fields.end_time = "Das Ende muss nach dem Beginn liegen.";
  }
  if (creating && draft.parents === "classes" && draft.classes.length === 0) {
    fields.classes = "Bitte wählen Sie mindestens eine Klasse.";
  }
  return fields;
}

export function schoolAppointmentRequest(
  draft: SchoolAppointmentDraft,
): SchoolAppointmentRequest {
  return {
    title: draft.title.trim(),
    description: draft.description.trim() || undefined,
    location: draft.location.trim() || undefined,
    start_date: draft.startDate,
    end_date: draft.endDate || draft.startDate,
    start_time: draft.allDay ? "00:00" : draft.startTime,
    end_time: draft.allDay ? "23:59" : draft.endTime,
    all_day: draft.allDay,
    school_event_type: draft.eventType,
    parents: draft.parents,
    classes: draft.parents === "classes" ? [...draft.classes] : undefined,
  };
}
