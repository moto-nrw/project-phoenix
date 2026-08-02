/**
 * Global SSE Hook
 *
 * Establishes a single SSE connection for the entire authenticated app.
 * When events arrive, it invalidates relevant SWR caches using targeted
 * keys (per active_group_id) instead of broad pattern matching.
 *
 * Debounces rapid events (e.g. morning check-in burst) so 10 checkins
 * within 500ms trigger a single refetch, not 10.
 *
 * This hook should be called ONCE in the auth layout to:
 * 1. Maintain a single SSE connection (not per-page)
 * 2. Automatically invalidate caches when events arrive
 * 3. Provide connection status for debugging/UI indicators
 *
 * @example
 * ```tsx
 * // In app/layout.tsx or auth layout wrapper
 * const { status } = useGlobalSSE();
 * ```
 */

"use client";

import { useCallback, useRef } from "react";
import { mutate } from "swr";
import { useSession } from "next-auth/react";
import { useSSE } from "~/lib/hooks/use-sse";
import { dispatchPhoenixNotification } from "~/lib/notification-events";
import { ROOM_LIST_CACHE_KEYS } from "~/lib/swr/room-derived-caches";
import {
  notifyStudentCompanionDisplayChanged,
  notifyStudentCompanionsChanged,
} from "~/lib/student-companion-api";
import type { SSEEvent, SSEHookState } from "~/lib/sse-types";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GlobalSSE" });

export const DEBOUNCE_MS = 500;
// Bounded random jitter added on top of the debounce (#2057). Every client of
// a tenant receives the same broadcast at the same instant; without jitter all
// of them fired their revalidations exactly DEBOUNCE_MS later, producing
// synchronized request herds (85 req/s bursts, saturated DB pool). The jitter
// is drawn ONCE per burst so the trailing debounce still collapses a burst
// into a single flush.
export const FLUSH_JITTER_MS = 1000;
// Upper bound on how long a sustained event stream (morning check-in rush) can
// keep postponing the flush. Measured from the first event of a burst.
export const MAX_FLUSH_WAIT_MS = 3000;

// The dashboard caches that reflect live check-in/out counts. Deliberately an
// explicit list instead of the old `key.includes("dashboard")`: that substring
// also matched "ogs-dashboard" (the OGS page's former 5-request BFF, since
// replaced by the aggregated ogs-students-{gid} live view, #2056) and
// "staff-dashboard-summary-" (staff time tracking, which has its own
// staff_time_tracking_changed trigger + refresh interval) — refetching both on
// every check-in was a large part of the #2057 request herd.
const DASHBOARD_COUNT_CACHE_KEYS = [
  "dashboard-analytics",
  "active-supervision-dashboard-",
] as const;

// Every cache family derived from work sessions, absences, balance
// adjustments, month-close snapshots, contractual schedules, or planned
// shifts. Keep this targeted:
// unrelated time-tracking configuration, holiday, closing-day, and assignment
// caches do not change when staff_time_tracking_changed fires.
const STAFF_TIME_TRACKING_CACHE_KEY_PARTS = [
  "staff-time-accounts-",
  "staff-dashboard-summary-",
  "staff-history-",
  "staff-absences-",
  "staff-pending-absences-",
  "staff-month-summary-",
  "staff-month-close-",
  "staff-balance-adjustments-",
  "staff-schedule-",
  "staff-shifts-visible-",
  "time-tracking-current",
  "time-tracking-history-",
  "time-tracking-absences-",
  "time-tracking-table-",
  "time-tracking-month-summary-",
  "time-tracking-schedule-targets-",
  "time-tracking-own-schedule-",
  "time-tracking-own-shifts-today-",
] as const;

// Per-student SWR keys carry the id as a segment: "student-detail-<id>",
// "care-plan-day-<id>-<date>", "care-plan-week-<id>-<from>-<to>". useSWRAuth
// prefixes the whole thing with the tenant slug ("<slug>:student-detail-7").
const STUDENT_SCOPED_KEY_PREFIXES = [
  "student-detail-",
  "care-plan-day-",
  "care-plan-week-",
] as const;

// The per-group student list on the OGS page ("<slug>:ogs-students-7").
const OGS_STUDENTS_KEY_PREFIX = "ogs-students-";

/**
 * Whether an SWR cache key belongs to exactly this id (student or group).
 *
 * A plain `key.includes("care-plan-day-" + id)` matches every id that merely
 * STARTS with it, so an event for child 1 also revalidated the cached plans of
 * children 10, 11 and 100 — silent extra requests that grow with the school,
 * and worse on a morning check-in burst where several such events land at once.
 * The id must therefore end the key or be followed by "-", and the prefix must
 * start the key or sit right after the tenant separator.
 *
 * Two id spaces go through here, with different privacy rules:
 * - student ids, only from events whose audience is ALREADY scoped to the
 *   child (group-topic check-in/checkout events). A tenant-wide event
 *   (pickup) must not carry a student id at all (GDPR:
 *   group_supervisors_only).
 * - educational group ids ("ogs-students-{gid}", #2057), which legitimately
 *   ride the tenant-wide dashboard_counts_changed event — a group id reveals
 *   "counts in group X changed", never a child's identity.
 */
function keyTargetsId(
  key: string,
  studentId: string,
  prefixes: readonly string[] = STUDENT_SCOPED_KEY_PREFIXES,
): boolean {
  return prefixes.some((prefix) => {
    const needle = `${prefix}${studentId}`;
    for (
      let at = key.indexOf(needle);
      at !== -1;
      at = key.indexOf(needle, at + 1)
    ) {
      const before = at === 0 ? undefined : key[at - 1];
      const after = key[at + needle.length];
      if (
        (before === undefined || before === ":") &&
        (after === undefined || after === "-")
      ) {
        return true;
      }
    }
    return false;
  });
}

/**
 * Global SSE hook that maintains a single connection for the entire app.
 *
 * Features:
 * - Single SSE connection (shared across all pages)
 * - Targeted cache invalidation based on active_group_id
 * - Debounced invalidation for burst events (morning rush)
 * - Only connects when authenticated (session has token)
 * - Exposes connection status for debugging/UI
 *
 * @returns SSE connection state (status, isConnected, error, reconnectAttempts)
 */
export function useGlobalSSE(): SSEHookState {
  const { data: session, status: sessionStatus } = useSession();

  // Enable SSE for staff and effective admins. The backend treats admin:* and
  // *:* as admin scope even when a custom role does not include the literal
  // "admin" role, so the connection gate must mirror that rule.
  const isStaff = session?.user?.roles?.includes("user") ?? false;
  const hasAdminWildcard =
    session?.user?.permissions?.some(
      (permission) => permission === "admin:*" || permission === "*:*",
    ) ?? false;
  const isAdmin =
    (session?.user?.roles?.includes("admin") ?? false) || hasAdminWildcard;
  const isAuthenticated =
    sessionStatus === "authenticated" &&
    !!session?.user?.token &&
    (isStaff || isAdmin);

  // Debounce state: collect affected group IDs, flush once after DEBOUNCE_MS
  // (+ per-burst jitter). pendingGroupIds holds ACTIVE-group ids (activity
  // session topics); pendingEduGroupIds holds EDUCATIONAL (OGS) group ids from
  // event.data.group_ids — different tables with overlapping id ranges, never
  // mix them (#2057).
  const pendingGroupIds = useRef(new Set<string>());
  const pendingStudentIds = useRef(new Set<string>());
  // Educational group ids whose ogs-students-{gid} caches need revalidation.
  const pendingEduGroupIds = useRef(new Set<string>());
  // Set when a count-affecting event arrives WITHOUT educational group scope
  // (old backend during a mixed-version deploy, a student without an OGS
  // group, activity lifecycle events). Falls back to the pre-#2057 broad
  // ogs-students-* invalidation so correctness never depends on the payload.
  const hasPendingBroadOgsEvent = useRef(false);
  const hasPendingActivityEvent = useRef(false);
  const hasPendingActiveSupervisionEvent = useRef(false);
  const hasPendingDashboardEvent = useRef(false);
  const hasPendingStaffTimeTrackingEvent = useRef(false);
  const hasPendingDailyCheckoutDashboardEvent = useRef(false);
  const hasPendingArrivalScheduleEvent = useRef(false);
  // Pickup (Gehzeit) writes get their own flag rather than riding the arrival
  // one: they invalidate a different key set, and merging them would refetch
  // the arrival caches on every pickup edit. Intentionally a plain flag with no
  // per-student set: the event is tenant-wide and carries NO student id (GDPR —
  // see the backend broadcast), so invalidation is broad, exactly like arrival.
  const hasPendingPickupScheduleEvent = useRef(false);
  const hasPendingStudentUpdateEvent = useRef(false);
  // Kept apart from the student-update flag on purpose: only a write that can
  // actually have changed the "läuft mit" links sets it.
  const hasPendingCompanionEvent = useRef(false);
  const hasPendingTimetableEvent = useRef(false);
  // A group handover or Vertretung changed WHICH groups the viewer may open
  // (#2084). Kept apart from the OGS count flags: it invalidates a different
  // key set (the group list itself, the Vertretungen page, the user-context
  // BFF) and, unlike a check-in, it is rare.
  const hasPendingSubstitutionEvent = useRef(false);
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Per-burst flush state (#2057): jitter drawn once when a burst starts, and
  // the burst's start time for the MAX_FLUSH_WAIT_MS cap.
  const burstStartedAt = useRef(0);
  const burstJitter = useRef(0);

  // SWR cache keys are tenant-prefixed by useSWRAuth (e.g. "tenant-slug:ogs-students-2").
  // All matchers must use includes() instead of startsWith() to match regardless of prefix.
  const flushInvalidations = useCallback(() => {
    if (hasPendingStaffTimeTrackingEvent.current) {
      mutate(
        (key) =>
          typeof key === "string" &&
          STAFF_TIME_TRACKING_CACHE_KEY_PARTS.some((part) =>
            key.includes(part),
          ),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "staff_time_tracking",
        });
      });
    }

    // Invalidate ALL supervision-visits caches for student/dashboard events.
    // A student checked out of Room A may appear on the Schulhof (catch-all),
    // so we can't limit to just the source group's cache key.
    // Zero-topic clients only receive dashboard_counts_changed, so include
    // that flag to keep their detail views in sync.
    if (pendingGroupIds.current.size > 0 || hasPendingDashboardEvent.current) {
      mutate(
        (key) => typeof key === "string" && key.includes("supervision-visits-"),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "supervision_visits",
        });
      });
    }

    // Invalidate the per-group OGS student lists ("Meine Gruppe"). Scoped
    // (#2057): when every pending count event carried its educational group
    // ids, only those groups' ogs-students-{gid} caches revalidate — a tab
    // viewing another group refetches nothing. The broad fallback covers
    // events without group scope (old backend, students without an OGS group,
    // activity lifecycle) and the structural events (arrival/pickup/student
    // update) whose payload can never carry a scope.
    //
    // Deliberately NOT triggered by pendingGroupIds/pendingStudentIds: those
    // are fed by the tenant-wide active_supervision_changed too, which fires
    // alongside every dashboard_counts_changed — keeping them as triggers
    // would re-broaden every scoped event and save nothing.
    {
      const broadOgs =
        hasPendingBroadOgsEvent.current ||
        hasPendingArrivalScheduleEvent.current ||
        // A Gehzeit change moves the pickup time the list rows render
        // (student list responses carry it for full-access rows).
        hasPendingPickupScheduleEvent.current ||
        hasPendingStudentUpdateEvent.current ||
        // A handover/Vertretung changes the GROUP LIST inside the aggregated
        // live response, and it cannot be scoped to the affected group id
        // (#2084): the recipient's open tab is keyed on a DIFFERENT group —
        // the one they were already viewing — or on "auto" when they had no
        // group at all, so a scoped invalidation would miss exactly the tab
        // this event exists for.
        hasPendingSubstitutionEvent.current;
      const scopedEduGroupIds = new Set(pendingEduGroupIds.current);
      if (broadOgs || scopedEduGroupIds.size > 0) {
        mutate(
          (key) =>
            typeof key === "string" &&
            (broadOgs
              ? key.includes(OGS_STUDENTS_KEY_PREFIX)
              : // Exact-boundary match per group id: gid "1" must not
                // revalidate "ogs-students-10" (same lesson as the
                // per-student keys below).
                [...scopedEduGroupIds].some((gid) =>
                  keyTargetsId(key, gid, [OGS_STUDENTS_KEY_PREFIX]),
                )),
        ).catch((err) => {
          logger.debug("swr_revalidation_failed", {
            error: err instanceof Error ? err.message : String(err),
            scope: "ogs_students",
          });
        });
      }
    }

    // Invalidate the cross-group student list caches so students/search and
    // the room views pick up location changes (e.g. Zuhause) without a manual
    // refresh. Also invalidate tracking indicator caches on
    // active-supervisions (tracking-supervisions-*) and students/search
    // (tracking-indicators-*). Triggered by pendingGroupIds (room-level
    // events), pendingStudentIds (daily checkout sends student_checkout
    // without an active_group_id), or hasPendingDashboardEvent
    // (dashboard_counts_changed reaches every client of the tenant on every
    // check-in/out — ensures search page updates even when the user doesn't
    // supervise the affected room/group). These caches cannot be scoped by
    // group id, but each is mounted only on its own page.
    if (
      pendingGroupIds.current.size > 0 ||
      pendingStudentIds.current.size > 0 ||
      hasPendingDashboardEvent.current ||
      hasPendingArrivalScheduleEvent.current ||
      hasPendingPickupScheduleEvent.current ||
      hasPendingStudentUpdateEvent.current
    ) {
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("database-students-list") ||
            key.includes("search-students-") ||
            key.includes("tracking-supervisions-") ||
            key.includes("tracking-indicators-") ||
            // Live "Kinder im Raum" view on /rooms/{id}. Cache key shape is
            // "room-students-{roomId}" — see
            // components/rooms/students-in-room-section.tsx. Student
            // checkin/checkout events do not carry room_id, so we cannot
            // narrow further here; refetching the section's SWR key is
            // cheap and gives the page live data without polling.
            // NOTE: includes() (not startsWith) — useSWRAuth prefixes keys
            // with the tenant slug, so the real key is
            // "<tenant>:room-students-1" and startsWith would never match.
            key.includes("room-students-") ||
            // Room header in the detail modal/subpage — supervisor,
            // groupName, studentCount, isOccupied. Without this, the
            // "Aktuell anwesend: X" InfoItem stays stale while the
            // section above it refreshes (#1374).
            key.includes("room-detail-") ||
            // Room overview cards on /rooms use their own direct rooms
            // list cache. A room move changes studentCount there even
            // though the Room entity itself did not change.
            // (ROOM_LIST_CACHE_KEYS = the three /api/rooms list caches —
            // NOT room-derived-caches' ROOM_DERIVED_CACHE_KEY_FRAGMENTS,
            // which contains "ogs-students-" and would
            // silently re-broaden the #2057 scoping if swapped in.)
            ROOM_LIST_CACHE_KEYS.some((cacheKey) => key.includes(cacheKey))),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "student_lists",
        });
      });
    }

    // Invalidate specific student detail caches. keyTargetsStudent covers
    // student-detail-* plus care-plan-* — the per-child Betreuungsplan day/week
    // view, whose timeline shows the attendance a check-in/out just changed —
    // and matches the id as a whole segment, never as a prefix of a longer id.
    for (const studentId of pendingStudentIds.current) {
      mutate(
        (key) => typeof key === "string" && keyTargetsId(key, studentId),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "student_detail",
        });
      });
    }

    if (hasPendingStudentUpdateEvent.current) {
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("student-detail-") ||
            key.includes("student-status-days-") ||
            key.includes("care-plan-day-") ||
            key.includes("care-plan-week-") ||
            // The staff detail header's "Heutige Abholung"/arrival slots. Parent
            // care-exception writes (submit AND delete) change the pickup and
            // arrival override for a day but announce ONLY student_updated —
            // no pickup_schedule_changed, no arrival_schedule_changed
            // (services/parent/parent_write_service.go). An approved care
            // request is the mirror gap: it emits arrival_schedule_changed but
            // also rewrites the weekly PICKUP plan. Both keys disable focus
            // revalidation, so without them an open detail page keeps showing
            // the superseded time until a manual reload.
            key.includes("pickup-data-") ||
            key.includes("arrival-data-")),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "student_detail",
        });
      });

      // A renamed or reassigned child changes what the OTHER children's
      // Laufgemeinschaft entries show, without changing a single link — so no
      // student_companions_changed follows, and the SWR invalidation above does
      // not reach the links either (own table, fetched outside SWR, keyed on a
      // student id this write never changes). Read-only cards would keep the
      // old name indefinitely, and after a group reassignment even one the
      // backend would now redact for this viewer. Announced on the separate
      // display bus: forms react to the companion bus by discarding drafts, and
      // any rename in the school must not cost a user their work.
      notifyStudentCompanionDisplayChanged();
    }

    if (hasPendingCompanionEvent.current) {
      // The Laufgemeinschaft ("läuft mit") lives in its own table and is
      // fetched outside SWR, so invalidating student-detail-* does not bring
      // it along — and the fetching card keys its effect on the student id,
      // which a remote edit never changes. Announcing the change on the same
      // bus a local save uses gives every mounted card one shared
      // revalidation path, so a link added in another tab, browser or by
      // another staff member stops being invisible until remount (#1694).
      //
      // Driven by the dedicated student_companions_changed event, NOT by
      // student_updated: an editing form reacts to this announcement by
      // discarding its draft or blocking the save, so firing it for every
      // rename, photo upload or sick flag in the school would cost users
      // their work for changes that never touched the links.
      notifyStudentCompanionsChanged();

      // The Kindersuche is the one companion view that does NOT fetch the links
      // separately: its grouping rides along inside the student list response
      // (companion_student_ids, only with include_companions=true), so the
      // announcement above never reaches it. Deleting a linked child announces
      // ONLY student_companions_changed — no student_updated, no checkin — so
      // without this the open "Nach Laufgemeinschaft" view keeps listing the
      // deleted child, and every grouping the delete changed, until something
      // unrelated revalidates the list.
      mutate(
        (key) => typeof key === "string" && key.includes("search-students-"),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "student_companions",
        });
      });
    }

    // Arrival schedule changes affect derived "Kommt heute nicht" badges and
    // arrival rows. These keys are independent from attendance/location caches.
    if (hasPendingArrivalScheduleEvent.current) {
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("arrival-search-") ||
            key.includes("arrival-supervisions-") ||
            key.includes("arrival-ogs-groups-") ||
            key.includes("arrival-data-") ||
            // the Betreuungsplan day/week view shows the resolved arrival slot
            key.includes("care-plan-day-") ||
            key.includes("care-plan-week-")),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "arrival_schedule",
        });
      });
    }

    // Pickup (Gehzeit) plan or exception changed. The per-child Betreuungsplan
    // renders the resolved pickup slot, the detail header shows "Gehzeit heute",
    // and the student detail response carries the day-planning pickup time —
    // all fetched with focus revalidation disabled, so this event is their ONLY
    // live update path: without it a colleague's Gehzeit edit stays invisible in
    // an open tab indefinitely.
    //
    // Broad by design, exactly like arrival: the event is tenant-wide and
    // carries no student id (a tenant-wide id would leak pickup activity to
    // staff outside gdpr.student_data_scope=group_supervisors_only — see the
    // backend broadcast), so it cannot be narrowed to one child here. The cost
    // is one re-check per open detail/care-plan page; each refetch is
    // server-access-filtered, so an out-of-scope staffer gets nothing back.
    if (hasPendingPickupScheduleEvent.current) {
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("care-plan-day-") ||
            key.includes("care-plan-week-") ||
            // the detail header's "Gehzeit heute" slot
            key.includes("pickup-data-") ||
            key.includes("student-detail-")),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "pickup_schedule",
        });
      });
    }

    // The Betreuungszeiten editor (CareScheduleManager) holds its arrival/pickup
    // data in local state, NOT SWR, and stays force-mounted across tabs — so the
    // mutate() calls above never reach it and a remote pickup/arrival change (a
    // colleague's edit, or an approved parent request) leaves it stale until the
    // page reloads. It owns its correctly-scoped refetch, so mirror the
    // reminders/tenant-settings decoupling: announce staleness on a window
    // event and let the mounted editor re-fetch. student_updated is included
    // because a parent care-exception submit/delete changes the day's
    // pickup/arrival override under that event alone.
    if (
      hasPendingPickupScheduleEvent.current ||
      hasPendingArrivalScheduleEvent.current ||
      hasPendingStudentUpdateEvent.current
    ) {
      if (typeof window !== "undefined") {
        window.dispatchEvent(new CustomEvent("phoenix:care-schedule-stale"));
      }
    }

    // Invalidate the count dashboards for activity events, explicit dashboard
    // broadcasts, and student movement fallbacks. The tenant broadcast is
    // best-effort, so a delivered student_checkin/student_checkout may be the
    // only signal that counts changed if dashboard_counts_changed gets
    // dropped under backpressure. Matches the explicit
    // DASHBOARD_COUNT_CACHE_KEYS list — NOT every key containing "dashboard"
    // (see the constant's comment; the old substring match dragged the OGS
    // BFF and staff time tracking into every check-in, #2057).
    if (
      pendingGroupIds.current.size > 0 ||
      pendingStudentIds.current.size > 0 ||
      hasPendingActivityEvent.current ||
      hasPendingActiveSupervisionEvent.current ||
      hasPendingDashboardEvent.current ||
      hasPendingDailyCheckoutDashboardEvent.current ||
      hasPendingArrivalScheduleEvent.current ||
      hasPendingStudentUpdateEvent.current
    ) {
      mutate(
        (key) =>
          typeof key === "string" &&
          DASHBOARD_COUNT_CACHE_KEYS.some((cacheKey) => key.includes(cacheKey)),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "dashboard",
        });
      });
    }

    // The OGS page's former 5-request BFF ("ogs-dashboard") is gone (#2056):
    // the aggregated live view rides the ogs-students-{gid} keys above, whose
    // trigger set already includes the structural events (student edits,
    // arrival plans, activity lifecycle via hasPendingBroadOgsEvent).

    // Activity events also need room/supervision refresh
    if (hasPendingActivityEvent.current) {
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("supervision") ||
            key.includes("active") ||
            key.includes("rooms") ||
            // Detail modal/subpage header (#1374) — activity_start /
            // activity_end change groupName/activityName/isOccupied,
            // which the room summary in the modal reflects.
            key.includes("room-detail-")),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "activity_supervision",
        });
      });
    }

    if (hasPendingActiveSupervisionEvent.current) {
      // No bare "dashboard" substring here (#2057): dashboard-analytics is
      // already covered by the count-dashboard block above (its trigger list
      // includes hasPendingActiveSupervisionEvent), and the old substring
      // dragged ogs-dashboard + staff-dashboard-summary into every check-in.
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("active-supervision-dashboard-") ||
            key.includes("supervision-visits-") ||
            key.includes("timetable-roster-") ||
            key.includes("room-detail-") ||
            key.includes("tracking-supervisions-") ||
            key.includes("tracking-indicators-")),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "active_supervision",
        });
      });
    }

    // A group handover or Vertretung changed who may open which group (#2084).
    // Besides the OGS live view above, three caches carry that fact and none of
    // them revalidates on focus: the admin Vertretungen page (its list and the
    // teacher availability it renders) and the user-context BFF, whose
    // educational-group list drives the sidebar group switcher and is fetched
    // as immutable — without an explicit invalidation it never refetches for
    // the whole session.
    if (hasPendingSubstitutionEvent.current) {
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("active-substitutions") ||
            key.includes("substitution-teachers") ||
            key.includes("user-context")),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "substitutions",
        });
      });

      // The sidebar's "Meine Gruppen" list lives in SupervisionContext, which
      // keeps it in local state behind its own fetch and only refreshes on a
      // one-minute tick — there is no SWR key to invalidate. That list is
      // exactly what a colleague looks at after a handover, so announce
      // staleness and let the provider refetch (same decoupling as reminders
      // and the care-schedule editor above).
      if (typeof window !== "undefined") {
        window.dispatchEvent(new CustomEvent("phoenix:supervision-stale"));
      }
    }

    if (hasPendingTimetableEvent.current) {
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("timetable-") ||
            // "Heute geplant" card on the Zeiterfassung page (#1844): its
            // assignments come from activity instances, so a same-day cancel/
            // start/complete — and staffing_deviation_changed, which covers
            // deviations (absence, substitute, understaffed-ack) plus ordinary
            // create/edit/delete of still-planned instances — must refetch it.
            // The card disables focus revalidation, making this its only live
            // update path.
            key.includes("time-tracking-own-assignments-") ||
            key.includes("database-calendar-periods-list") ||
            // the per-child Betreuungsplan renders these activity instances
            key.includes("care-plan-day-") ||
            key.includes("care-plan-week-")),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "timetable",
        });
      });
    }

    // Reminders (issue #1457): the header bell + /reminders page are computed
    // live from attendance, pickups, activity instances and student data.
    // Check-ins/outs, activity/instance lifecycle and student edits change
    // what's due — and student edits also change the name/class each row shows
    // and which rows fall inside the caller's readable scope — so the reminders
    // view must revalidate on these events. The 60s poll only catches the
    // time-based threshold crossings ("in 10 Min" → "überfällig"), not these.
    // Zero-topic admins (no supervised group / admin_supervision_overview off)
    // receive check-ins only as the dashboard_counts_changed broadcast, so
    // include hasPendingDashboardEvent or their bell/list stays stale until poll.
    //
    // This hook deliberately does not own the tenant-prefixed
    // "{slug}:reminders" SWR key. Dispatch a window event and let
    // useReminders() — which owns that key and knows whether the feature is
    // enabled — perform the revalidation. This mirrors the
    // tenant_settings_changed / parent_message decoupling above and keeps the
    // idle-poll throttle intact for disabled tenants (the consumer skips the
    // mutate when the feature is off).
    if (
      pendingGroupIds.current.size > 0 ||
      pendingStudentIds.current.size > 0 ||
      hasPendingActivityEvent.current ||
      hasPendingTimetableEvent.current ||
      hasPendingDashboardEvent.current ||
      hasPendingDailyCheckoutDashboardEvent.current ||
      // "Abholung in 10 Min" / "Abholung überfällig" rows are computed from the
      // EFFECTIVE pickup time (schedule resolved against the day's exception —
      // services/reminders reads GetBulkEffectivePickupTimesForDate), so a
      // Gehzeit edit adds, drops or re-times rows. The 60s poll only catches
      // thresholds crossed by time passing, not the plan changing underneath.
      // Arrival deliberately stays out: reminders cover pickups and activities
      // only, so an arrival edit changes no row.
      hasPendingPickupScheduleEvent.current ||
      hasPendingStudentUpdateEvent.current
    ) {
      if (typeof window !== "undefined") {
        window.dispatchEvent(new CustomEvent("phoenix:reminders-stale"));
      }
    }

    // Reset pending state
    pendingGroupIds.current.clear();
    pendingStudentIds.current.clear();
    pendingEduGroupIds.current.clear();
    hasPendingBroadOgsEvent.current = false;
    hasPendingActivityEvent.current = false;
    hasPendingActiveSupervisionEvent.current = false;
    hasPendingDashboardEvent.current = false;
    hasPendingStaffTimeTrackingEvent.current = false;
    hasPendingDailyCheckoutDashboardEvent.current = false;
    hasPendingArrivalScheduleEvent.current = false;
    hasPendingPickupScheduleEvent.current = false;
    hasPendingStudentUpdateEvent.current = false;
    hasPendingCompanionEvent.current = false;
    hasPendingTimetableEvent.current = false;
    hasPendingSubstitutionEvent.current = false;
  }, []);

  const scheduleFlush = useCallback(() => {
    const now = Date.now();
    if (debounceTimer.current) {
      clearTimeout(debounceTimer.current);
    } else {
      // First event of a burst: draw the jitter once and remember the burst
      // start. Re-drawing per event would make the effective delay drift
      // upward during a burst; drawing once keeps "burst collapses into one
      // flush" intact while still spreading DIFFERENT clients across the
      // jitter window (#2057 — every client receives the same broadcast at
      // the same instant, so a fixed delay produced synchronized herds).
      burstStartedAt.current = now;
      burstJitter.current = Math.random() * FLUSH_JITTER_MS; // NOSONAR typescript:S2245 non-cryptographic load-spreading jitter, no security context
    }
    // Trailing debounce with an upper bound: a sustained event stream
    // (morning rush) keeps pushing the flush back, but never beyond
    // MAX_FLUSH_WAIT_MS after the burst began.
    const target = Math.min(
      now + DEBOUNCE_MS + burstJitter.current,
      burstStartedAt.current + MAX_FLUSH_WAIT_MS,
    );
    debounceTimer.current = setTimeout(
      () => {
        debounceTimer.current = null;
        flushInvalidations();
      },
      Math.max(0, target - now),
    );
  }, [flushInvalidations]);

  // Records the educational-group scope of a count-affecting event (#2057):
  // known group ids feed the scoped ogs-students-{gid} invalidation; a
  // missing/empty list flags the broad fallback so correctness never depends
  // on the payload (old backend during a mixed-version deploy, students
  // without an OGS group).
  const collectEduGroupScope = useCallback((groupIds: string[] | undefined) => {
    if (groupIds && groupIds.length > 0) {
      for (const gid of groupIds) {
        pendingEduGroupIds.current.add(gid);
      }
    } else {
      hasPendingBroadOgsEvent.current = true;
    }
  }, []);

  // Handle SSE events by collecting targeted invalidations
  const handleSSEEvent = useCallback(
    (event: SSEEvent) => {
      switch (event.type) {
        case "student_checkin":
        case "student_checkout": {
          // Target the specific group affected
          if (event.active_group_id) {
            pendingGroupIds.current.add(event.active_group_id);
          }
          // Target the specific student detail cache
          if (event.data.student_id) {
            pendingStudentIds.current.add(event.data.student_id);
          }
          // Scope the ogs-students-{gid} invalidation to the affected
          // educational groups (#2057). These group-topic events carry the
          // scope so the invalidation stays scoped even when the companion
          // dashboard_counts_changed gets dropped under backpressure. An old
          // backend sends no group_ids → broad fallback.
          collectEduGroupScope(event.data.group_ids);
          // Daily "nach Hause" checkout emits only student_checkout on the
          // educational group topic without a companion dashboard event.
          if (event.type === "student_checkout" && !event.active_group_id) {
            hasPendingDailyCheckoutDashboardEvent.current = true;
          }
          scheduleFlush();
          break;
        }

        case "bulk_student_checkout": {
          // Whole-session end (#848): one event per topic carrying every
          // affected student. Mirror student_checkout — invalidate the
          // session's group caches plus each student's detail cache — but read
          // the batched student_ids list instead of a single student_id.
          if (event.active_group_id) {
            pendingGroupIds.current.add(event.active_group_id);
          }
          if (event.data.student_ids) {
            for (const id of event.data.student_ids) {
              pendingStudentIds.current.add(id);
            }
          }
          collectEduGroupScope(event.data.group_ids);
          scheduleFlush();
          break;
        }

        case "student_updated": {
          hasPendingStudentUpdateEvent.current = true;
          scheduleFlush();
          break;
        }

        case "student_companions_changed": {
          hasPendingCompanionEvent.current = true;
          scheduleFlush();
          break;
        }

        case "activity_start":
        case "activity_end":
        case "activity_update": {
          if (event.active_group_id) {
            pendingGroupIds.current.add(event.active_group_id);
          }
          hasPendingActivityEvent.current = true;
          // Session lifecycle changes the current_location of the group's
          // children but carries no educational group scope — refresh the
          // OGS lists broadly. Explicit on purpose: the ogs-students clause
          // no longer listens to pendingGroupIds (see flushInvalidations),
          // and relying on the id-less companion dashboard event would be an
          // implicit coupling.
          hasPendingBroadOgsEvent.current = true;
          scheduleFlush();
          break;
        }

        case "active_supervision_changed": {
          if (event.active_group_id) {
            pendingGroupIds.current.add(event.active_group_id);
          }
          if (event.data.student_id) {
            pendingStudentIds.current.add(event.data.student_id);
          }
          hasPendingActiveSupervisionEvent.current = true;
          scheduleFlush();
          break;
        }

        case "dashboard_counts_changed": {
          // Tenant-wide broadcast on every check-in/out — only refresh
          // dashboard counts and the (scoped) OGS student lists, NOT
          // room/supervision/active caches (those are for activity events).
          // group_ids carries the affected educational groups (#2057) so the
          // ogs-students invalidation can skip every other group's tab; an
          // event without them (old backend, activity lifecycle, student
          // without OGS group) falls back to the broad refresh.
          hasPendingDashboardEvent.current = true;
          collectEduGroupScope(event.data.group_ids);
          scheduleFlush();
          break;
        }

        case "substitution_changed": {
          // A group was handed over/taken back, or a Vertretung was
          // created/edited/deleted. Debounced like the other cache triggers so
          // an admin editing several Vertretungen in a row produces one flush,
          // not one per save.
          hasPendingSubstitutionEvent.current = true;
          scheduleFlush();
          break;
        }

        case "staff_time_tracking_changed": {
          hasPendingStaffTimeTrackingEvent.current = true;
          scheduleFlush();
          break;
        }

        case "arrival_schedule_changed": {
          hasPendingArrivalScheduleEvent.current = true;
          scheduleFlush();
          break;
        }

        case "pickup_schedule_changed": {
          // Staff-side Gehzeit write (weekly plan or date exception). Parent
          // submits do NOT emit this — they broadcast student_updated, which
          // already invalidates the same caches above. Tenant-wide and
          // deliberately id-less (GDPR), so invalidation stays broad.
          hasPendingPickupScheduleEvent.current = true;
          scheduleFlush();
          break;
        }

        case "change_requests_changed": {
          // A parent change-request queue changed (created/decided/withdrawn).
          // Re-dispatch the window event the staff review lists and the
          // "Änderungsanfragen" pending-count badge already listen on, so an open
          // review page updates in real time without depending on the parent-
          // messaging pill (the school may have messaging disabled). Fires only on
          // request transitions, so a plain refetch is fine — no debounce needed.
          if (typeof window !== "undefined") {
            window.dispatchEvent(new CustomEvent("change-requests-refresh"));
          }
          break;
        }

        case "tenant_settings_changed": {
          // Cross-origin tenant settings sync. The backend fires this when a
          // setting whose value travels through /auth/tenant/resolve flips
          // (currently operations.student_photos_enabled).
          // BroadcastChannel only reaches same-origin tabs, so an operator
          // toggle at operator.<domain> never reaches <slug>.<domain> tabs;
          // SSE crosses that boundary because every authenticated tab holds
          // an open connection regardless of its origin.
          //
          // Dispatch a window event instead of directly mutating SWR or
          // tenant context here: TenantProvider owns the resolve refetch and
          // already serialises concurrent triggers (BroadcastChannel +
          // visibilitychange + this event). Keeping this hook decoupled
          // avoids importing tenant internals into the global SSE handler.
          if (typeof window !== "undefined") {
            window.dispatchEvent(
              new CustomEvent("phoenix:tenant-settings-stale", {
                detail: { source: event.data.source ?? null },
              }),
            );
          }
          break;
        }

        case "instance_started":
        case "instance_completed":
        case "instance_cancelled":
        case "instance_overdue":
        // Staffing deviations (absence/substitute/understaffed-ack, #1844)
        // change is_absent/is_substitute/understaffed_ack without any
        // lifecycle transition — same caches go stale, same invalidation.
        case "staffing_deviation_changed": {
          hasPendingTimetableEvent.current = true;
          scheduleFlush();
          break;
        }

        case "notification": {
          // Notification abstraction (#1624): the payload is display-safe by
          // backend contract and rendered directly. This hook runs above
          // ToastProvider consumers, so hand the event to the notification
          // bridge (mounted under Providers) via a window event — mirroring
          // the reminders/tenant-settings decoupling above.
          dispatchPhoenixNotification(event);
          break;
        }

        // A counterpart read a conversation. Handled identically to a new
        // message on the staff side: the sidebar badge must refresh (a staff
        // member reading in another tab dropped the count) and any open
        // thread/card refetches to pick up the guardian "Gelesen" receipt. The
        // backend only emits this when a read cursor actually advanced, so the
        // refetch it triggers cannot loop back into another read event.
        case "parent_message_read":
        case "parent_message": {
          // A parent sent a message (or staff replied elsewhere). This single
          // app-wide connection fans the trigger out to the messaging surfaces
          // via window events, so the inbox/thread/student-card pages do NOT each
          // open their own EventSource to the same endpoint (mirrors the parents
          // portal's ParentRealtimeBridge). `messages-unread-refresh` refreshes
          // the sidebar badge; `messages-activity` carries thread_id/student_id so
          // an open inbox/thread/card can refetch (or skip) selectively.
          if (typeof window !== "undefined") {
            window.dispatchEvent(new CustomEvent("messages-unread-refresh"));
            window.dispatchEvent(
              new CustomEvent("messages-activity", {
                detail: {
                  threadId: event.data?.thread_id ?? null,
                  studentId: event.data?.student_id ?? null,
                },
              }),
            );
          }
          break;
        }
      }
    },
    [scheduleFlush, collectEduGroupScope],
  );

  // Use the underlying SSE hook with global event handler.
  // reconnectKey ensures the EventSource tears down and reconnects with a
  // fresh JWT whenever the user switches tenant.
  return useSSE("/api/sse/events", {
    onMessage: handleSSEEvent,
    enabled: isAuthenticated,
    reconnectKey: session?.user?.tenantId,
  });
}
