"use client";

import { useSession } from "next-auth/react";
import { isRefusal, metadataLoadError } from "./metadata-load-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerKeyDateCounts } from "~/lib/traeger-api";

/**
 * The captured Stichtagszahlen of every school of the Träger (#4673). They
 * change once a month, so the page reads them on mount and on focus only.
 */
export function useTraegerKeyDateCounts() {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.keyDateCounts() : null,
    fetchTraegerKeyDateCounts,
    { revalidateOnFocus: true },
  );
  const error = useSwrLoadError(
    response.error,
    "die Stichtagszahlen",
    response.mutate,
  );
  // A refusal withdraws the cached counts too, so the page head cannot keep
  // showing them above the error.
  const refused = isRefusal(response.error);
  return {
    counts: refused ? null : (response.data ?? null),
    error: metadataLoadError(
      error,
      response.data !== undefined,
      response.error,
    ),
  };
}
