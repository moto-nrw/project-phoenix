"use client";

import {
  useCallback,
  useLayoutEffect,
  useReducer,
  useRef,
  useState,
} from "react";
import { mutate as globalMutate } from "~/lib/swr/mutate";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import {
  isNotCheckedInLocation,
  isSchoolyardLocation,
  parseLocation,
} from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { activeService, type StudentMoveResult } from "~/lib/active-service";
import type { TargetRoomOption } from "~/lib/move-targets";
import {
  schoolCheckinStudent,
  schoolCheckinStudentsBatch,
  type SchoolCheckinAction,
  type SchoolCheckinBatchOutcome,
  type SchoolCheckoutReason,
} from "~/lib/student-api";
import { STUDENT_KEY_FAMILIES, studentKeys } from "~/lib/swr/portal-keys";
import { groupKeys } from "~/lib/swr/groups-keys";
import { SEARCH_STUDENTS_KEY_PREFIX } from "~/lib/swr/search-students-key";
import { roomKeys } from "~/lib/swr/rooms-keys";

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
 *   - empty / null / "Zuhause" / "Schule" / "Im Unterricht" → abwesend (red)
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
  if (isNotCheckedInLocation(currentLocation)) return "abwesend";
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
 * Names what an action changes in the shared error text: „Die Anwesenheit
 * konnte nicht geändert werden.“ Not „die Anmeldung“: in moto that word
 * also means a child's enrolment.
 */
const CHECKIN_OBJECT = "die Anwesenheit";

/** Checks the children in and places them in the target in one request (#6953). */
function checkInToTarget(
  studentIds: string[],
  target: TargetRoomOption,
): Promise<StudentMoveResult> {
  return target.kind === "openRoom"
    ? activeService.checkInStudentsToOpenRoom(studentIds, target.roomId)
    : activeService.checkInStudentsToActiveGroup(
        studentIds,
        target.activeGroupId,
      );
}

/**
 * The batch outcome of a check-in into a target (#6953), in the shape of the
 * school check-in batch, so the page's retry and failure handling stays one.
 * `presentIds` were already present and were not sent: they count as done
 * and unchanged. A child the backend skipped failed.
 */
export function placementOutcome(
  sentIds: readonly string[],
  presentIds: readonly string[],
  result: StudentMoveResult,
): SchoolCheckinBatchOutcome {
  const skipped = new Map(
    result.skipped.map((item) => [String(item.student_id), item.reason]),
  );
  const checkedIn = new Set((result.checked_in ?? []).map(String));
  const results: SchoolCheckinBatchOutcome["results"] = [
    ...sentIds.map((studentId) => {
      const reason = skipped.get(studentId);
      return reason
        ? { studentId, ok: false, changed: false, error: reason }
        : {
            studentId,
            ok: true,
            changed: checkedIn.has(studentId),
            location: "Anwesend" as const,
          };
    }),
    ...presentIds.map((studentId) => ({
      studentId,
      ok: true,
      changed: false,
    })),
  ];
  const failed = results.filter((item) => !item.ok).length;
  return {
    action: "in",
    results,
    succeeded: results.length - failed,
    failed,
  };
}

/** „5 Kinder in Mensa angemeldet. 2 waren schon da und bleiben, wo sie sind.“ */
export function placementMessage(
  placed: number,
  alreadyPresent: number,
  roomName: string,
): string {
  const head =
    placed === 0
      ? "Alle ausgewählten Kinder sind schon angemeldet."
      : `${placed} ${placed === 1 ? "Kind" : "Kinder"} in ${roomName} angemeldet.`;
  if (placed === 0 || alreadyPresent === 0) return head;
  return `${head} ${alreadyPresent} ${
    alreadyPresent === 1 ? "war" : "waren"
  } schon da und ${alreadyPresent === 1 ? "bleibt" : "bleiben"}, wo ${
    alreadyPresent === 1 ? "es ist" : "sie sind"
  }.`;
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
async function invalidatePresenceCachesBulk(
  placedInRoom: boolean,
): Promise<void> {
  await globalMutate(
    (key) =>
      typeof key === "string" &&
      (key.includes(groupKeys.liveFamily) ||
        key.includes(SEARCH_STUDENTS_KEY_PREFIX) ||
        key.includes("tracking-supervisions-") ||
        key.includes("tracking-indicators-") ||
        key.includes(STUDENT_KEY_FAMILIES.detail) ||
        (placedInRoom && isRoomPresenceKey(key))),
  );
}

/** Room lists and room pages count the children a check-in placed (#6953). */
function isRoomPresenceKey(key: string): boolean {
  return (
    key.includes("room-students-") ||
    key.includes("room-detail-") ||
    key.includes("room-bulk-") ||
    key.includes(roomKeys.family) ||
    key.includes("dashboard")
  );
}

interface UseSchoolCheckinModeOptions {
  /**
   * Where a check-in puts the children (#6953); null or missing keeps
   * „Anwesend (ohne Raum)“. Only check-ins use it: a checkout ends the stay
   * wherever it is.
   */
  readonly target?: TargetRoomOption | null;
  /** The backend refused the target (ended, no longer released): reload it. */
  readonly onTargetRefused?: () => void;
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
   * invalidates the SWR caches that surface presence data. `checkoutNote`
   * is the optional reason for an early checkout (#3324) and `checkoutReason`
   * marks a child going to class (#5656); both are sent only with "out".
   */
  toggle: (
    studentId: string,
    currentState: StudentCheckinState,
    checkoutNote?: string,
    checkoutReason?: SchoolCheckoutReason,
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
   * Mark or unmark several students at once: the table view's checkboxes,
   * including its "alle auswählen" box (#3834). Works without the check-in
   * mode — the table marks children directly.
   */
  setSelected: (studentIds: readonly string[], selected: boolean) => void;
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
   * `onlyIds` is a dialog's displayed snapshot and is executed EXACTLY as
   * given: the page passes the currently VISIBLE selected students, the
   * confirmation dialog's snapshot of them, or the failure dialog's failed
   * children — so a mark hidden by a filter is never executed without being
   * on screen, and a dialog executes exactly what it displayed (review
   * #2372). Deliberately NOT intersected with the live selection: a
   * search/filter change that commits while the failure dialog is open
   * clears the selection, and an intersected retry would silently shrink to
   * nothing despite the dialog's promise (review #2372). Without `onlyIds`
   * the run covers the live selection. After the run, successfully
   * processed students are removed from the selection while failures (and
   * any students marked mid-flight) stay selected where possible, so a
   * retry is one tap. The presence caches are revalidated once, bundled.
   * Returns null when nothing was selected, a run is already in flight, or
   * the whole request failed (the hook has toasted the error then).
   */
  runBulk: (
    action: SchoolCheckinAction,
    onlyIds?: readonly string[],
    checkoutReason?: SchoolCheckoutReason,
    /**
     * Children of the run who are present already. With a target they keep
     * their place: the check-in mode places only arriving children, so
     * „Alle auswählen“ never pulls a whole school into one room (#6953).
     */
    presentIds?: ReadonlySet<string>,
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
export function useSchoolCheckinMode(
  options: UseSchoolCheckinModeOptions = {},
): UseSchoolCheckinModeResult {
  const targetRef = useRef(options.target ?? null);
  const onTargetRefusedRef = useRef(options.onTargetRefused);
  useLayoutEffect(() => {
    targetRef.current = options.target ?? null;
    onTargetRefusedRef.current = options.onTargetRefused;
  });
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
  // A permission refusal fails on every retry, so the catalog text of that
  // class names the reason instead of offering a retry (#2220, #2513).
  const errors = useApiErrorDisplay();
  const showError = errors.show;
  const latestToggleRef = useRef<
    (
      studentId: string,
      currentState: StudentCheckinState,
      checkoutNote?: string,
      checkoutReason?: SchoolCheckoutReason,
    ) => Promise<void>
  >(async () => undefined);

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

  const setSelected = useCallback(
    (studentIds: readonly string[], selected: boolean) => {
      setSelectedIds((prev) => {
        const next = new Set(prev);
        for (const id of studentIds) {
          if (selected) next.add(id);
          else next.delete(id);
        }
        return next;
      });
    },
    [],
  );

  const toggle = useCallback(
    async (
      studentId: string,
      currentState: StudentCheckinState,
      checkoutNote?: string,
      checkoutReason?: SchoolCheckoutReason,
    ) => {
      if (pendingIds.has(studentId)) return;

      setPendingIds((prev) => {
        const next = new Set(prev);
        next.add(studentId);
        return next;
      });

      const action = actionForState(currentState);
      const target = action === "in" ? targetRef.current : null;

      try {
        await (target
          ? checkInToTarget([studentId], target)
          : checkoutReason
            ? schoolCheckinStudent(
                studentId,
                action,
                checkoutNote,
                checkoutReason,
              )
            : schoolCheckinStudent(studentId, action, checkoutNote));

        await globalMutate(
          (key) =>
            typeof key === "string" &&
            (key.includes(groupKeys.liveFamily) ||
              key.includes(SEARCH_STUDENTS_KEY_PREFIX) ||
              key.includes("tracking-supervisions-") ||
              key.includes("tracking-indicators-") ||
              key.endsWith(studentKeys.detail(studentId)) ||
              (target !== null && isRoomPresenceKey(key))),
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
          ...(target ? { target_kind: target.kind } : {}),
          error: message,
        });
        if (target) onTargetRefusedRef.current?.();
        void showError(error, {
          object: CHECKIN_OBJECT,
          retry: () =>
            void latestToggleRef.current(
              studentId,
              currentState,
              checkoutNote,
              checkoutReason,
            ),
        });
      } finally {
        setPendingIds((prev) => {
          const next = new Set(prev);
          next.delete(studentId);
          return next;
        });
      }
    },
    [pendingIds, showError],
  );
  useLayoutEffect(() => {
    latestToggleRef.current = toggle;
  });

  const runBulk = useCallback(
    async (
      action: SchoolCheckinAction,
      onlyIds?: readonly string[],
      checkoutReason?: SchoolCheckoutReason,
      presentIds?: ReadonlySet<string>,
    ): Promise<SchoolCheckinBatchOutcome | null> => {
      if (isBulkRunningRef.current) return null;
      const target = action === "in" ? targetRef.current : null;

      // An explicit id list is a dialog's displayed snapshot and runs
      // exactly as displayed — no intersection with the live selection (see
      // the interface doc: an intersected retry after a committed scope
      // change would be a silent no-op, review #2372). Every onlyIds caller
      // shows the user precisely this list before executing it.
      const ids = onlyIds ? [...onlyIds] : Array.from(selectedIds);
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
        let outcome: SchoolCheckinBatchOutcome;
        if (target) {
          const present = ids.filter((id) => presentIds?.has(id));
          const arriving = ids.filter((id) => !presentIds?.has(id));
          outcome =
            arriving.length > 0
              ? placementOutcome(
                  arriving,
                  present,
                  await checkInToTarget(arriving, target),
                )
              : placementOutcome([], present, {
                  moved: [],
                  unchanged: [],
                  skipped: [],
                });
        } else {
          outcome = await (checkoutReason
            ? schoolCheckinStudentsBatch(ids, action, checkoutReason)
            : schoolCheckinStudentsBatch(ids, action));
        }

        await invalidatePresenceCachesBulk(target !== null);
        dispatchSession({ type: "addSuccesses", count: outcome.succeeded });

        // Clean run → summary toast right here (counts only, no names). A
        // partial failure is the caller's job: naming the failed children
        // needs the student list, which this hook does not hold.
        if (outcome.failed === 0 && target) {
          const alreadyPresent = outcome.results.filter(
            (result) => presentIds?.has(result.studentId) ?? false,
          ).length;
          toast.success(
            placementMessage(
              outcome.succeeded - alreadyPresent,
              alreadyPresent,
              target.roomName,
            ),
          );
        } else if (outcome.failed === 0) {
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
          ...(target ? { target_kind: target.kind } : {}),
          error: message,
        });
        if (target) onTargetRefusedRef.current?.();
        // No retry button: the caller shows the outcome of a run, which a
        // toast retry would bypass. The selection stays marked for a new tap.
        void showError(error, { object: CHECKIN_OBJECT });
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
    [selectedIds, toast, showError],
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
    setSelected,
    clearSelection,
    isBulkRunning,
    runBulk,
  };
}
