/**
 * Custom SWR Hooks with Authentication and Tenant Integration
 *
 * These hooks wrap SWR with NextAuth session awareness and scoped cache
 * keys, ensuring requests only fire when the user is authenticated and the
 * cache is isolated per school and account (#5006).
 *
 * Every key starts with the session's scope prefix (scope.ts): inside a
 * TenantProvider the tenant slug and the account
 * (`school-a:42:students-list`), in the school portal its school and
 * account, in the other portals the portal, school or Träger, and account.
 */

"use client";

// swr/no-direct-import permits package imports inside this shared wrapper only.
import useSWR, {
  mutate as swrMutate,
  useSWRConfig,
  type Middleware,
  type SWRConfiguration,
  type SWRResponse,
  type MutatorOptions,
} from "swr";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useSyncExternalStore,
} from "react";
import { useSession } from "next-auth/react";
import { swrConfig, immutableConfig } from "./config";
import {
  currentSWRDataGeneration,
  currentSWRGeneration,
  dataGenerationOf,
  isCurrentSWRError,
  isCurrentSWRValue,
  isDiscardedAnswer,
  isFallbackVisible,
  noteFallbackShown,
  isStillCurrent,
  lockFetcher,
  subscribeSWRDataGeneration,
  lockMutation,
  requestGenerationOf,
  startOf,
  preloadSWR,
} from "./generation";
import {
  authGeneration,
  isSWRSessionPending,
  useSWRSessionRefresh,
  discardScopedEntries,
  invalidateSWRKeys,
  isEntityRefusal,
  scopedSWRKey,
  swrScopePrefix,
} from "./scope";
import type { ErrorCode } from "~/lib/error-codes.generated";
import { useTenantSlugSafe } from "~/lib/tenant-context";

/**
 * The scope of the signed-in session: the prefix of every key the wrappers
 * build (null without access, nothing is fetched) and its auth generation.
 * For keys built outside the wrappers, e.g. a server seed or a key factory.
 */
function useSWRScope(): {
  prefix: string | null;
  generation: string | null;
} {
  const { data: session, status } = useSession();
  const slug = useTenantSlugSafe();
  const { pending } = useSWRSessionRefresh();
  if (pending || status === "loading")
    return { prefix: null, generation: null };
  return {
    prefix: swrScopePrefix(session, slug),
    generation: authGeneration(session),
  };
}

/**
 * The write side of the generation lock (generation.ts) on the merged
 * configuration of every wrapper hook:
 * - the fetcher discards an answer whose generation ended while it ran,
 *   also when SWR hands over a preload or a deduped request;
 * - an error retry scheduled before a bump does not run after it;
 * - a discarded answer is no error: not logged, not retried;
 * - while another tab's session is being confirmed nothing revalidates.
 *   The live flag is read because a timer or mutation can run before React
 *   commits the disabled keys after an owner-change event.
 */
type OnError = NonNullable<SWRConfiguration["onError"]>;
type Compare = NonNullable<SWRConfiguration["compare"]>;
type OnErrorRetry = NonNullable<SWRConfiguration["onErrorRetry"]>;

const generationLockMiddleware: Middleware =
  (useSWRNext) => (key, fetcher, config) => {
    const locked =
      typeof key === "string" && fetcher ? lockFetcher(key, fetcher) : fetcher;
    const onError: OnError = (error, errorKey, current) => {
      if (!isDiscardedAnswer(error)) config.onError?.(error, errorKey, current);
    };
    const onErrorRetry: OnErrorRetry = (
      error,
      errorKey,
      current,
      revalidate,
      options,
    ) => {
      if (isDiscardedAnswer(error)) return;
      const start = startOf(errorKey);
      config.onErrorRetry?.(
        error,
        errorKey,
        current,
        (retry) => {
          if (isStillCurrent(errorKey, start)) return revalidate(retry);
        },
        options,
      );
    };
    // Values of different generations are never equal, in either order:
    // SWR keeps the cached reference on a match when an answer arrives, and
    // keeps its last snapshot on a match when the cache changes. Either
    // would leave the stamp of the old value on screen (hidden).
    const compare: Compare = (a, b) => {
      if (Object.is(a, b)) return true;
      if (
        typeof key === "string" &&
        isCurrentSWRValue(key, a) !== isCurrentSWRValue(key, b)
      ) {
        return false;
      }
      return config.compare?.(a, b) ?? false;
    };
    return useSWRNext(key, locked, {
      ...config,
      compare,
      isPaused: () => isSWRSessionPending() || config.isPaused?.() === true,
      onError,
      onErrorRetry,
    });
  };

/**
 * The value a mutate resolves to, only while it belongs to the key's current
 * generation: after a discarded revalidation SWR resolves to whatever the
 * cache still holds.
 */
function currentResult<V>(
  key: string,
  result: Promise<V>,
): Promise<V | undefined> {
  return Promise.resolve(result).then((value) =>
    isCurrentSWRValue(key, value) === false ? undefined : value,
  );
}

/** A bound mutate of `key` whose writes go through the generation lock. */
function lockBoundMutate<T>(
  key: string | null,
  mutate: ReturnType<typeof useSWRConfig>["mutate"],
  dataGeneration: number,
): SWRResponse<T>["mutate"] {
  return ((...args: unknown[]) => {
    // An action may retain this callback while awaiting an API response.
    // It must not introduce that response into a later data generation.
    if (
      key === null ||
      isSWRSessionPending() ||
      dataGenerationOf(key) !== dataGeneration
    )
      return Promise.resolve(undefined);
    // SWR's bound mutate follows the hook's latest key. A retained callback
    // must write to the same captured key that its generation lock checks.
    // SWR counts the arguments: `mutate()` revalidates, while an explicit
    // `undefined` writes it. Pass revalidations through unchanged.
    if (args[0] === undefined) {
      return currentResult(
        key,
        (mutate as (...given: unknown[]) => Promise<T | undefined>)(
          key,
          ...args,
        ),
      );
    }
    const [locked, lockedOptions] = lockMutation(key, args[0], args[1]);
    return currentResult(
      key,
      mutate(key, locked as never, lockedOptions as never),
    );
  }) as SWRResponse<T>["mutate"];
}

/**
 * useSWR under the session's scope and the generation lock (#5006).
 *
 * Read side: `data` (cache, `fallbackData`, a seed) is shown only while it
 * belongs to the key's current data generation; otherwise the hook reports
 * no data, and loads again once if no error explains the gap. An answer the
 * lock discarded is no error. A hook whose request was discarded loads
 * again under the new generation. Without a bump nothing loads again.
 *
 * `keepPreviousData` keeps an earlier key's data on screen only while no
 * generation moved since it arrived: a new key after an account switch, a
 * logout or a discard starts empty. The marker moves once data of the
 * current key arrived.
 */
function useScopedSWR<T, E>(
  key: string | null,
  fetcher: () => Promise<T>,
  options: SWRConfiguration<T, E> | undefined,
): SWRResponse<T, E> {
  const { prefix, generation } = useSWRScope();
  const { mutate: cacheMutate } = useSWRConfig();
  const fullKey =
    key !== null && prefix !== null ? scopedSWRKey(prefix, key) : null;
  // Renders again whenever values stop counting (scope or data bump).
  const moved = useSyncExternalStore(
    subscribeSWRDataGeneration,
    currentSWRDataGeneration,
    currentSWRDataGeneration,
  );
  const shown = useRef({ generation, moved });
  const configured = options?.keepPreviousData ?? swrConfig.keepPreviousData;
  const keep =
    configured === true &&
    shown.current.generation === generation &&
    shown.current.moved === moved;

  // The public fetcher contract has no SWR key argument. In particular a
  // factory's optional params must not receive the full scoped cache key.
  const response = useSWR<T, E>(fullKey, () => fetcher(), {
    ...swrConfig,
    ...options,
    keepPreviousData: keep,
    use: [generationLockMiddleware, ...(options?.use ?? [])],
  });

  const raw = response.data;
  const rawError = response.error;
  const discarded = isDiscardedAnswer(rawError);
  // The value this hook showed last: SWR's previous data under
  // `keepPreviousData`, which belongs to an earlier key.
  const lastShown = useRef<unknown>(undefined);
  // Every read path, against "nothing of an older generation is visible":
  // - a value ~/lib/swr produced (fetch, preload, mutation, seed) by its
  //   stamp;
  // - previous data of an earlier key only under the keep guard above;
  // - anything else (`fallbackData`, a provider fallback, a raw write) by
  //   the key's fallback state, which a drop ends until fresh data came;
  // - without a session nothing, not even `fallbackData`; a key the caller
  //   left out keeps SWR's own behaviour under the keep guard;
  // - a failure by its stamp; a discarded answer is no failure.
  let visible: boolean;
  let fallbackShown = false;
  if (fullKey === null) {
    visible = prefix !== null;
  } else {
    const stamped = isCurrentSWRValue(fullKey, raw);
    if (stamped !== undefined) {
      visible = stamped;
    } else if (keep && raw === lastShown.current) {
      visible = true;
    } else {
      visible = isFallbackVisible(fullKey);
      fallbackShown = visible;
    }
  }
  const fallbackGeneration = fullKey ? dataGenerationOf(fullKey) : 0;
  useLayoutEffect(() => {
    if (fallbackShown && fullKey)
      noteFallbackShown(fullKey, fallbackGeneration);
  }, [fallbackShown, fullKey, fallbackGeneration]);
  const data = visible ? raw : undefined;
  useLayoutEffect(() => {
    if (data !== undefined) lastShown.current = data;
  }, [data]);
  const errorVisible =
    !discarded && (fullKey === null || isCurrentSWRError(fullKey, rawError));
  const error = errorVisible ? rawError : undefined;

  const arrived = configured === true && !keep && data !== undefined;
  useLayoutEffect(() => {
    if (arrived) shown.current = { generation, moved };
  }, [arrived, generation, moved]);

  // Load again once per generation: after a discarded answer, or when the
  // shown data belongs to an older generation and no error explains it.
  const reload =
    fullKey === null
      ? null
      : discarded
        ? `request:${fullKey}:${requestGenerationOf(fullKey)}`
        : !visible && error === undefined
          ? `data:${fullKey}:${dataGenerationOf(fullKey)}`
          : null;
  const reloaded = useRef<string | null>(null);
  const revalidate = response.mutate;
  useEffect(() => {
    if (reload === null || reloaded.current === reload) return;
    reloaded.current = reload;
    void revalidate();
  }, [reload, revalidate]);

  const mutationGeneration = fullKey === null ? 0 : dataGenerationOf(fullKey);
  const mutate = useMemo(
    () => lockBoundMutate<T>(fullKey, cacheMutate, mutationGeneration),
    [fullKey, cacheMutate, mutationGeneration],
  );
  return {
    data,
    error,
    mutate,
    get isValidating() {
      return response.isValidating;
    },
    get isLoading() {
      return response.isLoading || (!visible && response.isValidating);
    },
  } as SWRResponse<T, E>;
}

/**
 * SWR hook with authentication and tenant cache isolation.
 *
 * Only fetches data while the session grants access (hasSessionAccess).
 * Cache keys are automatically prefixed with the session's scope (school and
 * account, see scope.ts), to prevent data from one session appearing in
 * another.
 *
 * @example
 * ```tsx
 * const { data, isLoading, error } = useSWRAuth(
 *   'students-list',
 *   () => studentService.getStudents()
 * );
 * ```
 *
 * @param key - Unique cache key for this data (use null to disable fetching)
 * @param fetcher - Async function that returns the data
 * @param options - Optional SWR configuration overrides
 */
export function useSWRAuth<T, E = Error>(
  key: string | null,
  fetcher: () => Promise<T>,
  options?: SWRConfiguration<T, E>,
): SWRResponse<T, E> {
  // Fetches only while the session grants access (hasSessionAccess) and, in
  // the school portal, carries its school: swrScopePrefix is null otherwise.
  return useScopedSWR<T, E>(key, fetcher, options);
}

/**
 * The cache key useSWRAuth reads for `key` under the session's scope; null
 * while the session grants no access. For state the readers of one key
 * share, e.g. a single refresh for all of them.
 */
export function useScopedSWRKey(key: string): string | null {
  const { prefix } = useSWRScope();
  return prefix === null ? null : scopedSWRKey(prefix, key);
}

/**
 * SWR hook for immutable/static data that rarely changes.
 *
 * Disables automatic revalidation to minimize unnecessary requests.
 * Perfect for: roles, permissions, categories, configuration data.
 *
 * @example
 * ```tsx
 * const { data: roles } = useImmutableSWR(
 *   'roles',
 *   () => authService.getRoles()
 * );
 * ```
 *
 * @param key - Unique cache key for this data (use null to disable fetching)
 * @param fetcher - Async function that returns the data
 */
export function useImmutableSWR<T, E = Error>(
  key: string | null,
  fetcher: () => Promise<T>,
): SWRResponse<T, E> {
  return useSWRAuth<T, E>(key, fetcher, immutableConfig);
}

/**
 * Hook that returns a tenant-aware mutate function.
 *
 * SWR cache keys are prefixed with the session's scope by useSWRAuth, so
 * plain `mutate("my-key")` misses the cache. This hook returns a function
 * that applies the same prefix before calling SWR mutate. A function kept
 * from an earlier account or school only reaches that scope's keys: a late
 * write or rollback never lands in the cache of the session after it.
 * Without a signed-in session there is no scope and nothing to mutate.
 *
 * @example
 * ```tsx
 * const tenantMutate = useTenantMutate();
 * await tenantMutate("database-teachers-list");
 * ```
 */
export function useTenantMutate() {
  const { prefix } = useSWRScope();
  useSyncExternalStore(
    subscribeSWRDataGeneration,
    currentSWRDataGeneration,
    currentSWRDataGeneration,
  );
  const originatingGeneration = currentSWRGeneration();

  return useCallback(
    (
      key: string,
      data?: unknown,
      options?: MutatorOptions,
    ): Promise<unknown> => {
      if (prefix === null || isSWRSessionPending())
        return Promise.resolve(undefined);
      const cacheKey = scopedSWRKey(prefix, key);
      // A retained action or passive effect must not restamp captured data
      // after a layout-effect reset. Only this key's data bumps count, so
      // an unrelated drop (including `except`) leaves the callback usable.
      if (dataGenerationOf(cacheKey) > originatingGeneration)
        return Promise.resolve(undefined);
      if (data === undefined) {
        return currentResult(cacheKey, swrMutate(cacheKey));
      }
      // A write that settles after the generation moved is not kept.
      const [locked, lockedOptions] = lockMutation(cacheKey, data, options);
      return currentResult(
        cacheKey,
        swrMutate(cacheKey, locked, lockedOptions as never),
      );
    },
    [prefix, originatingGeneration],
  );
}

/**
 * Hook that invalidates every tenant-scoped SWR cache entry whose key matches
 * a substring. Useful for cross-cutting changes that affect data displayed by
 * multiple unrelated pages — e.g. updating a room's color must refresh every
 * visit/student list that stamps `current_room_color` into its rows.
 *
 * Only keys of the session's scope (school and account) take part; caches
 * of another school or account (multi-tab) stay put, and without a
 * signed-in session nothing matches. Substrings should include enough
 * context to avoid over-invalidation. Pass an array of
 * substrings to match if any of them appears.
 *
 * **Picking substrings**: prefer fragments that are unlikely to collide with
 * future cache keys. `"ogs-students"` is safer than `"students"` because the
 * latter would also match `students-search`, `students-import`, etc., once
 * those caches exist. Substring match is a forward-compatibility hazard — a
 * new SWR consumer added later may accidentally land in your invalidation
 * net without anyone noticing. Err on the side of longer, more specific
 * fragments and revisit this list whenever a new cache key is introduced
 * that contains one of your substrings.
 *
 * **Convention**: existing SWR keys in this codebase put a trailing dash
 * before their first dynamic segment (`ogs-students-${groupId}`,
 * `active-supervision-dashboard-${refreshKey}`). Including that trailing
 * dash in your substring (`"ogs-students-"` instead of `"ogs-students"`)
 * narrows the match window and makes accidental collisions with future
 * keys that only happen to start with the same prefix less likely.
 *
 * **Stability**: Callers can pass a literal array each render — the hook
 * derives a stable string key internally from the joined substrings, so the
 * returned callback identity stays referentially stable across renders.
 * This avoids cascading useCallback invalidations in consumers that depend
 * on the returned function.
 *
 * **What is touched**: every place SWR keeps a matching key
 * (invalidateSWRKeys): cache entries also without a mounted reader,
 * `$inf$`/`$sub$` entries, requests in flight and inside the dedupe window,
 * preloads and mutations (#5006). A plain call revalidates: mounted readers
 * load again, keep their data until the answer, and no later reader reuses a
 * request or preload from before.
 *
 * **Clearing instead of revalidating**: pass `{ clear: true }` to DELETE the
 * matching entries without refetching. Data, error and loading state go,
 * requests and mutations in flight write nothing back, and data kept on
 * screen goes too — for consumers that must never act on pre-mutation data.
 * The next mount loads fresh with `isLoading: true`.
 *
 * **Leaving keys out**: `except` names unprefixed keys that are not touched
 * in any of these places, e.g. the list a decision was just made in, which
 * already shows its result (#4988).
 *
 * @example
 * ```tsx
 * const refreshRoomConsumers = useTenantMutateMatching([
 *   "active-supervision",
 *   "supervision-visits",
 *   "ogs-students",
 *   "search-students",
 * ]);
 * await refreshRoomConsumers(); // every cache key containing one of those substrings refetches
 * ```
 */
export function useTenantMutateMatching(substrings: readonly string[]) {
  const { prefix } = useSWRScope();
  const { cache } = useSWRConfig();
  // "|" is a safe joiner: substrings are caller-supplied cache-key fragments
  // (literals like "ogs-students"), which cannot legally contain pipe in any
  // SWR key the codebase produces. The joined string is the actual dep — a
  // fresh array literal with identical contents reuses the memoized callback.
  const joined = substrings.join("|");

  return useCallback(
    (options?: { clear?: boolean; except?: readonly string[] }) => {
      if (prefix === null || isSWRSessionPending()) return Promise.resolve([]);
      const needles = joined.length > 0 ? joined.split("|") : [];
      const kept = new Set(
        (options?.except ?? []).map((key) => scopedSWRKey(prefix, key)),
      );
      const scope = scopedSWRKey(prefix, "");
      // Every place SWR keeps a matching key, not only entries a mounted
      // hook keyed (#5006): a request or preload from before the change
      // must not answer the next reader. `except` keys stay untouched.
      return invalidateSWRKeys(
        cache,
        (key) =>
          !kept.has(key) &&
          key.startsWith(scope) &&
          needles.some((needle) => key.includes(needle)),
        options?.clear ? "clear" : "revalidate",
      );
    },
    [prefix, joined, cache],
  );
}

/**
 * SWR hook for data that depends on a parameter.
 *
 * Automatically generates a cache key that includes the parameter,
 * ensuring proper cache isolation per entity and per tenant.
 *
 * @example
 * ```tsx
 * const { data: student } = useSWRWithId(
 *   'student',
 *   studentId,
 *   (id) => studentService.getStudent(id)
 * );
 * ```
 *
 * @param baseKey - Base cache key prefix
 * @param id - Entity ID (use null/undefined to disable fetching)
 * @param fetcher - Async function that takes the ID and returns data
 * @param options - Optional SWR configuration overrides
 */
export function useSWRWithId<T, E = Error>(
  baseKey: string,
  id: string | null | undefined,
  fetcher: (id: string) => Promise<T>,
  options?: SWRConfiguration<T, E>,
): SWRResponse<T, E> {
  return useScopedSWR<T, E>(
    id == null ? null : `${baseKey}-${id}`,
    () => fetcher(id!),
    options,
  );
}

/**
 * Returns a function that drops every cache entry of one child or other
 * entity after the server refused it (401, 403, 404 with one of `codes`), so
 * no view keeps showing it: e.g. a guardian whose access to a child ended.
 * `matches` gets each unscoped key of the session's scope. Entries keep
 * their error and are not fetched again; data kept on screen goes too.
 * Returns whether the error was such a refusal.
 *
 * @example
 * ```tsx
 * const discard = useDiscardRefusedEntity();
 * if (discard(error, {
 *   codes: ["general.not_found", "general.permission"],
 *   matches: (key) => key.includes(`student-${studentId}`),
 * })) return <LoadErrorAlert … />;
 * ```
 */
export function useDiscardRefusedEntity() {
  const { prefix } = useSWRScope();
  const { cache } = useSWRConfig();

  return useCallback(
    (
      error: unknown,
      entity: {
        codes: readonly ErrorCode[];
        matches: (key: string) => boolean;
      },
    ): boolean => {
      if (!isEntityRefusal(error, entity.codes)) return false;
      if (prefix !== null) {
        discardScopedEntries(cache, prefix, entity.matches, error);
      }
      return true;
    },
    [prefix, cache],
  );
}

/** Resolve a factory key for cache reads/seeds without duplicating session scoping. */
export function useSWRCacheKey() {
  const { prefix } = useSWRScope();
  return useCallback(
    (key: string) => (prefix === null ? null : scopedSWRKey(prefix, key)),
    [prefix],
  );
}

/** Hover/intent preload, held to the same session and generation as mounted readers. */
export function useSWRPreload() {
  const { prefix } = useSWRScope();
  useSyncExternalStore(
    subscribeSWRDataGeneration,
    currentSWRDataGeneration,
    currentSWRDataGeneration,
  );
  const originatingGeneration = currentSWRGeneration();
  return useCallback(
    <T>(key: string, fetcher: () => Promise<T>): Promise<T | undefined> => {
      if (prefix === null || isSWRSessionPending())
        return Promise.resolve(undefined);
      const fullKey = scopedSWRKey(prefix, key);
      if (dataGenerationOf(fullKey) > originatingGeneration)
        return Promise.resolve(undefined);
      const start = startOf(fullKey);
      return preloadSWR(fullKey, fetcher).then(
        (value) =>
          isStillCurrent(fullKey, start) &&
          isCurrentSWRValue(fullKey, value) !== false
            ? value
            : undefined,
        (error: unknown) => {
          if (
            isDiscardedAnswer(error) ||
            !isStillCurrent(fullKey, start) ||
            !isCurrentSWRError(fullKey, error)
          )
            return undefined;
          throw error;
        },
      );
    },
    [prefix, originatingGeneration],
  );
}

/** Invalidate a resource family, including unmounted, infinite and preloaded keys. */
export function useInvalidateFamily() {
  const { prefix } = useSWRScope();
  const { cache } = useSWRConfig();
  return useCallback(
    (family: string, options?: { clear?: boolean }) => {
      if (prefix === null || isSWRSessionPending()) return Promise.resolve([]);
      const start = scopedSWRKey(prefix, family);
      return invalidateSWRKeys(
        cache,
        (key) => key.startsWith(start),
        options?.clear ? "clear" : "revalidate",
      );
    },
    [prefix, cache],
  );
}
