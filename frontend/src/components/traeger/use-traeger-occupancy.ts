"use client";

import { useSession } from "next-auth/react";
import { isRefusal, metadataLoadError } from "./metadata-load-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerOccupancy } from "~/lib/traeger-occupancy-api";

/**
 * The occupancy of every school of the Träger by group and care form (#4676).
 * It changes when a school books or moves a child, so the page reads it on
 * mount and on focus.
 */
export function useTraegerOccupancy() {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.occupancy() : null,
    fetchTraegerOccupancy,
    { revalidateOnFocus: true },
  );
  const error = useSwrLoadError(
    response.error,
    "die Belegung der Schulen",
    response.mutate,
  );
  // A refusal (revoked right, ended session, Träger gone) withdraws the
  // cached answer too: the page head, every tab, the search and the export
  // read only `occupancy`, so none of them can show the old figures.
  const refused = isRefusal(response.error);
  return {
    occupancy: refused ? null : (response.data ?? null),
    // A failed reload keeps confirmed figures above the error; a refusal
    // shows the error alone.
    error: metadataLoadError(
      error,
      response.data !== undefined,
      response.error,
    ),
  };
}
