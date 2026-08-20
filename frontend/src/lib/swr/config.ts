/**
 * SWR Global Configuration
 *
 * Provides default settings for all SWR hooks in the application.
 * Based on Vercel Engineering's React Best Practices.
 */

import type { SWRConfiguration } from "swr";
import { createLogger } from "~/lib/logger";
import {
  isRateLimitError,
  remainingRateLimitMs,
} from "~/lib/rate-limit-backoff";

const logger = createLogger({ component: "useSWRAuth" });

/**
 * Default error handler for useSWRAuth.
 *
 * SWR fires onError for every failed attempt (each retry counts), so a flaky
 * endpoint logs multiple times — that retry behavior is itself useful signal.
 * The cache key is the de-facto event identifier so dashboards can group
 * failures by domain (e.g., "tenant-slug:database-devices-list").
 */
function logSWRError(error: unknown, key: string): void {
  const errorMessage = error instanceof Error ? error.message : String(error);
  const isRateLimited = isRateLimitError(error);
  const logContext = {
    key,
    error: errorMessage,
    ...(isRateLimited && { rate_limited: true }),
  };

  if (isRateLimited) {
    logger.warn("swr_fetch_rate_limited", logContext);
    return;
  }

  logger.error("swr_fetch_failed", logContext);
}

/**
 * Default SWR configuration for the application.
 *
 * Key settings:
 * - dedupingInterval: 5000ms - Prevents duplicate requests within 5 seconds
 *   (wider window to absorb SSE event bursts)
 * - revalidateOnFocus: false - SSE handles freshness, tab focus is redundant
 * - revalidateOnReconnect: true - Important after network loss
 * - errorRetryCount: 3 - Retries failed requests up to 3 times
 */
export const swrConfig: SWRConfiguration = {
  // Deduplicate requests within 5 seconds (prevents rapid-fire from SSE bursts)
  dedupingInterval: 5000,

  // Revalidation triggers
  // SSE pushes real-time updates, so tab focus revalidation is redundant
  revalidateOnFocus: false,
  revalidateOnReconnect: true,
  revalidateIfStale: true,

  // Error handling
  errorRetryCount: 3,
  errorRetryInterval: 1000,
  onErrorRetry: (error, _key, _config, revalidate, options) => {
    if (isRateLimitError(error)) {
      setTimeout(
        () => revalidate({ retryCount: options.retryCount }),
        Math.max(50, remainingRateLimitMs() + 50),
      );
      return;
    }
    if (options.retryCount >= 3) return;
    setTimeout(
      () => revalidate({ retryCount: options.retryCount }),
      1000 * 2 ** Math.max(0, options.retryCount - 1),
    );
  },

  // Don't revalidate on mount if data exists (improves perceived performance)
  revalidateOnMount: undefined, // Let SWR decide based on staleness

  // Keep previous data while revalidating (prevents loading flash)
  keepPreviousData: true,

  onError: logSWRError,
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
