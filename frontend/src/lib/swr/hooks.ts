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
import { useCallback, useLayoutEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import { swrConfig, immutableConfig } from "./config";
import {
  authGeneration,
  isSWRSessionPending,
  useSWRSessionRefresh,
  discardScopedEntries,
  isEntityRefusal,
  scopedSWRKey,
  swrScopeEpoch,
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

// Wrap the merged configuration, preserving provider- and caller-level
// pauses. Read the live flag because a timer or mutation can run before
// React commits the disabled keys after an owner-change event.
const sessionRefreshMiddleware: Middleware =
  (useSWRNext) => (key, fetcher, config) =>
    useSWRNext(key, fetcher, {
      ...config,
      isPaused: () => isSWRSessionPending() || config.isPaused?.() === true,
    });

/**
 * useSWR under the session's scope. `keepPreviousData` keeps an earlier
 * key's data on screen only while it belongs to the same auth generation and
 * nothing was dropped since (#5006): a new key after an account switch, a
 * logout or a discard starts empty. The marker moves once data of the
 * current key arrived.
 */
function useScopedSWR<T, E>(
  key: string | null,
  fetcher: () => Promise<T>,
  options: SWRConfiguration<T, E> | undefined,
): SWRResponse<T, E> {
  const { prefix, generation } = useSWRScope();
  const epoch = swrScopeEpoch();
  const shown = useRef({ generation, epoch });
  const configured = options?.keepPreviousData ?? swrConfig.keepPreviousData;
  const keep =
    configured === true &&
    shown.current.generation === generation &&
    shown.current.epoch === epoch;

  const response = useSWR<T, E>(
    key !== null && prefix !== null ? scopedSWRKey(prefix, key) : null,
    fetcher,
    {
      ...swrConfig,
      ...options,
      keepPreviousData: keep,
      use: [sessionRefreshMiddleware, ...(options?.use ?? [])],
    },
  );

  // Read data only while the marker lags behind: reading it makes SWR
  // re-render this hook on every data change, which hooks that keep their
  // marker do not need.
  const arrived = configured === true && !keep && response.data !== undefined;
  useLayoutEffect(() => {
    if (arrived) shown.current = { generation, epoch };
  }, [arrived, generation, epoch]);

  return response;
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
      return swrMutate(cacheKey, data, options);
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
 * **Clearing instead of revalidating**: pass `{ clear: true }` to DELETE the
 * matching cache entries without refetching. The global `keepPreviousData`
 * config means a plain revalidation still serves the stale entry first
 * (`isLoading` stays false on remount) — when consumers must never act on
 * pre-mutation data, the entry has to go so the next mount loads fresh with
 * `isLoading: true`.
 *
 * **Leaving keys out**: `except` names unprefixed keys that are not touched,
 * e.g. the list a decision was just made in, which already shows its result.
 * A plain revalidation also drops SWR's marker of a request in flight, so an
 * entry that is not mounted fetches fresh on its next mount (#4988).
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
      const matches = (key: unknown): boolean => {
        if (typeof key !== "string" || kept.has(key)) return false;
        if (!key.startsWith(scope)) return false;
        return needles.some((needle) => key.includes(needle));
      };
      if (options?.clear) {
        return swrMutate(matches, undefined, { revalidate: false });
      }
      return swrMutate(matches);
    },
    [prefix, joined],
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
