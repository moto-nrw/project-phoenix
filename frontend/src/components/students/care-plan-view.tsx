"use client";

/**
 * CarePlanView — the per-child Betreuungsplan tab on students/[id]. A read-only
 * day/week overview that merges arrival, planned activities/AGs/Mensa/Lernzeit,
 * free care (Freispiel), and pickup into one timeline, with deviations
 * (krank / entschuldigt / Klassenfahrt / Absage) surfaced per day.
 *
 * Data: GET /api/timetable/student/{id}/{day,week} (student-care-plan-api.ts).
 * Deviations reuse the statusDays + sick/excused flags the detail page already
 * fetched — no extra page-level request. The Betreuungszeiten tab stays the
 * editor; this view cross-links to it via onEditSchedule.
 */

import {
  type ReactNode,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import {
  DetailSectionSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { resolveDayDeviation } from "~/lib/care-plan-helpers";
import {
  careWeekDayCount,
  formatDate,
  isCareWeekendDay,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import {
  fetchStudentCarePlanDay,
  fetchStudentCarePlanWeek,
  type CarePlanDay,
} from "~/lib/student-care-plan-api";
import type { StudentStatusDay } from "~/lib/student-status-days-api";
import { useApiLoadError } from "~/contexts/ToastContext";
import { useSWRAuth } from "~/lib/swr/hooks";
import { useWeekendFollowsFriday } from "~/lib/tenant-context";

import { CarePlanDayTimeline } from "./care-plan-day";

type ViewMode = "day" | "week";

interface CarePlanViewProps {
  readonly studentId: string;
  readonly statusDays: readonly StudentStatusDay[];
  readonly isSick?: boolean;
  readonly isExcused?: boolean;
  /** Switch the student detail page to the Betreuungszeiten (editor) tab. */
  readonly onEditSchedule?: () => void;
  /**
   * Report the visible day/week range so the parent can extend its status-day
   * fetch — otherwise deviation banners vanish when navigating past the
   * page's initial window. Mirrors the Betreuungszeiten editor.
   */
  readonly onVisibleDateRangeChange?: (from: string, to: string) => void;
  /**
   * Whether the Betreuungsplan tab is the active one. The tab is forceMounted
   * for deep-linking, so gate the SWR key (and the range report) on this to
   * avoid firing a timetable read on every student-profile load. Defaults to
   * true so the component is usable standalone.
   */
  readonly active?: boolean;
}

const WEEKDAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"] as const;

/**
 * Step one care day forward/back: skips Sa/So unless the school runs the
 * weekend on Friday's plan (operations.weekend_follows_friday, #3921) and it
 * is today's weekend (#5028, `isCareWeekendDay`).
 */
function stepWeekday(iso: string, dir: 1 | -1, weekendOpen: boolean): string {
  const d = parseISODate(iso);
  d.setDate(d.getDate() + dir);
  while (
    (d.getDay() === 0 || d.getDay() === 6) &&
    !isCareWeekendDay(toISODate(d), weekendOpen)
  ) {
    d.setDate(d.getDate() + dir);
  }
  return toISODate(d);
}

function berlinWeekStartISO(todayISO: string): string {
  const monday = parseISODate(todayISO);
  monday.setDate(monday.getDate() - ((monday.getDay() + 6) % 7));
  return toISODate(monday);
}

/**
 * The care days of the week identified by its Monday. The displayed week is
 * kept separate from today's Berlin week, so navigation does not shift when
 * Berlin crosses Monday midnight. Sa/So follow `careWeekDayCount` (#5028).
 */
function berlinWeekDays(
  weekStartISO: string,
  weekendOpen: boolean,
  todayISO: string,
): Date[] {
  const monday = parseISODate(weekStartISO);
  const days: Date[] = [];
  const dayCount = careWeekDayCount(toISODate(monday), weekendOpen, todayISO);
  for (let i = 0; i < dayCount; i++) {
    const d = new Date(monday);
    d.setDate(monday.getDate() + i);
    days.push(d);
  }
  return days;
}

function mobileDayIndexForToday(
  weekendOpen: boolean,
  todayISO: string,
): number {
  const weekday = parseISODate(todayISO).getDay(); // 0=Sun … 6=Sat
  if (weekendOpen) return (weekday + 6) % 7;
  return weekday >= 1 && weekday <= 5 ? weekday - 1 : 0;
}

function shortDate(date: Date): string {
  const day = date.getDate().toString().padStart(2, "0");
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  return `${day}.${month}.`;
}

function NavButton({
  ariaLabel,
  onClick,
  children,
}: {
  readonly ariaLabel: string;
  readonly onClick: () => void;
  readonly children: ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={ariaLabel}
      onClick={onClick}
      className="inline-flex h-9 items-center justify-center gap-1 rounded-lg border border-gray-200 bg-white px-2.5 text-sm font-semibold text-gray-600 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      {children}
    </button>
  );
}

export function CarePlanView({
  studentId,
  statusDays,
  isSick,
  isExcused,
  onEditSchedule,
  onVisibleDateRangeChange,
  active = true,
}: CarePlanViewProps) {
  const today = useBerlinToday();
  const weekendOpen = useWeekendFollowsFriday();
  const [viewMode, setViewMode] = useState<ViewMode>("day");
  const [selectedDate, setSelectedDate] = useState<string>(today);
  const currentWeekStartISO = useMemo(() => berlinWeekStartISO(today), [today]);
  const [weekStartISO, setWeekStartISO] = useState(currentWeekStartISO);
  const [mobileDayIndex, setMobileDayIndex] = useState(() =>
    mobileDayIndexForToday(weekendOpen, today),
  );
  const previousTodayRef = useRef(today);
  const previousCurrentWeekStartRef = useRef(currentWeekStartISO);

  // Keep an open day view on today, but leave a deliberately selected past or
  // future day alone when Berlin crosses midnight.
  useEffect(() => {
    const previousToday = previousTodayRef.current;
    previousTodayRef.current = today;
    setSelectedDate((selected) =>
      selected === previousToday ? today : selected,
    );
  }, [today]);

  // A page that remains open over Monday follows the new current week only
  // when it was already showing the old current week. Any navigated week
  // retains its absolute Monday.
  useLayoutEffect(() => {
    const previous = previousCurrentWeekStartRef.current;
    previousCurrentWeekStartRef.current = currentWeekStartISO;
    if (previous === currentWeekStartISO) return;
    setWeekStartISO((selected) =>
      selected === previous ? currentWeekStartISO : selected,
    );
  }, [currentWeekStartISO]);

  // The tenant arrives after the first render. On a weekend, the initial
  // closed-weekend state points at Monday; select today's newly available day
  // once the school's Friday-plan setting is known or Berlin reaches Saturday.
  useEffect(() => {
    setMobileDayIndex(mobileDayIndexForToday(weekendOpen, today));
  }, [today, weekendOpen]);

  // --- Day mode fetch (only when the tab is active and in day mode) ---
  const dayKey =
    active && viewMode === "day"
      ? `care-plan-day-${studentId}-${selectedDate}`
      : null;
  const {
    data: dayData,
    error: dayError,
    isLoading: dayLoading,
    mutate: reloadDay,
  } = useSWRAuth<CarePlanDay>(dayKey, () =>
    fetchStudentCarePlanDay(studentId, selectedDate),
  );

  // --- Week mode fetch (only active in week mode) ---
  const weekDates = useMemo(
    () => berlinWeekDays(weekStartISO, weekendOpen, today),
    [today, weekStartISO, weekendOpen],
  );
  const weekFrom = toISODate(weekDates[0]!);
  const weekTo = toISODate(weekDates[weekDates.length - 1]!);
  const weekKey =
    active && viewMode === "week"
      ? `care-plan-week-${studentId}-${weekFrom}-${weekTo}`
      : null;
  const {
    data: weekData,
    error: weekError,
    isLoading: weekLoading,
    mutate: reloadWeek,
  } = useSWRAuth(weekKey, () =>
    fetchStudentCarePlanWeek(studentId, weekFrom, weekTo),
  );

  const daysByDate = useMemo(() => {
    const map = new Map<string, CarePlanDay>();
    for (const d of weekData?.days ?? []) map.set(d.date, d);
    return map;
  }, [weekData]);

  const deviationFor = (dateISO: string) =>
    resolveDayDeviation(dateISO, statusDays, {
      isSick,
      isExcused,
      isToday: dateISO === today,
    });

  const isCurrentWeek = weekStartISO === currentWeekStartISO;

  function moveWeek(direction: -1 | 1): void {
    setWeekStartISO((selected) => {
      const next = parseISODate(selected);
      next.setDate(next.getDate() + direction * 7);
      return toISODate(next);
    });
  }

  // Tell the parent which dates are visible so it can widen its status-day
  // fetch to cover them (navigating past the initial window would otherwise
  // drop sick/excused/class-trip banners for real backend rows).
  useEffect(() => {
    if (!active || !onVisibleDateRangeChange) return;
    if (viewMode === "day") {
      onVisibleDateRangeChange(selectedDate, selectedDate);
    } else {
      onVisibleDateRangeChange(weekFrom, weekTo);
    }
  }, [
    active,
    onVisibleDateRangeChange,
    viewMode,
    selectedDate,
    weekFrom,
    weekTo,
  ]);

  const error: unknown = viewMode === "day" ? dayError : weekError;
  const loading = viewMode === "day" ? dayLoading : weekLoading;
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;
  const reloadRef = useRef<() => Promise<unknown>>(reloadDay);
  useLayoutEffect(() => {
    reloadRef.current = viewMode === "day" ? reloadDay : reloadWeek;
  });
  useEffect(() => {
    if (!error) {
      clearLoadError();
      return;
    }
    void showLoadError(error, {
      object: "die Ansicht des Betreuungsplans",
      retry: () => void reloadRef.current(),
    });
  }, [error, showLoadError, clearLoadError]);

  return (
    <section className="moto-content-surface overflow-hidden rounded-xl border border-gray-200 shadow-sm backdrop-blur-md sm:rounded-2xl">
      <>
        <ConceptSectionHeader
          className="border-b border-gray-100 p-4 sm:p-5"
          title="Betreuungsplan"
          concept="carePlan"
          subtitle={
            viewMode === "day"
              ? formatDate(selectedDate, true)
              : `${shortDate(weekDates[0]!)} – ${shortDate(weekDates[weekDates.length - 1]!)}`
          }
          actions={
            <SegmentedControl
              ariaLabel="Ansicht"
              value={viewMode}
              onChange={(next) => setViewMode(next as ViewMode)}
              items={[
                { value: "day", label: "Tag" },
                { value: "week", label: "Woche" },
              ]}
            />
          }
        />

        {/* Navigation row */}
        <div className="flex items-center justify-between gap-2 px-4 py-3 sm:px-5">
          {viewMode === "day" ? (
            <>
              <NavButton
                ariaLabel="Vorheriger Tag"
                onClick={() =>
                  setSelectedDate((d) => stepWeekday(d, -1, weekendOpen))
                }
              >
                <ChevronLeft className="h-4 w-4" aria-hidden="true" />
                <span className="hidden sm:inline">Vorheriger Tag</span>
              </NavButton>
              {selectedDate !== today ? (
                <button
                  type="button"
                  onClick={() => setSelectedDate(today)}
                  className="inline-flex h-9 items-center justify-center rounded-full bg-gray-100 px-3 text-sm font-semibold text-gray-600 transition-colors hover:bg-gray-200 hover:text-gray-900"
                >
                  Heute
                </button>
              ) : (
                <span className="inline-flex h-9 items-center justify-center rounded-full bg-gray-100 px-3 text-sm font-semibold text-gray-500">
                  Heute
                </span>
              )}
              <NavButton
                ariaLabel="Nächster Tag"
                onClick={() =>
                  setSelectedDate((d) => stepWeekday(d, 1, weekendOpen))
                }
              >
                <span className="hidden sm:inline">Nächster Tag</span>
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              </NavButton>
            </>
          ) : (
            <>
              <NavButton
                ariaLabel="Vorherige Woche"
                onClick={() => moveWeek(-1)}
              >
                <ChevronLeft className="h-4 w-4" aria-hidden="true" />
                <span className="hidden sm:inline">Vorherige Woche</span>
              </NavButton>
              {!isCurrentWeek ? (
                <button
                  type="button"
                  onClick={() => setWeekStartISO(currentWeekStartISO)}
                  className="inline-flex h-9 items-center justify-center rounded-full bg-gray-100 px-3 text-sm font-semibold text-gray-600 transition-colors hover:bg-gray-200 hover:text-gray-900"
                >
                  Diese Woche
                </button>
              ) : (
                <span className="inline-flex h-9 items-center justify-center rounded-full bg-gray-100 px-3 text-sm font-semibold text-gray-500">
                  Diese Woche
                </span>
              )}
              <NavButton ariaLabel="Nächste Woche" onClick={() => moveWeek(1)}>
                <span className="hidden sm:inline">Nächste Woche</span>
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              </NavButton>
            </>
          )}
        </div>

        {/* Body */}
        <div className="p-3 sm:p-4">
          {error ? (
            load.error ? (
              <LoadErrorAlert error={load.error} />
            ) : (
              <SkeletonRegion label="Betreuungsplan wird geladen">
                <DetailSectionSkeleton fields={4} />
              </SkeletonRegion>
            )
          ) : loading ? (
            <SkeletonRegion label="Betreuungsplan wird geladen">
              <DetailSectionSkeleton fields={4} />
            </SkeletonRegion>
          ) : viewMode === "day" ? (
            <CarePlanDayTimeline
              day={dayData ?? null}
              deviation={deviationFor(selectedDate)}
              onEditSchedule={onEditSchedule}
            />
          ) : (
            <WeekBody
              weekDates={weekDates}
              daysByDate={daysByDate}
              mobileDayIndex={mobileDayIndex}
              onSelectMobileDay={setMobileDayIndex}
              deviationFor={deviationFor}
            />
          )}
        </div>
      </>
    </section>
  );
}

interface WeekBodyProps {
  readonly weekDates: Date[];
  readonly daysByDate: Map<string, CarePlanDay>;
  readonly mobileDayIndex: number;
  readonly onSelectMobileDay: (index: number) => void;
  readonly deviationFor: (
    dateISO: string,
  ) => ReturnType<typeof resolveDayDeviation>;
}

function WeekBody({
  weekDates,
  daysByDate,
  mobileDayIndex,
  onSelectMobileDay,
  deviationFor,
}: WeekBodyProps) {
  // Today's Saturday or Sunday is selected on a weekend; a Mo–Fr week
  // without it falls back to Monday (#5028).
  const selectedIndex = mobileDayIndex < weekDates.length ? mobileDayIndex : 0;
  const selectedDate = weekDates[selectedIndex]!;
  const selectedISO = toISODate(selectedDate);
  const selectedDay = daysByDate.get(selectedISO) ?? null;

  return (
    <>
      {/* Desktop: compact cards for every care day of the week */}
      <div className="hidden xl:block">
        <div
          className={`grid gap-3 ${weekDates.length === 7 ? "xl:grid-cols-7" : "xl:grid-cols-5"}`}
        >
          {weekDates.map((date, i) => {
            const iso = toISODate(date);
            const day = daysByDate.get(iso) ?? null;
            return (
              <div
                key={iso}
                className="rounded-xl border border-gray-200 bg-white/60 p-2.5"
              >
                <div className="mb-2 flex items-baseline justify-between">
                  <span className="text-sm font-semibold text-gray-900">
                    {WEEKDAY_LABELS[i]}
                  </span>
                  <span className="text-xs text-gray-500">
                    {shortDate(date)}
                  </span>
                </div>
                <CarePlanDayTimeline
                  day={day}
                  deviation={deviationFor(iso)}
                  compact
                />
              </div>
            );
          })}
        </div>
      </div>

      {/* Mobile: day-strip selector + selected day */}
      <div className="xl:hidden">
        <div className="mb-3 flex gap-2 overflow-x-auto">
          {weekDates.map((date, i) => {
            const isSelected = i === selectedIndex;
            return (
              <button
                key={toISODate(date)}
                type="button"
                onClick={() => onSelectMobileDay(i)}
                className={`flex shrink-0 flex-col items-center rounded-lg border px-3 py-1.5 text-sm transition-colors ${
                  isSelected
                    ? "border-transparent bg-gray-900 text-white"
                    : "border-gray-200 bg-white text-gray-600 hover:bg-gray-50"
                }`}
              >
                <span className="font-semibold">{WEEKDAY_LABELS[i]}</span>
                <span
                  className={isSelected ? "text-gray-200" : "text-gray-400"}
                >
                  {shortDate(date)}
                </span>
              </button>
            );
          })}
        </div>
        <CarePlanDayTimeline
          day={selectedDay}
          deviation={deviationFor(selectedISO)}
        />
      </div>
    </>
  );
}
