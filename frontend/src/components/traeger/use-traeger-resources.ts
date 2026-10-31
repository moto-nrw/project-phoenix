"use client";

import { useMemo } from "react";
import { useSession } from "next-auth/react";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth, useSWRSeedFrom } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import {
  fetchHolidayPeriod,
  fetchHolidayPeriods,
  fetchTraegerSettingsSchema,
} from "~/lib/traeger-api";

export function useTraegerHolidayPeriods() {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.periods() : null,
    fetchHolidayPeriods,
    { keepPreviousData: false },
  );
  const error = useSwrLoadError(
    response.error,
    "die Liste der Ferienzeiträume",
    response.mutate,
  );
  return { ...response, loadError: error };
}

export function useTraegerHolidayPeriod(id: string) {
  const { data: session } = useSession();
  const key = traegerKeys.period(id);
  const response = useSWRAuth(
    session?.user.scope === "org" && id ? key : null,
    () => fetchHolidayPeriod(id),
    { keepPreviousData: false },
  );
  const seedFrom = useSWRSeedFrom();
  const needsPreview = response.data === undefined && !response.error;
  const preview = useMemo(
    () =>
      needsPreview
        ? seedFrom(traegerKeys.periods(), key, (data: unknown) =>
            traegerKeys.periodPreview(data, id),
          )
        : undefined,
    [needsPreview, seedFrom, key, id],
  );
  const error = useSwrLoadError(
    response.error,
    "den Ferienzeitraum",
    response.mutate,
  );
  return { ...response, preview, loadError: error };
}

export function useTraegerSettingsSchema() {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.settings() : null,
    fetchTraegerSettingsSchema,
    { keepPreviousData: false },
  );
  const error = useSwrLoadError(
    response.error,
    "die Liste der Einstellungen",
    response.mutate,
  );
  return { ...response, loadError: error };
}
