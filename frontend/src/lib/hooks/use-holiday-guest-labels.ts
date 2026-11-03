"use client";

/**
 * Hinweis „Gast aus …“ an Feriengästen in Gruppe, Check-in, Kinderliste und
 * Aufsicht (#5630). Ein Feriengast steht dort mit der Klasse „Feriengast“;
 * der Hinweis ersetzt diese Zeile und nennt seine Heimatschule.
 *
 * Geladen wird nur, wenn die Liste einen Feriengast enthält: Schulen ohne
 * Ferienbetreuung stellen keine zusätzliche Anfrage.
 */

import { useCallback, useMemo } from "react";

import {
  fetchHolidayGuestBadges,
  HOLIDAY_GUEST_SCHOOL_CLASS,
  holidayGuestLabel,
} from "~/lib/holiday-host-api";
import { useSWRAuth } from "~/lib/swr";
import { holidayKeys } from "~/lib/swr/portal-keys";

// Die Heimatschule ändert sich höchstens mit dem stündlichen Abgleich.
const HOLIDAY_GUEST_BADGES_OPTIONS = {
  revalidateOnFocus: false,
  dedupingInterval: 60_000,
} as const;

export interface HolidayGuestCandidate {
  readonly id: string | number;
  readonly school_class?: string | null;
}

/**
 * Gibt für jedes Kind den Text der Klassenzeile zurück: bei einem Feriengast
 * den Hinweis, sonst `undefined` (die Liste zeigt dann die Klasse wie
 * bisher).
 */
export function useHolidayGuestLabels(
  students: readonly HolidayGuestCandidate[],
): (student: HolidayGuestCandidate) => string | undefined {
  const hasGuest = students.some(
    (student) => student.school_class === HOLIDAY_GUEST_SCHOOL_CLASS,
  );
  // Scheitert das Laden (etwa ohne das Recht der Kinderliste), bleibt die
  // Zeile bei „Feriengast“ wie vor #5630: die Liste selbst ist geladen, der
  // Hinweis ergänzt sie nur, eine eigene Fehlermeldung hülfe niemandem.
  const { data } = useSWRAuth(
    hasGuest ? holidayKeys.guestBadges() : null,
    fetchHolidayGuestBadges,
    HOLIDAY_GUEST_BADGES_OPTIONS,
  );
  const schools = useMemo(
    () =>
      new Map(
        (hasGuest && Array.isArray(data) ? data : []).map((badge) => [
          badge.studentId,
          badge.schoolName,
        ]),
      ),
    [hasGuest, data],
  );
  return useCallback(
    (student: HolidayGuestCandidate) =>
      student.school_class === HOLIDAY_GUEST_SCHOOL_CLASS
        ? holidayGuestLabel(schools.get(String(student.id)) ?? "")
        : undefined,
    [schools],
  );
}
