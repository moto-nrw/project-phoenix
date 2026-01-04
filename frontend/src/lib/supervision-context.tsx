"use client";

import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  useMemo,
} from "react";
import { useSession } from "next-auth/react";

interface BackendEducationalGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
}

interface SupervisionState {
  // Group supervision
  hasGroups: boolean;
  isLoadingGroups: boolean;
  groups: BackendEducationalGroup[];

  // Room supervision (for active sessions)
  isSupervising: boolean;
  supervisedRoomId?: string;
  supervisedRoomName?: string;
  isLoadingSupervision: boolean;
}

interface SupervisionContextType extends SupervisionState {
  refresh: (silent?: boolean) => Promise<void>;
}

const SupervisionContext = createContext<SupervisionContextType | undefined>(
  undefined,
);

/**
 * Provider that manages dynamic supervision states
 * Checks for group assignments and active room supervision
 */
export function SupervisionProvider({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const { data: session } = useSession();

  const [state, setState] = useState<SupervisionState>({
    hasGroups: false,
    isLoadingGroups: true,
    groups: [],
    isSupervising: false,
    supervisedRoomId: undefined,
    supervisedRoomName: undefined,
    isLoadingSupervision: true,
  });

  // Debounce mechanism to prevent rapid successive calls
  const isRefreshingRef = React.useRef(false);
  const lastRefreshRef = React.useRef<number>(0);

  // Store token in ref to avoid dependency loops
  const tokenRef = React.useRef<string | undefined>(session?.user?.token);
  tokenRef.current = session?.user?.token;

  // Use a ref for the refresh function to break dependency cycles
  const refreshRef = React.useRef<((silent?: boolean) => Promise<void>) | null>(
    null,
  );

  // Check if user has any groups (as teacher or representative)
  const checkGroups = useCallback(async () => {
    const token = tokenRef.current;
    if (!token) {
      setState((prev) => ({
        ...prev,
        hasGroups: false,
        groups: [],
        isLoadingGroups: false,
      }));
      return;
    }

    try {
      const response = await fetch("/api/groups/context", {
        headers: {
          "Content-Type": "application/json",
        },
        // Add cache control to reduce redundant requests
        cache: "no-store",
      });

      if (response.ok) {
        const data = (await response.json()) as {
          groups?: BackendEducationalGroup[];
        };
        const groupList = data?.groups ?? [];
        const newHasGroups = groupList.length > 0;
        setState((prev) => {
          // Only update if value actually changed
          if (
            prev.hasGroups === newHasGroups &&
            prev.groups.length === groupList.length &&
            prev.groups.every(
              (group, index) => group.id === groupList[index]?.id,
            ) &&
            !prev.isLoadingGroups
          ) {
            return prev;
          }
          return {
            ...prev,
            hasGroups: newHasGroups,
            groups: groupList,
            isLoadingGroups: false,
          };
        });
      } else {
        setState((prev) => {
          // Only update if value actually changed
          if (
            !prev.hasGroups &&
            prev.groups.length === 0 &&
            !prev.isLoadingGroups
          ) {
            return prev;
          }
          return {
            ...prev,
            hasGroups: false,
            groups: [],
            isLoadingGroups: false,
          };
        });
      }
    } catch {
      setState((prev) => {
        // Only update if values actually changed
        if (
          !prev.hasGroups &&
          prev.groups.length === 0 &&
          !prev.isLoadingGroups
        ) {
          return prev;
        }
        return {
          ...prev,
          hasGroups: false,
          groups: [],
          isLoadingGroups: false,
        };
      });
    }
  }, []); // No dependencies - uses ref

  // Check if user is supervising an active room
  const checkSupervision = useCallback(async () => {
    const token = tokenRef.current;
    if (!token) {
      setState((prev) => ({
        ...prev,
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
      }));
      return;
    }

    try {
      const response = await fetch("/api/me/groups/supervised", {
        headers: {
          "Content-Type": "application/json",
        },
        // Add cache control to reduce redundant requests
        cache: "no-store",
      });

      if (response.ok) {
        const response_data = (await response.json()) as {
          success: boolean;
          message: string;
          data: Array<{
            id: number;
            room_id?: number;
            group_id: number;
            room?: {
              id: number;
              name: string;
            };
            actual_group?: {
              id: number;
              name: string;
            };
          }>;
        };

        // Check if user has any supervised groups (indicating room supervision)
        const supervisedGroups = response_data.data ?? [];
        const hasSupervision = supervisedGroups.length > 0;

        if (hasSupervision && supervisedGroups[0]) {
          const firstGroup = supervisedGroups[0];
          const newRoomId = firstGroup.room_id?.toString();
          const newRoomName =
            firstGroup.room?.name ??
            (firstGroup.room_id ? `Room ${firstGroup.room_id}` : undefined);

          setState((prev) => {
            // Only update if values actually changed
            if (
              prev.isSupervising &&
              prev.supervisedRoomId === newRoomId &&
              prev.supervisedRoomName === newRoomName &&
              !prev.isLoadingSupervision
            ) {
              return prev;
            }
            return {
              ...prev,
              isSupervising: true,
              supervisedRoomId: newRoomId,
              supervisedRoomName: newRoomName,
              isLoadingSupervision: false,
            };
          });
        } else {
          setState((prev) => {
            // Only update if values actually changed
            if (
              !prev.isSupervising &&
              prev.supervisedRoomId === undefined &&
              prev.supervisedRoomName === undefined &&
              !prev.isLoadingSupervision
            ) {
              return prev;
            }
            return {
              ...prev,
              isSupervising: false,
              supervisedRoomId: undefined,
              supervisedRoomName: undefined,
              isLoadingSupervision: false,
            };
          });
        }
      } else {
        setState((prev) => {
          // Only update if values actually changed
          if (
            !prev.isSupervising &&
            prev.supervisedRoomId === undefined &&
            prev.supervisedRoomName === undefined &&
            !prev.isLoadingSupervision
          ) {
            return prev;
          }
          return {
            ...prev,
            isSupervising: false,
            supervisedRoomId: undefined,
            supervisedRoomName: undefined,
            isLoadingSupervision: false,
          };
        });
      }
    } catch {
      setState((prev) => {
        // Only update if values actually changed
        if (
          !prev.isSupervising &&
          prev.supervisedRoomId === undefined &&
          prev.supervisedRoomName === undefined &&
          !prev.isLoadingSupervision
        ) {
          return prev;
        }
        return {
          ...prev,
          isSupervising: false,
          supervisedRoomId: undefined,
          supervisedRoomName: undefined,
          isLoadingSupervision: false,
        };
      });
    }
  }, []); // No dependencies - uses ref

  // Refresh all supervision states with debouncing
  const refresh = useCallback(
    async (silent = false) => {
      // Prevent rapid successive refreshes (min 5 seconds between refreshes)
      const now = Date.now();
      if (now - lastRefreshRef.current < 5000) {
        return;
      }
      lastRefreshRef.current = now;

      // Already refreshing, don't start another
      if (isRefreshingRef.current) return;
      isRefreshingRef.current = true;

      // Only show loading states if not a silent refresh
      if (!silent) {
        setState((s) => ({
          ...s,
          isLoadingGroups: true,
          isLoadingSupervision: true,
        }));
      }

      void Promise.all([checkGroups(), checkSupervision()]).finally(() => {
        isRefreshingRef.current = false;
      });
    },
    [checkGroups, checkSupervision],
  );

  // Store the refresh function in ref
  refreshRef.current = refresh;

  // Initial load and refresh on session changes only
  useEffect(() => {
    // Only refresh when session actually changes (not on every render)
    if (session?.user?.token) {
      refreshRef.current?.().catch(console.error);
    } else {
      // Clear state when no session
      setState({
        hasGroups: false,
        isLoadingGroups: false,
        groups: [],
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
      });
    }
  }, [session?.user?.token]); // Only depend on token

  // Periodic refresh every minute for timely supervision updates (silent mode)
  useEffect(() => {
    if (!session?.user?.token) return;

    const interval = setInterval(() => {
      // Use silent refresh to avoid UI flicker - errors handled internally
      if (refreshRef.current) {
        refreshRef.current(true).catch(() => {
          // Intentionally ignored - silent background refresh
        });
      }
    }, 60000); // 1 minute - ensures supervision changes are reflected quickly

    return () => clearInterval(interval);
  }, [session?.user?.token]); // Only depend on token

  const value = useMemo<SupervisionContextType>(
    () => ({ ...state, refresh }),
    [state, refresh],
  );

  return (
    <SupervisionContext.Provider value={value}>
      {children}
    </SupervisionContext.Provider>
  );
}

/**
 * Hook to access supervision context
 */
export function useSupervision() {
  const context = useContext(SupervisionContext);
  if (context === undefined) {
    throw new Error("useSupervision must be used within a SupervisionProvider");
  }
  return context;
}

/**
 * Hook to check if user has groups (convenience wrapper)
 */
export function useHasGroups(): boolean {
  const { hasGroups, isLoadingGroups } = useSupervision();
  return !isLoadingGroups && hasGroups;
}

/**
 * Hook to check if user is supervising a room (convenience wrapper)
 */
export function useIsSupervising(): boolean {
  const { isSupervising, isLoadingSupervision } = useSupervision();
  return !isLoadingSupervision && isSupervising;
}
