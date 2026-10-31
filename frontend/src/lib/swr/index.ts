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
  useInvalidateMatching,
  useImmutableSWR,
  useTenantMutate,
  useTenantMutateMatching,
} from "./hooks";
export {
  activityWaitlistKeys,
  courseWaitlistKeys,
} from "./course-waitlist-keys";
export {
  authGeneration,
  scopedSWRKey,
  tenantSWRScope,
  useSWRSessionRefresh,
} from "./scope";
export { stampSWRSeeds, writeSWRSeeds, useSWRSeedFrom } from "./seed";
export { IsolatedSWRCache, SWRSeedBoundary } from "./config-boundaries";
export { SWRScopeBoundary } from "./scope-boundary";

// Types only: no export of this module hands out the cache (#5007).
export type { ReaderOptions } from "./hooks";
export type { SWRConfiguration, SWRResponse, KeyedMutator } from "swr";
export { mutate } from "./mutate";
