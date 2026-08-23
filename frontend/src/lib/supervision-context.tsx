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
import { useOperationalOverviewScope } from "~/lib/tenant-context";

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
const RESYNC_INTERVAL_MS = 5 * 60 * 1000;

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

  // True when the caller fetched supervision rooms via the school-wide
  // overview endpoint (/api/active/supervisors/all), i.e. the server confirmed
  // this person may see and operate every running module (#2380). False when
  // the school keeps everyone on their own supervisions. Pages gate on this
  // rather than on room count, so a synthetic Schulhof entry never counts as
  // an enabled overview.
  overviewEnabled: boolean;
}

interface SupervisionContextType extends SupervisionState {
  refresh: (options?: {
    silent?: boolean;
    force?: boolean;
    groupsOnly?: boolean;
  }) => Promise<void>;
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
    overviewEnabled: false,
  });

  // Debounce mechanism to prevent rapid successive calls
  const isRefreshingRef = React.useRef(false);
  const lastRefreshRef = React.useRef<number>(0);
  const pendingGroupsRefreshRef = React.useRef(false);
  const pendingFullRefreshRef = React.useRef(false);

  // Store token and admin status in refs to avoid dependency loops.
  // EVERY caller with `groups:read` tries the school-wide overview endpoint
  // first — admins and caregivers alike (#2380). The server-side scope
  // setting is the single source of truth for whether all rooms are visible;
  // a 403 means this school keeps the caller on their own supervisions, and
  // the staff endpoint answers instead.
  const tokenRef = useLatest(session?.user?.token);
  const sessionIsAdmin = isAdmin(session);

  // The tenant's configured scope tells us whether asking for the school-wide
  // list can succeed at all. It is a hint that saves a guaranteed 403 per
  // refresh, never the decision: the server answers every request itself, and
  // a 403 still falls back to the caller's own supervisions below.
  const overviewScope = useOperationalOverviewScope();
  const mayHaveOverview =
    overviewScope === "all_staff" ||
    (overviewScope === "admins" && sessionIsAdmin);
  const mayHaveOverviewRef = useLatest(mayHaveOverview);

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
    | ((options?: {
        silent?: boolean;
        force?: boolean;
        groupsOnly?: boolean;
      }) => Promise<void>)
    | null
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
        overviewEnabled: false,
      }));
      return;
    }

    // Tracks whether the school-wide overview endpoint actually responded
    // with data. Only set to true on a successful response.
    let overviewOk = false;

    try {
      // Try the school-wide overview endpoint first for every caller who may
      // read groups at all. On any non-OK response (403 = this school keeps
      // the caller on their own supervisions; 5xx/network = transient), fall
      // back to the regular staff endpoint so the user at least keeps their
      // own supervisions instead of an empty sidebar.
      // The endpoint is gated on `groups:read`, so accounts without it skip
      // straight to the permission-less /me endpoint.
      const fetchSupervisedGroups = async (): Promise<Response> => {
        if (!canReadGroupsRef.current || !mayHaveOverviewRef.current) {
          return fetch("/api/me/groups/supervised", {
            headers: { "Content-Type": "application/json" },
            cache: "no-store",
          });
        }
        const overviewResponse = await fetch("/api/active/supervisors/all", {
          headers: { "Content-Type": "application/json" },
          cache: "no-store",
        });
        if (!overviewResponse.ok) {
          return fetch("/api/me/groups/supervised", {
            headers: { "Content-Type": "application/json" },
            cache: "no-store",
          });
        }
        overviewOk = true;
        return overviewResponse;
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
          const eligibleGroups = supervisedGroups.filter(
            (g) => g.room_id && g.room && g.room.name !== SCHULHOF_ROOM_NAME,
          );
          // Parallel sessions can share one room (#2265) — a room-name-only
          // label would render indistinguishable entries, so suffix the
          // activity name whenever a room appears more than once.
          const roomUseCount = new Map<number, number>();
          for (const g of eligibleGroups) {
            roomUseCount.set(
              g.room_id!,
              (roomUseCount.get(g.room_id!) ?? 0) + 1,
            );
          }
          let newSupervisedRooms: SupervisedRoom[] = eligibleGroups
            .map((g) => {
              const roomName = g.room?.name ?? `Room ${g.room_id}`;
              const shared = (roomUseCount.get(g.room_id!) ?? 0) > 1;
              return {
                id: g.room_id!.toString(),
                name:
                  shared && g.actual_group?.name
                    ? `${g.actual_group.name} · ${roomName}`
                    : roomName,
                groupId: g.id.toString(),
                groupName: g.actual_group?.name,
              };
            })
            .sort((a, b) => a.name.localeCompare(b.name, "de"));

          // Always add Schulhof at the end if it exists
          if (schulhofRoom) {
            newSupervisedRooms = [...newSupervisedRooms, schulhofRoom];
          }

          setState((prev) => {
            // Active groups can change while the physical room stays the same.
            const prevRoomKeys = prev.supervisedRooms
              .map((r) => `${r.id}:${r.groupId}`)
              .join(",");
            const newRoomKeys = newSupervisedRooms
              .map((r) => `${r.id}:${r.groupId}`)
              .join(",");
            if (
              prev.isSupervising &&
              prev.supervisedRoomId === newRoomId &&
              prev.supervisedRoomName === newRoomName &&
              prevRoomKeys === newRoomKeys &&
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
              overviewEnabled: overviewOk,
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
              overviewEnabled: overviewOk,
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
            overviewEnabled: false,
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
          overviewEnabled: false,
        };
      });
    }
  }, [canReadGroupsRef, mayHaveOverviewRef, tokenRef]);

  // Check Schulhof status and add to supervised rooms if exists
  // Refresh all supervision states with debouncing
  const refresh = useCallback(
    async (options?: {
      silent?: boolean;
      force?: boolean;
      groupsOnly?: boolean;
    }) => {
      const silent = options?.silent ?? false;
      const force = options?.force ?? false;
      // Skips the supervision half (own supervised rooms + Schulhof status)
      // for triggers that provably cannot have changed it.
      const groupsOnly = options?.groupsOnly ?? false;
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
      const work = groupsOnly
        ? [checkGroups()]
        : [checkGroups(), checkSupervision()];
      await Promise.all(work).finally(() => {
        isRefreshingRef.current = false;
        if (pendingFullRefreshRef.current) {
          pendingFullRefreshRef.current = false;
          pendingGroupsRefreshRef.current = false;
          void refreshRef.current?.({ silent: true, force: true });
        } else if (pendingGroupsRefreshRef.current) {
          pendingGroupsRefreshRef.current = false;
          void refreshRef.current?.({
            silent: true,
            force: true,
            groupsOnly: true,
          });
        }
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
        overviewEnabled: false,
      });
    }
  }, [session?.user?.token]); // Only depend on token

  // SSE is a trigger channel, not durable state. A low-frequency resync
  // repairs missed events or an unavailable connection without restoring the
  // former per-minute request load.
  useEffect(() => {
    if (!session?.user?.token) return;

    const interval = setInterval(() => {
      refreshRef.current?.({ silent: true, force: true }).catch(() => {
        // Intentionally ignored - silent background refresh
      });
    }, RESYNC_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [session?.user?.token]);

  // SSE announces access changes and activity lifecycle changes. The provider
  // owns these raw fetches, so it also owns the precise refresh scope.
  useEffect(() => {
    if (!session?.user?.token) return;

    const handleStale = (event: Event) => {
      const groupsOnly =
        !(event instanceof CustomEvent) || event.detail?.groupsOnly !== false;
      if (isRefreshingRef.current) {
        if (groupsOnly) {
          pendingGroupsRefreshRef.current = true;
        } else {
          pendingFullRefreshRef.current = true;
        }
        return;
      }
      refreshRef
        .current?.({ silent: true, force: true, groupsOnly })
        .catch(() => {
          // Intentionally ignored - silent background refresh
        });
    };

    window.addEventListener("phoenix:supervision-stale", handleStale);
    return () => {
      pendingGroupsRefreshRef.current = false;
      pendingFullRefreshRef.current = false;
      window.removeEventListener("phoenix:supervision-stale", handleStale);
    };
  }, [session?.user?.token]);

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
  overviewEnabled: false,
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
