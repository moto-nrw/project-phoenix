"use client";

import { useSession } from "next-auth/react";
import { metadataLoadError } from "./metadata-load-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerSchool } from "~/lib/traeger-api";

/** The Träger portal has no live channel; the figures refresh by the minute. */
const FIGURES_REFRESH_MS = 60_000;

/** One school with its live figures (#5686): numbers only, no person data. */
export function useTraegerSchool(id: string) {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" && id ? traegerKeys.school(id) : null,
    () => fetchTraegerSchool(id),
    {
      keepPreviousData: false,
      revalidateOnFocus: true,
      refreshInterval: FIGURES_REFRESH_MS,
    },
  );
  const error = useSwrLoadError(response.error, "die Schule", response.mutate);
  return {
    school: response.data ?? null,
    error: metadataLoadError(
      error,
      response.data !== undefined,
      response.error,
    ),
  };
}
