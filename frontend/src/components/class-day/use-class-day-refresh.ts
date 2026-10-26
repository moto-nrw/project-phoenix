"use client";

import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import {
  CLASS_DAY_REFRESH_DEBOUNCE_MS,
  CLASS_DAY_REFRESH_EVENT,
} from "./routes";

/**
 * Lädt eine offene Klassenansicht neu, wenn sich an dieser Schule ein
 * Klassentag geändert hat, etwa durch eine neue Krankmeldung (#4792).
 *
 * Mehrere Änderungen kurz hintereinander laden einmal. Ein verdeckter Tab
 * lädt erst, wenn er wieder sichtbar ist (`marksRead` hält das Ereignis bis
 * dahin zurück): die Übersicht lädt jede Klasse neu, und niemand sieht hin.
 */
export function useClassDayRefresh(onRefresh: () => void): void {
  useMessagesActivity({
    onMatch: onRefresh,
    eventName: CLASS_DAY_REFRESH_EVENT,
    debounceMs: CLASS_DAY_REFRESH_DEBOUNCE_MS,
    marksRead: true,
  });
}
