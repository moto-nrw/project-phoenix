"use client";

/**
 * CalendarPeriodsEditor is the standalone admin page for planning
 * periods (Halbjahre, Schuljahre, Ferien, Sonderzeiträume). It reuses
 * the existing CalendarPeriodModal from the timetable feature — the
 * periods managed here are the same rows the Betreuungsplan
 * materialization and the enrollment phases reference.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { CalendarDays, CalendarPlus, Plus } from "lucide-react";

import {
  CalendarPeriodModal,
  type LinkablePhase,
} from "~/components/timetable/calendar-period-modal";
import {
  DataTable,
  DataTableStatusBadge,
  type DataTableColumn,
} from "~/components/ui/data-table";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import {
  type Phase,
  listPhases,
  setPhaseCalendarPeriod,
} from "~/lib/enrollment-phase-api";
import {
  type CalendarPeriod,
  PERIOD_TYPE_LABELS,
  formatPeriodRange,
  formatPeriodUsage,
} from "~/lib/calendar-period-helpers";
import { useToast } from "~/contexts/ToastContext";
import { todayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CalendarPeriodsEditor" });

interface SemesterDefaults {
  name: string;
  periodType: "semester";
  startDate: string;
  endDate: string;
}

/**
 * Suggests the next upcoming Halbjahr based on today. German school
 * half-years run Aug 1 – Jan 31 (1. HJ) and Feb 1 – Jul 31 (2. HJ);
 * these are prefill suggestions only — the admin adjusts the dates in
 * the modal to match the actual school calendar.
 */
function nextSemesterDefaults(todayIso: string): SemesterDefaults {
  const year = Number(todayIso.slice(0, 4));
  const month = Number(todayIso.slice(5, 7));

  if (month >= 8) {
    // In the 1st half-year → suggest the 2nd half of the same school year.
    return {
      name: `2. Halbjahr ${year}/${String((year + 1) % 100).padStart(2, "0")}`,
      periodType: "semester",
      startDate: `${year + 1}-02-01`,
      endDate: `${year + 1}-07-31`,
    };
  }
  if (month === 1) {
    // January: still 1st half-year → suggest the 2nd half starting Feb 1.
    return {
      name: `2. Halbjahr ${year - 1}/${String(year % 100).padStart(2, "0")}`,
      periodType: "semester",
      startDate: `${year}-02-01`,
      endDate: `${year}-07-31`,
    };
  }
  // Feb–Jul: in the 2nd half-year → suggest the 1st half of the next school year.
  return {
    name: `1. Halbjahr ${year}/${String((year + 1) % 100).padStart(2, "0")}`,
    periodType: "semester",
    startDate: `${year}-08-01`,
    endDate: `${year + 1}-01-31`,
  };
}

export function CalendarPeriodsEditor() {
  const [periods, setPeriods] = useState<CalendarPeriod[]>([]);
  const [phases, setPhases] = useState<Phase[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<CalendarPeriod | null>(null);
  const [createDefaults, setCreateDefaults] =
    useState<Partial<SemesterDefaults>>();
  const { success: toastSuccess, error: toastError } = useToast();

  // silent: refresh data without the full-page loading state — used after
  // phase-link toggles so the open modal doesn't get unmounted.
  const load = useCallback(async (opts?: { silent?: boolean }) => {
    if (!opts?.silent) setLoading(true);
    setError(null);
    try {
      const [periodData, phaseData] = await Promise.all([
        calendarPeriodService.list(),
        // Phases power the bidirectional link section in the modal.
        // Their failure must not take the periods page down.
        listPhases().catch((err: unknown) => {
          logger.warn("calendar_periods_phases_load_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as Phase[];
        }),
      ]);
      setPeriods(
        [...periodData].sort((a, b) => a.startDate.localeCompare(b.startDate)),
      );
      setPhases(phaseData);
      // Keep the currently edited period fresh so the modal's usage-based
      // delete warning reflects just-toggled phase links.
      setEditing((prev) =>
        prev ? (periodData.find((p) => p.id === prev.id) ?? prev) : prev,
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("calendar_periods_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const beginCreate = () => {
    setEditing(null);
    setCreateDefaults(undefined);
    setModalOpen(true);
  };

  const beginCreateSemester = () => {
    setEditing(null);
    setCreateDefaults(nextSemesterDefaults(todayISO()));
    setModalOpen(true);
  };

  const handlePhaseLinkToggle = useCallback(
    async (phase: LinkablePhase, link: boolean) => {
      const target = editing;
      const full = phases.find((p) => p.id === phase.id);
      if (!target || !full) return;
      try {
        await setPhaseCalendarPeriod(full, link ? target.id : null);
        toastSuccess(
          link
            ? `Anmeldephase "${full.name}" mit "${target.name}" verknüpft`
            : `Verknüpfung von "${full.name}" entfernt`,
        );
      } catch (err) {
        const message =
          err instanceof Error
            ? err.message
            : "Verknüpfung konnte nicht gespeichert werden";
        logger.error("phase_link_toggle_failed", {
          phase_id: phase.id,
          error: message,
        });
        toastError(message);
      } finally {
        await load({ silent: true });
      }
    },
    [editing, phases, load, toastSuccess, toastError],
  );

  const beginEdit = useCallback((period: CalendarPeriod) => {
    setEditing(period);
    setCreateDefaults(undefined);
    setModalOpen(true);
  }, []);

  const columns = useMemo<DataTableColumn<CalendarPeriod>[]>(
    () => [
      {
        key: "name",
        header: "Zeitraum",
        sortValue: (period) => period.name,
        render: (period) => (
          <p className="truncate font-medium text-gray-900">{period.name}</p>
        ),
      },
      {
        key: "type",
        header: "Art",
        sortValue: (period) => PERIOD_TYPE_LABELS[period.periodType],
        render: (period) => (
          <span className="text-sm text-gray-600">
            {PERIOD_TYPE_LABELS[period.periodType]}
          </span>
        ),
      },
      {
        key: "range",
        header: "Von – Bis",
        sortValue: (period) => period.startDate,
        render: (period) => (
          <span className="text-sm whitespace-nowrap text-gray-600">
            {formatPeriodRange(period)}
          </span>
        ),
      },
      {
        key: "cycle",
        header: "Rhythmus",
        sortValue: (period) => period.weekCycleLength,
        render: (period) => (
          <span className="text-sm text-gray-600">
            {period.weekCycleLength > 1
              ? `Alle ${period.weekCycleLength} Wochen (A/B)`
              : "Jede Woche"}
          </span>
        ),
      },
      {
        key: "usage",
        header: "Verwendung",
        sortValue: (period) =>
          (period.enrollmentPhaseCount ?? 0) + (period.scheduleCount ?? 0),
        render: (period) => {
          const usage = formatPeriodUsage(
            period.enrollmentPhaseCount ?? 0,
            period.scheduleCount ?? 0,
          );
          return usage ? (
            <span className="text-sm whitespace-nowrap text-gray-600">
              {usage}
            </span>
          ) : (
            <span className="text-sm text-gray-400">Nicht verwendet</span>
          );
        },
      },
      {
        key: "status",
        header: "Status",
        sortValue: (period) => (period.isActive ? 0 : 1),
        render: (period) => <DataTableStatusBadge active={period.isActive} />,
      },
      {
        key: "actions",
        header: "",
        align: "right",
        render: (period) => (
          <button
            type="button"
            onClick={() => beginEdit(period)}
            className="inline-flex h-8 items-center justify-center rounded-md px-2.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Bearbeiten
          </button>
        ),
      },
    ],
    [beginEdit],
  );

  if (loading) {
    return (
      <div className="moto-content-surface rounded-2xl border px-5 py-10 text-center text-sm text-gray-500 shadow-sm">
        Kalenderzeiträume werden geladen...
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {error && (
        <div
          className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]"
          role="alert"
          aria-live="polite"
        >
          {error}
        </div>
      )}

      <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-gray-600">
            Halbjahre, Ferien und Sonderzeiträume als gemeinsame Basis für
            Anmeldung und Betreuungsplan.
          </p>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <button
              type="button"
              onClick={beginCreateSemester}
              className="inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <CalendarPlus className="h-4 w-4" aria-hidden="true" />
              Halbjahr anlegen
            </button>
            <button
              type="button"
              onClick={beginCreate}
              className="inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <Plus className="h-4 w-4" aria-hidden="true" />
              Zeitraum anlegen
            </button>
          </div>
        </div>
      </section>

      {periods.length === 0 ? (
        <section className="moto-content-surface rounded-2xl border px-6 py-12 text-center shadow-sm backdrop-blur-md">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-[#83CD2D]/10 text-[#669f21]">
            <CalendarDays className="h-6 w-6" aria-hidden="true" />
          </div>
          <h2 className="mt-4 text-base font-semibold text-gray-900">
            Noch keine Kalenderzeiträume
          </h2>
          <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-gray-600">
            Lege das nächste Halbjahr an, damit Anmeldephasen und Betreuungsplan
            darauf verweisen können.
          </p>
          <button
            type="button"
            onClick={beginCreateSemester}
            className="mt-5 inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <CalendarPlus className="h-4 w-4" aria-hidden="true" />
            Halbjahr anlegen
          </button>
        </section>
      ) : (
        <DataTable
          columns={columns}
          rows={periods}
          getRowKey={(period) => period.id}
          defaultSortKey="range"
          defaultSortDirection="asc"
        />
      )}

      <CalendarPeriodModal
        isOpen={modalOpen}
        onClose={() => setModalOpen(false)}
        onSaved={() => void load()}
        onDeleted={() => void load()}
        initial={editing}
        createDefaults={createDefaults}
        usage={
          editing
            ? {
                enrollmentPhaseCount: editing.enrollmentPhaseCount ?? 0,
                scheduleCount: editing.scheduleCount ?? 0,
              }
            : undefined
        }
        phaseLink={{ phases, onToggle: handlePhaseLinkToggle }}
      />
    </div>
  );
}
