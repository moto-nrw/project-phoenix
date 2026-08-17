"use client";

import { useCallback, useReducer, useRef, useState } from "react";
import { mutate as globalMutate } from "swr";
import { useToast } from "~/contexts/ToastContext";
import {
  isHomeLocation,
  isSchoolyardLocation,
  parseLocation,
} from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  schoolCheckinStudent,
  schoolCheckinStudentsBatch,
  type SchoolCheckinAction,
  type SchoolCheckinBatchOutcome,
} from "~/lib/student-api";

/**
 * Presence state shown in check-in mode. Maps 1:1 onto the brand color
 * palette (green / orange / red) plus an "unknown" sentinel for students
 * whose location can't be resolved. Defined here (the hook is the state's
 * source of truth via deriveCheckinState) and re-exported from
 * StudentCard for prop-type ergonomics.
 */
export type StudentCheckinState =
  "anwesend" | "schulhof" | "abwesend" | "unknown";

const logger = createLogger({ component: "useSchoolCheckinMode" });

/**
 * Maps a student's current_location string to the narrow StudentCheckinState
 * union that StudentCard uses to tint its card in check-in mode.
 *
 * Mirrors PresenceBadge.derivePresenceState so the card tint always agrees
 * with the badge on the same card:
 *   - empty / null / "Zuhause" → abwesend (red)
 *   - "Schulhof"              → schulhof (orange)
 *   - anything else incl.
 *     "Anwesend", "Unterwegs",
 *     room names               → anwesend (green)
 *
 * "Unterwegs" in particular is a detailed-mode transient state — the
 * student is checked in to the building but between rooms. They are
 * already present, so toggling them must fire checkout ("out"), which
 * this mapping guarantees via actionForState.
 */
export function deriveCheckinState(
  currentLocation?: string | null,
): StudentCheckinState {
  if (!currentLocation || isHomeLocation(currentLocation)) return "abwesend";
  if (isSchoolyardLocation(currentLocation)) return "schulhof";
  return "anwesend";
}

/**
 * Maps a current state to the API action that toggles it. "anwesend" or
 * "schulhof" means the student is in school → checkout. "abwesend" or
 * "unknown" means not in school → check in.
 */
function actionForState(state: StudentCheckinState): SchoolCheckinAction {
  return state === "anwesend" || state === "schulhof" ? "out" : "in";
}

/**
 * Room a checkout would interrupt, or null when there is none (#2220).
 *
 * Detailed-mode tenants resolve a present student's location to
 * "Anwesend - {Raum}" whenever an open `active.visits` row points at a room.
 * Checking such a student out of care also ends that visit (the backend does
 * it in the same transaction), so the UI asks first and names the room.
 *
 * Returns null for every state that carries no room: absent students, the
 * roomless "Unterwegs"/"Anwesend" states, and every binary-mode label — those
 * check out on a single tap, exactly like before.
 */
export function checkoutConfirmationRoom(
  currentLocation?: string | null,
): string | null {
  if (deriveCheckinState(currentLocation) !== "anwesend") return null;
  const { room } = parseLocation(currentLocation);
  return room && room.length > 0 ? room : null;
}

/**
 * Detects the 403 the school-checkin endpoint returns when the caller may not
 * use web check-in at all (missing users:checkin permission or web attendance
 * disabled). authFetch folds the status into the message
 * ("API error (403): Forbidden"), which is the same shape
 * `getApiErrorMessage` matches on.
 */
function isForbiddenError(error: unknown): boolean {
  return error instanceof Error && error.message.includes("403");
}

interface CheckinModeSession {
  isActive: boolean;
  successCount: number;
}

type CheckinModeSessionAction =
  | "toggle"
  | "deactivate"
  | "increment"
  | { type: "addSuccesses"; count: number };

function checkinModeSessionReducer(
  state: CheckinModeSession,
  action: CheckinModeSessionAction,
): CheckinModeSession {
  if (typeof action === "object") {
    return { ...state, successCount: state.successCount + action.count };
  }
  switch (action) {
    case "toggle":
      return state.isActive
        ? { ...state, isActive: false }
        : { isActive: true, successCount: 0 };
    case "deactivate":
      return { ...state, isActive: false };
    case "increment":
      return { ...state, successCount: state.successCount + 1 };
  }
}

/**
 * Bundled SWR invalidation after a batch toggle (#2359): one revalidation
 * sweep for the whole batch instead of one per student (cf. #848 — bulk
 * events already arrive as a burst; the page must not multiply that).
 * student-detail matches on the bare prefix because any of the batch's
 * children may have an open detail cache entry.
 */
async function invalidatePresenceCachesBulk(): Promise<void> {
  await globalMutate(
    (key) =>
      typeof key === "string" &&
      (key.includes("ogs-students-") ||
        key.includes("search-students-") ||
        key.includes("tracking-supervisions-") ||
        key.includes("tracking-indicators-") ||
        key.includes("student-detail-")),
  );
}

interface UseSchoolCheckinModeResult {
  /** Whether the page is currently in check-in/out mode. */
  isActive: boolean;
  /** Flip between active and inactive. Resets the per-session counter
   *  whenever the mode is freshly switched on. Ignored while a bulk request
   *  is in flight — leaving the mode clears the selection, which would
   *  discard the failed IDs the in-flight response is about to keep marked
   *  for the promised one-tap retry (review #2372). */
  toggleActive: () => void;
  /** Set mode off (used when navigating away or on unrecoverable errors).
   *  Ignored while a bulk request is in flight, like toggleActive. */
  deactivate: () => void;
  /** Set of student IDs whose API call is still in flight. */
  pendingIds: ReadonlySet<string>;
  /** Successful toggles in the current active session. Resets to 0 whenever
   *  the mode is freshly entered. */
  successCount: number;
  /**
   * Toggle a student's attendance. Resolves the next action from their
   * current StudentCheckinState, calls the school-checkin endpoint, and
   * invalidates the SWR caches that surface presence data.
   */
  toggle: (
    studentId: string,
    currentState: StudentCheckinState,
  ) => Promise<void>;
  /**
   * Selection sub-mode (#2359). While ON, tapping a card is expected to call
   * toggleSelected instead of toggle — nothing is written until runBulk.
   * Switching the sub-mode (either direction) and leaving check-in mode both
   * clear the selection.
   */
  selectionActive: boolean;
  setSelectionActive: (active: boolean) => void;
  /** Students currently marked for the next bulk action. */
  selectedIds: ReadonlySet<string>;
  toggleSelected: (studentId: string) => void;
  /**
   * Empty the selection. While a bulk request is in flight the clear is
   * DEFERRED until the run settles (review #2372): the page fires this on
   * every search/filter scope change, and clearing mid-flight would race the
   * response processing — the failed students would lose their retry marks
   * to a half-applied clear. Deferring keeps the outcome deterministic: the
   * batch finishes against the intact selection, then the deferred clear
   * drops every mark EXCEPT the run's failed students — the failure dialog
   * promises they stay selected for a one-tap retry, and that promise must
   * hold through a mid-flight scope change too (review #2372).
   */
  clearSelection: () => void;
  /** True while a bulk action's API call is in flight. */
  isBulkRunning: boolean;
  /**
   * Apply one explicit action to selected students via the batch endpoint.
   * `onlyIds` restricts the run to that subset of the selection — the page
   * passes the currently VISIBLE selected students (or the confirmation
   * dialog's snapshot of them) so a mark hidden by a filter is never
   * executed without being on screen, and a dialog executes exactly what it
   * displayed (review #2372). The run is always intersected with the still-
   * selected ids, so a stale caller list can only narrow, never widen. After
   * the run, successfully processed students are removed from the selection
   * while failures (and any students marked mid-flight) stay selected, so a
   * retry is one tap. The presence caches are revalidated once, bundled.
   * Returns null when nothing was selected, a run is already in flight, or
   * the whole request failed (the hook has toasted the error then).
   */
  runBulk: (
    action: SchoolCheckinAction,
    onlyIds?: readonly string[],
  ) => Promise<SchoolCheckinBatchOutcome | null>;
}

/**
 * Manages the page-level "Kinder an- und abmelden" toggle plus the
 * per-student API call fan-out. Keeps StudentCard dumb: the page holds
 * isActive and the pendingIds set, the card just renders what it's told.
 *
 * Cache invalidation mirrors use-global-sse: any key containing
 * ogs-students-, search-students-, or tracking-* gets revalidated so that
 * the card's locationBadge and pendingIds converge with the backend truth.
 */
export function useSchoolCheckinMode(): UseSchoolCheckinModeResult {
  const [{ isActive, successCount }, dispatchSession] = useReducer(
    checkinModeSessionReducer,
    { isActive: false, successCount: 0 },
  );
  const [pendingIds, setPendingIds] = useState<Set<string>>(() => new Set());
  const [selectionActive, setSelectionActiveState] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [isBulkRunning, setIsBulkRunning] = useState(false);
  // Ref twin of isBulkRunning so the selection-destroying callbacks below can
  // check it WITHOUT depending on the state: their identities stay stable
  // across a run, which matters because the page's scope-change effect keys
  // on clearSelection's identity — an identity churn per run would re-fire
  // that effect after every batch and wipe the retained failure marks.
  const isBulkRunningRef = useRef(false);
  // Scope clear requested mid-flight; applied when the run settles.
  const pendingClearRef = useRef(false);
  const toast = useToast();

  const clearSelection = useCallback(() => {
    if (isBulkRunningRef.current) {
      // Defer, don't drop (see the interface doc): the scope change still
      // wins — but only after the in-flight response has been processed
      // against the selection it was started from.
      pendingClearRef.current = true;
      return;
    }
    setSelectedIds(new Set());
  }, []);

  // Entering, leaving, or re-entering the mode always drops selection state:
  // a marked-but-never-executed selection must not survive into the next
  // session, where it would silently widen a bulk action.
  const resetSelectionState = useCallback(() => {
    setSelectionActiveState(false);
    setSelectedIds(new Set());
  }, []);

  // toggleActive/deactivate/setSelectionActive ignore calls while a bulk
  // request is in flight (review #2372): each of them empties selectedIds,
  // and an emptied selection can no longer retain the failed students the
  // response is about to report. The visible triggers ("Fertig", the
  // Sofort|Auswahl switch) are also disabled during a run — this guard is
  // the authority behind that affordance, covering every caller.
  const toggleActive = useCallback(() => {
    if (isBulkRunningRef.current) return;
    dispatchSession("toggle");
    resetSelectionState();
  }, [resetSelectionState]);

  const deactivate = useCallback(() => {
    if (isBulkRunningRef.current) return;
    dispatchSession("deactivate");
    resetSelectionState();
  }, [resetSelectionState]);

  const setSelectionActive = useCallback((active: boolean) => {
    if (isBulkRunningRef.current) return;
    setSelectionActiveState(active);
    // Both directions clear: leftover marks from a previous sub-mode session
    // would otherwise be executed later without being visible in between.
    setSelectedIds(new Set());
  }, []);

  const toggleSelected = useCallback((studentId: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(studentId)) {
        next.delete(studentId);
      } else {
        next.add(studentId);
      }
      return next;
    });
  }, []);

  const toggle = useCallback(
    async (studentId: string, currentState: StudentCheckinState) => {
      if (pendingIds.has(studentId)) return;

      setPendingIds((prev) => {
        const next = new Set(prev);
        next.add(studentId);
        return next;
      });

      const action = actionForState(currentState);

      try {
        await schoolCheckinStudent(studentId, action);

        await globalMutate(
          (key) =>
            typeof key === "string" &&
            (key.includes("ogs-students-") ||
              key.includes("search-students-") ||
              key.includes("tracking-supervisions-") ||
              key.includes("tracking-indicators-") ||
              key.includes(`student-detail-${studentId}`)),
        );

        // Bump the per-session counter so the ModeBar can surface "X
        // angemeldet" feedback. We count both directions equally — the
        // banner copy makes the direction unambiguous.
        dispatchSession("increment");
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        logger.error("school_checkin_toggle_failed", {
          student_id: studentId,
          action,
          error: message,
        });
        if (isForbiddenError(error)) {
          // Naming the reason matters here: a permission 403 fails on every
          // retry, and a bare "bitte erneut versuchen" would invite endless
          // retries (#2220).
          toast.error(
            action === "in"
              ? "Keine Berechtigung, Kinder anzumelden."
              : "Keine Berechtigung, Kinder abzumelden.",
          );
        } else {
          toast.error(
            action === "in"
              ? "Anmelden fehlgeschlagen. Bitte erneut versuchen."
              : "Abmelden fehlgeschlagen. Bitte erneut versuchen.",
          );
        }
      } finally {
        setPendingIds((prev) => {
          const next = new Set(prev);
          next.delete(studentId);
          return next;
        });
      }
    },
    [pendingIds, toast],
  );

  const runBulk = useCallback(
    async (
      action: SchoolCheckinAction,
      onlyIds?: readonly string[],
    ): Promise<SchoolCheckinBatchOutcome | null> => {
      if (isBulkRunningRef.current) return null;

      // Intersect with the live selection so a stale caller list can never
      // widen the run beyond what is actually marked right now.
      const ids = onlyIds
        ? onlyIds.filter((id) => selectedIds.has(id))
        : Array.from(selectedIds);
      if (ids.length === 0) return null;
      isBulkRunningRef.current = true;
      setIsBulkRunning(true);
      // Mark every batch member pending so the cards show the same in-flight
      // state a single toggle does.
      setPendingIds((prev) => new Set([...prev, ...ids]));

      // The students a deferred scope-change clear must NOT drop: until the
      // response says otherwise, that is the whole run (a total request
      // failure keeps everything selected for the retry); a processed
      // response narrows it to the reported failures.
      let failedRunIds: ReadonlySet<string> = new Set(ids);

      try {
        const outcome = await schoolCheckinStudentsBatch(ids, action);

        await invalidatePresenceCachesBulk();
        dispatchSession({ type: "addSuccesses", count: outcome.succeeded });

        // Clean run → summary toast right here (counts only, no names). A
        // partial failure is the caller's job: naming the failed children
        // needs the student list, which this hook does not hold.
        if (outcome.failed === 0) {
          const alreadyDone = outcome.results.filter(
            (result) => result.ok && !result.changed,
          ).length;
          const noun = outcome.succeeded === 1 ? "Kind" : "Kinder";
          const verb = action === "in" ? "angemeldet" : "abgemeldet";
          toast.success(
            alreadyDone > 0
              ? `${outcome.succeeded} ${noun} ${verb} (${alreadyDone} davon bereits ${verb}).`
              : `${outcome.succeeded} ${noun} ${verb}.`,
          );
        }

        // Subtract the successfully processed students instead of replacing
        // the set: failures stay visibly marked for a one-tap retry, and a
        // selection made while this request was in flight survives untouched
        // (review #2372 — replacement would silently discard it).
        const processedOkIds = new Set(
          outcome.results
            .filter((result) => result.ok)
            .map((result) => result.studentId),
        );
        setSelectedIds(
          (prev) => new Set([...prev].filter((id) => !processedOkIds.has(id))),
        );
        failedRunIds = new Set(
          outcome.results
            .filter((result) => !result.ok)
            .map((result) => result.studentId),
        );

        return outcome;
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        logger.error("school_checkin_bulk_failed", {
          action,
          student_count: ids.length,
          error: message,
        });
        if (isForbiddenError(error)) {
          toast.error(
            action === "in"
              ? "Keine Berechtigung, Kinder anzumelden."
              : "Keine Berechtigung, Kinder abzumelden.",
          );
        } else {
          toast.error(
            action === "in"
              ? "Anmelden fehlgeschlagen. Bitte erneut versuchen."
              : "Abmelden fehlgeschlagen. Bitte erneut versuchen.",
          );
        }
        return null;
      } finally {
        setPendingIds((prev) => {
          const next = new Set(prev);
          for (const id of ids) next.delete(id);
          return next;
        });
        isBulkRunningRef.current = false;
        setIsBulkRunning(false);
        // A scope change during the run requested a clear; the response has
        // been processed against the intact selection above, so the deferred
        // clear may now apply. It keeps exactly the run's failed students:
        // the failure dialog names them and promises a one-tap retry, so the
        // scope change may drop every OTHER mark (they belong to a view that
        // no longer exists) but not these (see the clearSelection doc).
        if (pendingClearRef.current) {
          pendingClearRef.current = false;
          setSelectedIds(
            (prev) => new Set([...prev].filter((id) => failedRunIds.has(id))),
          );
        }
      }
    },
    [selectedIds, toast],
  );

  return {
    isActive,
    toggleActive,
    deactivate,
    pendingIds,
    successCount,
    toggle,
    selectionActive,
    setSelectionActive,
    selectedIds,
    toggleSelected,
    clearSelection,
    isBulkRunning,
    runBulk,
  };
}
