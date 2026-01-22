/**
 * User Context Hook
 *
 * Provides shared user context data (educational groups, supervised groups, staff)
 * with SWR caching for automatic deduplication across pages.
 *
 * This hook uses useImmutableSWR because user context rarely changes during a session.
 * The 5-minute cache means navigating between pages won't trigger new API calls.
 *
 * @example
 * ```tsx
 * const { userContext, isLoading, error } = useUserContext();
 *
 * // Pre-computed arrays for components
 * const myGroupIds = userContext?.educationalGroupIds ?? [];
 * const myGroupRooms = userContext?.educationalGroupRoomNames ?? [];
 * ```
 */

"use client";

import { usePathname } from "next/navigation";
import { useImmutableSWR } from "~/lib/swr";
import type { UserContextResponse } from "~/app/api/user-context/route";

// Paths where user context should be skipped (no org context)
const CONTEXT_DISABLED_PATHS = ["/console"];

interface ApiResponse<T> {
  data: T;
  success: boolean;
  message: string;
}

interface UseUserContextReturn {
  /** The user context data, undefined while loading */
  userContext: UserContextResponse | undefined;
  /** True while the initial fetch is in progress */
  isLoading: boolean;
  /** Error object if the fetch failed */
  error: Error | undefined;
  /** True once data has been fetched (even if empty) */
  isReady: boolean;
}

/**
 * Fetches and caches user context from the BFF endpoint.
 *
 * Features:
 * - Automatic request deduplication (SWR)
 * - 5-minute cache for navigation performance
 * - React Strict Mode safe (no double-fetch)
 * - Pre-computed derived data (group IDs, room names)
 * - Skips fetching on /console paths (SaaS admins have no org context)
 */
export function useUserContext(): UseUserContextReturn {
  const pathname = usePathname();

  // Check if we're on a path where context should be skipped
  const isDisabledPath = CONTEXT_DISABLED_PATHS.some((path) =>
    pathname.startsWith(path),
  );

  const { data, isLoading, error } = useImmutableSWR<UserContextResponse>(
    // Use null key to skip fetching on disabled paths
    isDisabledPath ? null : "user-context",
    async () => {
      const response = await fetch("/api/user-context", {
        credentials: "include",
      });

      if (!response.ok) {
        throw new Error(`User context fetch failed: ${response.status}`);
      }

      const json = (await response.json()) as ApiResponse<UserContextResponse>;
      return json.data;
    },
  );

  return {
    userContext: data,
    isLoading: isDisabledPath ? false : isLoading,
    error,
    isReady: isDisabledPath || data !== undefined || error !== undefined,
  };
}

/**
 * Type export for consumers who need the response shape
 */
export type { UserContextResponse };
