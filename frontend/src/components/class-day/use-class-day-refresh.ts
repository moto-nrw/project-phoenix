"use client";

import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";

/**
 * Fensterereignis, mit dem die SSE-Verbindung des Schul-Portals eine offene
 * Klassenansicht neu laden lässt (#4792), etwa nach einer neuen Krankmeldung.
 */
export const CLASS_DAY_REFRESH_EVENT = "class-day-refresh";

/**
 * Mehrere Änderungen kurz hintereinander (etwa mehrere Krankmeldungen am
 * Morgen) laden die Ansicht nur einmal neu.
 */
export const CLASS_DAY_REFRESH_DEBOUNCE_MS = 1000;

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
