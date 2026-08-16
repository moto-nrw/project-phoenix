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
  type SchoolCheckinAction,
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

type CheckinModeSessionAction = "toggle" | "deactivate" | "increment";

function checkinModeSessionReducer(
  state: CheckinModeSession,
  action: CheckinModeSessionAction,
): CheckinModeSession {
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
  const toast = useToast();

  const toggleActive = useCallback(() => {
    dispatchSession("toggle");
  }, []);

  const deactivate = useCallback(() => {
    dispatchSession("deactivate");
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

  return {
    isActive,
    toggleActive,
    deactivate,
    pendingIds,
    successCount,
    toggle,
  };
}
