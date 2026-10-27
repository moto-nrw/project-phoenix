"use client";

import { useEffect, useMemo, useRef } from "react";

import { createLogger } from "~/lib/logger";
import type { RosterDeviation } from "~/lib/roster-deviation";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
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
 * a connection or „Bleibt von Hand gepflegt“ shows at once. `settled` turns
 * true once the first read answered, with hints or with an error.
 */
export function useRosterDeviations(
  periodId: string | undefined,
  enabled: boolean,
  templatesVersion: unknown,
): {
  deviations: ReadonlyMap<string, RosterDeviation>;
  settled: boolean;
} {
  const tenantMutate = useTenantMutate();
  const { data, error } = useSWRAuth(
    enabled && periodId ? rosterDeviationsKey(periodId) : null,
    () => timetableService.getRosterDeviations(periodId!),
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
      void tenantMutate(rosterDeviationsKey(periodId));
    }
  }, [templatesVersion, periodId, tenantMutate]);

  const deviations = useMemo(
    () => new Map((data ?? []).map((entry) => [entry.templateId, entry])),
    [data],
  );
  return { deviations, settled: data !== undefined || error !== undefined };
}
