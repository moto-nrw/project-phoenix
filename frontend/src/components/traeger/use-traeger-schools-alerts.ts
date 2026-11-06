"use client";

import { useCallback } from "react";
import { useSession } from "next-auth/react";
import { ApiError } from "~/lib/api-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerSchoolsAlerts } from "~/lib/traeger-api";

/** The Träger portal has no live channel; the figures refresh by the minute. */
const ALERTS_REFRESH_MS = 60_000;

/** A refusal ends the view even with confirmed figures on screen. */
function isRefusal(failure: unknown): boolean {
  return (
    failure instanceof ApiError && [401, 403, 404].includes(failure.status ?? 0)
  );
}

/**
 * The figures of now and the hints of every school of the Träger (#4674,
 * #4675): numbers only, no person data. One request carries both, so the
 * dashboard polls once a minute.
 *
 * `error` is the load error card: only without confirmed figures, or on a
 * refusal. A failed refresh with confirmed figures keeps them and sets
 * `refreshFailed`; `reload` asks again.
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
    "die Zahlen der Schulen",
    response.mutate,
  );
  const { mutate } = response;
  const reload = useCallback(() => void mutate(), [mutate]);
  const confirmed = response.data !== undefined;
  const refused = isRefusal(response.error);
  return {
    alerts: response.data ?? null,
    error: !confirmed || refused ? error : null,
    refreshFailed: confirmed && !refused && response.error !== undefined,
    reload,
  };
}
