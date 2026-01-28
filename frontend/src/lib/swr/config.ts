/**
 * SWR Global Configuration
 *
 * Provides default settings for all SWR hooks in the application.
 * Based on Vercel Engineering's React Best Practices.
 */

import type { SWRConfiguration } from "swr";

/**
 * Default SWR configuration for the application.
 *
 * Key settings:
 * - dedupingInterval: 2000ms - Prevents duplicate requests within 2 seconds
 * - revalidateOnFocus: true - Refetches data when user returns to tab
 * - revalidateOnReconnect: true - Refetches when network reconnects
 * - errorRetryCount: 3 - Retries failed requests up to 3 times
 */
export const swrConfig: SWRConfiguration = {
  // Deduplicate requests made within 2 seconds
  dedupingInterval: 2000,

  // Revalidation triggers
  revalidateOnFocus: true,
  revalidateOnReconnect: true,
  revalidateIfStale: true,

  // Error handling
  errorRetryCount: 3,
  errorRetryInterval: 1000,

  // Don't revalidate on mount if data exists (improves perceived performance)
  revalidateOnMount: undefined, // Let SWR decide based on staleness

  // Keep previous data while revalidating (prevents loading flash)
  keepPreviousData: true,
};

/**
 * Configuration for immutable/static data that rarely changes.
 * Examples: roles, permissions, categories
 */
export const immutableConfig: SWRConfiguration = {
  ...swrConfig,
  revalidateIfStale: false,
  revalidateOnFocus: false,
  revalidateOnReconnect: false,
  // Longer dedupe interval for truly static data
  dedupingInterval: 5 * 60 * 1000, // 5 minutes
};
