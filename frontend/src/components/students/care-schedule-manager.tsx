"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { mutate } from "swr";
import {
  ChevronLeft,
  ChevronRight,
  Loader2,
  SquarePen,
  Users,
  X,
} from "lucide-react";
import {
  type CareExceptionSubmit,
  type CarePlanWeeklySubmit,
  CarePlanEditorModal,
} from "./care-plan-editor-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import {
  DataTable,
  type DataTableColumn,
} from "~/components/ui/data-table";
import { StatusBadge } from "~/components/ui/status-badge";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  type ArrivalData,
  createArrivalException,
  deleteArrivalException,
  fetchArrivalData,
  updateArrivalException,
  updateArrivalSchedules,
} from "~/lib/student-arrival-api";
import {
  type ArrivalDayData,
  WEEKDAYS,
  formatDateISO,
  formatShortDate,
  getDayData as getArrivalDayData,
  getWeekDays,
  mergeSchedulesWithTemplate as mergeArrivalSchedulesWithTemplate,
} from "~/lib/arrival-schedule-helpers";
import {
  createStudentPickupException,
  deleteStudentPickupException,
  fetchStudentPickupData,
  updateStudentPickupException,
  updateStudentPickupSchedules,
} from "~/lib/pickup-schedule-api";
import {
  type DayData as PickupDayData,
  type PickupData,
  formatPickupTime,
  formatWeekRange,
  getDayData as getPickupDayData,
  mergeSchedulesWithTemplate as mergePickupSchedulesWithTemplate,
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

/** Wording of a whole-day status, unchanged from the card layout it replaces. */
function getAbsenceLabel(status: StudentStatusKind): string {
  if (status === "class_trip") return "Ganztägig Klassenfahrt";
  if (status === "sick") return "Ganztägig krank gemeldet";
  return "Ganztägig entschuldigt";
}

function getAbsenceColor(status: StudentStatusKind): string {
  if (status === "class_trip") return LOCATION_COLORS.CLASS_TRIP;
  if (status === "sick") return LOCATION_COLORS.SICK;
  return LOCATION_COLORS.EXCUSED;
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
      const [arrival, pickup] = await Promise.all([
        fetchArrivalData(studentId),
        fetchStudentPickupData(studentId),
      ]);
      // Older than what is on screen — drop it rather than clobber newer state.
      // A result is never dropped merely because a newer fetch is still running:
      // if that one succeeds it simply overwrites this a moment later, and if it
      // fails the user keeps real data instead of an empty editor.
      if (!isFresherThanRendered(requestId)) return;
      renderedCareDataRequestId.current = requestId;
      setArrivalData(arrival);
      setPickupData(pickup);
      // A newer read succeeded, so any banner or spinner an older attempt is
      // still going to leave behind is already obsolete. Clearing here (rather
      // than only in loadCareData) is what lets the failure path below bail out
      // without stranding the UI.
      setError(null);
      setIsLoading(false);
    },
    [studentId, isFresherThanRendered],
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
      await Promise.all([
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
        }),
      ]);
      await refreshCareData();
      invalidatePickupCaches();
    },
    [studentId, refreshCareData],
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

      if (arrival) {
        const existing = arrivalData.exceptions.find(
          (exception) => exception.exception_date.slice(0, 10) === dayISO,
        );
        if (arrival.kind === "regular") {
          if (existing) await deleteArrivalException(studentId, existing.id);
        } else {
          const input = {
            exception_date: dayISO,
            expected_arrival: arrival.kind === "time" ? arrival.time : null,
            reason: arrival.reason,
          };
          if (existing) {
            await updateArrivalException(studentId, existing.id, input);
          } else {
            await createArrivalException(studentId, input);
          }
        }
      }

      if (pickup) {
        const existing = pickupData.exceptions.find(
          (exception) => exception.exceptionDate.slice(0, 10) === dayISO,
        );
        if (pickup.kind === "regular") {
          if (existing) {
            await deleteStudentPickupException(studentId, existing.id);
          }
        } else {
          const input = {
            exceptionDate: dayISO,
            pickupTime: pickup.kind === "time" ? pickup.time : undefined,
            reason: pickup.reason ?? undefined,
          };
          if (existing) {
            await updateStudentPickupException(studentId, existing.id, input);
          } else {
            await createStudentPickupException(studentId, input);
          }
        }
      }

      await refreshCareData();
      invalidatePickupCaches();
    },
    [arrivalData.exceptions, pickupData.exceptions, studentId, refreshCareData],
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

  const onRequestDeleteStatusDayRef = useCallback(
    (statusDay: StudentStatusDay) => setStatusDayToDelete(statusDay),
    [],
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

  // The week is a table, not five stacked cards: every day carries the same
  // slots (two times plus an open number of hints), and a column layout keeps
  // them aligned no matter how much text a single day has. Cards drifted apart
  // as soon as one day had a reason or a note (#893 review).
  const weekColumns = useMemo<DataTableColumn<CareDayData>[]>(
    () => [
      {
        key: "day",
        header: "Tag",
        render: (day) => <DayCell day={day} />,
        className: "whitespace-nowrap",
      },
      {
        key: "arrival",
        header: "Ankunft",
        render: (day) => (
          <TimeCell
            value={getArrivalValue(day.arrival)}
            planned={formatPlannedArrival(day.arrival)}
            isException={day.arrival.isException}
            fromParent={day.arrival.exception?.source === "guardian"}
            muted={day.status !== null}
          />
        ),
      },
      {
        key: "pickup",
        header: "Abholung",
        render: (day) => (
          <TimeCell
            value={getPickupValue(day.pickup)}
            planned={formatPlannedPickup(day.pickup)}
            isException={day.pickup.isException}
            fromParent={day.pickup.exception?.source === "guardian"}
            muted={day.status !== null}
          />
        ),
      },
      {
        key: "hint",
        header: "Hinweis",
        render: (day) => (
          <HintCell
            day={day}
            readOnly={readOnly}
            isDeleting={deletingStatusDayId === day.statusDay?.id}
            onRequestDeleteStatusDay={onRequestDeleteStatusDayRef}
          />
        ),
        className: "w-full",
      },
      {
        key: "action",
        header: "",
        align: "right",
        render: (day) =>
          readOnly || day.status !== null ? null : (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              className="gap-1.5 whitespace-nowrap"
              onClick={() => setEditorTarget({ date: day.date })}
            >
              {/* Same word on every row: the row already shows whether a day
                  deviates, and an alternating label made the column width
                  jump from row to row. */}
              <SquarePen className="h-3.5 w-3.5" aria-hidden="true" />
              Ausnahme
            </Button>
          ),
        className: "whitespace-nowrap",
      },
    ],
    [readOnly, deletingStatusDayId, onRequestDeleteStatusDayRef],
  );

  if (isLoading && arrivalData.schedules.length === 0) {
    return (
      <div className="moto-content-surface flex items-center justify-center rounded-2xl border border-gray-200 py-12 shadow-sm">
        <Loader2 className="h-7 w-7 animate-spin text-gray-500" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
        {error}
      </div>
    );
  }

  return (
    <section className="space-y-4">
      <div>
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Wochenübersicht
            </p>
            <div className="mt-1 flex flex-col gap-2 lg:flex-row lg:flex-wrap lg:items-center lg:gap-3">
              <h2 className="text-lg font-semibold text-gray-900 sm:text-xl">
                Betreuungsplan
              </h2>
              <span className="rounded-full border border-gray-200 bg-gray-50 px-2.5 py-1 text-xs font-semibold text-gray-600">
                {weekRange}
              </span>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
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
        </div>
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
        <DataTable
          columns={weekColumns}
          rows={days}
          getRowKey={(day) => formatDateISO(day.date)}
          rowClassName={(day) => (day.isToday ? "bg-gray-50/60" : "")}
        />
      </div>

      <CarePlanEditorModal
        isOpen={editorTarget !== null}
        onClose={() => setEditorTarget(null)}
        date={editingDayDate}
        arrivalDay={currentEditingArrivalDay}
        pickupDay={currentEditingPickupDay}
        weeklyArrival={mergeArrivalSchedulesWithTemplate(arrivalData.schedules)}
        weeklyPickup={mergePickupSchedulesWithTemplate(pickupData.schedules)}
        onSubmitException={handleSubmitException}
        onSubmitWeekly={handleUpdateWeeklyPlan}
      />
      <ConfirmationModal
        isOpen={statusDayToDelete !== null}
        onClose={() => setStatusDayToDelete(null)}
        onConfirm={handleConfirmDeleteStatusDay}
        title="Geplanten Status entfernen?"
        confirmText="Entfernen"
        cancelText="Abbrechen"
        isConfirmLoading={deletingStatusDayId === statusDayToDelete?.id}
        confirmButtonClass="bg-[#CC2626] hover:bg-[#B91C1C]"
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
        <MobileDayDetail
          day={selectedDay}
          readOnly={readOnly}
          isDeleting={deletingStatusDayId === selectedDay.statusDay?.id}
          onEditDay={onEditDay}
          onRequestDeleteStatusDay={onRequestDeleteStatusDay}
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
            ? "bg-[#FF3130] text-white"
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

/**
 * The selected day on a phone. Same three slots as a table row, stacked, so
 * both layouts state a day the same way.
 */
function MobileDayDetail({
  day,
  readOnly,
  isDeleting,
  onEditDay,
  onRequestDeleteStatusDay,
}: {
  readonly day: CareDayData;
  readonly readOnly: boolean;
  readonly isDeleting: boolean;
  readonly onEditDay: (date: Date) => void;
  readonly onRequestDeleteStatusDay: (statusDay: StudentStatusDay) => void;
}) {
  return (
    <div className="moto-content-surface space-y-4 rounded-2xl border border-gray-200 p-4 shadow-sm">
      <DayCell day={day} />

      <div className="grid grid-cols-2 gap-3">
        <div>
          <div className="mb-1 text-xs font-medium text-gray-500">Ankunft</div>
          <TimeCell
            value={getArrivalValue(day.arrival)}
            planned={formatPlannedArrival(day.arrival)}
            isException={day.arrival.isException}
            fromParent={day.arrival.exception?.source === "guardian"}
            muted={day.status !== null}
          />
        </div>
        <div>
          <div className="mb-1 text-xs font-medium text-gray-500">Abholung</div>
          <TimeCell
            value={getPickupValue(day.pickup)}
            planned={formatPlannedPickup(day.pickup)}
            isException={day.pickup.isException}
            fromParent={day.pickup.exception?.source === "guardian"}
            muted={day.status !== null}
          />
        </div>
      </div>

      <div>
        <div className="mb-1 text-xs font-medium text-gray-500">Hinweis</div>
        <HintCell
          day={day}
          readOnly={readOnly}
          isDeleting={isDeleting}
          onRequestDeleteStatusDay={onRequestDeleteStatusDay}
        />
      </div>

      {!readOnly && day.status === null ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          className="w-full gap-1.5"
          onClick={() => onEditDay(day.date)}
        >
          <SquarePen className="h-4 w-4" aria-hidden="true" />
          {day.arrival.isException || day.pickup.isException
            ? "Ausnahme ändern"
            : "Ausnahme eintragen"}
        </Button>
      ) : null}
    </div>
  );
}

function DayCell({ day }: { readonly day: CareDayData }) {
  const weekdayInfo = WEEKDAYS[day.weekday - 1];
  return (
    <div className="flex items-center gap-2">
      <span className="font-semibold text-gray-900">
        {weekdayInfo?.shortLabel ?? "Tag"}
      </span>
      <span className="text-gray-500">{formatShortDate(day.date)}</span>
      {day.isToday ? <StatusBadge label="Heute" tone="gray" /> : null}
    </div>
  );
}

/**
 * One time of a day. A deviation is not marked with a badge but by naming the
 * value that was planned instead — the badge duplicated what the "statt" line
 * and the hint column already say, and it was the widest thing in the cell.
 */
function TimeCell({
  value,
  planned,
  isException,
  fromParent,
  muted,
}: {
  readonly value: string;
  readonly planned: string | null;
  readonly isException: boolean;
  readonly fromParent: boolean;
  readonly muted: boolean;
}) {
  if (muted) {
    return <span className="text-gray-300">—</span>;
  }
  return (
    <div className="min-w-0">
      <div className="font-semibold whitespace-nowrap text-gray-900">
        {value}
      </div>
      {isException && planned ? (
        <div className="text-xs whitespace-nowrap text-gray-500">
          statt {planned}
        </div>
      ) : null}
      {fromParent ? (
        <div className="mt-0.5 inline-flex items-center gap-1 text-xs text-[#3a63b0]">
          <Users className="h-3 w-3" aria-hidden="true" />
          Von Eltern
        </div>
      ) : null}
    </div>
  );
}

/**
 * Everything a day says in words. Three sources, and each line names which one
 * it is, because "gilt nur heute" and "gilt jede Woche" used to look identical
 * (#893): the reason of an exception, a note for this date, and a note that is
 * part of the weekly plan.
 */
function HintCell({
  day,
  readOnly,
  isDeleting,
  onRequestDeleteStatusDay,
}: {
  readonly day: CareDayData;
  readonly readOnly: boolean;
  readonly isDeleting: boolean;
  readonly onRequestDeleteStatusDay: (statusDay: StudentStatusDay) => void;
}) {
  if (day.status) {
    return (
      <div className="flex items-center gap-2">
        <StatusDotBadge
          label={isDeleting ? "Wird entfernt…" : getAbsenceLabel(day.status)}
          color={getAbsenceColor(day.status)}
        />
        {!readOnly && day.statusDay ? (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={`${getAbsenceLabel(day.status)} entfernen`}
            disabled={isDeleting}
            onClick={() => onRequestDeleteStatusDay(day.statusDay!)}
          >
            <X className="h-3.5 w-3.5" aria-hidden="true" />
          </Button>
        ) : null}
      </div>
    );
  }

  const hints = getCareHints(day);
  if (hints.length === 0) {
    return <span className="text-gray-300">—</span>;
  }
  return (
    <div className="space-y-1">
      {hints.map((hint) => (
        <div
          key={hint.key}
          className="flex min-w-0 items-baseline gap-1.5 text-sm text-gray-700"
        >
          <span className="shrink-0 text-xs whitespace-nowrap text-gray-400">
            {hint.scope === "week" ? "jede Woche" : "nur heute"}
          </span>
          <span className="shrink-0 font-medium text-gray-500">
            {hint.leg}:
          </span>
          <span className="min-w-0 break-words">{hint.text}</span>
        </div>
      ))}
    </div>
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

/** The weekly time a day deviates from, or null when nothing is planned. */
function formatPlannedArrival(day: ArrivalDayData): string | null {
  return day.baseSchedule?.expected_arrival
    ? day.baseSchedule.expected_arrival.slice(0, 5)
    : null;
}

function formatPlannedPickup(day: PickupDayData): string | null {
  return day.baseSchedule?.pickupTime
    ? formatPickupTime(day.baseSchedule.pickupTime)
    : null;
}

interface CareHint {
  readonly key: string;
  readonly scope: "day" | "week";
  readonly leg: string;
  readonly text: string;
}

/**
 * Every word a day carries, each tagged with where it comes from: the reason of
 * today's deviation, a note for this date, or a note that is part of the weekly
 * plan. Keeping the three apart in the UI is the point of #893.
 */
function getCareHints(day: CareDayData): CareHint[] {
  const hints: CareHint[] = [];

  if (day.arrival.isException && day.arrival.effectiveReason) {
    hints.push({
      key: "arrival-reason",
      scope: "day",
      leg: "Ankunft",
      text: day.arrival.effectiveReason,
    });
  }
  if (day.pickup.isException && day.pickup.effectiveNotes) {
    hints.push({
      key: "pickup-reason",
      scope: "day",
      leg: "Abholung",
      text: day.pickup.effectiveNotes,
    });
  }

  day.arrival.notes.forEach((note) => {
    hints.push({
      key: `arrival-note-${note.id}`,
      scope: "day",
      leg: "Ankunft",
      text: note.content,
    });
  });
  day.pickup.notes.forEach((note) => {
    hints.push({
      key: `pickup-note-${note.id}`,
      scope: "day",
      leg: "Abholung",
      text: note.content,
    });
  });

  if (day.arrival.baseSchedule?.notes) {
    hints.push({
      key: "arrival-weekly-note",
      scope: "week",
      leg: "Ankunft",
      text: day.arrival.baseSchedule.notes,
    });
  }
  if (day.pickup.baseSchedule?.notes) {
    hints.push({
      key: "pickup-weekly-note",
      scope: "week",
      leg: "Abholung",
      text: day.pickup.baseSchedule.notes,
    });
  }

  return hints;
}
