"use client";

/**
 * CalendarPeriodsEditor is the standalone admin page for planning
 * periods (Halbjahre, Schuljahre, Ferien, Sonderzeiträume). It reuses
 * the existing CalendarPeriodModal from the timetable feature — the
 * periods managed here are the same rows the Betreuungsplan
 * materialization and the enrollment phases reference.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

import {
  CalendarPeriodModal,
  type LinkablePhase,
} from "~/components/timetable/calendar-period-modal";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { PageIntro } from "~/components/ui/page-intro";
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

  const editingUsage = useMemo(() => {
    if (!editing) return undefined;
    const usageSource =
      periods.find((period) => period.id === editing.id) ?? editing;
    return {
      enrollmentPhaseCount: usageSource.enrollmentPhaseCount ?? 0,
      activityGroupCount: usageSource.activityGroupCount ?? 0,
      scheduleCount: usageSource.scheduleCount ?? 0,
      studentEnrollmentCount: usageSource.studentEnrollmentCount ?? 0,
      supervisorCount: usageSource.supervisorCount ?? 0,
      activityInstanceCount: usageSource.activityInstanceCount ?? 0,
    };
  }, [editing, periods]);

  const usageTotal = useCallback(
    (period: CalendarPeriod) =>
      (period.enrollmentPhaseCount ?? 0) +
      (period.activityGroupCount ?? 0) +
      (period.scheduleCount ?? 0) +
      (period.studentEnrollmentCount ?? 0) +
      (period.supervisorCount ?? 0) +
      (period.activityInstanceCount ?? 0),
    [],
  );

  const columns = useMemo<DataTableColumn<CalendarPeriod>[]>(
    () => [
      {
        key: "name",
        header: "Zeitraum",
        sortValue: (period) => period.name,
        // Auf schmalen Screens tragen die ausgeblendeten Spalten (Art,
        // Zeitraum, Status) hier als Unterzeile — die Tabelle bleibt damit
        // ohne Seitwärts-Scrollen lesbar (#2033).
        render: (period) => (
          // Kein fester max-w: die Zelle nimmt die Restbreite, die die
          // schmale Aktionsspalte (w-px) übrig lässt. Umbrechender Text
          // statt truncate hält die Spalte auch bei 320px im Container.
          <div className="min-w-0">
            {/* wrap-anywhere: ein einzelnes langes Wort im Namen darf die
                Spalte nicht über die Tabellenbreite hinaus aufziehen. */}
            <p className="font-medium wrap-anywhere text-gray-900">
              {period.name}
            </p>
            <p className="mt-0.5 text-xs break-words text-gray-500 sm:hidden">
              {PERIOD_TYPE_LABELS[period.periodType]}
            </p>
            <p className="text-xs leading-5 break-words text-gray-500 sm:hidden">
              {formatPeriodRange(period)}
            </p>
            {/* Der Status steht nur mobil hier, und nur wenn er vom Normalfall
                abweicht — angehängt an die Datumszeile würde er abgeschnitten. */}
            {!period.isActive && (
              <span className="mt-1 inline-flex sm:hidden">
                <DataTableStatusBadge active={false} />
              </span>
            )}
          </div>
        ),
      },
      {
        key: "type",
        header: "Art",
        className: "hidden sm:table-cell",
        headerClassName: "hidden sm:table-cell",
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
        className: "hidden sm:table-cell",
        headerClassName: "hidden sm:table-cell",
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
        className: "hidden lg:table-cell",
        headerClassName: "hidden lg:table-cell",
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
        className: "hidden lg:table-cell",
        headerClassName: "hidden lg:table-cell",
        sortValue: usageTotal,
        render: (period) => {
          const usage = formatPeriodUsage(
            period.enrollmentPhaseCount ?? 0,
            period.scheduleCount ?? 0,
            " · ",
            {
              activityGroupCount: period.activityGroupCount ?? 0,
              studentEnrollmentCount: period.studentEnrollmentCount ?? 0,
              supervisorCount: period.supervisorCount ?? 0,
              activityInstanceCount: period.activityInstanceCount ?? 0,
            },
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
        className: "hidden sm:table-cell",
        headerClassName: "hidden sm:table-cell",
        sortValue: (period) => (period.isActive ? 0 : 1),
        render: (period) => <DataTableStatusBadge active={period.isActive} />,
      },
      {
        key: "actions",
        header: "",
        align: "right",
        // w-px zwingt die Auto-Layout-Tabelle, dieser Spalte nur ihre
        // Mindestbreite zu geben — der Rest bleibt für den Namen (#2033).
        className: "w-px whitespace-nowrap",
        headerClassName: "w-px",
        render: (period) => (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => beginEdit(period)}
          >
            Bearbeiten
          </Button>
        ),
      },
    ],
    [beginEdit, usageTotal],
  );

  return (
    <div className="space-y-4">
      {error && <Alert type="error" message={error} />}

      {/* Kopfkarte der Seite: Titel, Erklärtext und die beiden Seitenaktionen
          in einer Zeile, wie in der Eltern-App. Die Aktionen hängen an den
          Modal-Zuständen dieses Editors und leben deshalb hier, nicht in
          page.tsx. */}
      <PageIntro
        kicker="Planung"
        title="Kalenderzeiträume"
        description="Halbjahre, Ferien und Sonderzeiträume als gemeinsame Basis für Anmeldung und Betreuungsplan."
        actions={
          <>
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={beginCreateSemester}
              className="shrink-0 gap-2"
            >
              <MotoConceptIcon concept="calendarPeriods" size={16} />
              Halbjahr anlegen
            </Button>
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={beginCreate}
              className="shrink-0 gap-2"
            >
              <Plus className="h-4 w-4" aria-hidden="true" />
              Zeitraum anlegen
            </Button>
          </>
        }
      />

      {!loading && periods.length === 0 ? (
        <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md sm:p-6">
          <EmptyState
            icon={
              <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100">
                <MotoConceptIcon concept="calendarPeriods" size={28} />
              </span>
            }
            title="Noch keine Kalenderzeiträume"
            description="Legen Sie das nächste Halbjahr an, damit Anmeldephasen und Betreuungsplan darauf verweisen können."
            action={
              <Button
                type="button"
                variant="primary"
                size="md"
                onClick={beginCreateSemester}
                className="gap-2"
              >
                <MotoConceptIcon concept="calendarPeriods" size={16} />
                Halbjahr anlegen
              </Button>
            }
          />
        </section>
      ) : (
        <DataTable
          columns={columns}
          rows={periods}
          getRowKey={(period) => period.id}
          defaultSortKey="range"
          defaultSortDirection="asc"
          isLoading={loading}
        />
      )}

      <CalendarPeriodModal
        isOpen={modalOpen}
        onClose={() => setModalOpen(false)}
        onSaved={() => void load({ silent: true })}
        onDeleted={() => void load()}
        initial={editing}
        createDefaults={createDefaults}
        usage={editingUsage}
        phaseLink={{ phases, onToggle: handlePhaseLinkToggle }}
      />
    </div>
  );
}
