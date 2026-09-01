"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { ChevronLeft, ChevronRight, Utensils } from "lucide-react";

import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState } from "~/components/ui/empty-state";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert } from "~/components/ui/alert";
import { Checkbox } from "~/components/ui/checkbox";
import { StatusBadge } from "~/components/ui/status-badge";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import {
  getChildFeatures,
  getChildMealPlan,
  getMealParticipation,
  listMyChildren,
  replaceMealParticipationSchedule,
  setMealParticipationDay,
  clearMealParticipationDay,
  type MealParticipation,
  type MealPlanEntry,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentMealPlanPage" });

// One school the parent has a child at, with a representative child used for the
// per-child (school-scoped) backend calls.
interface ChildOption {
  tenantId: string;
  schoolName: string;
  studentId: string;
  childName: string;
  registrationEnabled: boolean;
}

function mondayISOFromOffset(todayISO: string, weekOffset: number): string {
  const base = parseISODate(todayISO);
  base.setDate(base.getDate() + weekOffset * 7);
  const offset = (base.getDay() + 6) % 7; // Mon=0 .. Sun=6
  base.setDate(base.getDate() - offset);
  return toISODate(base);
}

function workWeekDates(mondayISO: string): string[] {
  const monday = parseISODate(mondayISO);
  return Array.from({ length: 5 }, (_, i) => {
    const d = new Date(monday);
    d.setDate(monday.getDate() + i);
    return toISODate(d);
  });
}

function isoWeek(iso: string): number {
  const date = parseISODate(iso);
  date.setHours(0, 0, 0, 0);
  date.setDate(date.getDate() + 3 - ((date.getDay() + 6) % 7));
  const weekOne = new Date(date.getFullYear(), 0, 4);
  return (
    1 +
    Math.round(
      ((date.getTime() - weekOne.getTime()) / 86_400_000 -
        3 +
        ((weekOne.getDay() + 6) % 7)) /
        7,
    )
  );
}

// One dish tile as shown in the today card and the desktop grid. On the
// highlighted "today" column the tile is solid white (neutral ring) so it stays
// opaque against the faint tint instead of looking washed out.
function DishCard({
  entry,
  highlight = false,
}: {
  entry: MealPlanEntry;
  highlight?: boolean;
}) {
  return (
    <div
      className={`rounded-lg px-3 py-2.5 ring-1 ${
        highlight
          ? "bg-white shadow-sm ring-gray-200"
          : "bg-gray-50 ring-gray-100"
      }`}
    >
      <p className="text-base leading-snug font-medium text-gray-900">
        {entry.dish}
      </p>
      {entry.note ? (
        <p className="mt-1 text-sm leading-snug text-gray-500">{entry.note}</p>
      ) : null}
    </div>
  );
}

function MealPlanWeekSkeleton({ loadingLabel }: { loadingLabel: string }) {
  const dishWidths = ["w-4/5", "w-3/5", "w-full", "w-4/5", "w-3/4"];

  return (
    <div aria-busy="true" data-testid="meal-plan-week-skeleton">
      <div role="status" aria-label={loadingLabel} className="sr-only">
        {loadingLabel}
      </div>

      <div className="md:hidden" aria-hidden="true">
        <div className="border-b border-gray-200">
          <div className="flex items-center justify-between border-b border-gray-200 px-4 py-3">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-5 w-12 rounded-full" />
          </div>
          <div className="p-3">
            <div className="rounded-lg bg-gray-50 p-3 ring-1 ring-gray-100">
              <Skeleton className="h-5 w-3/4" />
              <Skeleton className="mt-2 h-4 w-1/2" />
            </div>
          </div>
        </div>
        <div className="divide-y divide-gray-100">
          {dishWidths.slice(1).map((width, index) => (
            <div key={index} className="flex gap-3 px-4 py-3">
              <div className="w-20 shrink-0 space-y-2">
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-3 w-10" />
              </div>
              <div className="min-w-0 flex-1 space-y-2">
                <Skeleton className={`h-4 ${width}`} />
                <Skeleton className="h-3 w-1/2" />
              </div>
            </div>
          ))}
        </div>
      </div>

      <div
        className="hidden grid-cols-5 divide-x divide-gray-200 md:grid"
        aria-hidden="true"
      >
        {dishWidths.map((width, index) => (
          <div key={index}>
            <div className="space-y-2 border-b border-gray-200 px-4 py-3">
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-3 w-10" />
            </div>
            <div className="min-h-48 p-3">
              <div className="rounded-lg bg-gray-50 p-3 ring-1 ring-gray-100">
                <Skeleton className={`h-5 ${width}`} />
                <Skeleton className="mt-2 h-4 w-2/3" />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function MealPlanLoadingSection({ loadingLabel }: { loadingLabel: string }) {
  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      <div
        className="flex items-center justify-center gap-2 border-b border-gray-200 px-3 py-2.5"
        aria-hidden="true"
      >
        <Skeleton className="size-8 rounded-md" />
        <div className="min-w-40 space-y-2 py-0.5">
          <Skeleton className="mx-auto h-4 w-28" />
          <Skeleton className="mx-auto h-3 w-36" />
        </div>
        <Skeleton className="size-8 rounded-md" />
      </div>
      <MealPlanWeekSkeleton loadingLabel={loadingLabel} />
    </section>
  );
}

const participationWeekdays = [1, 2, 3, 4, 5] as const;

function ParticipationCard({
  studentId,
  childName,
  today,
}: Readonly<{ studentId: string; childName: string; today: string }>) {
  const t = useTranslations("parentMealPlan");
  const locale = useLocale();
  const [participation, setParticipation] = useState<MealParticipation | null>(
    null,
  );
  const [weekdays, setWeekdays] = useState<number[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(false);

  const range = useMemo(() => {
    const monday = mondayISOFromOffset(today, 0);
    const end = parseISODate(monday);
    end.setDate(end.getDate() + 11);
    return { from: monday, to: toISODate(end) };
  }, [today]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const value = await getMealParticipation(studentId, range.from, range.to);
      setParticipation(value);
      setWeekdays(value.weekdays);
    } catch (loadError) {
      logger.error("parent_meal_participation_load_failed", {
        error:
          loadError instanceof Error ? loadError.message : String(loadError),
      });
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [range.from, range.to, studentId]);

  useEffect(() => {
    void load();
  }, [load]);

  function toggleWeekday(weekday: number) {
    setWeekdays((current) =>
      current.includes(weekday)
        ? current.filter((value) => value !== weekday)
        : [...current, weekday].sort((a, b) => a - b),
    );
  }

  async function saveRegularDays() {
    setSaving(true);
    try {
      await replaceMealParticipationSchedule(studentId, weekdays);
      await load();
    } catch (saveError) {
      logger.error("parent_meal_participation_schedule_failed", {
        error:
          saveError instanceof Error ? saveError.message : String(saveError),
      });
      setError(true);
    } finally {
      setSaving(false);
    }
  }

  async function changeDay(
    date: string,
    participating: boolean,
    reset = false,
  ) {
    setSaving(true);
    try {
      if (reset) await clearMealParticipationDay(studentId, date);
      else await setMealParticipationDay(studentId, date, participating);
      await load();
    } catch (saveError) {
      logger.error("parent_meal_participation_day_failed", {
        error:
          saveError instanceof Error ? saveError.message : String(saveError),
      });
      setError(true);
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <h2 className="text-lg font-semibold text-gray-900">
        {t("participationTitle", { name: childName })}
      </h2>
      <p className="mt-1 text-sm text-gray-600">
        {participation
          ? t("participationCutoff", { time: participation.cutoff_time })
          : t("participationDescription")}
      </p>
      {participation ? (
        <p className="mt-1 text-sm text-gray-600">
          {t("participationSickness", { time: participation.cutoff_time })}
        </p>
      ) : null}

      {error ? (
        <div className="mt-4 space-y-3">
          <Alert type="error" message={t("participationError")} />
          <Button
            type="button"
            variant="outline"
            onClick={() => void load()}
            disabled={loading}
          >
            {t("retry")}
          </Button>
        </div>
      ) : loading || !participation ? (
        <div className="mt-4 space-y-3" aria-label={t("participationLoading")}>
          <Skeleton className="h-12 w-full rounded-xl" />
          <Skeleton className="h-28 w-full rounded-xl" />
        </div>
      ) : (
        <div className="mt-5 space-y-6">
          <div>
            <h3 className="text-sm font-semibold text-gray-900">
              {t("regularDays")}
            </h3>
            <p className="mt-1 text-sm text-gray-500">{t("regularDaysHint")}</p>
            <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-5">
              {participationWeekdays.map((weekday) => (
                <label
                  key={weekday}
                  className="moto-content-surface flex min-h-12 cursor-pointer items-center gap-2 rounded-xl border border-gray-200 px-3 py-2 text-sm font-medium text-gray-800 shadow-sm"
                >
                  <Checkbox
                    checked={weekdays.includes(weekday)}
                    onChange={() => toggleWeekday(weekday)}
                    disabled={saving}
                  />
                  {t(`weekday${weekday}`)}
                </label>
              ))}
            </div>
            <Button
              type="button"
              variant="primary"
              size="touch"
              className="mt-3 w-full sm:w-auto"
              onClick={() => void saveRegularDays()}
              disabled={saving}
              isLoading={saving}
              loadingText={t("saving")}
            >
              {t("saveRegularDays")}
            </Button>
            {participation.effective_from ? (
              <p className="mt-2 text-xs text-gray-500">
                {t("regularEffective", {
                  date: parseISODate(
                    participation.effective_from,
                  ).toLocaleDateString(locale),
                })}
              </p>
            ) : null}
          </div>

          <div>
            <h3 className="text-sm font-semibold text-gray-900">
              {t("individualDays")}
            </h3>
            <p className="mt-1 text-sm text-gray-500">
              {t("individualDaysHint")}
            </p>
            <div className="mt-3 grid gap-2 sm:grid-cols-2">
              {participation.days.map((day) => {
                const date = parseISODate(day.date);
                return (
                  <div
                    key={day.date}
                    className="moto-content-surface rounded-xl border border-gray-200 p-3 shadow-sm"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-sm font-semibold text-gray-900">
                          {date.toLocaleDateString(locale, {
                            weekday: "long",
                            day: "2-digit",
                            month: "2-digit",
                          })}
                        </p>
                        <p className="mt-0.5 text-sm text-gray-600">
                          {day.source === "sick"
                            ? t("cancelledBySickNote")
                            : day.participating
                              ? t("registered")
                              : t("notRegistered")}
                        </p>
                      </div>
                      {!day.changeable ? (
                        <StatusBadge
                          label={t("closed")}
                          tone="gray"
                          showDot={false}
                        />
                      ) : null}
                    </div>
                    {day.changeable && day.source !== "sick" ? (
                      <div className="mt-3 flex flex-wrap gap-2">
                        <Button
                          type="button"
                          variant={
                            day.participating ? "outline_danger" : "success"
                          }
                          size="compact"
                          disabled={saving}
                          onClick={() =>
                            void changeDay(day.date, !day.participating)
                          }
                        >
                          {day.participating ? t("unregister") : t("register")}
                        </Button>
                        {day.source === "override" ? (
                          <Button
                            type="button"
                            variant="ghost"
                            size="compact"
                            disabled={saving}
                            onClick={() =>
                              void changeDay(day.date, false, true)
                            }
                          >
                            {t("useRegular")}
                          </Button>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      )}
    </section>
  );
}

export function ParentMealPlanPage() {
  const t = useTranslations("parentMealPlan");
  const locale = useLocale();

  const [schools, setSchools] = useState<ChildOption[]>([]);
  const [hasLinkedChildren, setHasLinkedChildren] = useState(false);
  const [selectedStudent, setSelectedStudent] = useState<string | null>(null);
  const [weekOffset, setWeekOffset] = useState<0 | 1>(0);
  const [entries, setEntries] = useState<MealPlanEntry[]>([]);
  // The Monday the current `entries` were loaded for. Used to detect when
  // `entries` belongs to a previous week (a slower request still in flight, or
  // a week switch not yet resolved) so we never map stale dishes onto the newly
  // selected week's dates.
  const [loadedMonday, setLoadedMonday] = useState<string | null>(null);
  const [loadingSchools, setLoadingSchools] = useState(true);
  // Distinguish operational failures (backend/network/session errors) from a
  // genuinely empty result. Without these flags a failed request would render
  // as "no meal plan available" / "no plan entered" — false information.
  const [schoolsError, setSchoolsError] = useState(false);
  const [weekError, setWeekError] = useState(false);

  // useBerlinToday re-renders on the Berlin midnight rollover, so a page left
  // mounted overnight follows the date instead of freezing on yesterday.
  const today = useBerlinToday();
  // Anchor the week on the Berlin `today`, not just weekOffset: if the page
  // stays mounted across Berlin midnight, the selected week must roll forward
  // with the date. Keying the memo on `today` also keeps it aligned with the
  // backend's current/next-week guard, which is computed from today's Monday.
  const mondayISO = useMemo(
    () => mondayISOFromOffset(today, weekOffset),
    [today, weekOffset],
  );
  const weekDates = useMemo(() => workWeekDates(mondayISO), [mondayISO]);

  // Resolve which of the parent's schools actually run a meal plan.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setLoadingSchools(true);
      setSchoolsError(false);
      try {
        const children = await listMyChildren();
        // Let a failed feature lookup reject the whole resolution. A transient
        // 500/session/proxy error must surface as an error, never be treated
        // as meal_plan_enabled: false (which silently removes the school).
        const features = await Promise.all(
          children.map((child) => getChildFeatures(child.student_id)),
        );
        const enabled: ChildOption[] = [];
        children.forEach((child, index) => {
          if (features[index]?.meal_plan_enabled) {
            enabled.push({
              tenantId: child.tenant_id,
              schoolName: child.school_name,
              studentId: child.student_id,
              childName: `${child.first_name} ${child.last_name}`,
              registrationEnabled:
                features[index]?.meal_registration_enabled === true,
            });
          }
        });
        if (cancelled) return;
        setHasLinkedChildren(children.length > 0);
        setSchools(enabled);
        setSelectedStudent(enabled[0]?.studentId ?? null);
      } catch (err) {
        if (cancelled) return;
        logger.error("parent_meal_plan_schools_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setSchoolsError(true);
      } finally {
        if (!cancelled) setLoadingSchools(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const selectedSchool = useMemo(
    () =>
      schools.find((school) => school.studentId === selectedStudent) ?? null,
    [schools, selectedStudent],
  );

  // Switching schools must drop the previous school's menu immediately, or it
  // would show under the new school's label until the new request returns —
  // wrong information. Clearing loadedMonday makes the week "not ready" so the
  // loading skeleton shows until the new school's week resolves.
  useEffect(() => {
    setEntries([]);
    setLoadedMonday(null);
  }, [selectedStudent]);

  // Load the selected school's week. A slower response from a previous
  // school/week selection must never overwrite the current one, so each run
  // captures a `cancelled` flag the cleanup flips when the inputs change.
  useEffect(() => {
    if (!selectedSchool) return;
    let cancelled = false;
    const { studentId } = selectedSchool;
    setWeekError(false);
    void (async () => {
      try {
        const rows = await getChildMealPlan(studentId, mondayISO);
        if (cancelled) return;
        setEntries(rows);
      } catch (err) {
        if (cancelled) return;
        logger.error("parent_meal_plan_week_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        // Operational failure — show the load-error copy, not an empty week
        // (which would falsely tell parents no plan was entered).
        setEntries([]);
        setWeekError(true);
      } finally {
        if (!cancelled) {
          // Mark which week `entries` now reflects (success or error) so the
          // render can tell a resolved week from stale/in-flight data.
          setLoadedMonday(mondayISO);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [selectedSchool, mondayISO]);

  const dishesByDate = useMemo(() => {
    const map = new Map<string, MealPlanEntry[]>();
    for (const e of entries) {
      const list = map.get(e.date) ?? [];
      list.push(e);
      map.set(e.date, list);
    }
    for (const list of map.values())
      list.sort((a, b) => a.position - b.position);
    return map;
  }, [entries]);

  // `entries` only describes the selected week once the load for that exact
  // Monday has resolved. Until then (first load, a week switch, a slow request
  // still returning the previous week) the data is stale: rendering it would
  // map old dishes onto the new dates — a false-empty or wrong week.
  const weekReady = loadedMonday === mondayISO;
  const weekIsEmpty = entries.length === 0;

  const weekdayLabel = (iso: string) =>
    parseISODate(iso).toLocaleDateString(locale, { weekday: "long" });
  const shortDate = (iso: string) =>
    parseISODate(iso).toLocaleDateString(locale, {
      day: "2-digit",
      month: "2-digit",
    });
  const weekRange = t("dateRange", {
    start: parseISODate(weekDates[0]!).toLocaleDateString(locale, {
      day: "2-digit",
      month: "2-digit",
    }),
    end: parseISODate(weekDates[4]!).toLocaleDateString(locale, {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    }),
  });

  // Today only counts when it falls inside the displayed (work) week.
  const todayInWeek = weekDates.includes(today);

  return (
    <ParentPage>
      <ParentPageHeader
        kicker={t("kicker")}
        title={t("title")}
        description={t("subtitle")}
        actions={
          schools.length > 1 ? (
            <CustomSelect
              value={selectedStudent ?? ""}
              options={schools.map((school) => ({
                value: school.studentId,
                label: `${school.childName} · ${school.schoolName}`,
              }))}
              onChange={setSelectedStudent}
              ariaLabel={t("child")}
              className="w-full sm:w-60"
            />
          ) : undefined
        }
      />

      {loadingSchools ? (
        <MealPlanLoadingSection loadingLabel={t("loading")} />
      ) : schoolsError ? (
        <Alert type="error" message={t("loadError")} />
      ) : !hasLinkedChildren ? (
        <EmptyState
          icon={<Utensils className="h-10 w-10" />}
          title={t("noLinkedChildrenTitle")}
          description={t("noLinkedChildren")}
          className="moto-content-surface rounded-2xl border px-5 shadow-sm backdrop-blur-md"
        />
      ) : schools.length === 0 ? (
        <EmptyState
          icon={<Utensils className="h-10 w-10" />}
          title={t("emptyTitle")}
          description={t("empty")}
          className="moto-content-surface rounded-2xl border px-5 shadow-sm backdrop-blur-md"
        />
      ) : (
        <>
          {selectedSchool?.registrationEnabled ? (
            <ParticipationCard
              studentId={selectedSchool.studentId}
              childName={selectedSchool.childName}
              today={today}
            />
          ) : null}
          <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
            <nav
              aria-label={t("weekNavigation")}
              className="flex items-center justify-center gap-2 border-b border-gray-200 px-3 py-2.5"
            >
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={t("previousWeek")}
                disabled={!weekReady || weekOffset === 0}
                onClick={() => setWeekOffset(0)}
              >
                <ChevronLeft className="h-4 w-4" aria-hidden="true" />
              </Button>
              <div
                role="status"
                aria-live="polite"
                aria-atomic="true"
                className="min-w-40 text-center"
              >
                <p className="text-sm font-semibold text-gray-900">
                  {t("calendarWeek", { week: isoWeek(mondayISO) })}
                  <span className="ml-1.5 text-gray-500">
                    · {weekOffset === 0 ? t("thisWeek") : t("nextWeek")}
                  </span>
                </p>
                <p className="text-xs text-gray-500">{weekRange}</p>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={t("nextWeek")}
                disabled={!weekReady || weekOffset === 1}
                onClick={() => setWeekOffset(1)}
              >
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              </Button>
            </nav>

            {!weekReady ? (
              <MealPlanWeekSkeleton loadingLabel={t("loading")} />
            ) : weekError ? (
              <div className="p-4">
                <Alert type="error" message={t("loadError")} />
              </div>
            ) : weekIsEmpty ? (
              <EmptyState
                icon={<Utensils className="h-10 w-10" />}
                title={
                  weekOffset === 0
                    ? t("emptyThisWeekTitle")
                    : t("emptyNextWeekTitle")
                }
                description={t("emptyWeek")}
                className="px-5"
              />
            ) : (
              <>
                {/* Mobile: today first, then the rest of the week as a list. */}
                <div className="md:hidden">
                  {todayInWeek &&
                    (() => {
                      const dishes = dishesByDate.get(today) ?? [];
                      return (
                        <div className="overflow-hidden border-b border-gray-200">
                          <div className="flex items-center justify-between gap-2 border-b border-gray-200 px-4 py-3">
                            <div className="text-sm font-semibold text-gray-900">
                              {weekdayLabel(today)}
                            </div>
                            <span className="bg-moto-green/15 text-moto-green-strong rounded-full px-2 py-0.5 text-[11px] font-semibold">
                              {t("today")}
                            </span>
                          </div>
                          <div className="flex flex-col gap-2.5 p-3">
                            {dishes.length > 0 ? (
                              dishes.map((entry) => (
                                <DishCard key={entry.position} entry={entry} />
                              ))
                            ) : (
                              <p className="py-3 text-center text-sm text-gray-300">
                                {t("noMeal")}
                              </p>
                            )}
                          </div>
                        </div>
                      );
                    })()}

                  <div className="overflow-hidden">
                    <div className="divide-y divide-gray-100">
                      {weekDates
                        .filter((date) => !(todayInWeek && date === today))
                        .map((date) => {
                          const dishes = dishesByDate.get(date) ?? [];
                          return (
                            <div key={date} className="flex gap-3 px-4 py-3">
                              <div className="w-20 shrink-0">
                                <div className="text-sm font-semibold text-gray-900">
                                  {weekdayLabel(date)}
                                </div>
                                <div className="text-xs text-gray-500">
                                  {shortDate(date)}
                                </div>
                              </div>
                              <div className="min-w-0 flex-1 space-y-1.5">
                                {dishes.length > 0 ? (
                                  dishes.map((entry) => (
                                    <div key={entry.position}>
                                      <p className="text-sm leading-snug font-medium text-gray-900">
                                        {entry.dish}
                                      </p>
                                      {entry.note ? (
                                        <p className="text-xs leading-snug text-gray-500">
                                          {entry.note}
                                        </p>
                                      ) : null}
                                    </div>
                                  ))
                                ) : (
                                  <p className="text-sm text-gray-300">
                                    {t("noMeal")}
                                  </p>
                                )}
                              </div>
                            </div>
                          );
                        })}
                    </div>
                  </div>
                </div>

                {/* Desktop: full five-column week grid. */}
                <div className="hidden md:block">
                  <div className="grid grid-cols-5 divide-x divide-gray-200">
                    {weekDates.map((date) => {
                      const dishes = dishesByDate.get(date) ?? [];
                      const isToday = date === today;
                      return (
                        <div
                          key={date}
                          className={`flex flex-col ${
                            isToday ? "bg-moto-green/[0.04]" : ""
                          }`}
                        >
                          <div className="flex items-center justify-between gap-2 border-b border-gray-200 px-4 py-3">
                            <div>
                              <div className="text-sm font-semibold text-gray-900">
                                {weekdayLabel(date)}
                              </div>
                              <div className="text-xs text-gray-500">
                                {shortDate(date)}
                              </div>
                            </div>
                            {isToday && (
                              <span className="bg-moto-green/15 text-moto-green-strong rounded-full px-2 py-0.5 text-[11px] font-semibold">
                                {t("today")}
                              </span>
                            )}
                          </div>

                          <div className="flex flex-1 flex-col gap-2.5 p-3">
                            {dishes.length > 0 ? (
                              dishes.map((entry) => (
                                <DishCard
                                  key={entry.position}
                                  entry={entry}
                                  highlight={isToday}
                                />
                              ))
                            ) : (
                              <p className="py-6 text-center text-sm text-gray-300">
                                {t("noMeal")}
                              </p>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </>
            )}
          </section>
        </>
      )}
    </ParentPage>
  );
}
