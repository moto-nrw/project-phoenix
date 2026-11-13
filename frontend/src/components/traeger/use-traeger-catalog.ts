"use client";

// Anmeldephasen des Trägers (#4704) aus dem Cache des Träger-Portals.

import { useSession } from "next-auth/react";
import { metadataLoadError } from "./metadata-load-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import {
  fetchTraegerPhase,
  fetchTraegerPhases,
} from "~/lib/traeger-catalog-api";

export function useTraegerCatalogPhases() {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.catalogPhases() : null,
    fetchTraegerPhases,
    { keepPreviousData: false },
  );
  const error = useSwrLoadError(
    response.error,
    "die Anmeldephasen",
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

export function useTraegerCatalogPhase(id: string) {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" && id ? traegerKeys.catalogPhase(id) : null,
    () => fetchTraegerPhase(id),
    { keepPreviousData: false },
  );
  const error = useSwrLoadError(
    response.error,
    "die Anmeldephase",
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
