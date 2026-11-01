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
  isVisibleSWRValue,
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
 * The value a revalidation or clear resolves to, only while it belongs to
 * the key's current generation: after a discarded revalidation SWR resolves
 * to whatever the cache still holds.
 */
function currentResult<V>(
  key: string,
  result: Promise<V>,
): Promise<V | undefined> {
  return Promise.resolve(result).then((value) =>
    isVisibleSWRValue(key, value) ? value : undefined,
  );
}

/**
 * The result of a data mutation, only while its own run still counts (G25):
 * judged by the run's start token, never by the result equalling a value
 * stamped since. A run ended by a scope or data bump resolves to undefined.
 */
function runResult<V>(
  isCurrent: () => boolean,
  result: Promise<V>,
): Promise<V | undefined> {
  return Promise.resolve(result).then((value) =>
    isCurrent() ? value : undefined,
  );
}

/**
 * The options a reader of ~/lib/swr takes (#5007), an allowlist in type and
 * at runtime. Each is checked against the generation lock:
 * - timing only, every load still runs the locked fetcher:
 *   `dedupingInterval`, `focusThrottleInterval`, `refreshInterval`,
 *   `revalidateIfStale`, `revalidateOnFocus`, `revalidateOnMount`,
 *   `revalidateOnReconnect`;
 * - retries, which the lock drops after a generation moved:
 *   `errorRetryCount`, `shouldRetryOnError`;
 * - read-side values the lock judges: `fallbackData` (shown only under the
 *   fallback rules), `keepPreviousData` (only while no generation moved);
 * - `isPaused`, which can only add pauses to the session pause;
 * - `onSuccess` and `onError`, called with the value or failure alone,
 *   never with SWR's configuration and its cache or mutate.
 * Left out on purpose: `use` (a middleware gets raw useSWR and its mutate),
 * `provider`, `fallback`, `fetcher`, `compare`, `onErrorRetry` and every
 * other key of SWRConfiguration.
 */
export const READER_OPTION_KEYS = [
  "dedupingInterval",
  "errorRetryCount",
  "fallbackData",
  "focusThrottleInterval",
  "isPaused",
  "keepPreviousData",
  "onError",
  "onSuccess",
  "refreshInterval",
  "revalidateIfStale",
  "revalidateOnFocus",
  "revalidateOnMount",
  "revalidateOnReconnect",
  "shouldRetryOnError",
] as const satisfies readonly (keyof SWRConfiguration)[];

export type ReaderOptions<T, E = Error> = Pick<
  SWRConfiguration<T, E>,
  Exclude<(typeof READER_OPTION_KEYS)[number], "onSuccess" | "onError">
> & {
  onSuccess?: (data: T) => void;
  onError?: (error: E) => void;
};

/**
 * The options of a mutation through ~/lib/swr. lockMutation stamps
 * `optimisticData` and guards `populateCache`; the others decide only
 * whether to revalidate, roll back or throw.
 */
export const MUTATION_OPTION_KEYS = [
  "optimisticData",
  "populateCache",
  "revalidate",
  "rollbackOnError",
  "throwOnError",
] as const satisfies readonly (keyof MutatorOptions)[];

const allowedReaderKeys = new Set<string>(READER_OPTION_KEYS);
const allowedMutationKeys = new Set<string>(MUTATION_OPTION_KEYS);
// A clear writes nothing but `undefined`: it may revalidate and report a
// failed revalidation, never populate or show optimistic data.
const clearOptionKeys = new Set<string>(["revalidate", "throwOnError"]);

/**
 * Copies the allowed keys of `options`; any other key throws outside
 * production and is dropped in production. Copied, never spread: a key the
 * type let through by assignability does not reach SWR.
 */
function copyAllowed(
  options: object | undefined,
  allowed: ReadonlySet<string>,
  kind: string,
): Record<string, unknown> {
  const copy: Record<string, unknown> = {};
  if (options === undefined || options === null) return copy;
  for (const [name, value] of Object.entries(options)) {
    if (allowed.has(name)) copy[name] = value;
    else if (process.env.NODE_ENV !== "production")
      throw new TypeError(`~/lib/swr ${kind} takes no option '${name}'`);
  }
  return copy;
}

function readerConfig<T, E>(
  options: ReaderOptions<T, E> | undefined,
): SWRConfiguration<T, E> {
  const { onSuccess, onError, ...rest } = copyAllowed(
    options,
    allowedReaderKeys,
    "reader",
  ) as ReaderOptions<T, E>;
  return {
    ...rest,
    ...(onSuccess ? { onSuccess: (data: T) => onSuccess(data) } : {}),
    ...(onError ? { onError: (error: E) => onError(error) } : {}),
  };
}

function mutationOptions(options: unknown): unknown {
  return typeof options === "object" && options !== null
    ? copyAllowed(options, allowedMutationKeys, "mutation")
    : options;
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
    // `undefined` clears the entry. A clear takes only `revalidate` and
    // `throwOnError`: populateCache or optimisticData would write past the
    // lock.
    if (args[0] === undefined) {
      if (args.length === 0) return currentResult(key, mutate(key));
      const clear = copyAllowed(
        typeof args[1] === "boolean"
          ? { revalidate: args[1] }
          : ((args[1] ?? undefined) as object | undefined),
        clearOptionKeys,
        "clear",
      );
      return currentResult(key, mutate(key, undefined, clear as never));
    }
    const [locked, lockedOptions, isCurrent] = lockMutation(
      key,
      args[0],
      mutationOptions(args[1]),
    );
    return runResult(
      isCurrent,
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
type ScopedSWRResponse<T, E> = SWRResponse<T, E> & {
  /** Current protected values, including between renders. Never raw cache. */
  readCurrent?(): { data: T | undefined; error: E | undefined };
};

function useScopedSWR<T, E>(
  key: string | null,
  fetcher: () => Promise<T>,
  options: ReaderOptions<T, E> | undefined,
  base: SWRConfiguration = swrConfig,
): ScopedSWRResponse<T, E> {
  const { prefix, generation } = useSWRScope();
  const { cache, mutate: cacheMutate, fallback } = useSWRConfig();
  const fullKey =
    key !== null && prefix !== null ? scopedSWRKey(prefix, key) : null;
  // Renders again whenever values stop counting (scope or data bump).
  const moved = useSyncExternalStore(
    subscribeSWRDataGeneration,
    currentSWRDataGeneration,
    currentSWRDataGeneration,
  );
  const shown = useRef({ generation, moved });
  const configured = options?.keepPreviousData ?? base.keepPreviousData;
  const keep =
    configured === true &&
    shown.current.generation === generation &&
    shown.current.moved === moved;

  // The public fetcher contract has no SWR key argument. In particular a
  // factory's optional params must not receive the full scoped cache key.
  const response = useSWR<T, E>(fullKey, () => fetcher(), {
    ...(base as SWRConfiguration<T, E>),
    ...readerConfig(options),
    keepPreviousData: keep,
    use: [generationLockMiddleware],
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
  // - the caller's `fallbackData` or a provider fallback without a stamp by
  //   the key's fallback state, which a drop ends until fresh data came;
  // - any other value without a stamp (its stamp released at a scope
  //   boundary, #5223) only while no release or drop could have hit it;
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
    } else if (
      Object.is(raw, options?.fallbackData) ||
      Object.is(
        raw,
        (fallback as Record<string, unknown> | undefined)?.[fullKey],
      )
    ) {
      visible = isFallbackVisible(fullKey);
      fallbackShown = visible;
    } else {
      visible = isVisibleSWRValue(fullKey, raw);
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
    readCurrent() {
      if (
        fullKey === null ||
        isSWRSessionPending() ||
        dataGenerationOf(fullKey) !== mutationGeneration
      )
        return { data: undefined, error: undefined };
      const current = cache.get(fullKey);
      return {
        data:
          isCurrentSWRValue(fullKey, current?.data) === true
            ? (current?.data as T)
            : undefined,
        error: isCurrentSWRError(fullKey, current?.error)
          ? (current?.error as E)
          : undefined,
      };
    },
    get isValidating() {
      return response.isValidating;
    },
    get isLoading() {
      return response.isLoading || (!visible && response.isValidating);
    },
  } as ScopedSWRResponse<T, E>;
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
  options?: ReaderOptions<T, E>,
): ScopedSWRResponse<T, E> {
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
  return useScopedSWR<T, E>(key, fetcher, undefined, immutableConfig);
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
 * await tenantMutate(staffKeys.teachersKey());
 * ```
 */
export function useTenantMutate() {
  const { prefix } = useSWRScope();
  const origin = useDataOrigin();

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
      if (dataGenerationOf(cacheKey) > origin.generation)
        return Promise.resolve(undefined);
      if (data === undefined) {
        return currentResult(cacheKey, swrMutate(cacheKey));
      }
      // A write that settles after the generation moved is not kept.
      const [locked, lockedOptions, isCurrent] = lockMutation(
        cacheKey,
        data,
        mutationOptions(options),
      );
      return runResult(
        isCurrent,
        swrMutate(cacheKey, locked, lockedOptions as never),
      );
    },
    [prefix, origin],
  );
}

/**
 * The generation a callback of this render originates in (G21), renewed only
 * when values stop counting (scope or data bump). A request bump moves the
 * generation counter but ends no value, so it keeps the object and with it
 * the identity of every callback built on it (#5223). The callbacks take any
 * key: a data bump of any key renews them, a retained one keeps its origin.
 */
function useDataOrigin(): { readonly generation: number } {
  const moved = useSyncExternalStore(
    subscribeSWRDataGeneration,
    currentSWRDataGeneration,
    currentSWRDataGeneration,
  );
  return useMemo(
    () => ({ moved, generation: currentSWRGeneration() }),
    [moved],
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
  options?: ReaderOptions<T, E>,
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
  const origin = useDataOrigin();
  return useCallback(
    <T>(key: string, fetcher: () => Promise<T>): Promise<T | undefined> => {
      if (prefix === null || isSWRSessionPending())
        return Promise.resolve(undefined);
      const fullKey = scopedSWRKey(prefix, key);
      if (dataGenerationOf(fullKey) > origin.generation)
        return Promise.resolve(undefined);
      const start = startOf(fullKey);
      return preloadSWR(fullKey, fetcher).then(
        (value) =>
          isStillCurrent(fullKey, start) && isVisibleSWRValue(fullKey, value)
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
    [prefix, origin],
  );
}

/**
 * Invalidate every entry of the session's scope whose resource key `matches`,
 * wherever SWR keeps it (invalidateSWRKeys). `matches` gets the key without
 * the scope prefix, as the factories build it; `() => true` refreshes every
 * entry of the session. The predicate form of useTenantMutateMatching: the
 * cache of another school or account is never touched, and without a
 * signed-in session nothing matches. Components outside ~/lib/swr have no
 * other access to the cache (#5007).
 */
export function useInvalidateMatching() {
  const { prefix } = useSWRScope();
  const { cache } = useSWRConfig();
  return useCallback(
    (matches: (key: string) => boolean, options?: { clear?: boolean }) => {
      if (prefix === null || isSWRSessionPending()) return Promise.resolve([]);
      const scope = scopedSWRKey(prefix, "");
      return invalidateSWRKeys(
        cache,
        (key) => key.startsWith(scope) && matches(key.slice(scope.length)),
        options?.clear ? "clear" : "revalidate",
      );
    },
    [prefix, cache],
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
