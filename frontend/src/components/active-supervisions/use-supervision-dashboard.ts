"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import { useSWRAuth } from "~/lib/swr";
import { createLogger } from "~/lib/logger";
import type { BulkPickupTime } from "~/lib/pickup-schedule-api";
import type { BulkArrivalTime } from "~/lib/student-arrival-api";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";
import {
  SCHULHOF_ROOM_NAME,
  SCHULHOF_TAB_ID,
  buildGroupNameToIdMap,
  mapSupervisedGroupsToRooms,
  mapVisitsToSupervisionStudents,
  resolveSupervisionSelection,
} from "~/components/active-supervisions/view-model";
import type {
  ActiveSupervisionRoom,
  ActiveSupervisionStudent,
  MinimalActiveGroup,
  SchulhofStatusResponse,
  SupervisionSessionInfo,
} from "~/components/active-supervisions/view-model";

const logger = createLogger({ component: "useSupervisionDashboard" });

// BFF response type for consolidated dashboard data
interface BFFDashboardResponse {
  businessDay: string;
  spontaneousStartAvailability: {
    available: boolean;
    blockedReason?: "weekend";
  };
  supervisedGroups: Array<{
    id: string;
    name: string;
    isCurrentUserSupervising?: boolean;
    room_id?: string;
    room?: { id: string; name: string; color?: string | null };
  }>;
  unclaimedGroups: Array<{
    id: string;
    name: string;
    room?: { name: string };
  }>;
  currentStaff: { id: string } | null;
  educationalGroups: Array<{
    id: string;
    name: string;
    room?: { name: string };
  }>;
  firstRoomVisits: Array<{
    studentId: string;
    studentName: string;
    schoolClass: string;
    groupName: string;
    activeGroupId: string;
    checkInTime: string;
    actualArrivalTime?: string;
    actualPickupTime?: string;
    isActive: boolean;
    sick?: boolean;
    sickSince?: string;
    excused?: boolean;
    excusedSince?: string;
    class_trip?: boolean;
    class_trip_since?: string;
    // Authenticated proxy URL — backend rewrites the raw /uploads path
    // to /api/students/{id}/photo/{filename} before sending it down.
    photoUrl?: string;
  }>;
  firstRoomId: string | null;
  schulhofStatus: SchulhofStatusResponse | null;
  capabilities?: {
    webSpontaneousActivitiesEnabled: boolean;
  };
  // Plan windows of today's running sessions for tab labels (#2265);
  // optional so a cached older BFF payload degrades to name-only labels
  activeSessions?: Array<{
    activeGroupId: string;
    instanceId: string;
    title: string;
    startTime: string;
    endTime: string;
  }>;
  plannedNow: PlannedTimetableInstance[];
  // Folded-in sections of the selected session (#2096): the aggregate
  // resolves group_id (or the first supervised session) and ships its
  // visits, tracking indicators, and pickup/arrival times in the same
  // response. Optional so a cached payload from an older backend degrades
  // gracefully instead of breaking the page.
  selectedGroupId?: string | null;
  trackingIndicators?: TrackingIndicatorsResponse;
  pickupTimes?: Array<{
    studentId: string;
    date: string;
    weekdayName: string;
    pickupTime: string | null;
    isException: boolean;
    notes: string;
    dayNotes: Array<{ id: string; content: string }>;
  }>;
  arrivalTimes?: Array<{
    studentId: string;
    date: string;
    weekdayName: string;
    expectedArrival: string | null;
    isException: boolean;
    notes: string;
    dayNotes: Array<{ id: string; content: string }>;
  }>;
}

export interface SupervisionDashboardOptions {
  /** Bearer token of the signed-in user; no fetch while absent. */
  readonly sessionToken: string | undefined;
  /** `?session=` — precise session (active group) selection from the URL. */
  readonly sessionParam: string | null;
  /** Legacy `?room=` entry point (sidebar, old links). */
  readonly roomParam: string | null;
}

export interface SupervisionDashboard {
  // Fetch surface
  readonly dashboardError: Error | undefined;
  readonly mutateDashboard: () => Promise<unknown>;
  /** Bumps the SWR key — the "hard refresh" the claim/toggle flows use. */
  readonly refresh: () => void;

  // Derived data (pure projections of the retained aggregate)
  readonly allRooms: ActiveSupervisionRoom[];
  readonly plannedNow: PlannedTimetableInstance[];
  readonly currentStaffId: string | undefined;
  readonly myGroupRooms: string[];
  readonly myGroupIds: string[];
  readonly groupNameToIdMap: Map<string, string>;
  readonly cachedActiveGroups: MinimalActiveGroup[];
  readonly schulhofStatus: SchulhofStatusResponse | null;
  readonly schulhofTabEnabled: boolean;
  readonly schulhofTabAvailable: boolean;
  readonly webSpontaneousActivitiesEnabled: boolean;
  readonly businessDay: string | undefined;
  readonly spontaneousStartAvailability:
    BFFDashboardResponse["spontaneousStartAvailability"] | undefined;
  readonly sessionInfoByActiveGroup: Map<string, SupervisionSessionInfo>;
  readonly trackingData: TrackingIndicatorsResponse | undefined;
  readonly pickupTimesData: Map<string, BulkPickupTime> | undefined;
  readonly arrivalTimesData: Map<string, BulkArrivalTime> | undefined;
  readonly students: ActiveSupervisionStudent[];

  // Selection
  readonly selectedRoomId: string | null;
  readonly isSchulhofTabSelected: boolean;
  readonly selectedTimetableInstanceId: string | null;
  readonly currentRoom: ActiveSupervisionRoom | null;
  readonly setSelectedTimetableInstanceId: (id: string | null) => void;
  /**
   * Switch to another supervised session. Awaitable: resolves after the
   * aggregate re-ran for the target; a rejection surfaces as `error`.
   */
  readonly switchToRoom: (sessionId: string) => Promise<void>;
  /** Select the permanent Schulhof tab (state only — no navigation). */
  readonly selectSchulhof: (options?: {
    clearTimetableInstance?: boolean;
  }) => void;
  /** Leave the Schulhof tab (before switching to a normal session). */
  readonly deselectSchulhof: () => void;
  /**
   * Adopt a session this client just started (planned/spontaneous start):
   * selects it and pre-seeds the fetch parameter so the follow-up
   * revalidation requests the new session, not the previous one.
   */
  readonly adoptSession: (
    activeGroupId: string,
    timetableInstanceId: string | null,
  ) => void;

  // Page-level status
  readonly hasAccess: boolean | null;
  readonly isInitialLoading: boolean;
  readonly isSwitchingSession: boolean;
  readonly isWaitingForUrlRoomSelection: boolean;
  readonly error: string | null;
  readonly setError: (message: string | null) => void;
}

/**
 * The data spine of the "Aktuelle Aufsicht" page (#2421).
 *
 * Owns the aggregated dashboard fetch (#2096) — including the one-time 403
 * retry without group_id — the session selection, and every projection the
 * page renders. The page consumes the result; it holds no copy of SWR data.
 *
 * Contracts this hook keeps:
 * - The SWR key keeps the `active-supervision-dashboard-` prefix — the
 *   global SSE invalidation (use-global-sse.ts) and the room-derived cache
 *   registry match on it. Because the key cannot carry the selection, the
 *   selected session travels to the fetcher through `requestedGroupIdRef`
 *   (the request parameter the stable-key contract forces), and selection
 *   changes re-run the aggregate via `mutate` instead of a key change.
 * - The backend's `businessDay` is the only calendar day this surface uses.
 *   Browser time does not choose a school day or a spontaneous start window.
 * - A fetched aggregate is retained as a snapshot until the next one
 *   arrives (mirroring SWR's `keepPreviousData` across key changes), so
 *   values that must survive refetch gaps — e.g. the Schulhof tab
 *   capability — never flicker off.
 */
export function useSupervisionDashboard(
  options: SupervisionDashboardOptions,
): SupervisionDashboard {
  const { sessionToken, sessionParam, roomParam } = options;
  const { data: session } = useSession();
  const accountId = session?.user.id;

  const [refreshKey, setRefreshKey] = useState(0);
  const refresh = useCallback(() => setRefreshKey((prev) => prev + 1), []);

  // Selection state — genuine UI state, not a copy of server data.
  const [selectedRoomId, setSelectedRoomId] = useState<string | null>(null);
  const [isSchulhofTabSelected, setIsSchulhofTabSelected] = useState(false);
  const [selectedTimetableInstanceId, setSelectedTimetableInstanceId] =
    useState<string | null>(null);
  const [isSwitchingSession, setIsSwitchingSession] = useState(false);
  // Mutation/switch errors; dashboard fetch errors are folded in by the
  // effect below and cleared again when a fresh aggregate arrives.
  const [error, setError] = useState<string | null>(null);

  // The selected session for the fetcher. The SWR key is contractually
  // stable (SSE invalidation matches on its prefix), so the selection cannot
  // ride in the key — this ref is the request parameter instead.
  const requestedGroupIdRef = useRef<string | null>(null);

  // SWR-based aggregate fetching with caching. The key prefix
  // "active-supervision-dashboard-" is invalidated by global SSE on relevant
  // events; the account suffix prevents cached data crossing account changes
  // in the same tenant. The aggregate response is the only source for the
  // Berlin business day; browser time never participates in the request key.
  const {
    data: dashboardData,
    error: dashboardError,
    mutate: mutateDashboard,
    isLoading: isDashboardLoading,
  } = useSWRAuth<BFFDashboardResponse>(
    sessionToken
      ? `active-supervision-dashboard-${refreshKey}-${accountId}`
      : null,
    async () => {
      logger.debug("SWR fetching dashboard data");
      const start = performance.now();

      const fetchDashboard = async (groupId: string | null) => {
        const query =
          groupId && /^[1-9]\d{0,18}$/.test(groupId)
            ? `?group_id=${encodeURIComponent(groupId)}`
            : "";
        return fetch(`/api/active-supervision-dashboard${query}`, {
          headers: {
            Authorization: `Bearer ${sessionToken}`,
            "Content-Type": "application/json",
          },
        });
      };

      const requestedGroupId = requestedGroupIdRef.current;
      let response = await fetchDashboard(requestedGroupId);
      // A stale selection (supervision revoked, session ended) is a backend
      // 403 — retry once without group_id so the backend resolves the
      // caller's first supervised session; the selection effects then
      // re-align from the response.
      if (!response.ok && response.status === 403 && requestedGroupId) {
        response = await fetchDashboard(null);
      }

      if (!response.ok) {
        // No silent fallback fan-out (#2096): a failed aggregate is an
        // error, never a partial-empty payload — admins included.
        throw new Error(`BFF request failed: ${response.status}`);
      }

      const bffData = (await response.json()) as {
        data: BFFDashboardResponse;
      };

      logger.debug("SWR fetch complete", {
        duration_ms: Math.round(performance.now() - start),
      });
      return bffData.data;
    },
    {
      keepPreviousData: true,
      // Re-fetch server time without deriving a browser calendar day. Focus
      // covers backgrounded tabs; polling advances a continuously open tab
      // across the Berlin midnight boundary.
      refreshInterval: 60_000,
      revalidateOnFocus: true,
    },
  );

  // Retain the latest aggregate across renders where SWR reports no data
  // (key roll, transient cache states). This is the documented exception to
  // "derive, don't copy": ONE snapshot adjusted during render (the React
  // "storing information from previous renders" pattern), replacing the
  // former dozen per-field setState copies.
  const [snapshot, setSnapshot] = useState<BFFDashboardResponse | null>(null);
  const [snapshotSessionToken, setSnapshotSessionToken] =
    useState(sessionToken);
  if (snapshotSessionToken !== sessionToken) {
    // A token change can mean another account signed in on this tenant. Do
    // not retain the previous account's aggregate while the new key loads.
    setSnapshotSessionToken(sessionToken);
    setSnapshot(null);
  } else if (
    dashboardData &&
    !isDashboardLoading &&
    dashboardData !== snapshot
  ) {
    setSnapshot(dashboardData);
  }

  // A fresh aggregate supersedes any stale mutation/switch error, exactly
  // like the former sync effect's unconditional setError(null).
  const lastClearedSnapshotRef = useRef<BFFDashboardResponse | null>(null);
  useEffect(() => {
    if (!snapshot || lastClearedSnapshotRef.current === snapshot) return;
    lastClearedSnapshotRef.current = snapshot;
    setError(null);
  }, [snapshot]);

  // Fold dashboard fetch failures into the error surface.
  useEffect(() => {
    if (!dashboardError) return;
    setError(
      dashboardError.message.includes("403")
        ? "Sie haben aktuell keinen aktiven Raum zur Supervision."
        : "Fehler beim Laden der Aktivitätsdaten.",
    );
  }, [dashboardError]);

  // ---- Pure projections of the snapshot ----

  const allRoomsBase = useMemo(
    () =>
      snapshot ? mapSupervisedGroupsToRooms(snapshot.supervisedGroups) : [],
    [snapshot],
  );

  const groupNameToIdMap = useMemo(
    () =>
      snapshot
        ? buildGroupNameToIdMap(snapshot.educationalGroups)
        : new Map<string, string>(),
    [snapshot],
  );

  const myGroupRooms = useMemo(
    () =>
      snapshot?.educationalGroups
        .map((group) => group.room?.name)
        .filter((name): name is string => !!name) ?? [],
    [snapshot],
  );

  const myGroupIds = useMemo(
    () => snapshot?.educationalGroups.map((group) => group.id) ?? [],
    [snapshot],
  );

  const cachedActiveGroups = useMemo<MinimalActiveGroup[]>(() => {
    if (!snapshot || snapshot.supervisedGroups.length === 0) return [];
    return [
      ...snapshot.supervisedGroups.map((g) => ({
        id: g.id,
        room: g.room ? { name: g.room.name } : undefined,
      })),
      ...snapshot.unclaimedGroups.map((g) => ({
        id: g.id,
        room: g.room,
      })),
    ];
  }, [snapshot]);

  const plannedNow = useMemo(() => snapshot?.plannedNow ?? [], [snapshot]);

  const currentStaffId = snapshot?.currentStaff?.id;

  // Clear stale status on a failed revalidation as well: the dedicated
  // Schulhof workflow must fail closed while SWR may still expose the
  // previous dashboard data.
  const schulhofStatus = useMemo(
    () => (dashboardError ? null : (snapshot?.schulhofStatus ?? null)),
    [dashboardError, snapshot],
  );

  // #2161: the permanent Schulhof tab (one-tap "Beaufsichtigen") rides on
  // the generic spontaneous-start flow, so it is gated on the same
  // capability. Tenants without it see the yard as a normal room tab while
  // a planned or spontaneous session runs there. Read from the retained
  // snapshot so transient dashboard refetches don't drop the tab.
  const webSpontaneousActivitiesEnabled =
    snapshot?.capabilities?.webSpontaneousActivitiesEnabled === true;
  const businessDay = snapshot?.businessDay;
  const spontaneousStartAvailability = snapshot?.spontaneousStartAvailability;
  const schulhofTabEnabled = webSpontaneousActivitiesEnabled;
  const schulhofTabAvailable =
    schulhofTabEnabled && schulhofStatus?.exists === true;

  // Title + plan window per running session, so tab labels can show
  // "Aktivitätsname · Planzeit" (#2265). Sessions without a timetable
  // instance fall back to the session/room name.
  const sessionInfoByActiveGroup = useMemo(() => {
    const map = new Map<string, SupervisionSessionInfo>();
    for (const liveSession of snapshot?.activeSessions ?? []) {
      map.set(liveSession.activeGroupId, {
        title: liveSession.title,
        timeRange: `${liveSession.startTime}–${liveSession.endTime}`,
      });
    }
    return map;
  }, [snapshot]);

  // Tracking indicators, pickup times, and arrival times ride in the
  // aggregate for the selected session (#2096) — no separate fetches. SSE
  // invalidation of the dashboard key keeps them fresh together with the
  // visits they belong to.
  const trackingData = snapshot?.trackingIndicators;

  const pickupTimesData = useMemo(() => {
    if (!snapshot?.pickupTimes) return undefined;
    const map = new Map<string, BulkPickupTime>();
    for (const pickup of snapshot.pickupTimes) {
      map.set(pickup.studentId, {
        studentId: pickup.studentId,
        date: pickup.date,
        weekdayName: pickup.weekdayName,
        pickupTime: pickup.pickupTime ?? undefined,
        isException: pickup.isException,
        notes: pickup.notes || undefined,
        dayNotes: pickup.dayNotes,
      });
    }
    return map;
  }, [snapshot]);

  const arrivalTimesData = useMemo(() => {
    if (!snapshot?.arrivalTimes) return undefined;
    const map = new Map<string, BulkArrivalTime>();
    for (const arrival of snapshot.arrivalTimes) {
      map.set(arrival.studentId, {
        studentId: arrival.studentId,
        date: arrival.date,
        weekdayName: arrival.weekdayName,
        expectedArrival: arrival.expectedArrival ?? undefined,
        isException: arrival.isException,
        notes: arrival.notes || undefined,
        dayNotes: arrival.dayNotes,
      });
    }
    return map;
  }, [snapshot]);

  // The session whose visits the aggregate carries. A payload without
  // selectedGroupId (older backend) keeps the former semantics: its visits
  // belong to the first room.
  const resolvedRoom = useMemo(() => {
    if (!snapshot) return null;
    const firstRoom = allRoomsBase[0] ?? null;
    return snapshot.selectedGroupId
      ? (allRoomsBase.find((r) => r.id === snapshot.selectedGroupId) ?? null)
      : firstRoom;
  }, [snapshot, allRoomsBase]);

  // A URL that names a different session/room than the aggregate resolved:
  // its visits must not flash under the wrong heading while the selection
  // effect below is still switching (#2096).
  const isUrlTargetingDifferentRoom = useMemo(() => {
    const firstRoom = allRoomsBase[0];
    return sessionParam
      ? sessionParam !== SCHULHOF_TAB_ID &&
          allRoomsBase.some((room) => room.id === sessionParam) &&
          firstRoom?.id !== sessionParam
      : !!roomParam &&
          roomParam !== SCHULHOF_TAB_ID &&
          allRoomsBase.some((room) => room.room_id === roomParam) &&
          firstRoom?.room_id !== roomParam;
  }, [allRoomsBase, sessionParam, roomParam]);

  // The visits of the selected session, derived instead of copied: shown
  // only when the aggregate's resolved session IS the one the user is
  // looking at — an SSE revalidation while the user views another session
  // must not surface foreign visits, and a pending switch shows nothing.
  const students = useMemo<ActiveSupervisionStudent[]>(() => {
    if (!snapshot) return [];
    if (isSchulhofTabSelected) {
      if (
        snapshot.selectedGroupId &&
        schulhofStatus?.isUserSupervising &&
        schulhofStatus.activeGroupId === snapshot.selectedGroupId
      ) {
        return mapVisitsToSupervisionStudents(snapshot.firstRoomVisits, {
          roomName: SCHULHOF_ROOM_NAME,
          groupNameToId: groupNameToIdMap,
        });
      }
      return [];
    }
    if (isUrlTargetingDifferentRoom) return [];
    if (!resolvedRoom) return [];
    if (selectedRoomId && selectedRoomId !== resolvedRoom.id) return [];
    return mapVisitsToSupervisionStudents(snapshot.firstRoomVisits, {
      roomName: resolvedRoom.room_name,
      roomColor: resolvedRoom.room_color,
      groupNameToId: groupNameToIdMap,
    });
  }, [
    snapshot,
    isSchulhofTabSelected,
    schulhofStatus,
    groupNameToIdMap,
    isUrlTargetingDifferentRoom,
    resolvedRoom,
    selectedRoomId,
  ]);

  // Stamp the resolved session's live count onto its room entry (badge).
  const allRooms = useMemo(() => {
    if (!resolvedRoom || isSchulhofTabSelected || isUrlTargetingDifferentRoom)
      return allRoomsBase;
    if (selectedRoomId && selectedRoomId !== resolvedRoom.id)
      return allRoomsBase;
    return allRoomsBase.map((room) =>
      room.id === resolvedRoom.id
        ? { ...room, student_count: students.length }
        : room,
    );
  }, [
    allRoomsBase,
    resolvedRoom,
    isSchulhofTabSelected,
    isUrlTargetingDifferentRoom,
    selectedRoomId,
    students.length,
  ]);

  // Current selected session (null if Schulhof tab is selected but the user
  // isn't supervising).
  const currentRoom = useMemo(
    () =>
      isSchulhofTabSelected
        ? schulhofStatus?.isUserSupervising && schulhofStatus?.activeGroupId
          ? {
              id: schulhofStatus.activeGroupId,
              name: SCHULHOF_ROOM_NAME,
              room_name: SCHULHOF_ROOM_NAME,
              room_id: schulhofStatus.roomId ?? undefined,
              student_count: schulhofStatus.studentCount,
            }
          : null
        : (allRooms.find((r) => r.id === selectedRoomId) ??
          allRooms[0] ??
          null),
    [isSchulhofTabSelected, schulhofStatus, allRooms, selectedRoomId],
  );

  const hasAccess: boolean | null = dashboardError?.message.includes("403")
    ? false
    : snapshot
      ? true
      : null;

  const isInitialLoading = !snapshot && !dashboardError;

  const isWaitingForUrlRoomSelection = sessionParam
    ? sessionParam !== SCHULHOF_TAB_ID &&
      allRooms.some((room) => room.id === sessionParam) &&
      currentRoom?.id !== sessionParam
    : !!roomParam &&
      roomParam !== SCHULHOF_TAB_ID &&
      allRooms.some((room) => room.room_id === roomParam) &&
      // A selected session inside the named room settles a room-keyed URL —
      // parallel sessions share the room, so never wait for a "better" match.
      currentRoom?.room_id !== roomParam;

  // ---- Selection adjustments (state follows the refreshed aggregate) ----

  // Lock in the resolved session when nothing is selected yet (so the URL
  // sync below won't "switch" to it via localStorage), and reset a selection
  // whose session vanished from the refreshed list (supervision revoked,
  // session ended) so the visible data stays in sync with the UI.
  useEffect(() => {
    if (!snapshot) return;
    if (selectedRoomId && !allRoomsBase.some((r) => r.id === selectedRoomId)) {
      setSelectedRoomId(resolvedRoom?.id ?? allRoomsBase[0]?.id ?? null);
      return;
    }
    if (
      !isSchulhofTabSelected &&
      !selectedRoomId &&
      !isUrlTargetingDifferentRoom &&
      resolvedRoom
    ) {
      setSelectedRoomId(resolvedRoom.id);
    }
  }, [
    snapshot,
    allRoomsBase,
    resolvedRoom,
    selectedRoomId,
    isSchulhofTabSelected,
    isUrlTargetingDifferentRoom,
  ]);

  // Leave the Schulhof tab when its capability or provisioning disappears.
  useEffect(() => {
    if (schulhofTabAvailable || !isSchulhofTabSelected) return;
    setIsSchulhofTabSelected(false);
    setSelectedRoomId(allRoomsBase[0]?.id ?? null);
    setSelectedTimetableInstanceId(null);
  }, [allRoomsBase, isSchulhofTabSelected, schulhofTabAvailable]);

  // Auto-select the Schulhof tab when it's the only available option.
  useEffect(() => {
    if (
      allRoomsBase.length === 0 &&
      schulhofTabAvailable &&
      !isSchulhofTabSelected
    ) {
      setIsSchulhofTabSelected(true);
    }
  }, [allRoomsBase.length, schulhofTabAvailable, isSchulhofTabSelected]);

  // ---- Selection → fetch reconciliation ----

  // The session the UI explicitly wants from the aggregate. Null while the
  // backend's default (first supervised session) is fine — a fetch without
  // group_id resolves exactly that, so no reconciliation is needed then.
  const explicitDesiredGroupId = isSchulhofTabSelected
    ? schulhofStatus?.isUserSupervising && schulhofStatus.activeGroupId
      ? schulhofStatus.activeGroupId
      : null
    : selectedRoomId;
  const resolvedGroupId = resolvedRoom?.id ?? null;
  const hasSnapshot = snapshot !== null;

  // Re-run the aggregate when the explicitly selected session changes and
  // the cached aggregate belongs to another one — the data-driven paths
  // (Schulhof toggle materializing a session, localStorage restore) that
  // never go through switchToRoom. Loop-safe without a comparison ref: the
  // effect fires on VALUE changes of desired/resolved only, so a response
  // that keeps resolving another session cannot re-trigger it.
  useEffect(() => {
    requestedGroupIdRef.current = explicitDesiredGroupId;
    if (!explicitDesiredGroupId || !hasSnapshot) return;
    if (resolvedGroupId === explicitDesiredGroupId) return;
    void Promise.resolve(mutateDashboard()).catch(() => {
      // Errors surface via dashboardError handling
    });
  }, [explicitDesiredGroupId, resolvedGroupId, hasSnapshot, mutateDashboard]);

  // ---- Selection actions ----

  const switchToRoom = useCallback(
    async (sessionId: string) => {
      if (sessionId === selectedRoomId) return;
      const targetRoom = allRoomsBase.find((r) => r.id === sessionId);
      if (!targetRoom) return;

      setIsSwitchingSession(true);
      setSelectedTimetableInstanceId(null);
      setSelectedRoomId(sessionId);

      try {
        // Re-run the aggregate for the newly selected session; the derived
        // projections pick up its visits and student count on arrival.
        requestedGroupIdRef.current = sessionId;
        await mutateDashboard();
        setError(null);
      } catch (err) {
        // Handle 403 gracefully - show message but don't break the UI
        if (err instanceof Error && err.message.includes("403")) {
          setError(
            `Keine Berechtigung für "${targetRoom.name}". Kontaktieren Sie einen Administrator.`,
          );
        } else {
          setError("Fehler beim Laden der Raumdaten.");
          logger.error("failed to load room data", {
            error: err instanceof Error ? err.message : String(err),
          });
        }
      } finally {
        setIsSwitchingSession(false);
      }
    },
    [selectedRoomId, allRoomsBase, mutateDashboard],
  );

  const selectSchulhof = useCallback(
    (opts?: { clearTimetableInstance?: boolean }) => {
      setIsSchulhofTabSelected(true);
      setSelectedRoomId(null);
      if (opts?.clearTimetableInstance) {
        setSelectedTimetableInstanceId(null);
      }
    },
    [],
  );

  const deselectSchulhof = useCallback(() => {
    setIsSchulhofTabSelected(false);
  }, []);

  const adoptSession = useCallback(
    (activeGroupId: string, timetableInstanceId: string | null) => {
      // Pre-seed the request parameter so the caller's follow-up
      // revalidation targets the new session immediately.
      requestedGroupIdRef.current = activeGroupId;
      setSelectedTimetableInstanceId(timetableInstanceId);
      setSelectedRoomId(activeGroupId);
      setIsSchulhofTabSelected(false);
    },
    [],
  );

  // Sync the selected session with the URL / localStorage. The resolution
  // order (session param > legacy room param > saved session > saved room)
  // lives in resolveSupervisionSelection; this effect only executes the
  // target it returns. A "none" target keeps the current selection — the
  // resolver never switches between parallel sessions in the same room
  // just because a refresh re-resolved a room-keyed URL (#2265).
  useEffect(() => {
    const target = resolveSupervisionSelection({
      sessionParam,
      roomParam,
      savedSessionId: localStorage.getItem("supervision-last-session"),
      savedRoomId: localStorage.getItem("sidebar-last-room"),
      rooms: allRoomsBase,
      currentSessionId: selectedRoomId,
      schulhofAvailable: schulhofTabAvailable,
    });

    if (target.kind === "schulhof") {
      if (!isSchulhofTabSelected) {
        selectSchulhof();
      }
      return;
    }
    if (allRoomsBase.length === 0) return;
    if (target.kind === "session") {
      if (isSchulhofTabSelected) {
        setIsSchulhofTabSelected(false);
      }
      localStorage.setItem("supervision-last-session", target.sessionId);
      void switchToRoom(target.sessionId);
      return;
    }
    if (target.kind === "persist-first") {
      // Nothing saved (e.g. fresh login) — persist the default so the next
      // sidebar click and reload land on the same session.
      const firstRoom = allRoomsBase[0];
      if (firstRoom) {
        localStorage.setItem("supervision-last-session", firstRoom.id);
        if (firstRoom.room_id) {
          localStorage.setItem("sidebar-last-room", firstRoom.room_id);
        }
      }
    }
    // "none": already in sync
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    allRoomsBase,
    sessionParam,
    roomParam,
    schulhofTabAvailable,
    schulhofStatus?.activeGroupId,
    schulhofStatus?.isUserSupervising,
  ]);

  return {
    dashboardError: dashboardError ?? undefined,
    mutateDashboard,
    refresh,
    allRooms,
    plannedNow,
    currentStaffId,
    myGroupRooms,
    myGroupIds,
    groupNameToIdMap,
    cachedActiveGroups,
    schulhofStatus,
    schulhofTabEnabled,
    schulhofTabAvailable,
    webSpontaneousActivitiesEnabled,
    businessDay,
    spontaneousStartAvailability,
    sessionInfoByActiveGroup,
    trackingData,
    pickupTimesData,
    arrivalTimesData,
    students,
    selectedRoomId,
    isSchulhofTabSelected,
    selectedTimetableInstanceId,
    currentRoom,
    setSelectedTimetableInstanceId,
    switchToRoom,
    selectSchulhof,
    deselectSchulhof,
    adoptSession,
    hasAccess,
    isInitialLoading,
    isSwitchingSession,
    isWaitingForUrlRoomSelection,
    error,
    setError,
  };
}
