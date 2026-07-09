"use client";

/**
 * Vertretungsplan (#1840) — the weekly substitution view.
 *
 * Deliberately separate from the Betreuungsplan (/timetables): this surface
 * shows the materialized week as a deviation over the base plan and lets an
 * admin record absences, substitutes, cancellations, and deliberately-open
 * positions for a week or a single day — without touching the half-year
 * template. It reuses the WeeklyCalendarGrid and the timetable API; the
 * per-block actions live in the SubstitutionSlideOver.
 */

import { ChevronLeft, ChevronRight, TriangleAlert, UserX } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { Suspense, useCallback, useMemo } from "react";

import { Button } from "~/components/ui/button";
import { PageHeader } from "~/components/ui/page-header/PageHeader";
import { SubstitutionSlideOver } from "~/components/timetable/substitution-slide-over";
import { TimetableStatCard } from "~/components/timetable/timetable-stat-card";
import { timetableSurface } from "~/components/timetable/timetable-style";
import { WeeklyCalendarGrid } from "~/components/timetable/weekly-calendar-grid";
import { useToast } from "~/contexts/ToastContext";
import { todayISO, toISODate } from "~/lib/date-helpers";
import { useTimetableDayHours } from "~/lib/hooks/use-timetable-day-hours";
import { createLogger } from "~/lib/logger";
import { staffService } from "~/lib/staff-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";
import {
  formatWeekLabel,
  getWeekRange,
  getWeekdays,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";

const logger = createLogger({ component: "Vertretungsplan" });
const HOUR_HEIGHT_PX = 90;

function parseIntParam(value: string | null, fallback: number): number {
  if (value === null) return fallback;
  const n = Number.parseInt(value, 10);
  return Number.isFinite(n) ? n : fallback;
}

function VertretungsplanContent() {
  const { status } = useSession();
  const searchParams = useSearchParams();
  const toast = useToast();
  const tenantMutate = useTenantMutate();
  const { dayStartHour, dayEndHour } = useTimetableDayHours();

  const weekOffset = parseIntParam(searchParams.get("week"), 0);
  const selectedInstanceId = searchParams.get("instance");

  const [range] = useMemo(() => {
    const r = getWeekRange(new Date(), weekOffset);
    return [r];
  }, [weekOffset]);
  const fromISO = toISODate(range.from);
  const toISO = toISODate(range.to);
  const weekDays = useMemo(() => getWeekdays(range.from), [range.from]);

  const updateUrlParams = useCallback(
    (updates: Record<string, string | null>) => {
      const params = new URLSearchParams(window.location.search);
      for (const [key, value] of Object.entries(updates)) {
        if (value === null) params.delete(key);
        else params.set(key, value);
      }
      const qs = params.toString();
      window.history.replaceState(
        null,
        "",
        qs ? `?${qs}` : window.location.pathname,
      );
    },
    [],
  );

  const swrKey = `vertretungsplan-week-${fromISO}-${toISO}`;
  // Gap detection is forward-looking: the endpoint rejects a past `date`, so
  // clamp the window start to today and skip entirely for fully-past weeks.
  const today = todayISO();
  const gapsFromISO = fromISO < today ? today : fromISO;
  const loadGaps = toISO >= today;
  const gapsSWRKey = `vertretungsplan-gaps-${gapsFromISO}-${toISO}`;

  const { data, isLoading } = useSWRAuth(
    status === "authenticated" ? swrKey : null,
    () => timetableService.getWeek(fromISO, toISO),
  );
  const { data: gapsData } = useSWRAuth(
    status === "authenticated" && loadGaps ? gapsSWRKey : null,
    () => timetableService.getGaps(gapsFromISO, toISO),
  );
  const { data: staffData } = useSWRAuth(
    status === "authenticated" ? "vertretungsplan-staff-list" : null,
    () => staffService.getAllStaff(),
  );

  const instances = useMemo(() => data?.instances ?? [], [data?.instances]);
  const staffOptions = useMemo(
    () => (staffData ?? []).map((s) => ({ id: s.id, name: s.name })),
    [staffData],
  );
  const staffNames = useMemo(
    () => new Map((staffData ?? []).map((s) => [s.id, s.name])),
    [staffData],
  );
  const selectedInstance = useMemo(
    () => instances.find((inst) => inst.id === selectedInstanceId) ?? null,
    [instances, selectedInstanceId],
  );

  const openGaps = gapsData?.gaps ?? [];
  const acknowledgedGaps = gapsData?.acknowledged ?? [];

  const revalidate = useCallback(async () => {
    await tenantMutate(swrKey);
    await tenantMutate(gapsSWRKey);
  }, [gapsSWRKey, swrKey, tenantMutate]);

  const handleSelectInstance = useCallback(
    (instance: EnrichedInstance | null) => {
      updateUrlParams({ instance: instance?.id ?? null });
    },
    [updateUrlParams],
  );

  const handleMarkAbsent = useCallback(
    async (absentStaffId: string, date: string) => {
      try {
        const result = await timetableService.markAbsent(absentStaffId, date);
        toast.success(
          `Abwesenheit gemeldet: ${result.affectedInstances.length} Termin(e)`,
        );
        await revalidate();
      } catch (err) {
        logger.error("mark_absent_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Abwesenheit konnte nicht gemeldet werden",
        );
        throw err;
      }
    },
    [revalidate, toast],
  );

  const handleSubstitute = useCallback(
    async (absentStaffId: string, substituteStaffId: string, date: string) => {
      try {
        const result = await timetableService.substitute(
          absentStaffId,
          substituteStaffId,
          date,
        );
        toast.success(
          `Ersatz eingetragen: ${result.affectedInstances.length} Termin(e)`,
        );
        if (result.warnings.length > 0) {
          toast.error(
            `${result.warnings.length} mögliche Zeitüberschneidung(en) prüfen.`,
          );
        }
        await revalidate();
      } catch (err) {
        logger.error("substitute_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Ersatz konnte nicht eingetragen werden",
        );
        throw err;
      }
    },
    [revalidate, toast],
  );

  const handleCancelBlock = useCallback(
    async (instance: EnrichedInstance) => {
      try {
        await timetableService.cancel(instance.id);
        toast.success("Block abgesagt");
        updateUrlParams({ instance: null });
        await revalidate();
      } catch (err) {
        logger.error("cancel_block_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Block konnte nicht abgesagt werden",
        );
        throw err;
      }
    },
    [revalidate, toast, updateUrlParams],
  );

  const handleAcknowledge = useCallback(
    async (instance: EnrichedInstance, ack: boolean, note?: string) => {
      try {
        await timetableService.acknowledgeUnderstaffed(instance.id, ack, note);
        toast.success(
          ack ? "Als bewusst unbesetzt markiert" : "Markierung aufgehoben",
        );
        await revalidate();
      } catch (err) {
        logger.error("acknowledge_understaffed_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Markierung konnte nicht geändert werden",
        );
        throw err;
      }
    },
    [revalidate, toast],
  );

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="Vertretungsplan" />

      <div
        className={`${timetableSurface} flex items-center justify-between gap-2 p-3`}
      >
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label="Vorherige Woche"
          onClick={() => updateUrlParams({ week: String(weekOffset - 1) })}
        >
          <ChevronLeft className="h-5 w-5" />
        </Button>
        <div className="text-center">
          <div className="text-sm font-semibold text-gray-900 tabular-nums">
            {formatWeekLabel(range.from, range.to)}
          </div>
          {weekOffset !== 0 && (
            <button
              type="button"
              className="text-[11px] font-medium text-[#5A8E1F] hover:underline"
              onClick={() => updateUrlParams({ week: "0" })}
            >
              Heute
            </button>
          )}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label="Nächste Woche"
          onClick={() => updateUrlParams({ week: String(weekOffset + 1) })}
        >
          <ChevronRight className="h-5 w-5" />
        </Button>
      </div>

      <VertretungsplanOverview
        openCount={openGaps.length}
        ackCount={acknowledgedGaps.length}
      />

      <WeeklyCalendarGrid
        weekDays={weekDays}
        instances={instances}
        selectedId={selectedInstanceId}
        onInstanceClick={handleSelectInstance}
        todayISO={todayISO()}
        dayStartHour={dayStartHour}
        dayEndHour={dayEndHour}
        hourHeightPx={HOUR_HEIGHT_PX}
        emptyState={
          instances.length > 0
            ? undefined
            : isLoading
              ? { title: "Lädt…", description: "Termine werden geladen." }
              : {
                  title: "Keine Termine",
                  description: "Für diese Woche sind keine Termine geplant.",
                }
        }
      />

      <SubstitutionSlideOver
        instance={selectedInstance}
        staffOptions={staffOptions}
        staffNames={staffNames}
        onClose={() => handleSelectInstance(null)}
        onMarkAbsent={handleMarkAbsent}
        onSubstitute={handleSubstitute}
        onCancelBlock={handleCancelBlock}
        onAcknowledge={handleAcknowledge}
      />
    </div>
  );
}

// VertretungsplanOverview mirrors the Betreuungsplan's TimetableOverview so the
// two planning pages share the same top-of-page shell (kicker + title +
// description + two KPI stat cards). The purpose differs — this one tracks
// staffing shortfalls, not planned counts.
function VertretungsplanOverview({
  openCount,
  ackCount,
}: {
  openCount: number;
  ackCount: number;
}) {
  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md sm:p-5">
      <div>
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
          Planung
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          Vertretungsplan im Blick
        </h2>
        <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
          Kurzfristige Abweichungen vom Betreuungsplan für eine Woche oder einen
          Tag — Abwesenheiten, Ersatz und ausfallende oder bewusst unbesetzte
          Blöcke, ohne die Halbjahresvorlage zu ändern.
        </p>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <TimetableStatCard
          size="lg"
          icon={<UserX className="h-4 w-4" />}
          label="Offene Lücken"
          value={String(openCount)}
          sublabel="brauchen Personal"
          tone={openCount > 0 ? "danger" : "neutral"}
        />
        <TimetableStatCard
          size="lg"
          icon={<TriangleAlert className="h-4 w-4" />}
          label="Bewusst unbesetzt"
          value={String(ackCount)}
          sublabel="ohne Personal akzeptiert"
          tone={ackCount > 0 ? "warning" : "neutral"}
        />
      </div>
    </section>
  );
}

export default function VertretungsplanPage() {
  return (
    <Suspense fallback={null}>
      <VertretungsplanContent />
    </Suspense>
  );
}
