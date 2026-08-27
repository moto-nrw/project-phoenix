"use client";

/**
 * Inhaltsblock der Kalenderzeiträume (Halbjahre, Schuljahre, Ferien,
 * Sonderzeiträume) auf /calendar-periods. Er nutzt den bestehenden
 * CalendarPeriodModal aus dem Betreuungsplan — die hier verwalteten Zeiträume
 * sind dieselben Zeilen, auf die Materialisierung und Anmeldephasen verweisen.
 *
 * Titel, Statuszeile und die beiden Anlegen-Aktionen trägt die Seite über das
 * Seitengerüst; dieser Block liefert nur seine Karte. Leer- und Fehlerzustand
 * bleiben bewusst hier statt im Gerüst: die Seite zeigt darunter noch die
 * Schließtage, die von einem leeren oder fehlgeschlagenen Zeitraum-Abruf nicht
 * verschwinden dürfen.
 */

import { useCallback, useMemo } from "react";
import { Plus } from "lucide-react";

import { CalendarPeriodModal } from "~/components/timetable/calendar-period-modal";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  DataTable,
  DataTableStatusBadge,
  type DataTableColumn,
} from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import {
  type CalendarPeriod,
  PERIOD_TYPE_LABELS,
  formatPeriodRange,
  formatPeriodUsage,
} from "~/lib/calendar-period-helpers";
import type { CalendarPeriodsState } from "~/components/planning/use-calendar-periods";

export { useCalendarPeriods } from "~/components/planning/use-calendar-periods";

/**
 * Die beiden Anlegen-Aktionen der Kopfkarte. Sie hängen an den
 * Modal-Zuständen des Bereichs und stehen deshalb hier, obwohl sie oben im
 * Seitengerüst erscheinen.
 */
export function CalendarPeriodsActions({
  state,
}: Readonly<{ state: CalendarPeriodsState }>) {
  return (
    <>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={state.beginCreateSemester}
        className="shrink-0 gap-2"
      >
        <MotoConceptIcon concept="calendarPeriods" size={16} />
        Halbjahr anlegen
      </Button>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={state.beginCreate}
        className="shrink-0 gap-2"
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
        Zeitraum anlegen
      </Button>
    </>
  );
}

export function CalendarPeriodsEditor({
  state,
}: Readonly<{ state: CalendarPeriodsState }>) {
  const {
    periods,
    phases,
    loading,
    error,
    modalOpen,
    editing,
    createDefaults,
    editingUsage,
    beginCreateSemester,
    beginEdit,
    closeModal,
    reload,
    handlePhaseLinkToggle,
  } = state;

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

      <SectionCard
        title="Angelegte Zeiträume"
        description="Halbjahre, Schuljahre, Ferien und Sonderzeiträume. Anmeldephasen und Betreuungsplan verweisen auf diese Zeiträume."
      >
        {!loading && periods.length === 0 ? (
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
      </SectionCard>

      <CalendarPeriodModal
        isOpen={modalOpen}
        onClose={closeModal}
        onSaved={() => void reload({ silent: true })}
        onDeleted={() => void reload()}
        initial={editing}
        createDefaults={createDefaults}
        usage={editingUsage}
        phaseLink={{ phases, onToggle: handlePhaseLinkToggle }}
      />
    </div>
  );
}
