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
import {
  hasEffectiveAdminScope,
  hasPermission,
  isCaregiver,
} from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import { useLatest } from "~/lib/hooks/use-latest";
import { useOperationalOverviewScope } from "~/lib/tenant-context";
import {
  deriveSupervision,
  sameGroups,
  sameSupervision,
  sortNavigationGroups,
  type DerivedSupervision,
  type SchulhofStatus,
  type SupervisedGroupPayload,
  type SupervisedRoom,
  type SupervisionSnapshot,
} from "~/lib/supervision-derive";
import type { NavigationEducationalGroup } from "~/lib/usercontext-helpers";

const logger = createLogger({ component: "SupervisionContext" });

const RESYNC_INTERVAL_MS = 5 * 60 * 1000;

interface SupervisionState extends DerivedSupervision {
  // Group supervision
  hasGroups: boolean;
  isLoadingGroups: boolean;
  groups: NavigationEducationalGroup[];

  // Room supervision (for active sessions)
  supervisedRooms: SupervisedRoom[];
  isLoadingSupervision: boolean;

  // overviewEnabled (from DerivedSupervision): true when the caller fetched
  // supervision rooms via the school-wide overview endpoint
  // (/api/active/supervisors/all), i.e. the server confirmed this person may
  // see every running module (#2380). Pages gate on this rather than on room
  // count, so a synthetic Schulhof entry never counts as an enabled overview.
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

const EMPTY_SUPERVISION_STATE: DerivedSupervision = {
  isSupervising: false,
  supervisedRoomId: undefined,
  supervisedRoomName: undefined,
  supervisedRooms: [],
  overviewEnabled: false,
};

function initialState(initial: SupervisionSnapshot | null): SupervisionState {
  if (!initial) {
    return {
      hasGroups: false,
      isLoadingGroups: true,
      groups: [],
      ...EMPTY_SUPERVISION_STATE,
      isLoadingSupervision: true,
    };
  }
  const groups = sortNavigationGroups(initial.groups ?? []);
  return {
    hasGroups: groups.length > 0,
    isLoadingGroups: false,
    groups,
    ...deriveSupervision(
      initial.supervised,
      initial.schulhof,
      initial.overviewOk,
    ),
    isLoadingSupervision: false,
  };
}

/**
 * Provider that manages dynamic supervision states
 * Checks for group assignments and active room supervision
 *
 * `initial` is the server-preloaded snapshot from the tenant layout (#2973):
 * when present, the first render already carries groups and rooms and the
 * mount fetch is skipped; the 5-minute resync and SSE triggers stay in place.
 */
export function SupervisionProvider({
  children,
  initial = null,
}: Readonly<{
  children: React.ReactNode;
  initial?: SupervisionSnapshot | null;
}>) {
  const { data: session } = useSession();

  const [state, setState] = useState<SupervisionState>(() =>
    initialState(initial),
  );
  const seededRef = React.useRef(initial !== null);

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
  const sessionHasEffectiveAdminScope = hasEffectiveAdminScope(session);

  // The tenant's configured scope tells us whether asking for the school-wide
  // list can succeed at all. It is a hint that saves a guaranteed 403 per
  // refresh, never the decision: the server answers every request itself, and
  // a 403 still falls back to the caller's own supervisions below.
  const overviewScope = useOperationalOverviewScope();
  const mayHaveOverview =
    sessionHasEffectiveAdminScope || overviewScope === "all_staff";
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
    sessionHasEffectiveAdminScope ||
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

  const applyGroups = useCallback((groupList: NavigationEducationalGroup[]) => {
    setState((prev) => {
      // Only update if value actually changed
      if (sameGroups(prev.groups, groupList) && !prev.isLoadingGroups) {
        return prev;
      }
      return {
        ...prev,
        hasGroups: groupList.length > 0,
        groups: groupList,
        isLoadingGroups: false,
      };
    });
  }, []);

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
          data?: { groups?: NavigationEducationalGroup[] };
          groups?: NavigationEducationalGroup[];
        };
        // Route wrapper wraps response as { success, data: { groups } }
        applyGroups(
          sortNavigationGroups(json.data?.groups ?? json.groups ?? []),
        );
      } else {
        applyGroups([]);
      }
    } catch {
      applyGroups([]);
    }
  }, [applyGroups, tokenRef]);

  const applySupervision = useCallback((next: DerivedSupervision) => {
    setState((prev) => {
      // Only update if values actually changed
      if (sameSupervision(prev, next) && !prev.isLoadingSupervision) {
        return prev;
      }
      return { ...prev, ...next, isLoadingSupervision: false };
    });
  }, []);

  // Check if user is supervising an active room (also fetches Schulhof status)
  const checkSupervision = useCallback(async () => {
    const token = tokenRef.current;
    if (!token) {
      applySupervision(EMPTY_SUPERVISION_STATE);
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
      let schulhof: SchulhofStatus | null = null;
      if (schulhofResponse?.ok) {
        // Response is double-wrapped: { success, data: { status, data: SchulhofStatus } }
        const schulhofJson = (await schulhofResponse.json()) as {
          data?: { data?: SchulhofStatus };
        };
        schulhof = schulhofJson.data?.data ?? null;
      }

      let supervised: SupervisedGroupPayload[] | null = null;
      if (response.ok) {
        const responseData = (await response.json()) as {
          success: boolean;
          message: string;
          data: SupervisedGroupPayload[] | null;
        };
        supervised = responseData.data ?? [];
      }

      applySupervision(deriveSupervision(supervised, schulhof, overviewOk));
    } catch {
      // On error, we can't fetch Schulhof either, so just clear
      applySupervision(EMPTY_SUPERVISION_STATE);
    }
  }, [applySupervision, canReadGroupsRef, mayHaveOverviewRef, tokenRef]);

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

  const previousOverviewScopeRef = React.useRef(overviewScope);

  // Saving the setting refreshes supervision before tenant metadata has
  // necessarily revalidated. Refresh once more after the resolved scope
  // changes so the room list cannot remain widened or restricted until SSE.
  useEffect(() => {
    if (previousOverviewScopeRef.current === overviewScope) return;
    previousOverviewScopeRef.current = overviewScope;
    void refreshRef.current?.({ silent: true, force: true });
  }, [overviewScope]);

  // Initial load and refresh on session changes only
  useEffect(() => {
    // Only refresh when session actually changes (not on every render)
    if (session?.user?.token) {
      // The server snapshot IS the initial load; count it as one so the
      // 5-second throttle applies exactly as after a fetched load.
      if (seededRef.current) {
        seededRef.current = false;
        lastRefreshRef.current = Date.now();
        return;
      }
      refreshRef.current?.().catch((err: unknown) => {
        logger.error("failed to refresh supervision context", {
          error: String(err),
        });
      });
    } else {
      // Clear state when no session
      seededRef.current = false;
      setState({
        hasGroups: false,
        isLoadingGroups: false,
        groups: [],
        ...EMPTY_SUPERVISION_STATE,
        isLoadingSupervision: false,
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
