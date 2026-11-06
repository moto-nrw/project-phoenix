"use client";

import { useSession } from "next-auth/react";
import { metadataLoadError } from "./metadata-load-error";
import {
  useConfirmedResource,
  useRefusalGuard,
} from "./use-traeger-confirmed-resource";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerOccupancy } from "~/lib/traeger-occupancy-api";

/**
 * The occupancy of every school of the Träger by group and care form (#4676).
 * It changes when a school books or moves a child, so the page reads it on
 * mount and on focus. A refusal withdraws it everywhere: from the cache, the
 * page head, every tab, the search and the export (`useConfirmedResource`).
 */
export function useTraegerOccupancy() {
  const { data: session } = useSession();
  const key = session?.user.scope === "org" ? traegerKeys.occupancy() : null;
  const guard = useRefusalGuard(key);
  const response = useSWRAuth(key, fetchTraegerOccupancy, {
    revalidateOnFocus: true,
    ...guard.callbacks,
  });
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
  };
}
