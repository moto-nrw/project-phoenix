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
  dataGenerationOf,
  isCurrentSWRValue,
  isDiscardedAnswer,
  isStillCurrent,
  lockFetcher,
  subscribeSWRDataGeneration,
  lockMutation,
  requestGenerationOf,
  stampSWRValue,
  startOf,
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
    return useSWRNext(key, locked, {
      ...config,
      isPaused: () => isSWRSessionPending() || config.isPaused?.() === true,
      onError,
      onErrorRetry,
    });
  };

/** A bound mutate of `key` whose writes go through the generation lock. */
function lockBoundMutate<T>(
  key: string,
  mutate: SWRResponse<T>["mutate"],
): SWRResponse<T>["mutate"] {
  return ((...args: unknown[]) => {
    // SWR counts the arguments: `mutate()` revalidates, while an explicit
    // `undefined` writes it. Pass revalidations through unchanged.
    if (args[0] === undefined) {
      return (mutate as (...given: unknown[]) => Promise<T | undefined>)(
        ...args,
      );
    }
    const [locked, lockedOptions] = lockMutation(key, args[0], args[1]);
    return mutate(locked as never, lockedOptions as never);
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

  const response = useSWR<T, E>(fullKey, fetcher, {
    ...swrConfig,
    ...options,
    keepPreviousData: keep,
    use: [generationLockMiddleware, ...(options?.use ?? [])],
  });

  const raw = response.data;
  const rawError = response.error;
  const discarded = isDiscardedAnswer(rawError);
  // A caller's `fallbackData` counts from the generation this hook first
  // showed it in; any other value no path of ~/lib/swr produced (a raw
  // write) is stamped on first sight.
  const fallbackSeen = useRef<{ value: unknown; generation: number } | null>(
    null,
  );
  let fallbackShown: { value: unknown; generation: number } | null = null;
  let visible: boolean;
  if (fullKey === null) {
    // Without a session nothing is shown, not even `fallbackData`; a key
    // the caller left out keeps SWR's own behaviour (the keep guard above).
    visible = prefix !== null;
  } else {
    const current = isCurrentSWRValue(fullKey, raw);
    if (current !== undefined) {
      visible = current;
    } else if (raw === options?.fallbackData) {
      const seen = fallbackSeen.current;
      fallbackShown = {
        value: raw,
        generation:
          seen !== null && seen.value === raw
            ? seen.generation
            : dataGenerationOf(fullKey),
      };
      visible = fallbackShown.generation >= dataGenerationOf(fullKey);
    } else {
      stampSWRValue(fullKey, raw, dataGenerationOf(fullKey));
      visible = true;
    }
  }
  // Remember the generation a fallback was first shown in, after render.
  useLayoutEffect(() => {
    if (fallbackShown && fallbackSeen.current?.value !== fallbackShown.value) {
      fallbackSeen.current = fallbackShown;
    }
  });
  const data = visible ? raw : undefined;
  const error = discarded ? undefined : rawError;

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
        : !visible && rawError === undefined
          ? `data:${fullKey}:${dataGenerationOf(fullKey)}`
          : null;
  const reloaded = useRef<string | null>(null);
  const revalidate = response.mutate;
  useEffect(() => {
    if (reload === null || reloaded.current === reload) return;
    reloaded.current = reload;
    void revalidate();
  }, [reload, revalidate]);

  const mutate = useMemo(
    () => (fullKey ? lockBoundMutate(fullKey, revalidate) : revalidate),
    [fullKey, revalidate],
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

  return useCallback(
    (
      key: string,
      data?: unknown,
      options?: { revalidate?: boolean },
    ): Promise<unknown> => {
      if (prefix === null || isSWRSessionPending())
        return Promise.resolve(undefined);
      const cacheKey = scopedSWRKey(prefix, key);
      if (data === undefined) {
        return swrMutate(cacheKey);
      }
      // A write that settles after the generation moved is not kept.
      const [locked, lockedOptions] = lockMutation(cacheKey, data, options);
      return swrMutate(cacheKey, locked, lockedOptions as never);
    },
    [prefix],
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
      if (prefix !== null) discardScopedEntries(cache, prefix, entity.matches);
      return true;
    },
    [prefix, cache],
  );
}
