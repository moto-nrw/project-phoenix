"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import type { FormError } from "~/components/ui/form-error";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import { ApiError, wireErrorCode } from "~/lib/api-error";
import { errorStatus } from "~/lib/expected-failure";
import { createLogger } from "~/lib/logger";
import { fetchStudents } from "~/lib/student-api";
import {
  isReopenUnavailableError,
  timetableOperationsApi,
} from "~/lib/timetable-operations-api";
import type {
  PlannedTimetableInstance,
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import type { Student } from "~/lib/student-helpers";
import { useLatest } from "~/lib/hooks/use-latest";
import { isTimetableOperationForbidden } from "~/lib/timetable-operation-access";
import { useOptionalSupervision } from "~/lib/supervision-context";
import {
  RestOfDayNotSavedError,
  moveNoticeFromRoster,
  runOwnAttendanceMutation,
  runRestOfDayExcusalRequest,
  runRosterActionRequest,
  type RosterAction,
} from "~/components/active-supervisions/timetable-roster";
import type { SpontaneousActivityStartPayload } from "~/components/active-supervisions/spontaneous-activity-start";
import type { ActiveSupervisionRoom } from "~/components/active-supervisions/view-model";
import type { ReopenableInstance } from "~/components/active-supervisions/use-reopen-banner";

const logger = createLogger({ component: "ActiveSupervisionsPage" });

/**
 * Without an own staff profile no session can be started. The check runs
 * before sending, but the message is the catalog text of the backend code for
 * the same refusal, so both paths read the same.
 */
function noStaffProfileError(): ApiError {
  return new ApiError("no staff profile for spontaneous start", 403, {
    code: "timetable.no_staff_profile",
  });
}

interface TimetableActionsOptions {
  readonly allRooms: readonly ActiveSupervisionRoom[];
  readonly currentStaffId: string | undefined;
  readonly activeTimetableInstanceId: string | null;
  readonly currentTimetableRoster: TimetableRoster | null;
  readonly mutateRoster: (
    data?: TimetableRoster | null,
    opts?: { revalidate?: boolean },
  ) => Promise<unknown>;
  readonly mutateDashboard: () => Promise<unknown>;
  readonly adoptSession: (
    activeGroupId: string,
    timetableInstanceId: string | null,
    roomId: string,
  ) => string;
  readonly setSelectedTimetableInstanceId: (id: string | null) => void;
  readonly router: { push: (url: string) => void };
  readonly reopenable: ReopenableInstance | null;
  readonly rememberReopenable: (
    instance: ReopenableInstance,
    reopenUntil: string | null | undefined,
  ) => void;
  readonly clearReopenable: () => void;
}

export interface TimetableActions {
  readonly isStartingInstance: string | null;
  readonly isStartingSpontaneous: boolean;
  /**
   * True from „Aktivität beenden“ until the page and the sidebar have
   * reloaded, so the dialog shows the pending end instead of a block that
   * still looks active (#3888).
   */
  readonly isCompletingInstance: boolean;
  readonly isReopeningInstance: boolean;
  readonly isConfirmingExpected: boolean;
  readonly isAddingStudent: boolean;
  readonly showCompleteConfirmation: boolean;
  readonly setShowCompleteConfirmation: (open: boolean) => void;
  /** Failed „Beenden“; stays in the open confirmation dialog. */
  readonly completeError: FormError | null;
  readonly moveNotice: string | null;
  readonly addStudentSearch: string;
  /** Failed search or add; stays in the dialog „Kind ungeplant hinzufügen“. */
  readonly addStudentError: FormError | null;
  readonly addStudentResults: Student[];
  readonly handleAddStudentSearchChange: (value: string) => void;
  readonly handleStartPlannedInstance: (
    instance: PlannedTimetableInstance,
  ) => Promise<void>;
  readonly handleStartSpontaneousActivity: (
    payload: SpontaneousActivityStartPayload,
  ) => Promise<void>;
  readonly handleRosterAction: (
    action: RosterAction,
    row: TimetableRosterRow,
  ) => Promise<void>;
  readonly handleExcuseRestOfDay: (row: TimetableRosterRow) => Promise<void>;
  readonly confirmCompleteTimetableInstance: () => Promise<void>;
  readonly handleCompleteTimetableInstance: () => Promise<void>;
  readonly handleReopenTimetableInstance: () => Promise<void>;
  readonly handleConfirmExpectedStudents: (
    rows: TimetableRosterRow[],
  ) => Promise<void>;
  readonly handleAddUnplannedStudent: (studentId: string) => Promise<boolean>;
}

/**
 * The mutation side of the Web-Anwesenheit roster and timetable session
 * lifecycle: start planned/spontaneous sessions, per-child roster actions,
 * bulk confirm, add unplanned children, complete, and reopen. Pure
 * orchestration around the APIs — all data the page renders keeps coming
 * from useSupervisionDashboard / useTimetableRoster.
 *
 * Errors go through the shared display path (#2517): actions without a form
 * as a toast with the catalog text, the two dialogs (complete, add a child)
 * keep theirs inside the dialog.
 */
export function useTimetableActions(
  options: TimetableActionsOptions,
): TimetableActions {
  const {
    allRooms,
    currentStaffId,
    activeTimetableInstanceId,
    currentTimetableRoster,
    mutateRoster,
    mutateDashboard,
    adoptSession,
    setSelectedTimetableInstanceId,
    router,
    reopenable,
    rememberReopenable,
    clearReopenable,
  } = options;
  const { refresh: refreshSupervision } = useOptionalSupervision();

  const activeTimetableInstanceIdRef = useLatest(activeTimetableInstanceId);
  const { show: showActionError } = useApiErrorDisplay();
  const addStudentErrors = useApiFormError();
  const { clear: clearAddStudentError, show: showAddStudentError } =
    addStudentErrors;
  const completeErrors = useApiFormError();
  const { clear: clearCompleteError, show: showCompleteError } = completeErrors;

  const [isStartingInstance, setIsStartingInstance] = useState<string | null>(
    null,
  );
  const [isStartingSpontaneous, setIsStartingSpontaneous] = useState(false);
  const [isCompletingInstance, setIsCompletingInstance] = useState(false);
  const [isReopeningInstance, setIsReopeningInstance] = useState(false);
  const [showCompleteConfirmation, setShowCompleteConfirmation] =
    useState(false);
  const [isConfirmingExpected, setIsConfirmingExpected] = useState(false);
  const [addStudentSearch, setAddStudentSearch] = useState("");
  const [addStudentResult, setAddStudentResult] = useState<{
    readonly instanceId: string;
    readonly students: Student[];
  } | null>(null);
  const [isAddingStudent, setIsAddingStudent] = useState(false);
  // Info notice after a check-in auto-moved the child out of another running
  // session (#2386). Cleared by the next roster action.
  const [moveNotice, setMoveNotice] = useState<string | null>(null);

  // „Wiederholen“ läuft mit dem aktuellen Stand der Seite, nie mit dem der
  // fehlgeschlagenen Aktion (eine andere Sitzung, eine neue Liste).
  const retryRef = useRef<{
    startPlanned: (instance: PlannedTimetableInstance) => void;
    startSpontaneous: (payload: SpontaneousActivityStartPayload) => void;
    rosterAction: (action: RosterAction, row: TimetableRosterRow) => void;
    restOfDay: (row: TimetableRosterRow) => void;
    complete: () => void;
    confirmExpected: (rows: TimetableRosterRow[]) => void;
    reopen: () => void;
  }>({
    startPlanned: () => undefined,
    startSpontaneous: () => undefined,
    rosterAction: () => undefined,
    restOfDay: () => undefined,
    complete: () => undefined,
    confirmExpected: () => undefined,
    reopen: () => undefined,
  });

  // Starting, ending or reopening a block changes which supervisions run.
  // The sidebar („Aktuelle Aufsichten“) keeps its own copy and otherwise only
  // hears SSE, which may drop events. So the own action reloads both lists;
  // a failed reload leaves the next refresh to fix it and never reports the
  // write itself as failed. No extra SWR key bump afterwards: it fetched the
  // same aggregate a second time and kept the old view on screen meanwhile
  // (#3888).
  const reloadAfterLifecycleChange = useCallback(async () => {
    await Promise.allSettled([
      mutateDashboard(),
      refreshSupervision({ silent: true, force: true }),
    ]);
  }, [mutateDashboard, refreshSupervision]);

  const addStudentResults =
    addStudentResult?.instanceId === activeTimetableInstanceId
      ? addStudentResult.students
      : [];

  // Hinweise und Fehler gehören zur aktiven Sitzung und dürfen nicht in eine
  // andere Aufsicht übernommen werden.
  useEffect(() => {
    setMoveNotice(null);
    clearAddStudentError();
    clearCompleteError();
  }, [activeTimetableInstanceId, clearAddStudentError, clearCompleteError]);

  useEffect(() => {
    if (!activeTimetableInstanceId || addStudentSearch.trim().length < 2) {
      setAddStudentResult(null);
      return;
    }

    setAddStudentResult(null);
    let cancelled = false;
    const timeout = window.setTimeout(() => {
      fetchStudents({
        search: addStudentSearch.trim(),
        page: 1,
        page_size: 5,
      })
        .then((result) => {
          if (!cancelled) {
            setAddStudentResult({
              instanceId: activeTimetableInstanceId,
              students: result.students,
            });
          }
        })
        .catch((err) => {
          if (cancelled) return;
          logger.warn("failed to search students for timetable roster", {
            error: err instanceof Error ? err.message : String(err),
          });
          setAddStudentResult(null);
          // Kein Wiederholen-Link: die Suche läuft beim nächsten Tippen neu.
          void showAddStudentError(err, { object: "die Suche nach Kindern" });
        });
    }, 250);

    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
    };
  }, [activeTimetableInstanceId, addStudentSearch, showAddStudentError]);

  const handleAddStudentSearchChange = useCallback(
    (value: string) => {
      setAddStudentSearch(value);
      setAddStudentResult(null);
      clearAddStudentError();
    },
    [clearAddStudentError],
  );

  // After a planning denial the open list is stale: reload it so the actions
  // the caller may no longer use disappear.
  const revalidateRoster = useCallback(async () => {
    try {
      await mutateRoster();
    } catch (err) {
      // Bewusst still: die Ablehnung steht schon in der Meldung; die Liste
      // lädt beim nächsten Abruf neu.
      logger.warn("timetable_roster_revalidate_failed_after_forbidden", {
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }, [mutateRoster]);

  // A planning denial (`timetable.operation_not_planned`, #3167) reloads the
  // list so its actions disappear; the message itself is the catalog text.
  const reloadAfterDenial = useCallback(
    (err: unknown) => {
      if (isTimetableOperationForbidden(err)) void revalidateRoster();
    },
    [revalidateRoster],
  );

  const handleStartPlannedInstance = useCallback(
    async (instance: PlannedTimetableInstance) => {
      try {
        setIsStartingInstance(instance.id);
        const result = await timetableOperationsApi.start(instance.id);
        const startedRoom = allRooms.find(
          (room) => room.room_id === instance.roomId,
        );
        router.push(
          adoptSession(result.activeGroupId, instance.id, instance.roomId),
        );
        localStorage.setItem("supervision-last-session", result.activeGroupId);
        localStorage.setItem("sidebar-last-room", instance.roomId);
        if (startedRoom?.room_name) {
          localStorage.setItem("sidebar-last-room-name", startedRoom.room_name);
        } else {
          localStorage.removeItem("sidebar-last-room-name");
        }
        await reloadAfterLifecycleChange();
      } catch (err) {
        logger.error("failed to start planned timetable instance", {
          instance_id: instance.id,
          error: err instanceof Error ? err.message : String(err),
          status: errorStatus(err),
        });
        void showActionError(err, {
          object: "die geplante Aktivität",
          retry: () => retryRef.current.startPlanned(instance),
        });
      } finally {
        setIsStartingInstance(null);
      }
    },
    [
      allRooms,
      adoptSession,
      reloadAfterLifecycleChange,
      router,
      showActionError,
    ],
  );

  const handleStartSpontaneousActivity = useCallback(
    async (payload: SpontaneousActivityStartPayload) => {
      const staffIds = currentStaffId
        ? Array.from(new Set([currentStaffId, ...payload.additionalStaffIds]))
            .map(Number)
            .filter((id) => Number.isSafeInteger(id) && id > 0)
        : [];
      if (staffIds.length === 0) {
        logger.warn("spontaneous timetable start without staff profile");
        void showActionError(noStaffProfileError(), {
          object: "die spontane Aktivität",
        });
        return;
      }

      try {
        setIsStartingSpontaneous(true);
        const result = await timetableOperationsApi.createAndStartSpontaneous({
          title: payload.title,
          room_id: Number(payload.roomId),
          activity_group_id: payload.activityGroupId
            ? Number(payload.activityGroupId)
            : undefined,
          staff_ids: staffIds,
        });
        router.push(
          adoptSession(result.activeGroupId, result.instanceId, payload.roomId),
        );
        localStorage.setItem("supervision-last-session", result.activeGroupId);
        localStorage.setItem("sidebar-last-room", payload.roomId);
        await reloadAfterLifecycleChange();
      } catch (err) {
        const context = {
          title: payload.title,
          room_id: payload.roomId,
          error: err instanceof Error ? err.message : String(err),
        };
        if (
          err instanceof ApiError &&
          wireErrorCode(err.code) === "timetable.room_occupied"
        ) {
          logger.warn("spontaneous timetable room already occupied", context);
        } else {
          logger.error(
            "failed to start spontaneous timetable instance",
            context,
          );
        }
        void showActionError(err, {
          object: "die spontane Aktivität",
          retry: () => retryRef.current.startSpontaneous(payload),
        });
      } finally {
        setIsStartingSpontaneous(false);
      }
    },
    [
      currentStaffId,
      adoptSession,
      reloadAfterLifecycleChange,
      router,
      showActionError,
    ],
  );

  const handleRosterAction = useCallback(
    async (action: RosterAction, row: TimetableRosterRow) => {
      if (!activeTimetableInstanceId) return;
      const instanceId = activeTimetableInstanceId;
      setMoveNotice(null);
      let rosterResult: TimetableRoster | null;
      try {
        rosterResult = await runRosterActionRequest(
          action,
          instanceId,
          row.studentId,
        );
      } catch (err) {
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
        logger.error("failed timetable roster action", {
          action,
          student_id: row.studentId,
          error: err instanceof Error ? err.message : String(err),
          status: errorStatus(err),
        });
        reloadAfterDenial(err);
        void showActionError(err, {
          object: `die Anwesenheit von ${row.studentName}`,
          retry: () => retryRef.current.rosterAction(action, row),
        });
        return;
      }
      if (activeTimetableInstanceIdRef.current !== instanceId) return;
      try {
        if (action === "check-in" && rosterResult) {
          setMoveNotice(moveNoticeFromRoster(rosterResult, row.studentId));
        }
        await (rosterResult
          ? mutateRoster(rosterResult, { revalidate: false })
          : mutateRoster());
      } catch (err) {
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
        // Sichtbar über das Neuladen: die Aktion ist gespeichert, nur die
        // Liste ist nicht mehr aktuell.
        logger.warn("timetable_roster_sync_failed_after_successful_action", {
          action,
          student_id: row.studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        void logger.flush();
        window.location.reload();
      }
    },
    [
      activeTimetableInstanceId,
      activeTimetableInstanceIdRef,
      mutateRoster,
      reloadAfterDenial,
      showActionError,
    ],
  );

  // „Rest des Tages“ (#3166): this block by hand, every later block of the day
  // through a partial absence from this block's start.
  const handleExcuseRestOfDay = useCallback(
    async (row: TimetableRosterRow) => {
      const instance = currentTimetableRoster?.instance;
      if (
        !activeTimetableInstanceId ||
        instance?.id !== activeTimetableInstanceId
      )
        return;
      const instanceId = activeTimetableInstanceId;
      setMoveNotice(null);
      try {
        await runRestOfDayExcusalRequest(instance, row.studentId);
      } catch (err) {
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
        const blockExcused = err instanceof RestOfDayNotSavedError;
        logger.error("timetable_rest_of_day_excusal_failed", {
          instance_id: instanceId,
          student_id: row.studentId,
          block_excused: blockExcused,
          error: err instanceof Error ? err.message : String(err),
        });
        // This block is excused; only the later blocks failed. The object
        // names that part, the catalog text says what to do.
        void showActionError(blockExcused ? err.cause : err, {
          object: blockExcused
            ? `die Entschuldigung der späteren Blöcke von ${row.studentName}`
            : `die Entschuldigung von ${row.studentName}`,
          retry: () => retryRef.current.restOfDay(row),
        });
      }
      if (activeTimetableInstanceIdRef.current !== instanceId) return;
      try {
        await mutateRoster();
      } catch (err) {
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
        // Sichtbar über das Neuladen: die Liste ist nicht mehr aktuell.
        logger.warn("timetable_roster_sync_failed_after_successful_action", {
          action: "excused-rest-of-day",
          student_id: row.studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        void logger.flush();
        window.location.reload();
      }
    },
    [
      activeTimetableInstanceId,
      activeTimetableInstanceIdRef,
      currentTimetableRoster,
      mutateRoster,
      showActionError,
    ],
  );

  const confirmCompleteTimetableInstance = useCallback(async () => {
    if (!activeTimetableInstanceId) return;
    clearCompleteError();
    try {
      setIsCompletingInstance(true);
      const completed = await timetableOperationsApi.complete(
        activeTimetableInstanceId,
        currentTimetableRoster?.rows
          .filter((row) => row.currentlyPresent)
          .map((row) => row.studentId) ?? [],
      );
      rememberReopenable(
        {
          instanceId: activeTimetableInstanceId,
          title: currentTimetableRoster?.instance.title ?? null,
          roomId: currentTimetableRoster?.instance.roomId ?? null,
        },
        completed.reopenUntil,
      );
      // The dialog stays open with its pending button until both lists no
      // longer carry the ended block; closing first would show it as still
      // running for the length of the reload (#3888).
      await reloadAfterLifecycleChange();
      setShowCompleteConfirmation(false);
      setSelectedTimetableInstanceId(null);
    } catch (err) {
      logger.error("failed to complete timetable instance", {
        instance_id: activeTimetableInstanceId,
        error: err instanceof Error ? err.message : String(err),
        status: errorStatus(err),
      });
      reloadAfterDenial(err);
      // Der Fehler bleibt im offenen Bestätigungsdialog.
      void showCompleteError(err, {
        object: "das Beenden der Aktivität",
        retry: () => retryRef.current.complete(),
      });
    } finally {
      setIsCompletingInstance(false);
    }
  }, [
    activeTimetableInstanceId,
    clearCompleteError,
    currentTimetableRoster,
    rememberReopenable,
    reloadAfterDenial,
    reloadAfterLifecycleChange,
    setSelectedTimetableInstanceId,
    showCompleteError,
  ]);

  const handleCompleteTimetableInstance = useCallback(async () => {
    clearCompleteError();
    setShowCompleteConfirmation(true);
  }, [clearCompleteError]);

  const handleReopenTimetableInstance = useCallback(async () => {
    if (!reopenable) return;
    try {
      setIsReopeningInstance(true);
      const result = await timetableOperationsApi.reopen(reopenable.instanceId);
      clearReopenable();
      // Undo opens the restored block, wherever the page went after the end.
      // An entry stored before the room was remembered keeps the old
      // behaviour: it only selects the block.
      if (reopenable.roomId) {
        router.push(
          adoptSession(
            result.activeGroupId,
            result.instanceId,
            reopenable.roomId,
          ),
        );
        localStorage.setItem("supervision-last-session", result.activeGroupId);
        localStorage.setItem("sidebar-last-room", reopenable.roomId);
      } else {
        setSelectedTimetableInstanceId(result.instanceId);
      }
      await reloadAfterLifecycleChange();
    } catch (err) {
      logger.error("failed to reopen timetable instance", {
        instance_id: reopenable.instanceId,
        error: err instanceof Error ? err.message : String(err),
        status: errorStatus(err),
      });
      const unavailable = isReopenUnavailableError(err);
      if (unavailable) {
        clearReopenable();
      }
      // A full room names itself through its code and details (#3633).
      // Wiederholen only while the banner still offers the undo.
      void showActionError(err, {
        object: "die Rücknahme",
        retry: unavailable ? undefined : () => retryRef.current.reopen(),
      });
    } finally {
      setIsReopeningInstance(false);
    }
  }, [
    adoptSession,
    clearReopenable,
    reloadAfterLifecycleChange,
    reopenable,
    router,
    setSelectedTimetableInstanceId,
    showActionError,
  ]);

  const handleConfirmExpectedStudents = useCallback(
    async (rows: TimetableRosterRow[]) => {
      if (!activeTimetableInstanceId || rows.length === 0) return;
      const instanceId = activeTimetableInstanceId;
      setMoveNotice(null);
      try {
        setIsConfirmingExpected(true);
        let nextRoster: TimetableRoster | null = null;
        const notices: string[] = [];
        for (const row of rows) {
          nextRoster = await runOwnAttendanceMutation(
            "student_checkin",
            row.studentId,
            () => timetableOperationsApi.checkIn(instanceId, row.studentId),
          );
          if (activeTimetableInstanceIdRef.current !== instanceId) continue;
          const notice = moveNoticeFromRoster(nextRoster, row.studentId);
          if (notice) notices.push(notice);
        }
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
        if (notices.length > 0) setMoveNotice(notices.join(" "));
        if (nextRoster) {
          await mutateRoster(nextRoster, { revalidate: false });
        } else {
          await mutateRoster();
        }
        await mutateDashboard();
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
      } catch (err) {
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
        logger.error("failed to confirm expected timetable students", {
          instance_id: instanceId,
          count: rows.length,
          error: err instanceof Error ? err.message : String(err),
        });
        reloadAfterDenial(err);
        void showActionError(err, {
          object: "die Anwesenheit der erwarteten Kinder",
          retry: () => retryRef.current.confirmExpected(rows),
        });
      } finally {
        setIsConfirmingExpected(false);
      }
    },
    [
      activeTimetableInstanceId,
      activeTimetableInstanceIdRef,
      mutateDashboard,
      mutateRoster,
      reloadAfterDenial,
      showActionError,
    ],
  );

  const handleAddUnplannedStudent = useCallback(
    async (studentId: string) => {
      if (!activeTimetableInstanceId) return false;
      const instanceId = activeTimetableInstanceId;
      setMoveNotice(null);
      clearAddStudentError();
      try {
        setIsAddingStudent(true);
        const rosterResult = await runOwnAttendanceMutation(
          "student_checkin",
          studentId,
          () => timetableOperationsApi.checkIn(instanceId, studentId),
        );
        if (activeTimetableInstanceIdRef.current !== instanceId) return false;
        setMoveNotice(moveNoticeFromRoster(rosterResult, studentId));
        setAddStudentSearch("");
        setAddStudentResult(null);
        await mutateRoster(rosterResult, { revalidate: false });
        return true;
      } catch (err) {
        if (activeTimetableInstanceIdRef.current !== instanceId) return false;
        logger.error("failed to add unplanned timetable student", {
          student_id: studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        reloadAfterDenial(err);
        // Im Dialog; „Hinzufügen“ steht direkt darunter, deshalb ohne
        // eigenen Wiederholen-Link.
        void showAddStudentError(err, { object: "das Kind" });
        return false;
      } finally {
        setIsAddingStudent(false);
      }
    },
    [
      activeTimetableInstanceId,
      activeTimetableInstanceIdRef,
      clearAddStudentError,
      mutateRoster,
      reloadAfterDenial,
      showAddStudentError,
    ],
  );

  useLayoutEffect(() => {
    retryRef.current = {
      startPlanned: (instance) => void handleStartPlannedInstance(instance),
      startSpontaneous: (payload) =>
        void handleStartSpontaneousActivity(payload),
      rosterAction: (action, row) => void handleRosterAction(action, row),
      restOfDay: (row) => void handleExcuseRestOfDay(row),
      complete: () => void confirmCompleteTimetableInstance(),
      confirmExpected: (rows) => void handleConfirmExpectedStudents(rows),
      reopen: () => void handleReopenTimetableInstance(),
    };
  });

  return {
    isStartingInstance,
    isStartingSpontaneous,
    isCompletingInstance,
    isReopeningInstance,
    isConfirmingExpected,
    isAddingStudent,
    showCompleteConfirmation,
    setShowCompleteConfirmation,
    completeError: completeErrors.error,
    moveNotice,
    addStudentSearch,
    addStudentError: addStudentErrors.error,
    addStudentResults,
    handleAddStudentSearchChange,
    handleStartPlannedInstance,
    handleStartSpontaneousActivity,
    handleRosterAction,
    handleExcuseRestOfDay,
    confirmCompleteTimetableInstance,
    handleCompleteTimetableInstance,
    handleReopenTimetableInstance,
    handleConfirmExpectedStudents,
    handleAddUnplannedStudent,
  };
}
