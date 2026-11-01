"use client";

import { useEffect, useMemo, useRef } from "react";

import { createLogger } from "~/lib/logger";
import type { RosterDeviation } from "~/lib/roster-deviation";
import { useSWRAuth } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";

const logger = createLogger({ component: "useRosterDeviations" });

export function rosterDeviationsKey(periodId: string): string {
  return `roster-deviations-${periodId}`;
}

/**
 * Regeltermine of the period whose children list differs from an offering's
 * bookings (#5103), by template id. The hint is information next to the
 * plan: a failed read shows no hint and is logged, the plan itself stays
 * usable. Every reload of the template list reloads the hints too, so a save,
 * a connection or „Liste so lassen“ shows at once. A deep link uses
 * its own read so cached hints cannot decide its one-time action; that read
 * also reviews the linked Regeltermin when it is kept by hand. `settled`
 * requires a successful read with no revalidation in flight.
 */
export function useRosterDeviations(
  periodId: string | undefined,
  enabled: boolean,
  templatesVersion: unknown,
  freshReadID?: string | null,
): {
  deviations: ReadonlyMap<string, RosterDeviation>;
  settled: boolean;
  error: unknown;
  retry: () => unknown;
} {
  const key = periodId
    ? `${rosterDeviationsKey(periodId)}${freshReadID ? `-connect-${freshReadID}` : ""}`
    : null;
  const { data, error, isValidating, mutate } = useSWRAuth(
    enabled ? key : null,
    () =>
      freshReadID
        ? timetableService.getRosterDeviations(periodId!, freshReadID)
        : timetableService.getRosterDeviations(periodId!),
    {
      keepPreviousData: false,
      ...(freshReadID ? { revalidateOnMount: true, dedupingInterval: 0 } : {}),
    },
  );

  useEffect(() => {
    if (!error) return;
    logger.warn("roster_deviations_load_failed", {
      period_id: periodId,
      error: error instanceof Error ? error.message : String(error),
    });
  }, [error, periodId]);

  const seenVersion = useRef(templatesVersion);
  useEffect(() => {
    if (seenVersion.current === templatesVersion) return;
    const first = seenVersion.current === undefined;
    seenVersion.current = templatesVersion;
    // The first template load runs next to the first hint load already.
    if (!first && templatesVersion !== undefined && periodId) {
      void mutate();
    }
  }, [templatesVersion, periodId, mutate]);

  const deviations = useMemo(
    () => new Map((data ?? []).map((entry) => [entry.templateId, entry])),
    [data],
  );
  return {
    deviations,
    settled: data !== undefined && !error && !isValidating,
    error,
    retry: () => mutate(),
  };
}
