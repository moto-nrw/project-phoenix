"use client";

import { useMemo } from "react";
import { useSession } from "next-auth/react";
import { metadataLoadError } from "./metadata-load-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth, useSWRSeedFrom } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import {
  fetchHolidayPeriod,
  fetchHolidayPeriods,
  fetchTraegerSettingsSchema,
} from "~/lib/traeger-api";
import { fetchHolidayCriteria } from "~/lib/traeger-holiday-allocation-api";

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
  return {
    ...response,
    loadError: metadataLoadError(
      error,
      response.data !== undefined,
      response.error,
    ),
  };
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
  return {
    ...response,
    preview,
    loadError: metadataLoadError(
      error,
      response.data !== undefined,
      response.error,
    ),
  };
}

/** The Träger's criteria of the allocation (#4700), highest priority first. */
export function useTraegerHolidayCriteria() {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.holidayCriteria() : null,
    fetchHolidayCriteria,
    { keepPreviousData: false },
  );
  const error = useSwrLoadError(
    response.error,
    "die Kriterien der Platzvergabe",
    response.mutate,
  );
  return {
    ...response,
    loadError: metadataLoadError(
      error,
      response.data !== undefined,
      response.error,
    ),
  };
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
  return {
    ...response,
    loadError: metadataLoadError(
      error,
      response.data !== undefined,
      response.error,
    ),
  };
}
