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
import { hasPermission, isAdmin, isCaregiver } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import { useLatest } from "~/lib/hooks/use-latest";

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

  // True when the caller fetched supervision rooms via the admin overview
  // endpoint (/api/active/supervisors/all). False for regular staff or when
  // the setting is disabled. Used by pages to gate admin-only views so that
  // a synthetic Schulhof entry does not count as an enabled overview.
  adminOverviewEnabled: boolean;
}

interface SupervisionContextType extends SupervisionState {
  refresh: (options?: { silent?: boolean; force?: boolean }) => Promise<void>;
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
    adminOverviewEnabled: false,
  });

  // Debounce mechanism to prevent rapid successive calls
  const isRefreshingRef = React.useRef(false);
  const lastRefreshRef = React.useRef<number>(0);

  // Store token and admin status in refs to avoid dependency loops.
  // Any admin (including dual-role teacher-admins) tries the admin-overview
  // endpoint first. The server-side setting is the single source of truth for
  // whether all rooms are visible; users without opt-in fall back to their
  // own scope via the staff endpoint.
  const tokenRef = useLatest(session?.user?.token);
  const sessionIsAdmin = isAdmin(session);
  const isAdminRef = useLatest(sessionIsAdmin);

  // Whether the user may read group/supervision data. The Schulhof status
  // endpoint is gated by `groups:read` on the backend, so accounts with an
  // explicit permissions list that lacks it (e.g. the limited `guest` role)
  // get skipped instead of flooding production with guaranteed 403s. Admins
  // keep their role fallback; staff sessions issued before the permissions
  // claim existed have no list at all, so keep their role-based access alive
  // during that rollout window and let the backend remain the final authority.
  const sessionPermissions = session?.user?.permissions;
  const hasExplicitPermissions = Array.isArray(sessionPermissions);
  const canReadGroups =
    sessionIsAdmin ||
    hasPermission(session, "groups:read") ||
    (!hasExplicitPermissions && isCaregiver(session));
  const canReadGroupsRef = useLatest(canReadGroups);

  // Use a ref for the refresh function to break dependency cycles
  const refreshRef = React.useRef<
    ((options?: { silent?: boolean; force?: boolean }) => Promise<void>) | null
  >(null);

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
  }, [tokenRef]);

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
        adminOverviewEnabled: false,
      }));
      return;
    }

    // Tracks whether the admin overview endpoint actually responded with data
    // (i.e. the setting is enabled). Only set to true on a successful response.
    let adminOverviewOk = false;

    try {
      // Admins: try the admin overview endpoint first. On any non-OK
      // response (403 = setting disabled; 5xx/network = transient), fall
      // back to the regular staff endpoint so the user at least keeps their
      // own supervisions instead of an empty sidebar.
      // Regular staff: fetch own supervised groups directly.
      const fetchSupervisedGroups = async (): Promise<Response> => {
        if (!isAdminRef.current) {
          return fetch("/api/me/groups/supervised", {
            headers: { "Content-Type": "application/json" },
            cache: "no-store",
          });
        }
        const adminResponse = await fetch("/api/active/supervisors/all", {
          headers: { "Content-Type": "application/json" },
          cache: "no-store",
        });
        if (!adminResponse.ok) {
          return fetch("/api/me/groups/supervised", {
            headers: { "Content-Type": "application/json" },
            cache: "no-store",
          });
        }
        adminOverviewOk = true;
        return adminResponse;
      };

      // Fetch supervised groups and Schulhof status in parallel.
      // Skip the Schulhof fetch for accounts without `groups:read`: the
      // backend gates /schulhof/status on that permission, so polling it
      // would only ever 403 (issue #846). The supervised-groups fetch below
      // hits permission-less /me endpoints and is safe for everyone.
      const [response, schulhofResponse] = await Promise.all([
        fetchSupervisedGroups(),
        canReadGroupsRef.current
          ? fetch("/api/active/schulhof/status", {
              headers: { "Content-Type": "application/json" },
              cache: "no-store",
            }).catch(() => null) // Schulhof is optional
          : Promise.resolve(null),
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
              adminOverviewEnabled: adminOverviewOk,
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
              adminOverviewEnabled: adminOverviewOk,
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
            adminOverviewEnabled: false,
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
          adminOverviewEnabled: false,
        };
      });
    }
  }, [canReadGroupsRef, isAdminRef, tokenRef]);

  // Check Schulhof status and add to supervised rooms if exists
  // Refresh all supervision states with debouncing
  const refresh = useCallback(
    async (options?: { silent?: boolean; force?: boolean }) => {
      const silent = options?.silent ?? false;
      const force = options?.force ?? false;
      // Prevent rapid successive refreshes (min 5 seconds between refreshes).
      // `force` bypasses the throttle for deliberate external triggers
      // (e.g. after saving a setting that changes supervision visibility).
      const now = Date.now();
      if (!force && now - lastRefreshRef.current < 5000) {
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

  // Store the refresh function only after its render commits.
  useEffect(() => {
    refreshRef.current = refresh;
  }, [refresh]);

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
        adminOverviewEnabled: false,
      });
    }
  }, [session?.user?.token]); // Only depend on token

  // A group handover or Vertretung changed which groups this account may open
  // (#2084). This provider holds its group list in local state behind its own
  // fetch, not SWR, so the global SSE cache invalidation cannot reach it — and
  // the sidebar's "Meine Gruppen" list is exactly what a colleague looks at
  // after a handover. useGlobalSSE announces the change on this window event
  // (mirroring the reminders / care-schedule decoupling) and the provider owns
  // the refetch. force: true bypasses the 5-second throttle, which a handover
  // arriving right after another refresh would otherwise swallow, leaving the
  // group invisible until the next minute tick.
  useEffect(() => {
    if (!session?.user?.token) return;

    const handleStale = () => {
      refreshRef.current?.({ silent: true, force: true }).catch(() => {
        // Intentionally ignored - silent background refresh
      });
    };

    window.addEventListener("phoenix:supervision-stale", handleStale);
    return () =>
      window.removeEventListener("phoenix:supervision-stale", handleStale);
  }, [session?.user?.token]);

  // Periodic refresh every minute for timely supervision updates (silent mode)
  useEffect(() => {
    if (!session?.user?.token) return;

    const interval = setInterval(() => {
      // Use silent refresh to avoid UI flicker - errors handled internally
      if (refreshRef.current) {
        refreshRef.current({ silent: true }).catch(() => {
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
 * Non-throwing variant for components shared between tenant and operator layouts.
 * Returns safe defaults when no SupervisionProvider is present.
 */
const EMPTY_SUPERVISION: SupervisionContextType = {
  hasGroups: false,
  isLoadingGroups: false,
  groups: [],
  isSupervising: false,
  supervisedRooms: [],
  isLoadingSupervision: false,
  adminOverviewEnabled: false,
  // eslint-disable-next-line no-empty-function -- safe no-op default for operator context
  refresh: async () => {},
};

export function useOptionalSupervision(): SupervisionContextType {
  const context = useContext(SupervisionContext);
  return context ?? EMPTY_SUPERVISION;
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
