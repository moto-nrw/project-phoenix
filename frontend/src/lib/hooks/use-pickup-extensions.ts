"use client";

import { useSWRAuth } from "~/lib/swr/hooks";
import {
  fetchPickupExtensions,
  PICKUP_EXTENSIONS_SWR_KEY,
  type PickupExtension,
} from "~/lib/pickup-extension-api";

/**
 * Open later-pickup decisions of the school (#3261). Only for people who may
 * change the Betreuungsplan; everyone else passes enabled=false and gets an
 * empty list without a request.
 */
export function usePickupExtensions(enabled: boolean) {
  const { data, error, isLoading, mutate } = useSWRAuth<PickupExtension[]>(
    enabled ? PICKUP_EXTENSIONS_SWR_KEY : null,
    () => fetchPickupExtensions(),
    { revalidateOnFocus: true },
  );
  return {
    tasks: data ?? [],
    error,
    isLoading: enabled && isLoading,
    refresh: () => mutate(),
  };
}
