"use client";

import { useSession } from "next-auth/react";
import { metadataLoadError } from "./metadata-load-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerSchoolsToday } from "~/lib/traeger-api";

/** The Träger portal has no live channel; the figures refresh by the minute. */
const FIGURES_REFRESH_MS = 60_000;

/**
 * The figures of now for every school of the Träger and their sum (#4674):
 * numbers only, no person data.
 */
export function useTraegerSchoolsToday() {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.schoolsToday() : null,
    fetchTraegerSchoolsToday,
    { revalidateOnFocus: true, refreshInterval: FIGURES_REFRESH_MS },
  );
  const error = useSwrLoadError(
    response.error,
    "die Zahlen der Schulen",
    response.mutate,
  );
  return {
    today: response.data ?? null,
    error: metadataLoadError(
      error,
      response.data !== undefined,
      response.error,
    ),
  };
}
