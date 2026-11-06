"use client";

import { useSession } from "next-auth/react";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerSchoolsAlerts } from "~/lib/traeger-api";

/** Same rhythm as the figures beside them: the portal has no live channel. */
const ALERTS_REFRESH_MS = 60_000;

/**
 * The hints of every school of the Träger against the Träger's thresholds
 * (#4675): numbers only, no person data.
 */
export function useTraegerSchoolsAlerts() {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.schoolsAlerts() : null,
    fetchTraegerSchoolsAlerts,
    { revalidateOnFocus: true, refreshInterval: ALERTS_REFRESH_MS },
  );
  const error = useSwrLoadError(
    response.error,
    "die Auffälligkeiten der Schulen",
    response.mutate,
  );
  return {
    alerts: response.data ?? null,
    // Error cards only without confirmed hints: a failed reload keeps them.
    error: response.data === undefined ? error : null,
  };
}
