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
import useSWR, { type SWRConfiguration, type SWRResponse } from "swr";
import { useSession } from "next-auth/react";
import { swrConfig, immutableConfig } from "./config";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";

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
