"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError } from "~/lib/api-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import type { SWRResponse } from "~/lib/swr";

/**
 * Whether a failure is a refusal (401, 403, 404): the session, the right or
 * the object is gone, so nothing loaded before may be shown again.
 */
export function isRefusal(failure: unknown): boolean {
  return (
    failure instanceof ApiError && [401, 403, 404].includes(failure.status ?? 0)
  );
}

/*
 * The one implementation of what every Träger read shows after a failure
 * (#4676). A hook builds its key in ~/lib/swr, then:
 *
 *   const key = session ? traegerKeys.x() : null;
 *   const guard = useRefusalGuard(key);
 *   const response = useSWRAuth(key, fetcher, { ...options, ...guard.callbacks });
 *   const x = useConfirmedResource(response, guard, "die …");
 *
 * Invariants:
 * 1. A refusal (401, 403, 404) removes the key's data from the SWR cache,
 *    not only from the screen, so no other reader of the key and no later
 *    render shows it.
 * 2. After a refusal, data shows again only after a successful fetch. A later
 *    5xx or network failure never brings refused data back, and neither does
 *    another view writing the old answer into the cache; the refusal stays the
 *    shown failure until then.
 * 3. A transient failure without an earlier refusal keeps confirmed data;
 *    `failure` is set, so the view shows a hint with „Wiederholen“.
 */

/** The refusal in force for one key, cleared only by a successful fetch. */
export interface RefusalGuard {
  readonly key: string | null;
  readonly refusal: unknown;
  /** Pass to the reader: they record refusals and successes of the fetch. */
  readonly callbacks: {
    readonly onSuccess: () => void;
    readonly onError: (error: unknown) => void;
  };
}

export function useRefusalGuard(key: string | null): RefusalGuard {
  // SWR's own error is cleared by the cache removal and replaced by any later
  // failure, so the guard keeps the refusal itself, per key.
  const [state, setState] = useState<{ key: string; error: unknown } | null>(
    null,
  );
  const callbacks = useMemo(
    () => ({
      onSuccess: () =>
        setState((current) => (current?.key === key ? null : current)),
      onError: (error: unknown) => {
        if (key !== null && isRefusal(error)) setState({ key, error });
      },
    }),
    [key],
  );
  return {
    key,
    refusal: key !== null && state?.key === key ? state.error : undefined,
    callbacks,
  };
}

export interface ConfirmedResource<T> {
  /** Confirmed data, never data a refusal withdrew. */
  readonly data: T | null;
  /** The failure to show: the refusal in force, else SWR's error. */
  readonly failure: unknown;
  /** The failure as the error card of the load path, with retry. */
  readonly loadError: ReturnType<typeof useSwrLoadError>;
  /** A refusal is in force: no data until a fetch succeeds. */
  readonly refused: boolean;
  readonly reload: () => void;
}

export function useConfirmedResource<T>(
  response: Pick<SWRResponse<T>, "data" | "error" | "mutate">,
  guard: RefusalGuard,
  object: string,
): ConfirmedResource<T> {
  const { mutate } = response;
  const refusedNow = isRefusal(response.error);
  const refused = refusedNow || guard.refusal !== undefined;
  const cached = response.data !== undefined;

  // Invariant 1: drop the refused answer from the cache itself.
  useEffect(() => {
    if (refused && cached) void mutate(undefined, { revalidate: false });
  }, [refused, cached, mutate]);

  // Invariant 2: while a refusal is in force it stays the shown failure.
  const failure = refusedNow
    ? response.error
    : refused
      ? guard.refusal
      : response.error;
  const loadError = useSwrLoadError(failure, object, mutate);
  const reload = useCallback(() => void mutate(), [mutate]);

  return {
    data: refused ? null : (response.data ?? null),
    failure,
    loadError,
    refused,
    reload,
  };
}
