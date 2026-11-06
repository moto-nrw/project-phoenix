"use client";

import { useSession } from "next-auth/react";
import { metadataLoadError } from "./metadata-load-error";
import {
  useConfirmedResource,
  useRefusalGuard,
} from "./use-traeger-confirmed-resource";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerSchool } from "~/lib/traeger-api";

/** The Träger portal has no live channel; the figures refresh by the minute. */
const FIGURES_REFRESH_MS = 60_000;

/**
 * One school with its live figures (#5686): numbers only, no person data. A
 * refusal withdraws them everywhere (`useConfirmedResource`).
 */
export function useTraegerSchool(id: string) {
  const { data: session } = useSession();
  const key =
    session?.user.scope === "org" && id ? traegerKeys.school(id) : null;
  const guard = useRefusalGuard(key);
  const response = useSWRAuth(key, () => fetchTraegerSchool(id), {
    keepPreviousData: false,
    revalidateOnFocus: true,
    refreshInterval: FIGURES_REFRESH_MS,
    ...guard.callbacks,
  });
  const school = useConfirmedResource(response, guard, "die Schule");
  return {
    school: school.data,
    error: metadataLoadError(
      school.loadError,
      school.data !== null,
      school.failure,
    ),
  };
}
