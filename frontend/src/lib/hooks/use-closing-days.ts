"use client";

/**
 * OGS-Schließtage für die Planungsraster (#2032).
 *
 * Betreuungsplan und Dienstplan markieren Schließtage im Raster und warnen,
 * bevor ein Termin oder eine Schicht darauf gelegt wird. Beide Bereiche
 * brauchen dafür dieselbe Tag→Grund-Zuordnung, deshalb liegt sie hier.
 *
 * Datenquelle ist `/api/timetable/closing-days` (schedules:read). Der
 * Endpunkt liefert ALLE gespeicherten Zeiträume, unabhängig vom sichtbaren
 * Fenster — ein einziger SWR-Key genügt also für jede Woche, jeden Monat und
 * jedes Halbjahr, und ein Wochenwechsel löst keinen neuen Request aus.
 */

import { useMemo } from "react";
import { useSession } from "next-auth/react";

import { hasPermission } from "~/lib/auth-utils";
import { closingDayService } from "~/lib/closing-day-api";
import {
  expandClosingDaysToMap,
  type ClosingDayRange,
} from "~/lib/closing-day-helpers";
import { timeTrackingService } from "~/lib/time-tracking-api";
import { useSWRAuth } from "~/lib/swr";

/** Shared SWR key of the full closure list — deduped across both plans. */
const CLOSING_DAYS_SWR_KEY = "planning-closing-days";

const EMPTY_RANGES: readonly ClosingDayRange[] = [];
const EMPTY_CLOSING_DAYS: ReadonlyMap<string, string> = new Map();

/**
 * Alle gespeicherten Schließtag-Zeiträume des Tenants.
 *
 * Für Nachschlagen an einem BELIEBIGEN Datum (z. B. dem im Dialog gewählten
 * Zieltag), das außerhalb des sichtbaren Fensters liegen kann. Ohne
 * `schedules:read` bleibt die Liste leer — dann entfällt die Warnung, geplant
 * werden kann weiterhin.
 */
export function useClosingDayRanges(
  enabled = true,
): readonly ClosingDayRange[] {
  const { data: session } = useSession();
  const canReadSchedules = hasPermission(session, "schedules:read");

  const { data } = useSWRAuth(
    enabled && canReadSchedules ? CLOSING_DAYS_SWR_KEY : null,
    () => closingDayService.list(),
    { revalidateOnFocus: false },
  );

  return data ?? EMPTY_RANGES;
}

/**
 * Schließtage im Fenster [fromISO, toISO], keyed YYYY-MM-DD → Grund.
 *
 * Ein leeres Fenster (`""`) oder `enabled: false` schaltet den Abruf ab und
 * liefert eine leere Map. Wer nur `time_tracking:*` hat (reduzierter
 * Dienstplan-Pfad), bekommt die Tage über den fensterbasierten
 * Zeiterfassungs-Endpunkt. Fehler werden bewusst verschluckt: die Markierung
 * ist ein Hinweis, kein Sperrmechanismus — ein fehlgeschlagener Abruf darf
 * die Planung nicht blockieren.
 */
export function useClosingDays(
  fromISO: string,
  toISO: string,
  enabled = true,
): ReadonlyMap<string, string> {
  const { data: session } = useSession();
  const canReadSchedules = hasPermission(session, "schedules:read");
  const canReadTimeTracking =
    hasPermission(session, "time_tracking:manage") ||
    hasPermission(session, "time_tracking:own");
  const windowValid = fromISO !== "" && toISO !== "";

  const ranges = useClosingDayRanges(enabled && windowValid);

  const { data: windowMap } = useSWRAuth(
    enabled && windowValid && !canReadSchedules && canReadTimeTracking
      ? `${CLOSING_DAYS_SWR_KEY}-${fromISO}-${toISO}`
      : null,
    () => timeTrackingService.getClosingDays(fromISO, toISO),
    { revalidateOnFocus: false },
  );

  return useMemo(() => {
    if (!enabled || !windowValid) return EMPTY_CLOSING_DAYS;
    if (canReadSchedules) {
      return expandClosingDaysToMap(ranges, fromISO, toISO);
    }
    return windowMap ?? EMPTY_CLOSING_DAYS;
  }, [
    canReadSchedules,
    enabled,
    fromISO,
    ranges,
    toISO,
    windowMap,
    windowValid,
  ]);
}
