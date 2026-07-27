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

import { useCallback, useMemo } from "react";
import { useSession } from "next-auth/react";

import { hasPermission } from "~/lib/auth-utils";
import { closingDayService } from "~/lib/closing-day-api";
import {
  expandClosingDaysToMap,
  type ClosingDayRange,
} from "~/lib/closing-day-helpers";
import { timeTrackingService } from "~/lib/time-tracking-api";
import { chunkDateRange } from "~/lib/timetable-helpers";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

/** Shared SWR key of the full closure list — deduped across both plans. */
const CLOSING_DAYS_SWR_KEY = "planning-closing-days";

// Schließtage ändern sich ein paar Mal im Schuljahr. Die globale Voreinstellung
// (5 s) würde bei jedem Wechsel zwischen Betreuungs- und Dienstplan neu laden;
// eine Minute bündelt das, ohne einen frisch angelegten Schließtag für den Rest
// der Sitzung zu verstecken (revalidateIfStale bleibt an).
const CLOSING_DAYS_SWR_OPTIONS = {
  revalidateOnFocus: false,
  dedupingInterval: 60_000,
} as const;

const EMPTY_RANGES: readonly ClosingDayRange[] = [];
const EMPTY_CLOSING_DAYS: ReadonlyMap<string, string> = new Map();
const TIME_TRACKING_CLOSING_DAY_CHUNK_DAYS = 401;

async function getTimeTrackingClosingDays(
  fromISO: string,
  toISO: string,
): Promise<ReadonlyMap<string, string>> {
  const chunks = chunkDateRange(
    fromISO,
    toISO,
    TIME_TRACKING_CLOSING_DAY_CHUNK_DAYS,
  );
  const chunkMaps = await Promise.all(
    chunks.map((chunk) =>
      timeTrackingService.getClosingDays(chunk.from, chunk.to),
    ),
  );
  const combined = new Map<string, string>();
  for (const chunkMap of chunkMaps) {
    for (const [date, reason] of chunkMap) combined.set(date, reason);
  }
  return combined;
}

/** Invalidates the tenant-scoped full closing-day list after CRUD writes. */
export function useInvalidateClosingDays() {
  const tenantMutate = useTenantMutate();
  return useCallback(() => tenantMutate(CLOSING_DAYS_SWR_KEY), [tenantMutate]);
}

/**
 * Alle gespeicherten Schließtag-Zeiträume des Tenants.
 *
 * Für Nutzer mit `schedules:read` werden alle Zeiträume einmal geladen.
 * Reduzierte Dienstplan-Pfade laden stattdessen nur das benötigte Fenster
 * über den Zeiterfassungs-Endpunkt.
 */
function useClosingDayRangesData(canReadSchedules: boolean) {
  const { data, isLoading } = useSWRAuth(
    canReadSchedules ? CLOSING_DAYS_SWR_KEY : null,
    () => closingDayService.list(),
    CLOSING_DAYS_SWR_OPTIONS,
  );

  return { ranges: data ?? EMPTY_RANGES, isLoading };
}

interface ClosingDaysState {
  readonly closingDays: ReadonlyMap<string, string>;
  readonly closingDayRanges: readonly ClosingDayRange[];
  readonly isLoading: boolean;
}

/**
 * Schließtage im Fenster [fromISO, toISO], keyed YYYY-MM-DD → Grund.
 *
 * Ein leeres Fenster (`""`) schaltet den Abruf ab. Wer nur `time_tracking:*`
 * hat (reduzierter Dienstplan-Pfad), bekommt die Tage über den
 * fensterbasierten Zeiterfassungs-Endpunkt. Lange Serienfenster werden passend
 * zu dessen 400-Tage-Grenze gestückelt. Fehler werden bewusst verschluckt: die
 * Markierung ist ein Hinweis, kein Sperrmechanismus — ein fehlgeschlagener
 * Abruf darf die Planung nicht blockieren.
 */
export function useClosingDaysState(
  fromISO: string,
  toISO: string,
): ClosingDaysState {
  const { data: session, status: sessionStatus } = useSession();
  const canReadSchedules = hasPermission(session, "schedules:read");
  const canReadTimeTracking =
    hasPermission(session, "time_tracking:manage") ||
    hasPermission(session, "time_tracking:own");
  const windowValid = fromISO !== "" && toISO !== "";

  const { ranges, isLoading: rangesLoading } =
    useClosingDayRangesData(canReadSchedules);

  const { data: windowMap, isLoading: windowLoading } = useSWRAuth(
    windowValid && !canReadSchedules && canReadTimeTracking
      ? `${CLOSING_DAYS_SWR_KEY}-${fromISO}-${toISO}`
      : null,
    () => getTimeTrackingClosingDays(fromISO, toISO),
    CLOSING_DAYS_SWR_OPTIONS,
  );

  const closingDays = useMemo(() => {
    if (!windowValid) return EMPTY_CLOSING_DAYS;
    if (canReadSchedules) {
      return expandClosingDaysToMap(ranges, fromISO, toISO);
    }
    return windowMap ?? EMPTY_CLOSING_DAYS;
  }, [canReadSchedules, fromISO, ranges, toISO, windowMap, windowValid]);

  return {
    closingDays,
    closingDayRanges: ranges,
    isLoading:
      windowValid &&
      (sessionStatus === "loading" ||
        (canReadSchedules
          ? rangesLoading
          : canReadTimeTracking && windowLoading)),
  };
}
