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

import { useImmutableSWR } from "~/lib/swr";
import type { UserContextResponse } from "~/app/api/user-context/route";

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
 */
export function useUserContext(): UseUserContextReturn {
  const { data, isLoading, error } = useImmutableSWR<UserContextResponse>(
    "user-context",
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
    isLoading,
    error,
    isReady: data !== undefined || error !== undefined,
  };
}

/**
 * Type export for consumers who need the response shape
 */
export type { UserContextResponse };
