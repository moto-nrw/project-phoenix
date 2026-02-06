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
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SupervisionContext" });

interface BackendEducationalGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
}

interface SupervisedRoom {
  id: string;
  name: string;
  groupId: string;
  groupName?: string;
  isSchulhof?: boolean; // Special flag for Schulhof permanent tab
}

// Schulhof status from API
interface SchulhofStatus {
  exists: boolean;
  room_id?: number;
  room_name: string;
  active_group_id?: number;
  is_user_supervising: boolean;
}

const SCHULHOF_ROOM_NAME = "Schulhof";
const SCHULHOF_TAB_ID = "schulhof";

interface SupervisionState {
  // Group supervision
  hasGroups: boolean;
  isLoadingGroups: boolean;
  groups: BackendEducationalGroup[];

  // Room supervision (for active sessions)
  isSupervising: boolean;
  supervisedRoomId?: string;
  supervisedRoomName?: string;
  supervisedRooms: SupervisedRoom[];
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
    supervisedRooms: [],
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
        const json = (await response.json()) as {
          data?: { groups?: BackendEducationalGroup[] };
          groups?: BackendEducationalGroup[];
        };
        // Route wrapper wraps response as { success, data: { groups } }
        const groupList = (json.data?.groups ?? json.groups ?? []).sort(
          (a, b) => a.name.localeCompare(b.name, "de"),
        );
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

  // Check if user is supervising an active room (also fetches Schulhof status)
  const checkSupervision = useCallback(async () => {
    const token = tokenRef.current;
    if (!token) {
      setState((prev) => ({
        ...prev,
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        supervisedRooms: [],
        isLoadingSupervision: false,
      }));
      return;
    }

    try {
      // Fetch supervised groups and Schulhof status in parallel
      const [response, schulhofResponse] = await Promise.all([
        fetch("/api/me/groups/supervised", {
          headers: { "Content-Type": "application/json" },
          cache: "no-store",
        }),
        fetch("/api/active/schulhof/status", {
          headers: { "Content-Type": "application/json" },
          cache: "no-store",
        }).catch(() => null), // Schulhof is optional
      ]);

      // Parse Schulhof status
      let schulhofRoom: SupervisedRoom | null = null;
      if (schulhofResponse?.ok) {
        // Response is double-wrapped: { success, data: { status, data: SchulhofStatus } }
        const schulhofJson = (await schulhofResponse.json()) as {
          data?: { data?: SchulhofStatus };
        };
        // Extract the actual Schulhof status from nested structure
        const schulhofData = schulhofJson.data?.data;
        // Intentionally check `exists` only, NOT `is_user_supervising`.
        // The Schulhof tab must be visible to ALL staff so anyone can
        // opt-in to supervise. Multiple supervisors are expected.
        // `is_user_supervising` is available for UI hints (e.g. badge)
        // but must not gate tab visibility.
        if (schulhofData?.exists) {
          schulhofRoom = {
            id: SCHULHOF_TAB_ID,
            name: SCHULHOF_ROOM_NAME,
            groupId:
              schulhofData.active_group_id?.toString() ?? SCHULHOF_TAB_ID,
            isSchulhof: true,
          };
        }
      }

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

          // Map all supervised groups to rooms, sorted by name
          // Filter out Schulhof from regular rooms (it's handled separately)
          let newSupervisedRooms: SupervisedRoom[] = supervisedGroups
            .filter(
              (g) => g.room_id && g.room && g.room.name !== SCHULHOF_ROOM_NAME,
            )
            .map((g) => ({
              id: g.room_id!.toString(),
              name: g.room?.name ?? `Room ${g.room_id}`,
              groupId: g.id.toString(),
              groupName: g.actual_group?.name,
            }))
            .sort((a, b) => a.name.localeCompare(b.name, "de"));

          // Always add Schulhof at the end if it exists
          if (schulhofRoom) {
            newSupervisedRooms = [...newSupervisedRooms, schulhofRoom];
          }

          setState((prev) => {
            // Only update if values actually changed (compare room IDs, not just length)
            const prevRoomIds = prev.supervisedRooms.map((r) => r.id).join(",");
            const newRoomIds = newSupervisedRooms.map((r) => r.id).join(",");
            if (
              prev.isSupervising &&
              prev.supervisedRoomId === newRoomId &&
              prev.supervisedRoomName === newRoomName &&
              prevRoomIds === newRoomIds &&
              !prev.isLoadingSupervision
            ) {
              return prev;
            }
            return {
              ...prev,
              isSupervising: true,
              supervisedRoomId: newRoomId,
              supervisedRoomName: newRoomName,
              supervisedRooms: newSupervisedRooms,
              isLoadingSupervision: false,
            };
          });
        } else {
          // No regular supervision, but still include Schulhof if it exists
          const roomsWithSchulhof = schulhofRoom ? [schulhofRoom] : [];
          const isSchulhofSupervising = schulhofRoom !== null;

          setState((prev) => {
            const prevRoomIds = prev.supervisedRooms.map((r) => r.id).join(",");
            const newRoomIds = roomsWithSchulhof.map((r) => r.id).join(",");
            const newRoomId = isSchulhofSupervising
              ? SCHULHOF_TAB_ID
              : undefined;
            const newRoomName = isSchulhofSupervising
              ? SCHULHOF_ROOM_NAME
              : undefined;
            // Only update if values actually changed
            if (
              prev.isSupervising === isSchulhofSupervising &&
              prev.supervisedRoomId === newRoomId &&
              prev.supervisedRoomName === newRoomName &&
              prevRoomIds === newRoomIds &&
              !prev.isLoadingSupervision
            ) {
              return prev;
            }
            return {
              ...prev,
              isSupervising: isSchulhofSupervising,
              supervisedRoomId: newRoomId,
              supervisedRoomName: newRoomName,
              supervisedRooms: roomsWithSchulhof,
              isLoadingSupervision: false,
            };
          });
        }
      } else {
        // Response not OK, but still include Schulhof if it exists
        const roomsOnError = schulhofRoom ? [schulhofRoom] : [];
        const isSchulhofSupervising = schulhofRoom !== null;
        setState((prev) => {
          const prevRoomIds = prev.supervisedRooms.map((r) => r.id).join(",");
          const newRoomIds = roomsOnError.map((r) => r.id).join(",");
          const newRoomId = isSchulhofSupervising ? SCHULHOF_TAB_ID : undefined;
          const newRoomName = isSchulhofSupervising
            ? SCHULHOF_ROOM_NAME
            : undefined;
          // Only update if values actually changed
          if (
            prev.isSupervising === isSchulhofSupervising &&
            prev.supervisedRoomId === newRoomId &&
            prev.supervisedRoomName === newRoomName &&
            prevRoomIds === newRoomIds &&
            !prev.isLoadingSupervision
          ) {
            return prev;
          }
          return {
            ...prev,
            isSupervising: isSchulhofSupervising,
            supervisedRoomId: newRoomId,
            supervisedRoomName: newRoomName,
            supervisedRooms: roomsOnError,
            isLoadingSupervision: false,
          };
        });
      }
    } catch {
      // On error, we can't fetch Schulhof either, so just clear
      setState((prev) => {
        // Only update if values actually changed
        if (
          !prev.isSupervising &&
          prev.supervisedRoomId === undefined &&
          prev.supervisedRoomName === undefined &&
          prev.supervisedRooms.length === 0 &&
          !prev.isLoadingSupervision
        ) {
          return prev;
        }
        return {
          ...prev,
          isSupervising: false,
          supervisedRoomId: undefined,
          supervisedRoomName: undefined,
          supervisedRooms: [],
          isLoadingSupervision: false,
        };
      });
    }
  }, []); // No dependencies - uses ref

  // Check Schulhof status and add to supervised rooms if exists
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

      // checkSupervision now handles Schulhof internally
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
      refreshRef.current?.().catch((err: unknown) => {
        logger.error("failed to refresh supervision context", {
          error: String(err),
        });
      });
    } else {
      // Clear state when no session
      setState({
        hasGroups: false,
        isLoadingGroups: false,
        groups: [],
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        supervisedRooms: [],
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
