"use client";

import { useSession } from "next-auth/react";
import {
  useConfirmedResource,
  useRefusalGuard,
} from "./use-traeger-confirmed-resource";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerSchoolsAlerts } from "~/lib/traeger-api";

/** The Träger portal has no live channel; the figures refresh by the minute. */
const ALERTS_REFRESH_MS = 60_000;

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
  const key =
    session?.user.scope === "org" ? traegerKeys.schoolsAlerts() : null;
  const guard = useRefusalGuard(key);
  const response = useSWRAuth(key, fetchTraegerSchoolsAlerts, {
    revalidateOnFocus: true,
    refreshInterval: ALERTS_REFRESH_MS,
    ...guard.callbacks,
  });
  const alerts = useConfirmedResource(
    response,
    guard,
    "die Zahlen der Schulen",
  );
  const confirmed = alerts.data !== null;
  return {
    alerts: alerts.data,
    // A failed reload keeps confirmed figures and says so; a refusal
    // withdraws them (`useConfirmedResource`).
    error: confirmed ? null : alerts.loadError,
    refreshFailed: confirmed && alerts.failure !== undefined,
    reload: alerts.reload,
  };
}
