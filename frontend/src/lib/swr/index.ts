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
  authGeneration,
  scopedSWRKey,
  tenantSWRScope,
  useSWRSessionRefresh,
} from "./scope";
export { stampSWRSeeds, writeSWRSeeds, useSWRSeedFrom } from "./seed";
export { IsolatedSWRCache, SWRSeedBoundary } from "./config-boundaries";
export { SWRScopeBoundary } from "./scope-boundary";
export {
  LIVE_FALLBACK_INTERVAL_MS,
  LIVE_RECONCILE_INTERVAL_MS,
  useLiveEventScope,
  useLiveRefreshInterval,
  usePublishLiveChannel,
} from "./live-sync";

// Types only: no export of this module hands out the cache (#5007).
export type { ReaderOptions } from "./hooks";
export type { SWRConfiguration, SWRResponse, KeyedMutator } from "swr";
export { mutate } from "./mutate";
