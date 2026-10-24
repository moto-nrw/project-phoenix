// Hausaufgaben und die Rückmeldung der OGS (#4796): die Wörter und Farben
// der drei Antworten, gleich in der Klassenansicht der Lehrkraft und in der
// Lernzeit der OGS.

import { LOCATION_COLORS } from "~/lib/location-helper";

export type HomeworkFeedbackStatus = "done" | "not_done" | "difficult";

export const HOMEWORK_FEEDBACK_STATUSES: readonly HomeworkFeedbackStatus[] = [
  "done",
  "not_done",
  "difficult",
];

/** Die Antwort, wie sie auf dem Knopf und am Kind steht. */
export const HOMEWORK_FEEDBACK_LABELS: Record<HomeworkFeedbackStatus, string> =
  {
    done: "Fertig",
    not_done: "Nicht fertig",
    difficult: "War schwer",
  };

/** Drei Antworten, drei verschiedene Farben. */
export const HOMEWORK_FEEDBACK_COLORS: Record<HomeworkFeedbackStatus, string> =
  {
    done: LOCATION_COLORS.GROUP_ROOM,
    not_done: LOCATION_COLORS.DANGER,
    difficult: LOCATION_COLORS.WARNING,
  };

/** Höchstlängen wie im Backend. */
export const HOMEWORK_SUBJECT_MAX_LENGTH = 60;
export const HOMEWORK_DESCRIPTION_MAX_LENGTH = 2000;
export const HOMEWORK_NOTE_MAX_LENGTH = 200;

/** Eine Hausaufgabe als eine Zeile: „Mathe: Seite 12“ oder nur der Text. */
export function homeworkLine(entry: {
  readonly subject?: string | null;
  readonly description: string;
}): string {
  return entry.subject
    ? `${entry.subject}: ${entry.description}`
    : entry.description;
}
