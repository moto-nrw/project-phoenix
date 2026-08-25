"use client";

import { useCallback, useEffect, useState } from "react";
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
import {
  moveNoticeFromRoster,
  runOwnAttendanceMutation,
  runRosterActionRequest,
  type RosterAction,
} from "~/components/active-supervisions/timetable-roster";
import { spontaneousActivityWindow } from "~/components/active-supervisions/spontaneous-window";
import type { SpontaneousActivityStartPayload } from "~/components/active-supervisions/spontaneous-activity-start";
import type { ActiveSupervisionRoom } from "~/components/active-supervisions/view-model";

const logger = createLogger({ component: "ActiveSupervisionsPage" });

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
  readonly refresh: () => void;
  readonly adoptSession: (
    activeGroupId: string,
    timetableInstanceId: string | null,
  ) => void;
  readonly setSelectedTimetableInstanceId: (id: string | null) => void;
  readonly setError: (message: string | null) => void;
  readonly router: { push: (url: string) => void };
  readonly reopenableInstanceId: string | null;
  readonly rememberReopenable: (
    instanceId: string,
    reopenUntil: string | null | undefined,
  ) => void;
  readonly clearReopenable: () => void;
}

export interface TimetableActions {
  readonly isStartingInstance: string | null;
  readonly isStartingSpontaneous: boolean;
  readonly isCompletingInstance: boolean;
  readonly isConfirmingExpected: boolean;
  readonly isAddingStudent: boolean;
  readonly showCompleteConfirmation: boolean;
  readonly setShowCompleteConfirmation: (open: boolean) => void;
  readonly moveNotice: string | null;
  readonly addStudentSearch: string;
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
    refresh,
    adoptSession,
    setSelectedTimetableInstanceId,
    setError,
    router,
    reopenableInstanceId,
    rememberReopenable,
    clearReopenable,
  } = options;

  const activeTimetableInstanceIdRef = useLatest(activeTimetableInstanceId);

  const [isStartingInstance, setIsStartingInstance] = useState<string | null>(
    null,
  );
  const [isStartingSpontaneous, setIsStartingSpontaneous] = useState(false);
  const [isCompletingInstance, setIsCompletingInstance] = useState(false);
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

  const addStudentResults =
    addStudentResult?.instanceId === activeTimetableInstanceId
      ? addStudentResult.students
      : [];

  // The move notice belongs to the session it happened in — drop it when the
  // supervisor switches to another session tab.
  useEffect(() => {
    setMoveNotice(null);
  }, [activeTimetableInstanceId]);

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
        });
    }, 250);

    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
    };
  }, [activeTimetableInstanceId, addStudentSearch]);

  const handleAddStudentSearchChange = useCallback((value: string) => {
    setAddStudentSearch(value);
    setAddStudentResult(null);
  }, []);

  const handleStartPlannedInstance = useCallback(
    async (instance: PlannedTimetableInstance) => {
      try {
        setIsStartingInstance(instance.id);
        const result = await timetableOperationsApi.start(instance.id);
        const startedRoom = allRooms.find(
          (room) => room.room_id === instance.roomId,
        );
        adoptSession(result.activeGroupId, instance.id);
        router.push(`/active-supervisions?session=${result.activeGroupId}`);
        localStorage.setItem("supervision-last-session", result.activeGroupId);
        localStorage.setItem("sidebar-last-room", instance.roomId);
        if (startedRoom?.room_name) {
          localStorage.setItem("sidebar-last-room-name", startedRoom.room_name);
        } else {
          localStorage.removeItem("sidebar-last-room-name");
        }
        await mutateDashboard();
        refresh();
      } catch (err) {
        logger.error("failed to start planned timetable instance", {
          instance_id: instance.id,
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Geplante Aktivität konnte nicht gestartet werden.");
      } finally {
        setIsStartingInstance(null);
      }
    },
    [allRooms, adoptSession, mutateDashboard, refresh, router, setError],
  );

  const handleStartSpontaneousActivity = useCallback(
    async (payload: SpontaneousActivityStartPayload) => {
      if (!currentStaffId) {
        setError(
          "Aktivität konnte nicht gestartet werden: kein Betreuerprofil.",
        );
        return;
      }

      try {
        setIsStartingSpontaneous(true);
        const window = spontaneousActivityWindow(new Date());
        const staffIds = Array.from(
          new Set([currentStaffId, ...payload.additionalStaffIds]),
        )
          .map(Number)
          .filter((id) => Number.isSafeInteger(id) && id > 0);
        if (staffIds.length === 0) {
          throw new Error("current staff id is not numeric");
        }
        const result = await timetableOperationsApi.createAndStartSpontaneous({
          date: window.date,
          start_time: window.startTime,
          end_time: window.endTime,
          title: payload.title,
          room_id: Number(payload.roomId),
          activity_group_id: payload.activityGroupId
            ? Number(payload.activityGroupId)
            : undefined,
          staff_ids: staffIds,
          student_ids: [],
        });
        adoptSession(result.activeGroupId, result.instanceId);
        router.push(`/active-supervisions?session=${result.activeGroupId}`);
        localStorage.setItem("supervision-last-session", result.activeGroupId);
        localStorage.setItem("sidebar-last-room", payload.roomId);
        await mutateDashboard();
        refresh();
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        const isRoomOccupied = message.includes("room is already occupied");
        const context = {
          title: payload.title,
          room_id: payload.roomId,
          error: message,
        };
        if (isRoomOccupied) {
          logger.warn("spontaneous timetable room already occupied", context);
        } else {
          logger.error(
            "failed to start spontaneous timetable instance",
            context,
          );
        }
        setError(
          isRoomOccupied
            ? "Der Raum ist bereits belegt."
            : "Spontane Aktivität konnte nicht gestartet werden.",
        );
      } finally {
        setIsStartingSpontaneous(false);
      }
    },
    [currentStaffId, adoptSession, mutateDashboard, refresh, router, setError],
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
        });
        setError("Aktion im Betreuungsplan konnte nicht ausgeführt werden.");
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
      setError,
    ],
  );

  const confirmCompleteTimetableInstance = useCallback(async () => {
    if (!activeTimetableInstanceId) return;
    try {
      setIsCompletingInstance(true);
      const completed = await timetableOperationsApi.complete(
        activeTimetableInstanceId,
        currentTimetableRoster?.rows
          .filter((row) => row.currentlyPresent)
          .map((row) => row.studentId) ?? [],
      );
      rememberReopenable(activeTimetableInstanceId, completed.reopenUntil);
      setShowCompleteConfirmation(false);
      setSelectedTimetableInstanceId(null);
      await mutateDashboard();
      refresh();
    } catch (err) {
      logger.error("failed to complete timetable instance", {
        instance_id: activeTimetableInstanceId,
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Aktivität konnte nicht beendet werden.");
    } finally {
      setIsCompletingInstance(false);
    }
  }, [
    activeTimetableInstanceId,
    currentTimetableRoster,
    mutateDashboard,
    refresh,
    rememberReopenable,
    setSelectedTimetableInstanceId,
    setError,
  ]);

  const handleCompleteTimetableInstance = useCallback(async () => {
    setShowCompleteConfirmation(true);
  }, []);

  const handleReopenTimetableInstance = useCallback(async () => {
    if (!reopenableInstanceId) return;
    try {
      const result = await timetableOperationsApi.reopen(reopenableInstanceId);
      clearReopenable();
      setSelectedTimetableInstanceId(result.instanceId);
      await mutateDashboard();
      refresh();
    } catch (err) {
      if (isReopenUnavailableError(err)) {
        clearReopenable();
      }
      setError(
        err instanceof Error
          ? err.message
          : "Aktivität konnte nicht wieder geöffnet werden.",
      );
    }
  }, [
    clearReopenable,
    mutateDashboard,
    refresh,
    reopenableInstanceId,
    setSelectedTimetableInstanceId,
    setError,
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
        setError("Erwartete Kinder konnten nicht bestätigt werden.");
      } finally {
        setIsConfirmingExpected(false);
      }
    },
    [
      activeTimetableInstanceId,
      activeTimetableInstanceIdRef,
      mutateDashboard,
      mutateRoster,
      setError,
    ],
  );

  const handleAddUnplannedStudent = useCallback(
    async (studentId: string) => {
      if (!activeTimetableInstanceId) return false;
      const instanceId = activeTimetableInstanceId;
      setMoveNotice(null);
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
        setError("Kind konnte nicht zur Aktivität hinzugefügt werden.");
        return false;
      } finally {
        setIsAddingStudent(false);
      }
    },
    [
      activeTimetableInstanceId,
      activeTimetableInstanceIdRef,
      mutateRoster,
      setError,
    ],
  );

  return {
    isStartingInstance,
    isStartingSpontaneous,
    isCompletingInstance,
    isConfirmingExpected,
    isAddingStudent,
    showCompleteConfirmation,
    setShowCompleteConfirmation,
    moveNotice,
    addStudentSearch,
    addStudentResults,
    handleAddStudentSearchChange,
    handleStartPlannedInstance,
    handleStartSpontaneousActivity,
    handleRosterAction,
    confirmCompleteTimetableInstance,
    handleCompleteTimetableInstance,
    handleReopenTimetableInstance,
    handleConfirmExpectedStudents,
    handleAddUnplannedStudent,
  };
}
