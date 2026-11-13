import { parseISODate } from "./date-helpers";
import type {
  CalendarEvent,
  CalendarResponseStatus,
} from "./personal-calendar-api";

/**
 * Wofür eine Zu- oder Absage gilt (#6987): nur für den geöffneten Termin
 * einer Reihe oder für den ganzen Termin bzw. alle Termine der Reihe.
 */
export type AnswerScope = "occurrence" | "series";

/** Eine gespeicherte Antwort, wie sie die Kalenderansichten übernehmen. */
export interface CalendarAnswer {
  readonly recipientId: string;
  /** Gesetzt, wenn die Antwort nur für diesen Termin der Reihe gilt. */
  readonly occurrenceDate?: string;
  readonly status: Extract<CalendarResponseStatus, "accepted" | "declined">;
}

/**
 * Die vorgewählte Reichweite einer Antwort. Solange die Reihe keine Antwort
 * hat, gilt die erste für alle Termine. Danach ändert eine Antwort nur den
 * geöffneten Termin, damit sie keine Antwort für einen anderen Tag
 * überschreibt.
 */
export function defaultAnswerScope(event: CalendarEvent): AnswerScope {
  if (!event.recurring) return "series";
  const series = event.series_response_status ?? event.response_status;
  return series === "accepted" || series === "declined"
    ? "occurrence"
    : "series";
}

/** Ob dieser Termin einer Reihe eine eigene, abweichende Antwort trägt. */
export function answerDiffersFromSeries(event: CalendarEvent): boolean {
  return (
    event.recurring === true &&
    event.series_response_status !== undefined &&
    event.response_status !== undefined &&
    event.response_status !== event.series_response_status
  );
}

/**
 * Übernimmt eine gespeicherte Antwort in die geladenen Termine. Eine Antwort
 * für einen Termin ändert nur ihn; eine Antwort für die Reihe gilt wieder für
 * jeden Termin derselben Einladung, wie im Backend.
 */
export function applyAnswer<T extends CalendarEvent>(
  events: readonly T[],
  answer: CalendarAnswer,
): T[] {
  return events.map((event) => {
    if (event.recipient_id !== answer.recipientId) return event;
    if (answer.occurrenceDate) {
      return event.occurrence_date === answer.occurrenceDate
        ? { ...event, response_status: answer.status }
        : event;
    }
    return {
      ...event,
      response_status: answer.status,
      ...(event.series_response_status === undefined
        ? {}
        : { series_response_status: answer.status }),
    };
  });
}

/** „Mittwoch, 25.11.“ für Texte über einen Termin einer Reihe. */
export function formatOccurrenceDay(dateISO: string): string {
  return parseISODate(dateISO).toLocaleDateString("de-DE", {
    weekday: "long",
    day: "2-digit",
    month: "2-digit",
  });
}

/**
 * Die Rückmeldung nach einer gespeicherten Antwort im OGS- und im
 * Schulportal. Eine Antwort für einen Termin nennt seinen Tag.
 */
export function answerSavedMessage(
  status: "accepted" | "declined",
  occurrenceDate?: string,
): string {
  const kind = status === "accepted" ? "Zusage" : "Absage";
  return occurrenceDate
    ? `Ihre ${kind} für ${formatOccurrenceDay(occurrenceDate)} ist gespeichert.`
    : `Ihre ${kind} ist gespeichert.`;
}
