"use client";

/**
 * Hinweis im Schließtag-Dialog der Schule (#6434): Liegt der Schließtag in
 * einer Ferienwoche, die die Schule für ihren Träger ausrichtet, nennt der
 * Dialog die Woche und wie viele Kinder dort einen Platz haben. Die Eltern
 * kennt nur der Träger; er sieht den Schließtag danach unter „Schließtage“
 * und informiert sie. Die Zahlen kommen aus den Teilnehmerlisten (ADR 0116).
 */

import { useEffect, useState } from "react";

import type { ClosingDayDraft } from "~/components/planning/closing-day-modal";
import { ApiError } from "~/lib/api-error";
import {
  fetchHostedHolidayWeeks,
  type HostedHolidayWeek,
} from "~/lib/holiday-host-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ClosingDayHolidayHint" });

/** Die ausgerichteten Wochen mit Kindern, die den Zeitraum berühren. */
export function hostedWeeksInRange(
  weeks: readonly HostedHolidayWeek[],
  draft: ClosingDayDraft | null,
): HostedHolidayWeek[] {
  if (!draft?.startDate || !draft.endDate || draft.endDate < draft.startDate) {
    return [];
  }
  return weeks.filter(
    (week) =>
      week.participants > 0 &&
      week.startsOn <= draft.endDate &&
      week.endsOn >= draft.startDate,
  );
}

export function useHostedHolidayWeeks(open: boolean) {
  const [weeks, setWeeks] = useState<readonly HostedHolidayWeek[]>([]);
  useEffect(() => {
    if (!open) return;
    let current = true;
    fetchHostedHolidayWeeks()
      .then((loaded) => {
        if (current) setWeeks(loaded);
      })
      .catch((err: unknown) => {
        // Bewusst still: Der Hinweis ergänzt den Dialog nur. Ohne Recht auf
        // die Teilnehmerlisten oder bei einem Fehler bleibt das Speichern
        // möglich, und der Träger sieht den Schließtag trotzdem.
        if (
          err instanceof ApiError &&
          (err.status === 401 || err.status === 403)
        ) {
          return;
        }
        logger.warn("closing_day_hosted_weeks_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      current = false;
    };
  }, [open]);
  return weeks;
}
