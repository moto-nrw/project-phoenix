"use client";

import { useSession } from "next-auth/react";
import { metadataLoadError } from "./metadata-load-error";
import {
  useConfirmedResource,
  useRefusalGuard,
} from "./use-traeger-confirmed-resource";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerKeyDateCounts } from "~/lib/traeger-api";

/**
 * The captured Stichtagszahlen of every school of the Träger (#4673). They
 * change once a month, so the page reads them on mount and on focus only.
 * A refusal withdraws them everywhere, also from the page head
 * (`useConfirmedResource`).
 */
export function useTraegerKeyDateCounts() {
  const { data: session } = useSession();
  const key =
    session?.user.scope === "org" ? traegerKeys.keyDateCounts() : null;
  const guard = useRefusalGuard(key);
  const response = useSWRAuth(key, fetchTraegerKeyDateCounts, {
    revalidateOnFocus: true,
    ...guard.callbacks,
  });
  const counts = useConfirmedResource(response, guard, "die Stichtagszahlen");
  return {
    counts: counts.data,
    error: metadataLoadError(
      counts.loadError,
      counts.data !== null,
      counts.failure,
    ),
  };
}
