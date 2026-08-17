"use client";

import { useCallback, useReducer, useState } from "react";
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
   *  whenever the mode is freshly switched on. */
  toggleActive: () => void;
  /** Set mode off (used when navigating away or on unrecoverable errors). */
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
  clearSelection: () => void;
  /** True while a bulk action's API call is in flight. */
  isBulkRunning: boolean;
  /**
   * Apply one explicit action to every selected student via the batch
   * endpoint. On success the selection keeps only the failed students (so a
   * retry is one tap) and the presence caches are revalidated once, bundled.
   * Returns null when nothing was selected, a run is already in flight, or
   * the whole request failed (the hook has toasted the error then).
   */
  runBulk: (
    action: SchoolCheckinAction,
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
  const toast = useToast();

  const clearSelection = useCallback(() => {
    setSelectedIds(new Set());
  }, []);

  // Entering, leaving, or re-entering the mode always drops selection state:
  // a marked-but-never-executed selection must not survive into the next
  // session, where it would silently widen a bulk action.
  const resetSelectionState = useCallback(() => {
    setSelectionActiveState(false);
    setSelectedIds(new Set());
  }, []);

  const toggleActive = useCallback(() => {
    dispatchSession("toggle");
    resetSelectionState();
  }, [resetSelectionState]);

  const deactivate = useCallback(() => {
    dispatchSession("deactivate");
    resetSelectionState();
  }, [resetSelectionState]);

  const setSelectionActive = useCallback((active: boolean) => {
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
    ): Promise<SchoolCheckinBatchOutcome | null> => {
      if (isBulkRunning || selectedIds.size === 0) return null;

      const ids = Array.from(selectedIds);
      setIsBulkRunning(true);
      // Mark every batch member pending so the cards show the same in-flight
      // state a single toggle does.
      setPendingIds((prev) => new Set([...prev, ...ids]));

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

        // Keep only the failed students selected: they stay visibly marked
        // and a retry is one tap, while the processed ones drop out.
        const failedIds = new Set(
          outcome.results
            .filter((result) => !result.ok)
            .map((result) => result.studentId),
        );
        setSelectedIds(failedIds);

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
        setIsBulkRunning(false);
      }
    },
    [isBulkRunning, selectedIds, toast],
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
