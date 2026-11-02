"use client";

/**
 * Schulferien der Schule auf „Schuljahr und Kalender“ (#5345).
 *
 * moto liest sie jeden Tag aus dem öffentlichen Ferienkalender OpenHolidays,
 * für das Bundesland der Schule. Die Liste ist nicht bearbeitbar: das
 * Bundesland ändert nur das moto-Team. Ferien sind keine Schließtage; ob die
 * OGS schließt, trägt die Schule weiter selbst als Schließtag ein.
 */

import { useMemo } from "react";

import { holidayWindow } from "~/components/planning/school-holidays-list";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { formatDate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  getSchoolVacations,
  type SchoolVacation,
  type SchoolVacations,
} from "~/lib/school-vacations-api";
import { useSWRAuth } from "~/lib/swr";
import { timetableKeys } from "~/lib/swr/portal-keys";

function vacationRange(vacation: SchoolVacation): string {
  if (vacation.startDate === vacation.endDate) {
    return formatDate(vacation.startDate);
  }
  return `${formatDate(vacation.startDate)} bis ${formatDate(vacation.endDate)}`;
}

const columns: DataTableColumn<SchoolVacation>[] = [
  {
    key: "range",
    header: "Zeitraum",
    sortValue: (vacation) => vacation.startDate,
    render: (vacation) => (
      <span className="text-sm whitespace-nowrap text-gray-600 tabular-nums">
        {vacationRange(vacation)}
      </span>
    ),
  },
  {
    key: "name",
    header: "Ferien",
    sortValue: (vacation) => vacation.name,
    render: (vacation) => (
      <span className="flex flex-wrap items-center gap-2">
        <span className="font-medium wrap-anywhere text-gray-900">
          {vacation.name}
        </span>
        {vacation.acknowledged ? (
          <StatusBadge tone="green" label="Betreuung geplant" />
        ) : null}
      </span>
    ),
  },
];

function vacationsDescription(
  data: SchoolVacations | undefined,
  schoolYearName: string | null,
): string {
  const scope = schoolYearName
    ? `für „${schoolYearName}“`
    : "in den nächsten zwölf Monaten";
  const state = data ? ` in ${data.federalStateName}` : "";
  return `Nur zur Information: die Schulferien${state} ${scope}. moto holt sie jeden Tag aus dem öffentlichen Ferienkalender OpenHolidays. Ob die OGS schließt, tragen Sie als Schließtag ein. Stimmt das Bundesland nicht, wenden Sie sich an das moto-Team.`;
}

function emptyState(data: SchoolVacations | undefined) {
  if (data && data.syncedAt === null) {
    return (
      <EmptyState
        title="Noch keine Schulferien"
        description="moto hat die Ferien für Ihr Bundesland noch nicht geholt. Das passiert einmal am Tag. Schauen Sie später noch einmal nach."
      />
    );
  }
  return (
    <EmptyState
      title="Keine Schulferien"
      description="In diesem Zeitraum gibt es keine Schulferien."
    />
  );
}

export function SchoolVacationsList({
  periods,
  periodsLoading,
}: {
  readonly periods: readonly CalendarPeriod[];
  /** Solange die Zeiträume laden, steht das Fenster noch nicht fest. */
  readonly periodsLoading: boolean;
}) {
  const today = useBerlinToday();
  const window = useMemo(
    () => holidayWindow(periods, today),
    [periods, today],
  );
  const { data, error, isLoading, mutate } = useSWRAuth<SchoolVacations>(
    periodsLoading
      ? null
      : timetableKeys.schoolVacations(window.from, window.to),
    () => getSchoolVacations(window.from, window.to),
  );
  const loadError = useSwrLoadError(error, "die Liste der Schulferien", () =>
    mutate(),
  );
  const failedWithoutData = Boolean(error) && !data;

  return (
    <SectionCard
      title="Schulferien"
      description={vacationsDescription(data, window.schoolYearName)}
      testId="school-vacations-list"
    >
      <LoadErrorAlert
        error={loadError}
        className={failedWithoutData ? undefined : "mb-4"}
      />
      {failedWithoutData && loadError ? null : (
        <DataTable
          columns={columns}
          rows={[...(data?.vacations ?? [])]}
          getRowKey={(vacation) => `${vacation.startDate}-${vacation.endDate}`}
          defaultSortKey="range"
          defaultSortDirection="asc"
          isLoading={periodsLoading || isLoading || failedWithoutData}
          emptyState={emptyState(data)}
        />
      )}
    </SectionCard>
  );
}
