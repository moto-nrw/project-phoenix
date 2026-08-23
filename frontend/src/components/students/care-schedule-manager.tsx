"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { mutate } from "swr";
import {
  ChevronLeft,
  ChevronRight,
  Loader2,
  SquarePen,
  StickyNote,
  X,
} from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import {
  type CareExceptionSubmit,
  type CarePlanWeeklySubmit,
  CarePlanEditorModal,
} from "./care-plan-editor-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  type ArrivalData,
  createArrivalException,
  createArrivalNote,
  deleteArrivalException,
  deleteArrivalNote,
  fetchArrivalData,
  fetchArrivalSettings,
  updateArrivalException,
  updateArrivalNote,
  updateArrivalSchedules,
  type CareDaysSource,
} from "~/lib/student-arrival-api";
import {
  type ArrivalDayData,
  WEEKDAYS,
  formatDateISO,
  formatShortDate,
  getDayData as getArrivalDayData,
  getWeekDays,
  mergeSchedulesWithTemplate as mergeArrivalSchedulesWithTemplate,
  arrivalScheduleSourceLabel,
} from "~/lib/arrival-schedule-helpers";
import {
  createStudentPickupException,
  createStudentPickupNote,
  deleteStudentPickupException,
  deleteStudentPickupNote,
  fetchStudentPickupData,
  resetStudentPickupToOffering,
  updateStudentPickupException,
  updateStudentPickupNote,
  updateStudentPickupSchedules,
} from "~/lib/pickup-schedule-api";
import {
  type DayData as PickupDayData,
  type PickupData,
  formatPickupTime,
  formatWeekRange,
  getDayData as getPickupDayData,
  mergeSchedulesWithTemplate as mergePickupSchedulesWithTemplate,
  pickupScheduleSourceLabel,
} from "~/lib/pickup-schedule-helpers";
import { createLogger } from "~/lib/logger";
import type {
  StudentStatusDay,
  StudentStatusKind,
} from "~/lib/student-status-days-api";

const logger = createLogger({ component: "CareScheduleManager" });
const statusDayDateFormatter = new Intl.DateTimeFormat("de-DE", {
  weekday: "long",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
});
const weekMonthFormatter = new Intl.DateTimeFormat("de-DE", {
  month: "long",
  year: "numeric",
});
const EMPTY_STATUS_DAYS: StudentStatusDay[] = [];

interface CareScheduleManagerProps {
  readonly studentId: string;
  readonly readOnly?: boolean;
  readonly onUpdate?: () => void;
  readonly isSick?: boolean;
  readonly isExcused?: boolean;
  readonly statusDays?: StudentStatusDay[];
  readonly onDeleteStatusDay?: (statusDayId: string) => Promise<void>;
  readonly onVisibleDateRangeChange?: (from: string, to: string) => void;
}

interface CareDayData {
  readonly date: Date;
  readonly weekday: number;
  readonly isToday: boolean;
  readonly status: StudentStatusKind | null;
  readonly statusDay: StudentStatusDay | null;
  readonly arrival: ArrivalDayData;
  readonly pickup: PickupDayData;
}

type CareEventTone = "neutral" | "warning" | "purple";

interface CareBoundaryItem {
  readonly key: string;
  readonly label: string;
  readonly value: string;
  readonly description?: string;
  readonly marker: string | null;
  readonly icon: React.ReactNode;
  readonly onEdit?: () => void;
}

interface CareAppointmentItem {
  readonly key: string;
  readonly title: string;
  readonly timeRange: string;
  readonly tone: CareEventTone;
}

interface CareNoteItem {
  readonly key: string;
  readonly label: string;
  readonly value: string;
}

function invalidatePickupCaches() {
  try {
    mutate(
      (key) =>
        typeof key === "string" &&
        (key.includes("ogs-students-") || key.includes("dashboard")),
    ).catch(() => undefined);
  } catch {
    return;
  }
}

function getStatusLabel(status: StudentStatusKind): string {
  if (status === "sick") return "Krank";
  if (status === "class_trip") return "Klassenfahrt";
  return "Entschuldigt";
}

function formatStatusDayDate(date: string): string {
  return statusDayDateFormatter.format(new Date(`${date}T00:00:00`));
}

function formatWeekMonth(days: CareDayData[]): string {
  const firstDay = days[0]?.date;
  const lastDay = days[days.length - 1]?.date;
  if (!firstDay) return "";
  if (!lastDay || firstDay.getMonth() === lastDay.getMonth()) {
    return weekMonthFormatter.format(firstDay);
  }
  const firstMonth = firstDay.toLocaleDateString("de-DE", { month: "long" });
  const lastMonth = lastDay.toLocaleDateString("de-DE", {
    month: "long",
    year: "numeric",
  });
  return `${firstMonth} / ${lastMonth}`;
}

function getAbsenceStatus(
  arrival: ArrivalDayData,
  pickup: PickupDayData,
): StudentStatusKind | null {
  if (arrival.showSick || pickup.showSick) return "sick";
  if (arrival.showClassTrip || pickup.showClassTrip) return "class_trip";
  if (arrival.showExcused || pickup.showExcused) return "excused";
  return null;
}

export function CareScheduleManager({
  studentId,
  readOnly = false,
  onUpdate,
  isSick = false,
  isExcused = false,
  statusDays = EMPTY_STATUS_DAYS,
  onDeleteStatusDay,
  onVisibleDateRangeChange,
}: CareScheduleManagerProps) {
  const [arrivalData, setArrivalData] = useState<ArrivalData>({
    schedules: [],
    exceptions: [],
    notes: [],
  });
  const [pickupData, setPickupData] = useState<PickupData>({
    schedules: [],
    exceptions: [],
    notes: [],
  });
  const [isLoading, setIsLoading] = useState(true);
  const [careDaysSource, setCareDaysSource] = useState<CareDaysSource | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);
  const [weekOffset, setWeekOffset] = useState(0);
  const [selectedDateKey, setSelectedDateKey] = useState<string | null>(null);
  // The single care-plan editor. null = closed; { date: Date } = opened from a
  // day card (all three scopes offered); { date: null } = opened from the week
  // header, where there is no day context and only the weekly plan applies.
  const [editorTarget, setEditorTarget] = useState<{
    date: Date | null;
  } | null>(null);
  const [statusDayToDelete, setStatusDayToDelete] =
    useState<StudentStatusDay | null>(null);
  const [deletingStatusDayId, setDeletingStatusDayId] = useState<string | null>(
    null,
  );

  const weekDays = useMemo(() => getWeekDays(weekOffset), [weekOffset]);
  const weekRange = useMemo(
    () => formatWeekRange(weekDays[0] ?? new Date(), weekDays[4] ?? new Date()),
    [weekDays],
  );

  function showWeek(offset: number): void {
    setWeekOffset(offset);
    const visibleDays = getWeekDays(offset);
    const firstDay = visibleDays[0];
    const lastDay = visibleDays[visibleDays.length - 1];
    if (!firstDay || !lastDay) return;
    onVisibleDateRangeChange?.(formatDateISO(firstDay), formatDateISO(lastDay));
  }

  const statusDayByDate = useMemo(() => {
    const entries = new Map<string, StudentStatusDay>();
    for (const day of statusDays) {
      if (!day.cleared_at) {
        entries.set(day.date, day);
      }
    }
    return entries;
  }, [statusDays]);

  const statusByDate = useMemo(() => {
    const entries = new Map<string, StudentStatusKind>();
    for (const [date, day] of statusDayByDate) {
      entries.set(date, day.status);
    }
    return entries;
  }, [statusDayByDate]);

  const days = useMemo<CareDayData[]>(
    () =>
      weekDays.map((date) => {
        const dateKey = formatDateISO(date);
        const statusForDate = statusByDate.get(dateKey) ?? null;
        const arrival = getArrivalDayData(
          date,
          arrivalData.schedules,
          arrivalData.exceptions,
          arrivalData.notes,
          isSick,
          isExcused,
          statusForDate,
        );
        const pickup = getPickupDayData(
          date,
          pickupData.schedules,
          pickupData.exceptions,
          isSick,
          pickupData.notes,
          isExcused,
          statusForDate,
          pickupData.effectiveSchedules,
        );
        return {
          date,
          weekday: arrival.weekday,
          isToday: arrival.isToday || pickup.isToday,
          status: getAbsenceStatus(arrival, pickup),
          statusDay: statusDayByDate.get(dateKey) ?? null,
          arrival,
          pickup,
        };
      }),
    [
      weekDays,
      statusByDate,
      statusDayByDate,
      arrivalData.schedules,
      arrivalData.exceptions,
      arrivalData.notes,
      pickupData.schedules,
      pickupData.exceptions,
      pickupData.notes,
      pickupData.effectiveSchedules,
      isSick,
      isExcused,
    ],
  );
  const selectedMobileDay = useMemo(() => {
    const today = days.find((day) => day.isToday);
    if (!selectedDateKey) return today ?? days[0] ?? null;
    return (
      days.find((day) => formatDateISO(day.date) === selectedDateKey) ??
      today ??
      days[0] ??
      null
    );
  }, [days, selectedDateKey]);
  const weekMonth = useMemo(() => formatWeekMonth(days), [days]);

  useEffect(() => {
    if (days.length === 0) return;
    const selectedStillVisible = days.some(
      (day) => formatDateISO(day.date) === selectedDateKey,
    );
    if (selectedStillVisible) return;
    const today = days.find((day) => day.isToday);
    setSelectedDateKey(formatDateISO((today ?? days[0])!.date));
  }, [days, selectedDateKey]);

  // Monotonic id claimed by every care-data fetch, in start order: arrival and
  // pickup are two independent requests and several fetches can be in flight at
  // once (two SSE announcements, or one racing the refetch after a local save).
  // A later-started fetch reads the server later, so its data is at least as
  // fresh — which makes start order the right ordering key.
  const careDataRequestId = useRef(0);
  // The newest id whose result actually reached the screen. Ordering is decided
  // against THIS, not against the newest claim: a claim only means a fetch
  // started, and it may still fail. Comparing against the newest claim discarded
  // a successful result the moment any later fetch existed, so an initial load
  // that succeeded while a doomed SSE refresh was in flight was thrown away and
  // the editor rendered blank — no data, no error, nothing to retry.
  const renderedCareDataRequestId = useRef(0);

  const claimCareDataRequest = useCallback(
    () => ++careDataRequestId.current,
    [],
  );
  /** Would this result be newer than what the user is already looking at? */
  const isFresherThanRendered = useCallback(
    (requestId: number) => requestId > renderedCareDataRequestId.current,
    [],
  );

  const fetchCareDataInto = useCallback(
    async (requestId: number) => {
      const firstDay = weekDays[0];
      const lastDay = weekDays[weekDays.length - 1];
      if (!firstDay || !lastDay) throw new Error("Ungültige Wochenansicht");
      const [arrival, pickup, settings] = await Promise.all([
        fetchArrivalData(
          studentId,
          formatDateISO(firstDay),
          formatDateISO(lastDay),
        ),
        fetchStudentPickupData(studentId, {
          from: formatDateISO(firstDay),
          to: formatDateISO(lastDay),
        }),
        fetchArrivalSettings(),
      ]);
      // Older than what is on screen — drop it rather than clobber newer state.
      // A result is never dropped merely because a newer fetch is still running:
      // if that one succeeds it simply overwrites this a moment later, and if it
      // fails the user keeps real data instead of an empty editor.
      if (!isFresherThanRendered(requestId)) return;
      renderedCareDataRequestId.current = requestId;
      setArrivalData(arrival);
      setPickupData(pickup);
      setCareDaysSource(settings.care_days_source);
      // A newer read succeeded, so any banner or spinner an older attempt is
      // still going to leave behind is already obsolete. Clearing here (rather
      // than only in loadCareData) is what lets the failure path below bail out
      // without stranding the UI.
      setError(null);
      setIsLoading(false);
    },
    [studentId, isFresherThanRendered, weekDays],
  );

  const loadCareData = useCallback(async () => {
    const requestId = claimCareDataRequest();
    try {
      setIsLoading(true);
      setError(null);
      await fetchCareDataInto(requestId);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Betreuungsplan konnte nicht geladen werden";
      logger.error("care_schedule_load_failed", {
        error: message,
        student_id: studentId,
      });
      // Same ordering rule as the success path, against the same marker: only
      // stay silent when newer data is ALREADY rendered. If nothing has been
      // rendered yet the failure is what the user needs to see, even when a
      // later fetch happens to be in flight.
      if (!isFresherThanRendered(requestId)) return;
      setError(message);
    } finally {
      // Deliberately NOT gated on the request id: this is the only path that
      // sets isLoading, so skipping it for a superseded attempt could strand the
      // spinner forever when the superseding fetch also fails (the remote
      // refresh path reports failures to the log only). Clearing it early at
      // worst hides the spinner a moment before newer data lands.
      setIsLoading(false);
    }
  }, [
    claimCareDataRequest,
    fetchCareDataInto,
    isFresherThanRendered,
    studentId,
  ]);

  const refreshCareData = useCallback(async () => {
    await fetchCareDataInto(claimCareDataRequest());
    onUpdate?.();
  }, [claimCareDataRequest, fetchCareDataInto, onUpdate]);

  useEffect(() => {
    loadCareData().catch(() => undefined);
  }, [loadCareData]);

  // An open modal holds an unsaved draft that is seeded from arrivalData /
  // pickupData: CareWeeklyPlanModal re-runs its row-building effect whenever
  // those props change identity, so writing them mid-edit silently discards
  // whatever the user has typed.
  const isEditorOpen = editorTarget !== null;
  // Read through a ref inside the listener so opening/closing a modal does not
  // resubscribe it, and so the check uses the state at event time.
  const isEditorOpenRef = useRef(isEditorOpen);
  useEffect(() => {
    isEditorOpenRef.current = isEditorOpen;
  }, [isEditorOpen]);
  const hasDeferredRemoteRefresh = useRef(false);

  const refreshFromRemote = useCallback(() => {
    // Background refresh: a failure is logged, never surfaced — the user did not
    // ask for it, and the data already on screen stays valid.
    fetchCareDataInto(claimCareDataRequest()).catch((err) => {
      logger.debug("care_schedule_remote_refresh_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
    });
  }, [claimCareDataRequest, fetchCareDataInto, studentId]);

  // React to REMOTE arrival/pickup changes. This editor keeps its arrival/pickup
  // in local state (not SWR) and stays force-mounted across tabs, so the global
  // SSE hook's SWR invalidation never reaches it. It announces staleness on this
  // window event instead; re-fetch quietly (no spinner, no onUpdate — that would
  // ripple a parent refresh for a change the parent already heard about).
  //
  // While a modal is open the refresh is DEFERRED, not dropped: someone else's
  // edit must never cost this user their in-progress work, and the update is
  // still applied the moment the editor closes. Local saves keep refreshing
  // immediately — that path is the user's own action, and the day-override modal
  // needs to see its own write.
  useEffect(() => {
    const onStale = () => {
      if (isEditorOpenRef.current) {
        hasDeferredRemoteRefresh.current = true;
        return;
      }
      refreshFromRemote();
    };
    window.addEventListener("phoenix:care-schedule-stale", onStale);
    return () =>
      window.removeEventListener("phoenix:care-schedule-stale", onStale);
  }, [refreshFromRemote]);

  // Drain a refresh that arrived while the user was editing.
  useEffect(() => {
    if (isEditorOpen || !hasDeferredRemoteRefresh.current) return;
    hasDeferredRemoteRefresh.current = false;
    refreshFromRemote();
  }, [isEditorOpen, refreshFromRemote]);

  const handleUpdateWeeklyPlan = useCallback(
    async (data: CarePlanWeeklySubmit) => {
      // Arrival and pickup are separate endpoints, so one leg can persist while
      // the other fails. Wait for both, refresh whenever anything landed, and
      // only then report the failure. Retrying on stale rows would otherwise
      // resubmit values the server already replaced.
      const results = await Promise.allSettled([
        updateArrivalSchedules(
          studentId,
          data.arrivalSchedules.map((schedule) => ({
            weekday: schedule.weekday,
            expected_arrival: schedule.expected_arrival,
            notes: schedule.notes ?? null,
          })),
        ),
        updateStudentPickupSchedules(studentId, {
          schedules: data.pickupSchedules,
          effectiveDate: formatDateISO(weekDays[0]!),
        }),
      ]);

      const failure = results.find(
        (result): result is PromiseRejectedResult =>
          result.status === "rejected",
      );

      if (results.some((result) => result.status === "fulfilled")) {
        try {
          await refreshCareData();
          invalidatePickupCaches();
        } catch (refreshErr) {
          if (!failure) throw refreshErr;
          logger.error("care_schedule_partial_weekly_refresh_failed", {
            error:
              refreshErr instanceof Error
                ? refreshErr.message
                : String(refreshErr),
            student_id: studentId,
          });
        }
      }

      if (failure) throw failure.reason;
    },
    [studentId, refreshCareData, weekDays],
  );

  const editingDayDate = editorTarget?.date ?? null;

  const currentEditingArrivalDay = useMemo(() => {
    if (!editingDayDate) return null;
    const dateKey = formatDateISO(editingDayDate);
    return getArrivalDayData(
      editingDayDate,
      arrivalData.schedules,
      arrivalData.exceptions,
      arrivalData.notes,
      isSick,
      isExcused,
      statusByDate.get(dateKey) ?? null,
    );
  }, [editingDayDate, arrivalData, isSick, isExcused, statusByDate]);

  const currentEditingPickupDay = useMemo(() => {
    if (!editingDayDate) return null;
    const dateKey = formatDateISO(editingDayDate);
    return getPickupDayData(
      editingDayDate,
      pickupData.schedules,
      pickupData.exceptions,
      isSick,
      pickupData.notes,
      isExcused,
      statusByDate.get(dateKey) ?? null,
      pickupData.effectiveSchedules,
    );
  }, [editingDayDate, pickupData, isSick, isExcused, statusByDate]);

  /**
   * Write the exception for one day. "Regulär" means the day has no override,
   * so it maps to a delete; every other mode creates or updates the row. A leg
   * the editor reports as untouched (`null`) is skipped entirely — writing it
   * would reclaim a guardian-authored day from the parent for no reason.
   */
  const handleSubmitException = useCallback(
    async (payload: CareExceptionSubmit) => {
      const { date: dayISO, arrival, pickup } = payload;
      let persistedChange = false;

      try {
        if (arrival) {
          const existing = arrivalData.exceptions.find(
            (exception) => exception.exception_date.slice(0, 10) === dayISO,
          );
          if (arrival.kind === "regular") {
            if (existing) {
              await deleteArrivalException(studentId, existing.id);
              persistedChange = true;
            }
          } else {
            const input = {
              exception_date: dayISO,
              expected_arrival: arrival.kind === "time" ? arrival.time : null,
              ...(arrival.kind === "none"
                ? { clear_expected_arrival: true }
                : {}),
              // An empty string deliberately clears an existing reason. A null
              // JSON value means "leave unchanged" to the patch endpoint.
              reason: existing && arrival.reason === null ? "" : arrival.reason,
            };
            if (existing) {
              await updateArrivalException(studentId, existing.id, input);
            } else {
              await createArrivalException(studentId, input);
            }
            persistedChange = true;
          }
        }

        if (pickup) {
          const existing = pickupData.exceptions.find(
            (exception) => exception.exceptionDate.slice(0, 10) === dayISO,
          );
          if (pickup.kind === "regular") {
            if (existing) {
              await deleteStudentPickupException(studentId, existing.id);
              persistedChange = true;
            }
          } else {
            const input = {
              exceptionDate: dayISO,
              pickupTime: pickup.kind === "time" ? pickup.time : undefined,
              ...(pickup.kind === "none" ? { clearPickupTime: true } : {}),
              // See the arrival leg above: only an existing row needs the
              // distinguishable empty value to clear its persisted reason.
              reason:
                existing && pickup.reason === null
                  ? ""
                  : (pickup.reason ?? undefined),
            };
            if (existing) {
              await updateStudentPickupException(studentId, existing.id, input);
            } else {
              await createStudentPickupException(studentId, input);
            }
            persistedChange = true;
          }
        }
      } catch (err) {
        // Arrival and pickup are separate endpoints, so a failed second request
        // can leave the first change persisted. Refresh before reporting the
        // error so the open editor reflects that server state.
        if (persistedChange) {
          try {
            await refreshCareData();
            invalidatePickupCaches();
          } catch (refreshErr) {
            logger.error("care_schedule_partial_exception_refresh_failed", {
              error:
                refreshErr instanceof Error
                  ? refreshErr.message
                  : String(refreshErr),
              student_id: studentId,
            });
          }
        }
        throw err;
      }

      await refreshCareData();
      invalidatePickupCaches();
    },
    [arrivalData.exceptions, pickupData.exceptions, studentId, refreshCareData],
  );

  const handleCreateArrivalNote = useCallback(
    async (date: string, content: string) => {
      await createArrivalNote(studentId, { note_date: date, content });
      await refreshCareData();
    },
    [studentId, refreshCareData],
  );
  const handleUpdateArrivalNote = useCallback(
    async (date: string, noteId: number, content: string) => {
      await updateArrivalNote(studentId, noteId, { note_date: date, content });
      await refreshCareData();
    },
    [studentId, refreshCareData],
  );
  const handleDeleteArrivalNote = useCallback(
    async (noteId: number) => {
      await deleteArrivalNote(studentId, noteId);
      await refreshCareData();
    },
    [studentId, refreshCareData],
  );
  const handleCreatePickupNote = useCallback(
    async (date: string, content: string) => {
      await createStudentPickupNote(studentId, { noteDate: date, content });
      await refreshCareData();
      invalidatePickupCaches();
    },
    [studentId, refreshCareData],
  );
  const handleUpdatePickupNote = useCallback(
    async (date: string, noteId: string, content: string) => {
      await updateStudentPickupNote(studentId, noteId, {
        noteDate: date,
        content,
      });
      await refreshCareData();
      invalidatePickupCaches();
    },
    [studentId, refreshCareData],
  );
  const handleDeletePickupNote = useCallback(
    async (noteId: string) => {
      await deleteStudentPickupNote(studentId, noteId);
      await refreshCareData();
      invalidatePickupCaches();
    },
    [studentId, refreshCareData],
  );

  const handleResetPickupToOffering = useCallback(
    async (weekday: number, date: string) => {
      await resetStudentPickupToOffering(studentId, weekday, date);
      await refreshCareData();
      invalidatePickupCaches();
      onUpdate?.();
    },
    [studentId, refreshCareData, onUpdate],
  );

  const handleDeleteStatusDay = useCallback(
    async (statusDayId: string) => {
      if (!onDeleteStatusDay) return;
      setDeletingStatusDayId(statusDayId);
      try {
        await onDeleteStatusDay(statusDayId);
      } finally {
        setDeletingStatusDayId(null);
      }
    },
    [onDeleteStatusDay],
  );

  const handleConfirmDeleteStatusDay = useCallback(async () => {
    if (!statusDayToDelete) return;
    try {
      await handleDeleteStatusDay(statusDayToDelete.id);
      setStatusDayToDelete(null);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Geplanter Status konnte nicht entfernt werden";
      logger.error("care_schedule_status_delete_failed", {
        error: message,
        student_status_day_id: statusDayToDelete.id,
      });
    }
  }, [handleDeleteStatusDay, statusDayToDelete]);

  if (isLoading && arrivalData.schedules.length === 0) {
    return (
      <div className="moto-content-surface flex items-center justify-center rounded-2xl border border-gray-200 py-12 shadow-sm">
        <Loader2 className="h-7 w-7 animate-spin text-gray-500" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-2xl border p-4 text-sm">
        {error}
      </div>
    );
  }

  return (
    <section className="moto-content-surface overflow-hidden rounded-xl border border-gray-200 shadow-sm backdrop-blur-md sm:rounded-2xl">
      <div className="border-b border-gray-100 p-4 sm:p-5">
        <ConceptSectionHeader
          title="Betreuungszeiten"
          concept="careTimes"
          subtitle={weekRange}
          actions={
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => showWeek(0)}
                disabled={weekOffset === 0}
                className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-100 px-3 text-sm font-semibold text-gray-600 transition-colors hover:bg-gray-200 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:hidden"
              >
                Heute
              </button>
              {/* Labelled on purpose: an unlabelled pencil here was
                indistinguishable from the per-day pencils below, which is the
                confusion issue #893 reports. */}
              {!readOnly ? (
                <button
                  type="button"
                  onClick={() => setEditorTarget({ date: null })}
                  className="inline-flex h-9 items-center justify-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  title="Wochenplan bearbeiten"
                >
                  <SquarePen className="h-4 w-4 shrink-0" aria-hidden="true" />
                  {/* Label kept on mobile too: an icon-only pencil here is the
                    exact ambiguity with the per-day pencils that #893 reports,
                    and the header has room for the word on both layouts. */}
                  <span>Wochenplan</span>
                </button>
              ) : null}
            </div>
          }
        />
        <div className="relative mt-4 hidden items-center justify-between gap-2 xl:flex">
          <div>
            <WeekNavButton
              ariaLabel="Vorherige Woche"
              onClick={() => showWeek(weekOffset - 1)}
            >
              <ChevronLeft className="h-4 w-4" aria-hidden="true" />
              <span className="hidden sm:inline">Vorherige Woche</span>
            </WeekNavButton>
          </div>
          {weekOffset === 0 ? (
            <span className="absolute left-1/2 hidden h-9 -translate-x-1/2 items-center justify-center rounded-full bg-gray-100 px-3 text-sm font-semibold text-gray-500 sm:inline-flex">
              Aktuelle Woche
            </span>
          ) : (
            <button
              type="button"
              onClick={() => showWeek(0)}
              className="absolute left-1/2 hidden h-9 -translate-x-1/2 items-center justify-center rounded-full bg-gray-100 px-3 text-sm font-semibold text-gray-600 transition-colors hover:bg-gray-200 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:inline-flex"
            >
              Zurück zur aktuellen Woche
            </button>
          )}
          <div>
            <WeekNavButton
              ariaLabel="Nächste Woche"
              onClick={() => showWeek(weekOffset + 1)}
            >
              <span className="hidden sm:inline">Nächste Woche</span>
              <ChevronRight className="h-4 w-4" aria-hidden="true" />
            </WeekNavButton>
          </div>
        </div>
      </div>

      <div className="p-3 sm:p-4">
        <div className="xl:hidden">
          <MobileCareWeek
            days={days}
            weekMonth={weekMonth}
            selectedDay={selectedMobileDay}
            readOnly={readOnly}
            deletingStatusDayId={deletingStatusDayId}
            onPreviousWeek={() => showWeek(weekOffset - 1)}
            onNextWeek={() => showWeek(weekOffset + 1)}
            onSelectDay={(day) => setSelectedDateKey(formatDateISO(day.date))}
            onEditDay={(day) => setEditorTarget({ date: day })}
            onRequestDeleteStatusDay={setStatusDayToDelete}
          />
        </div>
        <div className="hidden xl:block">
          <div className="grid gap-3 xl:grid-cols-5">
            {days.map((day) => (
              <CareDayCard
                key={formatDateISO(day.date)}
                day={day}
                readOnly={readOnly}
                onEditDay={(day) => setEditorTarget({ date: day })}
                deletingStatusDayId={deletingStatusDayId}
                onRequestDeleteStatusDay={setStatusDayToDelete}
              />
            ))}
          </div>
        </div>
      </div>

      {careDaysSource ? (
        <CarePlanEditorModal
          isOpen={editorTarget !== null}
          careDaysSource={careDaysSource}
          onClose={() => setEditorTarget(null)}
          date={editingDayDate}
          arrivalDay={currentEditingArrivalDay}
          pickupDay={currentEditingPickupDay}
          weeklyArrival={mergeArrivalSchedulesWithTemplate(
            arrivalData.schedules,
          )}
          weeklyPickup={mergePickupSchedulesWithTemplate(pickupData.schedules)}
          onSubmitException={handleSubmitException}
          onSubmitWeekly={handleUpdateWeeklyPlan}
          onCreateArrivalNote={handleCreateArrivalNote}
          onUpdateArrivalNote={handleUpdateArrivalNote}
          onDeleteArrivalNote={handleDeleteArrivalNote}
          onCreatePickupNote={handleCreatePickupNote}
          onUpdatePickupNote={handleUpdatePickupNote}
          onDeletePickupNote={handleDeletePickupNote}
          onResetPickupToOffering={
            readOnly ? undefined : handleResetPickupToOffering
          }
        />
      ) : null}
      <ConfirmationModal
        isOpen={statusDayToDelete !== null}
        onClose={() => setStatusDayToDelete(null)}
        onConfirm={handleConfirmDeleteStatusDay}
        title="Geplanten Status entfernen?"
        confirmText="Entfernen"
        cancelText="Abbrechen"
        isConfirmLoading={deletingStatusDayId === statusDayToDelete?.id}
        confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
      >
        <p className="text-sm leading-6 text-gray-600">
          {statusDayToDelete
            ? `Dieser Eintrag wird nur für ${formatStatusDayDate(
                statusDayToDelete.date,
              )} entfernt. Andere geplante Tage bleiben bestehen.`
            : "Dieser geplante Status wird entfernt."}
        </p>
      </ConfirmationModal>
    </section>
  );
}

function WeekNavButton({
  ariaLabel,
  children,
  onClick,
}: {
  readonly ariaLabel: string;
  readonly children: React.ReactNode;
  readonly onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={ariaLabel}
      onClick={onClick}
      className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      {children}
    </button>
  );
}

function MobileCareWeek({
  days,
  weekMonth,
  selectedDay,
  readOnly,
  deletingStatusDayId,
  onPreviousWeek,
  onNextWeek,
  onSelectDay,
  onEditDay,
  onRequestDeleteStatusDay,
}: {
  readonly days: CareDayData[];
  readonly weekMonth: string;
  readonly selectedDay: CareDayData | null;
  readonly readOnly: boolean;
  readonly deletingStatusDayId: string | null;
  readonly onPreviousWeek: () => void;
  readonly onNextWeek: () => void;
  readonly onSelectDay: (day: CareDayData) => void;
  readonly onEditDay: (date: Date) => void;
  readonly onRequestDeleteStatusDay: (statusDay: StudentStatusDay) => void;
}) {
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-[2.75rem_minmax(0,1fr)_2.75rem] items-center gap-2">
        <WeekIconButton ariaLabel="Vorherige Woche" onClick={onPreviousWeek}>
          <ChevronLeft className="h-4 w-4" aria-hidden="true" />
        </WeekIconButton>
        <div className="min-w-0 text-center text-lg font-semibold text-gray-800">
          {weekMonth}
        </div>
        <WeekIconButton ariaLabel="Nächste Woche" onClick={onNextWeek}>
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </WeekIconButton>
      </div>

      <div className="grid grid-cols-5 gap-1.5">
        {days.map((day) => (
          <MobileDayButton
            key={formatDateISO(day.date)}
            day={day}
            isSelected={
              selectedDay !== null &&
              formatDateISO(selectedDay.date) === formatDateISO(day.date)
            }
            onClick={() => onSelectDay(day)}
          />
        ))}
      </div>

      {selectedDay ? (
        <CareDayCard
          day={selectedDay}
          readOnly={readOnly}
          onEditDay={onEditDay}
          deletingStatusDayId={deletingStatusDayId}
          onRequestDeleteStatusDay={onRequestDeleteStatusDay}
          isMobileDetail
        />
      ) : null}
    </div>
  );
}

function WeekIconButton({
  ariaLabel,
  children,
  onClick,
}: {
  readonly ariaLabel: string;
  readonly children: React.ReactNode;
  readonly onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={ariaLabel}
      onClick={onClick}
      className="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      {children}
    </button>
  );
}

function MobileDayButton({
  day,
  isSelected,
  onClick,
}: {
  readonly day: CareDayData;
  readonly isSelected: boolean;
  readonly onClick: () => void;
}) {
  const weekdayInfo = WEEKDAYS[day.weekday - 1];
  const statusLabel = day.status
    ? day.status === "class_trip"
      ? "Klasse"
      : getStatusLabel(day.status)
    : null;
  return (
    <button
      type="button"
      onClick={onClick}
      className={`min-w-0 rounded-lg border px-1 py-2 text-center transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
        isSelected
          ? "border-gray-300 bg-gray-100 text-gray-900 shadow-sm"
          : "border-gray-200 bg-white text-gray-700 shadow-sm hover:bg-gray-50"
      }`}
    >
      <span className="block truncate text-xs font-semibold">
        {weekdayInfo?.shortLabel ?? "Tag"}
      </span>
      <span
        className={`mt-1 inline-flex h-7 min-w-7 items-center justify-center rounded-full px-1.5 text-sm font-semibold ${
          day.isToday
            ? "bg-moto-orange text-white"
            : isSelected
              ? "bg-white text-gray-900"
              : "bg-gray-100 text-gray-700"
        }`}
      >
        {day.date.getDate()}
      </span>
      <span
        className={`mt-1 block truncate text-[10px] font-semibold ${
          isSelected ? "text-gray-500" : "text-gray-400"
        }`}
      >
        {statusLabel ?? " "}
      </span>
    </button>
  );
}

function CareDayCard({
  day,
  readOnly,
  onEditDay,
  deletingStatusDayId,
  onRequestDeleteStatusDay,
  isMobileDetail = false,
}: {
  readonly day: CareDayData;
  readonly readOnly: boolean;
  readonly onEditDay: (date: Date) => void;
  readonly deletingStatusDayId: string | null;
  readonly onRequestDeleteStatusDay: (statusDay: StudentStatusDay) => void;
  readonly isMobileDetail?: boolean;
}) {
  const weekdayInfo = WEEKDAYS[day.weekday - 1];
  const hasStatus = day.status !== null;
  const hasException = day.arrival.isException || day.pickup.isException;
  // A guardian-sourced exception means a parent changed this day's pickup or
  // arrival time from the parents portal — flag it so staff don't mistake it
  // for their own edit at the door.
  const parentChanged =
    day.pickup.exception?.source === "guardian" ||
    day.arrival.exception?.source === "guardian";
  const boundaries = getCareBoundaries(day, {
    onEditArrival: () => onEditDay(day.date),
    onEditPickup: () => onEditDay(day.date),
  });
  const appointments = getCareAppointments();
  const notes = getCareNotes(day);

  return (
    <article
      className={`flex flex-col rounded-xl border border-gray-200 bg-white p-3 shadow-sm ${
        isMobileDetail ? "min-h-[236px]" : "min-h-[260px]"
      }`}
    >
      <div className="flex min-h-[52px] items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-semibold text-gray-900">
              {weekdayInfo?.shortLabel ?? "Tag"}
            </span>
            {day.isToday ? (
              <span className="text-xs font-semibold text-gray-500">Heute</span>
            ) : null}
            {parentChanged ? (
              <span className="bg-moto-blue/10 text-moto-blue-hover inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-semibold">
                <MotoConceptIcon concept="parents" size={14} />
                Von Eltern
              </span>
            ) : null}
          </div>
          <div className="mt-1 flex items-center gap-1 text-sm text-gray-500">
            {day.isToday ? (
              <>
                <span className="bg-moto-orange flex h-7 min-w-7 items-center justify-center rounded-full px-1.5 text-sm font-semibold text-white">
                  {day.date.getDate()}
                </span>
                <span>.</span>
                <span>
                  {(day.date.getMonth() + 1).toString().padStart(2, "0")}.
                </span>
              </>
            ) : (
              formatShortDate(day.date)
            )}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {hasStatus ? <StatusPill status={day.status} /> : null}
          {/* ONE labelled entry per day, top right where the eye lands. Two
              pencils per card that both opened the same dialog, plus a third
              nameless one for the week, is the confusion #893 reports. */}
          {!readOnly && !hasStatus ? (
            <button
              type="button"
              onClick={() => onEditDay(day.date)}
              className="inline-flex h-8 items-center gap-1.5 rounded-md border border-gray-200 bg-white px-2.5 text-xs font-semibold text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              title={hasException ? "Ausnahme ändern" : "Ausnahme eintragen"}
            >
              <SquarePen className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              Ausnahme
            </button>
          ) : null}
        </div>
      </div>

      <div className="mt-3 flex flex-1 flex-col space-y-3">
        {hasStatus ? (
          <AbsencePlaceholder
            status={day.status}
            statusDay={day.statusDay}
            readOnly={readOnly}
            isDeleting={deletingStatusDayId === day.statusDay?.id}
            onRequestDeleteStatusDay={onRequestDeleteStatusDay}
          />
        ) : (
          <CareBoundarySection
            boundaries={boundaries}
            onEdit={readOnly ? undefined : () => onEditDay(day.date)}
          />
        )}
        {appointments.length > 0 ? (
          <CareAppointmentSection appointments={appointments} />
        ) : null}
        {notes.length > 0 ? <CareNotesSection notes={notes} /> : null}
      </div>
    </article>
  );
}

function AbsencePlaceholder({
  status,
  statusDay,
  readOnly,
  isDeleting,
  onRequestDeleteStatusDay,
}: {
  readonly status: StudentStatusKind | null;
  readonly statusDay: StudentStatusDay | null;
  readonly readOnly: boolean;
  readonly isDeleting: boolean;
  readonly onRequestDeleteStatusDay: (statusDay: StudentStatusDay) => void;
}) {
  if (!status) return null;
  const concept: MotoConceptKey =
    status === "class_trip"
      ? "classTrip"
      : status === "sick"
        ? "sick"
        : "excused";
  const label =
    status === "class_trip"
      ? "Ganztägig Klassenfahrt"
      : status === "sick"
        ? "Ganztägig krank gemeldet"
        : "Ganztägig entschuldigt";
  const isInteractive = !readOnly && statusDay !== null;
  const content = (
    <div className="flex min-w-0 items-center gap-3 pr-8 text-left">
      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-100 shadow-sm">
        <MotoConceptIcon concept={concept} size={18} />
      </span>
      <div className="min-w-0">
        <div className="text-[11px] font-semibold tracking-wide text-gray-500 uppercase">
          Status
        </div>
        <div className="text-sm leading-5 font-semibold text-gray-900">
          {isDeleting ? "Wird entfernt..." : label}
        </div>
      </div>
    </div>
  );

  if (isInteractive) {
    return (
      <button
        type="button"
        onClick={() => onRequestDeleteStatusDay(statusDay)}
        disabled={isDeleting}
        className="moto-content-surface group relative flex min-h-[134px] w-full items-center rounded-lg border border-gray-200 p-3 text-left shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-70"
        aria-label={`${label} entfernen`}
      >
        <span className="absolute top-3 right-3 flex h-6 w-6 items-center justify-center rounded-md text-gray-400 transition-colors group-hover:bg-gray-100 group-hover:text-gray-700">
          <X className="h-3.5 w-3.5" aria-hidden="true" />
        </span>
        {content}
      </button>
    );
  }

  return (
    <div className="moto-content-surface flex min-h-[134px] items-center rounded-lg border border-gray-200 p-3 shadow-sm">
      {content}
    </div>
  );
}

function CareBoundarySection({
  boundaries,
  onEdit,
}: {
  readonly boundaries: CareBoundaryItem[];
  /** undefined = read-only; the whole block opens the day editor otherwise. */
  readonly onEdit?: () => void;
}) {
  const rows = (
    <div className="flex flex-1 flex-col divide-y divide-gray-100">
      {boundaries.map((boundary) => (
        <CareBoundaryRow key={boundary.key} boundary={boundary} />
      ))}
    </div>
  );

  // The block is ONE target rather than one per line: tapping a time still
  // opens the editor, but the card shows a single pencil (in its header)
  // instead of one per row.
  if (onEdit) {
    return (
      <button
        type="button"
        onClick={onEdit}
        className="flex min-h-[134px] w-full flex-col rounded-lg border border-gray-100 bg-gray-50 text-left transition-colors hover:bg-gray-100/70 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        {rows}
      </button>
    );
  }

  return (
    <div className="flex min-h-[134px] flex-col rounded-lg border border-gray-100 bg-gray-50">
      {rows}
    </div>
  );
}

function CareBoundaryRow({
  boundary,
}: {
  readonly boundary: CareBoundaryItem;
}) {
  return (
    <div className="flex flex-1 px-3 py-2.5">
      <div className="flex min-w-0 flex-1 items-center gap-2.5">
        {/* Neutrale Kachel, wie bei AbsencePlaceholder in derselben Datei:
            MotoConceptIcon setzt seine Farbe selbst aus MOTO_CONCEPTS und
            ueberschreibt ein geerbtes color. Eine getoente Kachel ergab
            deshalb eine indigofarbene Uhr auf Gruen und ein petrolfarbenes
            Auto auf Orange. */}
        <span className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-gray-100 shadow-sm">
          {boundary.icon}
        </span>
        <div className="min-w-0 flex-1">
          <div className="text-[11px] font-semibold tracking-wide text-gray-500 uppercase">
            {boundary.label}
          </div>
          {/* Flex-wrapped rather than inline: the badge is taller than the 20px
              line box, so as an inline element it painted over the next line
              whenever the value wrapped ("Keine Abholung" in a day column). */}
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm leading-5 font-semibold text-gray-900">
            <span className="min-w-0 break-words">{boundary.value}</span>
            {boundary.marker ? (
              <span className="shrink-0 rounded-full bg-white px-1.5 py-0.5 text-[11px] font-semibold text-gray-500 shadow-sm">
                {boundary.marker}
              </span>
            ) : null}
          </div>
          {boundary.description ? (
            <div className="mt-0.5 text-xs leading-5 text-gray-500">
              {boundary.description}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function CareAppointmentSection({
  appointments,
}: {
  readonly appointments: CareAppointmentItem[];
}) {
  return (
    <div className="space-y-1.5">
      {appointments.map((appointment) => (
        <div
          key={appointment.key}
          className="rounded-lg border border-gray-100 bg-white px-3 py-2.5 shadow-sm"
        >
          <div className="text-[11px] font-semibold tracking-wide text-gray-500 uppercase">
            {appointment.timeRange}
          </div>
          <div className="text-sm font-semibold text-gray-900">
            {appointment.title}
          </div>
        </div>
      ))}
    </div>
  );
}

function CareNotesSection({ notes }: { readonly notes: CareNoteItem[] }) {
  return (
    <div className="space-y-1.5 border-t border-gray-100 pt-3">
      {notes.map((note) => (
        <div
          key={note.key}
          className="flex min-w-0 items-start gap-2 text-xs leading-5 text-gray-500"
        >
          <StickyNote className="mt-0.5 h-3.5 w-3.5 shrink-0 text-gray-400" />
          <span className="font-semibold text-gray-600">{note.label}:</span>
          <span className="min-w-0 break-words">{note.value}</span>
        </div>
      ))}
    </div>
  );
}

// Muss dieselbe Dreiteilung tragen wie AbsencePlaceholder in derselben Karte,
// sonst zeigt ein Klassenfahrt-Tag eine lila Pille ueber einem cyanfarbenen
// Icon und ist ausserdem nicht von "Entschuldigt" zu unterscheiden.
const STATUS_PILL_CLASSES: Record<StudentStatusKind, string> = {
  sick: "border-moto-red/30 bg-moto-red/10 text-moto-red-strong",
  class_trip: "border-moto-cyan/30 bg-moto-cyan/10 text-moto-cyan-strong",
  excused: "border-moto-purple/25 bg-moto-purple/10 text-moto-purple-strong",
};

function StatusPill({ status }: { readonly status: StudentStatusKind }) {
  return (
    <span
      className={`rounded-full border px-2 py-0.5 text-xs font-semibold ${
        STATUS_PILL_CLASSES[status]
      }`}
    >
      {getStatusLabel(status)}
    </span>
  );
}

function getArrivalValue(day: ArrivalDayData): string {
  if (day.isAbsent) return "Kommt nicht";
  return day.effectiveTime ?? "Nicht geplant";
}

function getPickupValue(day: PickupDayData): string {
  if (day.isException && !day.effectiveTime) return "Keine Abholung";
  return day.effectiveTime
    ? formatPickupTime(day.effectiveTime)
    : "Nicht geplant";
}

function getArrivalMarker(day: ArrivalDayData): string | null {
  if (day.isException) return "Ausnahme";
  return arrivalScheduleSourceLabel(day.baseSchedule);
}

function getPickupMarker(day: PickupDayData): string | null {
  if (day.isException) return "Ausnahme";
  return pickupScheduleSourceLabel(day.baseSchedule);
}

function getArrivalDescription(day: ArrivalDayData): string | undefined {
  if (!day.isException) return undefined;
  return day.effectiveReason
    ? `Grund: ${day.effectiveReason}`
    : "Tagesänderung";
}

function getPickupDescription(day: PickupDayData): string | undefined {
  if (!day.isException) return undefined;
  return day.effectiveNotes ? `Grund: ${day.effectiveNotes}` : "Tagesänderung";
}

function getCareBoundaries(
  day: CareDayData,
  actions: {
    readonly onEditArrival: () => void;
    readonly onEditPickup: () => void;
  },
): CareBoundaryItem[] {
  return [
    {
      key: "arrival",
      label: "Ankunft",
      value: getArrivalValue(day.arrival),
      description: getArrivalDescription(day.arrival),
      marker: getArrivalMarker(day.arrival),
      icon: <MotoConceptIcon concept="careTimes" size={18} />,
      onEdit: actions.onEditArrival,
    },
    {
      key: "pickup",
      label: "Abholung",
      value: getPickupValue(day.pickup),
      description: getPickupDescription(day.pickup),
      marker: getPickupMarker(day.pickup),
      icon: <MotoConceptIcon concept="pickup" size={18} />,
      onEdit: actions.onEditPickup,
    },
  ];
}

function getCareAppointments(): CareAppointmentItem[] {
  return [];
}

function getCareNotes(day: CareDayData): CareNoteItem[] {
  const notes: CareNoteItem[] = [];
  const arrivalNotes = getArrivalDisplayNotes(day.arrival);
  const pickupNotes = getPickupDisplayNotes(day.pickup);
  if (arrivalNotes.length > 0) {
    notes.push({
      key: "arrival-notes",
      label: "Ankunft",
      value: arrivalNotes.join(", "),
    });
  }
  if (pickupNotes.length > 0) {
    notes.push({
      key: "pickup-notes",
      label: "Abholung",
      value: pickupNotes.join(", "),
    });
  }
  return notes;
}

function getArrivalDisplayNotes(day: ArrivalDayData): string[] {
  return [
    day.baseSchedule?.notes ?? null,
    ...day.notes.map((note) => note.content),
  ].filter((note): note is string => !!note);
}

function getPickupDisplayNotes(day: PickupDayData): string[] {
  return [
    day.baseSchedule?.notes ?? null,
    ...day.notes.map((note) => note.content),
  ].filter((note): note is string => !!note);
}
