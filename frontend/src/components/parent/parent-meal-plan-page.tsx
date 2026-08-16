"use client";

import { useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";

import { CustomSelect } from "~/components/ui/custom-select";
import { Alert } from "~/components/ui/alert";
import {
  CardGridSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import {
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
} from "~/components/parent/parent-page";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import {
  getChildFeatures,
  getChildMealPlan,
  listMyChildren,
  type Child,
  type MealPlanEntry,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentMealPlanPage" });

// One school the parent has a child at, with a representative child used for the
// per-child (school-scoped) backend calls.
interface SchoolOption {
  tenantId: string;
  schoolName: string;
  studentId: string;
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

export function ParentMealPlanPage() {
  const t = useTranslations("parentMealPlan");
  const locale = useLocale();

  const [schools, setSchools] = useState<SchoolOption[]>([]);
  const [selectedTenant, setSelectedTenant] = useState<string | null>(null);
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
        // One representative child per school (tenant).
        const byTenant = new Map<string, Child>();
        for (const child of children) {
          if (!byTenant.has(child.tenant_id))
            byTenant.set(child.tenant_id, child);
        }
        const reps = [...byTenant.values()];
        // Let a failed feature lookup reject the whole resolution. A transient
        // 500/session/proxy error must surface as an error, never be treated
        // as meal_plan_enabled: false (which silently removes the school).
        const features = await Promise.all(
          reps.map((c) => getChildFeatures(c.student_id)),
        );
        const enabled: SchoolOption[] = [];
        reps.forEach((c, i) => {
          if (features[i]?.meal_plan_enabled) {
            enabled.push({
              tenantId: c.tenant_id,
              schoolName: c.school_name,
              studentId: c.student_id,
            });
          }
        });
        if (cancelled) return;
        setSchools(enabled);
        setSelectedTenant(enabled[0]?.tenantId ?? null);
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
    () => schools.find((s) => s.tenantId === selectedTenant) ?? null,
    [schools, selectedTenant],
  );

  // Switching schools must drop the previous school's menu immediately, or it
  // would show under the new school's label until the new request returns —
  // wrong information. Clearing loadedMonday makes the week "not ready" so the
  // loading skeleton shows until the new school's week resolves.
  useEffect(() => {
    setEntries([]);
    setLoadedMonday(null);
  }, [selectedTenant]);

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

  // Today only counts when it falls inside the displayed (work) week.
  const todayInWeek = weekDates.includes(today);

  if (loadingSchools) {
    return <ParentPageSkeleton rows={2} />;
  }

  return (
    <ParentPage>
      <ParentPageHeader title={t("title")} description={t("subtitle")} />

      {schoolsError ? (
        <Alert type="error" message={t("loadError")} />
      ) : schools.length === 0 ? (
        <div className="moto-content-surface rounded-2xl border p-5 text-sm text-gray-500 shadow-sm backdrop-blur-md">
          {t("empty")}
        </div>
      ) : (
        <>
          <section className="moto-content-surface flex flex-col gap-4 rounded-2xl border p-4 shadow-sm backdrop-blur-md sm:flex-row sm:flex-wrap sm:items-start">
            {schools.length > 1 && (
              <div className="flex w-full flex-col gap-1.5 sm:w-64">
                <span className="text-[11px] font-medium tracking-wide text-gray-500 uppercase">
                  {t("school")}
                </span>
                <CustomSelect
                  value={selectedTenant ?? ""}
                  options={schools.map((s) => ({
                    value: s.tenantId,
                    label: s.schoolName,
                  }))}
                  onChange={(v) => setSelectedTenant(v)}
                  ariaLabel={t("school")}
                />
              </div>
            )}

            <div className="flex w-full flex-col gap-1.5 sm:w-56">
              <span className="text-[11px] font-medium tracking-wide text-gray-500 uppercase">
                {t("week")}
              </span>
              <CustomSelect
                value={String(weekOffset)}
                options={[
                  { value: "0", label: t("thisWeek") },
                  { value: "1", label: t("nextWeek") },
                ]}
                onChange={(v) => setWeekOffset(Number(v) as 0 | 1)}
                ariaLabel={t("week")}
              />
            </div>
          </section>

          {!weekReady ? (
            <SkeletonRegion label="Essensplan wird geladen">
              <CardGridSkeleton
                cards={5}
                rowsPerCard={1}
                className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-5"
              />
            </SkeletonRegion>
          ) : weekError ? (
            <Alert type="error" message={t("loadError")} />
          ) : weekIsEmpty ? (
            <div className="moto-content-surface rounded-2xl border p-8 text-center text-sm text-gray-500 shadow-sm backdrop-blur-md">
              {t("emptyWeek")}
            </div>
          ) : (
            <div>
              {/* Mobile: today first, then the rest of the week as a list. */}
              <div className="space-y-4 md:hidden">
                {todayInWeek &&
                  (() => {
                    const dishes = dishesByDate.get(today) ?? [];
                    return (
                      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
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

                <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
                  <div className="border-b border-gray-200 px-4 py-2.5 text-xs font-semibold tracking-wide text-gray-500 uppercase">
                    {weekOffset === 0 ? t("weekHeading") : t("nextWeek")}
                  </div>
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
              <div className="moto-content-surface hidden overflow-hidden rounded-2xl border shadow-sm md:block">
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
            </div>
          )}
        </>
      )}
    </ParentPage>
  );
}
