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
export {
  useSWRAuth,
  useSWRCacheKey,
  useSWRPreload,
  useInvalidateFamily,
  useImmutableSWR,
  useTenantMutate,
  useTenantMutateMatching,
} from "./hooks";
export { scopedSWRKey, tenantSWRScope } from "./scope";

// Re-export SWR utilities that consumers might need
export {
  SWRConfig,
  type Cache,
  type SWRConfiguration,
  type SWRResponse,
  type KeyedMutator,
} from "swr";
export { mutate } from "./mutate";
