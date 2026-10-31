"use client";

import { useSession } from "next-auth/react";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { fetchTraegerSchools } from "~/lib/traeger-api";

/** Shared school metadata, never school sessions or person data. */
export function useTraegerSchools() {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.schools() : null,
    fetchTraegerSchools,
    { keepPreviousData: false, revalidateIfStale: false },
  );
  const error = useSwrLoadError(
    response.error,
    "die Liste der Schulen",
    response.mutate,
  );
  return { schools: response.data ?? null, error, mutate: response.mutate };
}
