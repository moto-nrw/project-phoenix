"use client";

/**
 * /database/timetables — admin weekly planner.
 *
 * Read-only MVP scope:
 * - Fetches one week of materialized instances via GET /api/timetable/instances
 * - Click on a card opens a right-side slide-over with the full state
 * - Lifecycle actions (Starten, Beenden, Absagen) wired to the existing
 *   POST /instances/{id}/{action} endpoints
 * - "Plan aktualisieren" runs POST /api/timetable/materialize for the
 *   visible week
 *
 * Deferred to a follow-up PR (matching the Read+Edit+Spontan plan):
 * - Editing time/room/staff (PUT /instances/{id})
 * - Spontaneous instance create (POST /instances)
 * - Per-page SSE wiring for instance_* events (today the global SSE
 *   handler does not invalidate timetable caches — lifecycle actions
 *   trigger refetch manually)
 */

import { Suspense, useCallback, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";

import { Loading } from "~/components/ui/loading";
import { useToast } from "~/contexts/ToastContext";
import { ConflictWarningsBanner } from "~/components/timetable/conflict-warnings-banner";
import {
  InstanceDetailSlideOver,
  type LifecycleAction,
} from "~/components/timetable/instance-detail-slide-over";
import { MaterializeButton } from "~/components/timetable/materialize-button";
import { SpontaneousInstanceModal } from "~/components/timetable/spontaneous-instance-modal";
import { WeekNavigator } from "~/components/timetable/week-navigator";
import { WeeklyPlannerGrid } from "~/components/timetable/weekly-planner-grid";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";
import {
  formatWeekLabel,
  getWeekRange,
  getWeekdays,
  toISODate,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";

const logger = createLogger({ component: "DatabaseTimetablesPage" });

function parseWeekOffset(raw: string | null): number {
  if (raw === null) return 0;
  const n = Number.parseInt(raw, 10);
  return Number.isFinite(n) ? n : 0;
}

function TimetablesContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const { status } = useSession({ required: true });
  const { success: toastSuccess, error: toastError } = useToast();
  const tenantMutate = useTenantMutate();

  const weekOffset = parseWeekOffset(searchParams.get("week"));
  const selectedInstanceId = searchParams.get("instance");
  const [createModalOpen, setCreateModalOpen] = useState(false);

  const updateUrlParams = useCallback(
    (patch: Record<string, string | null>) => {
      const next = new URLSearchParams(searchParams.toString());
      for (const [key, value] of Object.entries(patch)) {
        if (value === null || value === "") {
          next.delete(key);
        } else {
          next.set(key, value);
        }
      }
      const qs = next.toString();
      router.replace(qs ? `?${qs}` : "?", { scroll: false });
    },
    [router, searchParams],
  );

  const handleWeekChange = useCallback(
    (newOffset: number) => {
      updateUrlParams({
        week: newOffset === 0 ? null : String(newOffset),
        // changing weeks closes the slide-over to avoid stale selection
        instance: null,
      });
    },
    [updateUrlParams],
  );

  const handleSelectInstance = useCallback(
    (instance: EnrichedInstance | null) => {
      updateUrlParams({ instance: instance?.id ?? null });
    },
    [updateUrlParams],
  );

  // Week range. useMemo prevents re-derivation on every render — the SWR
  // key depends on these strings.
  const weekRange = useMemo(
    () => getWeekRange(new Date(), weekOffset),
    [weekOffset],
  );
  const fromISO = toISODate(weekRange.from);
  const toISO = toISODate(weekRange.to);
  const weekDays = useMemo(() => getWeekdays(weekRange.from), [weekRange.from]);
  const todayISO = useMemo(() => toISODate(new Date()), []);
  const weekLabel = useMemo(
    () => formatWeekLabel(weekRange.from, weekRange.to),
    [weekRange.from, weekRange.to],
  );

  const swrKey = `timetable-week-${fromISO}-${toISO}`;
  const { data, error, isLoading } = useSWRAuth(
    status === "authenticated" ? swrKey : null,
    () => timetableService.getWeek(fromISO, toISO),
  );

  // Memoise on data?.instances so the reference is stable between renders
  // when SWR returns the same response (the linter warns when arrays are
  // derived inline because `?? []` produces a new array each render).
  const instances = useMemo(() => data?.instances ?? [], [data?.instances]);
  const conflictCount = useMemo(
    () =>
      instances.reduce((sum, inst) => sum + inst.conflictWarnings.length, 0),
    [instances],
  );

  const selectedInstance = useMemo(
    () => instances.find((inst) => inst.id === selectedInstanceId) ?? null,
    [instances, selectedInstanceId],
  );

  const handleMaterialize = useCallback(async () => {
    try {
      const result = await timetableService.materialize(fromISO, toISO);
      toastSuccess(
        `Plan aktualisiert: ${result.instancesCreated} ${
          result.instancesCreated === 1 ? "Aktivität" : "Aktivitäten"
        } angelegt`,
      );
      await tenantMutate(swrKey);
    } catch (err) {
      logger.error("materialize_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toastError("Plan konnte nicht aktualisiert werden");
    }
  }, [fromISO, toISO, swrKey, toastSuccess, toastError, tenantMutate]);

  const handleLifecycle = useCallback(
    async (action: LifecycleAction) => {
      if (!selectedInstance) return;
      try {
        if (action === "start") {
          const res = await timetableService.start(selectedInstance.id);
          if (res.warnings.length > 0) {
            toastSuccess(
              `Gestartet — ${res.warnings.length} Hinweis(e): ${res.warnings.map((w) => w.message).join(", ")}`,
            );
          } else {
            toastSuccess("Aktivität gestartet");
          }
        } else if (action === "complete") {
          await timetableService.complete(selectedInstance.id);
          toastSuccess("Aktivität beendet");
        } else {
          await timetableService.cancel(selectedInstance.id);
          toastSuccess("Aktivität abgesagt");
        }
        await tenantMutate(swrKey);
      } catch (err) {
        logger.error("lifecycle_action_failed", {
          action,
          instance_id: selectedInstance.id,
          error: err instanceof Error ? err.message : String(err),
        });
        toastError(
          err instanceof Error
            ? err.message
            : "Aktion konnte nicht durchgeführt werden",
        );
        throw err;
      }
    },
    [selectedInstance, swrKey, tenantMutate, toastSuccess, toastError],
  );

  if (status === "loading" || isLoading) {
    return (
      <div className="p-6">
        <Loading />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 p-6">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-bold text-slate-900">Stundenplan</h1>
          <p className="text-sm text-slate-500">Wochenplan • {weekLabel}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <MaterializeButton
            onMaterialize={handleMaterialize}
            weekLabel={weekLabel}
          />
        </div>
      </header>

      <div className="flex flex-wrap items-center gap-3">
        <WeekNavigator
          weekOffset={weekOffset}
          weekRange={weekRange}
          onChange={handleWeekChange}
        />
        <div className="flex-1" />
        <button
          type="button"
          onClick={() => setCreateModalOpen(true)}
          className="inline-flex items-center gap-2 rounded-md border border-[#5080D8] bg-white px-3 py-2 text-xs font-semibold text-[#5080D8] shadow-sm transition-colors hover:bg-[#EBF0FB]"
        >
          + Spontane Aktivität
        </button>
      </div>

      <ConflictWarningsBanner conflictCount={conflictCount} />

      {error ? (
        <div className="rounded-lg border border-[#FECACA] bg-[#FEF2F2] p-4 text-sm text-[#7F1D1D]">
          Stundenplan konnte nicht geladen werden:{" "}
          {error instanceof Error ? error.message : "Unbekannter Fehler"}.
          <button
            type="button"
            onClick={() => void tenantMutate(swrKey)}
            className="ml-2 font-semibold underline hover:no-underline"
          >
            Erneut versuchen
          </button>
        </div>
      ) : (
        <WeeklyPlannerGrid
          weekDays={weekDays}
          instances={instances}
          selectedId={selectedInstanceId}
          onInstanceClick={handleSelectInstance}
          todayISO={todayISO}
        />
      )}

      {instances.length === 0 && !error && (
        <div className="rounded-md border border-dashed border-slate-300 bg-slate-50 p-4 text-center text-sm text-slate-500">
          Für diese Woche sind noch keine Aktivitäten geplant.
          <br />
          Klicke auf <span className="font-semibold">Plan aktualisieren</span>,
          um die Vorlagen zu materialisieren.
        </div>
      )}

      <InstanceDetailSlideOver
        instance={selectedInstance}
        onClose={() => handleSelectInstance(null)}
        onLifecycleAction={handleLifecycle}
        editDeferred
      />

      <SpontaneousInstanceModal
        isOpen={createModalOpen}
        onClose={() => setCreateModalOpen(false)}
        defaultDate={fromISO}
        onCreated={() => {
          // Refetch the visible week so the new card appears immediately.
          // The created instance is in the response too, but going through
          // SWR mutate keeps a single source of truth (the cached week).
          void tenantMutate(swrKey);
        }}
      />
    </div>
  );
}

export default function TimetablesPage() {
  return (
    <Suspense
      fallback={
        <div className="p-6">
          <Loading />
        </div>
      }
    >
      <TimetablesContent />
    </Suspense>
  );
}
