"use client";

import { useEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import { metadataLoadError } from "./metadata-load-error";
import {
  useConfirmedResource,
  useRefusalGuard,
} from "./use-traeger-confirmed-resource";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerOccupancy } from "~/lib/traeger-occupancy-api";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";

/**
 * The occupancy of every school of the Träger by group and care form (#4676).
 * It changes when a school books or moves a child, so the page reads it on
 * mount, on focus and when the Berlin day changes. A refusal withdraws it
 * everywhere: from the cache, the page head, every tab, the search and the
 * export (`useConfirmedResource`).
 */
export function useTraegerOccupancy() {
  const { data: session } = useSession();
  const today = useBerlinToday();
  const previousDay = useRef(today);
  const key = session?.user.scope === "org" ? traegerKeys.occupancy() : null;
  const guard = useRefusalGuard(key);
  const response = useSWRAuth(key, fetchTraegerOccupancy, {
    revalidateOnFocus: true,
    ...guard.callbacks,
  });
  const { mutate } = response;
  useEffect(() => {
    if (previousDay.current === today) return;
    previousDay.current = today;
    if (key !== null) void mutate();
  }, [today, key, mutate]);
  const occupancy = useConfirmedResource(
    response,
    guard,
    "die Belegung der Schulen",
  );
  return {
    occupancy: occupancy.data,
    // A failed reload keeps confirmed figures above the error; a refusal
    // shows the error alone.
    error: metadataLoadError(
      occupancy.loadError,
      occupancy.data !== null,
      occupancy.failure,
    ),
    reload: occupancy.reload,
    // Related actions (CSV export) can also discover a revoked permission.
    reportError: guard.callbacks.onError,
  };
}
