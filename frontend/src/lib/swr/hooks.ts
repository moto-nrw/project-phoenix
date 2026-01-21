/**
 * Custom SWR Hooks with Authentication Integration
 *
 * These hooks wrap SWR with BetterAuth session awareness,
 * ensuring requests only fire when the user is authenticated.
 * BetterAuth: Authentication handled via cookies, no manual token management needed
 */

"use client";

import useSWR, { type SWRConfiguration, type SWRResponse } from "swr";
import { useSession } from "~/lib/auth-client";
import { swrConfig, immutableConfig } from "./config";

/**
 * SWR hook with authentication integration.
 *
 * Only fetches data when the user is authenticated (has a valid token).
 * Uses the existing service layer functions as fetchers.
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
  const { data: session, isPending } = useSession();

  // Determine if we should fetch:
  // - key must be non-null
  // - session must be loaded (not pending)
  // - user must be authenticated
  // BetterAuth: cookies handle auth, no token check needed
  const shouldFetch = key !== null && !isPending && !!session?.user;

  return useSWR<T, E>(shouldFetch ? key : null, fetcher, {
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
 * ensuring proper cache isolation per entity.
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
  const { data: session, isPending } = useSession();

  // BetterAuth: cookies handle auth, no token check needed
  const shouldFetch = id != null && !isPending && !!session?.user;

  return useSWR<T, E>(
    shouldFetch ? `${baseKey}-${id}` : null,
    () => fetcher(id!),
    {
      ...swrConfig,
      ...options,
    },
  );
}
