/**
 * SWR Utilities for Project Phoenix
 *
 * This module provides SWR hooks integrated with NextAuth authentication.
 * Based on Vercel Engineering's React Best Practices for automatic
 * request deduplication, caching, and revalidation.
 *
 * @example
 * ```tsx
 * import { useSWRAuth, useImmutableSWR } from '~/lib/swr';
 *
 * // For regular data that should revalidate
 * const { data, isLoading } = useSWRAuth('students', () => getStudents());
 *
 * // For static data (roles, permissions)
 * const { data: roles } = useImmutableSWR('roles', () => getRoles());
 * ```
 */

// Re-export hooks
export { useSWRAuth, useImmutableSWR, useSWRWithId } from "./hooks";

// Re-export config for advanced usage
export { swrConfig, immutableConfig } from "./config";

// Re-export SWR utilities that consumers might need
export { mutate } from "swr";
