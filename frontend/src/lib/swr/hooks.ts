/**
 * Custom SWR Hooks with Authentication and Tenant Integration
 *
 * These hooks wrap SWR with NextAuth session awareness and tenant-scoped
 * cache keys, ensuring requests only fire when the user is authenticated
 * and cache is isolated per tenant.
 *
 * All hooks automatically prefix cache keys with the tenant slug when
 * used inside a TenantProvider (e.g., `school-a:students-list`).
 * Outside a TenantProvider (e.g., operator dashboard), keys are unchanged.
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
function tenantKey(
  key: string | null,
  tenantSlug: string | null,
): string | null {
  if (key === null) return null;
  if (tenantSlug) return `${tenantSlug}:${key}`;
  return key;
}

/**
 * SWR hook with authentication and tenant cache isolation.
 *
 * Only fetches data when the user is authenticated (has a valid token).
 * Cache keys are automatically prefixed with the tenant slug to prevent
 * cross-tenant data leaks when switching tenants.
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

  // Determine if we should fetch:
  // - key must be non-null
  // - session must be loaded (not "loading")
  // - user must have a token
  const shouldFetch =
    key !== null && status !== "loading" && !!session?.user?.token;

  return useSWR<T, E>(shouldFetch ? tenantKey(key, slug) : null, fetcher, {
    ...swrConfig,
    ...options,
  });
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

  return useCallback((key: string) => swrMutate(tenantKey(key, slug)), [slug]);
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

  return useCallback(() => {
    const needles = joined.length > 0 ? joined.split("|") : [];
    return swrMutate((key) => {
      if (typeof key !== "string") return false;
      // Limit to the active tenant so cross-tenant caches (multi-tab) stay put.
      if (slug && !key.startsWith(`${slug}:`)) return false;
      return needles.some((needle) => key.includes(needle));
    });
  }, [slug, joined]);
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

  const shouldFetch =
    id != null && status !== "loading" && !!session?.user?.token;

  return useSWR<T, E>(
    shouldFetch ? tenantKey(`${baseKey}-${id}`, slug) : null,
    () => fetcher(id!),
    {
      ...swrConfig,
      ...options,
    },
  );
}
