"use client";

import { useSession } from "next-auth/react";
import { metadataLoadError } from "./metadata-load-error";
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
  return {
    occupancy: response.data ?? null,
    // A failed reload keeps confirmed figures; a refusal (revoked right,
    // school gone) hides them and shows the error instead.
    error: metadataLoadError(
      error,
      response.data !== undefined,
      response.error,
    ),
  };
}
