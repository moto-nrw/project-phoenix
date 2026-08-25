/**
 * Custom SWR Hooks with Authentication and Tenant Integration
 *
 * These hooks wrap SWR with NextAuth session awareness and tenant-scoped
 * cache keys, ensuring requests only fire when the user is authenticated
 * and cache is isolated per tenant.
 *
 * All hooks automatically prefix cache keys with the tenant slug when
 * used inside a TenantProvider (e.g., `school-a:students-list`). School
 * portal sessions use their tenant and account identity instead, because
 * they deliberately do not have a TenantProvider. Other portals' keys are
 * unchanged.
 */

"use client";

// eslint-disable-next-line no-restricted-imports -- this IS the tenant-aware wrapper
import useSWR, {
  mutate as swrMutate,
  type SWRConfiguration,
  type SWRResponse,
} from "swr";
import { useCallback } from "react";
import { useSession } from "next-auth/react";
import { swrConfig, immutableConfig } from "./config";
import { useTenantSlugSafe } from "~/lib/tenant-context";

/**
 * Prefix a cache key with the tenant slug for cross-tenant cache isolation.
 * Returns the key unchanged when no tenant context is available (e.g., operator dashboard).
 */
function scopedKey(
  key: string | null,
  tenantSlug: string | null,
  schoolSession: { tenantId: number; accountId: string } | null,
): string | null {
  if (key === null) return null;
  if (tenantSlug) return `${tenantSlug}:${key}`;
  if (schoolSession) {
    return `school:${schoolSession.tenantId}:${schoolSession.accountId}:${key}`;
  }
  return key;
}

/**
 * SWR hook with authentication and tenant cache isolation.
 *
 * Only fetches data when the user is authenticated (has a valid token).
 * Cache keys are automatically prefixed with the tenant slug, or with the
 * school tenant and account identity in the school portal, to prevent data
 * from one session appearing in another.
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
  const { data: session, status } = useSession();
  const slug = useTenantSlugSafe();
  const isSchoolSession = session?.user.scope === "school";
  const schoolSession =
    isSchoolSession && session.user.tenantId != null && session.user.id
      ? { tenantId: session.user.tenantId, accountId: session.user.id }
      : null;

  // Determine if we should fetch:
  // - key must be non-null
  // - session must be loaded (not "loading")
  // - user must have a token
  const shouldFetch =
    key !== null &&
    status !== "loading" &&
    !!session?.user?.token &&
    (!isSchoolSession || schoolSession !== null);

  return useSWR<T, E>(
    shouldFetch ? scopedKey(key, slug, schoolSession) : null,
    fetcher,
    {
      ...swrConfig,
      ...options,
    },
  );
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
 * SWR cache keys are prefixed with the tenant slug by useSWRAuth, so plain
 * `mutate("my-key")` misses the cache. This hook returns a function that
 * applies the same tenant prefix before calling SWR mutate.
 *
 * @example
 * ```tsx
 * const tenantMutate = useTenantMutate();
 * await tenantMutate("database-teachers-list");
 * ```
 */
export function useTenantMutate() {
  const slug = useTenantSlugSafe();

  return useCallback(
    (
      key: string,
      data?: unknown,
      options?: { revalidate?: boolean },
    ): Promise<unknown> => {
      const cacheKey = scopedKey(key, slug, null);
      if (data === undefined) {
        return swrMutate(cacheKey);
      }
      return swrMutate(cacheKey, data, options);
    },
    [slug],
  );
}

/**
 * Hook that invalidates every tenant-scoped SWR cache entry whose key matches
 * a substring. Useful for cross-cutting changes that affect data displayed by
 * multiple unrelated pages — e.g. updating a room's color must refresh every
 * visit/student list that stamps `current_room_color` into its rows.
 *
 * Match is checked against the tenant-prefixed key, so substrings should
 * include enough context to avoid over-invalidation. Pass an array of
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
  const slug = useTenantSlugSafe();
  // "|" is a safe joiner: substrings are caller-supplied cache-key fragments
  // (literals like "ogs-students"), which cannot legally contain pipe in any
  // SWR key the codebase produces. The joined string is the actual dep — a
  // fresh array literal with identical contents reuses the memoized callback.
  const joined = substrings.join("|");

  return useCallback(
    (options?: { clear?: boolean }) => {
      const needles = joined.length > 0 ? joined.split("|") : [];
      const matches = (key: unknown): boolean => {
        if (typeof key !== "string") return false;
        // Limit to the active tenant so cross-tenant caches (multi-tab) stay put.
        if (slug && !key.startsWith(`${slug}:`)) return false;
        return needles.some((needle) => key.includes(needle));
      };
      if (options?.clear) {
        return swrMutate(matches, undefined, { revalidate: false });
      }
      return swrMutate(matches);
    },
    [slug, joined],
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
  const { data: session, status } = useSession();
  const slug = useTenantSlugSafe();
  const isSchoolSession = session?.user.scope === "school";
  const schoolSession =
    isSchoolSession && session.user.tenantId != null && session.user.id
      ? { tenantId: session.user.tenantId, accountId: session.user.id }
      : null;

  const shouldFetch =
    id != null &&
    status !== "loading" &&
    !!session?.user?.token &&
    (!isSchoolSession || schoolSession !== null);

  return useSWR<T, E>(
    shouldFetch ? scopedKey(`${baseKey}-${id}`, slug, schoolSession) : null,
    () => fetcher(id!),
    {
      ...swrConfig,
      ...options,
    },
  );
}
