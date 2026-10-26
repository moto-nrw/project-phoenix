"use client";

/**
 * Gesetzliche Feiertage der Schule auf „Schuljahr und Kalender“ (#4775).
 *
 * moto berechnet sie aus dem Bundesland der Schule; die Leitung prüft hier,
 * ob das richtige Bundesland hinterlegt ist. Die Liste ist nicht bearbeitbar:
 * das Bundesland ändert nur das moto-Team.
 */

import { useMemo } from "react";

import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { berlinTodayISO, parseISODate, toISODate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  getSchoolHolidays,
  type SchoolHoliday,
  type SchoolHolidays,
} from "~/lib/school-holidays-api";
import { useSWRAuth } from "~/lib/swr";

export interface HolidayWindow {
  readonly from: string;
  readonly to: string;
  /** Name des laufenden Schuljahres, null ohne laufendes Schuljahr. */
  readonly schoolYearName: string | null;
}

/**
 * Das Fenster der Liste: das laufende aktive Schuljahr. Ohne eines zeigt die
 * Liste die nächsten zwölf Monate ab heute.
 */
export function holidayWindow(
  periods: readonly CalendarPeriod[],
  todayISO: string,
): HolidayWindow {
  const schoolYear = periods.find(
    (period) =>
      period.isActive &&
      period.periodType === "school_year" &&
      period.startDate <= todayISO &&
      todayISO <= period.endDate,
  );
  if (schoolYear) {
    return {
      from: schoolYear.startDate,
      to: schoolYear.endDate,
      schoolYearName: schoolYear.name,
    };
  }
  const end = parseISODate(todayISO);
  end.setFullYear(end.getFullYear() + 1);
  end.setDate(end.getDate() - 1);
  return { from: todayISO, to: toISODate(end), schoolYearName: null };
}

function formatHolidayDate(iso: string): string {
  return parseISODate(iso).toLocaleDateString("de-DE", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

const columns: DataTableColumn<SchoolHoliday>[] = [
  {
    key: "date",
    header: "Datum",
    sortValue: (holiday) => holiday.date,
    render: (holiday) => (
      <span className="text-sm whitespace-nowrap text-gray-600 tabular-nums">
        {formatHolidayDate(holiday.date)}
      </span>
    ),
  },
  {
    key: "name",
    header: "Feiertag",
    sortValue: (holiday) => holiday.name,
    render: (holiday) => (
      <span className="font-medium wrap-anywhere text-gray-900">
        {holiday.name}
      </span>
    ),
  },
];

function holidaysDescription(
  data: SchoolHolidays | undefined,
  window: HolidayWindow,
): string {
  const scope = window.schoolYearName
    ? `für „${window.schoolYearName}“`
    : "in den nächsten zwölf Monaten";
  const state = data ? ` in ${data.federalStateName}` : "";
  return `Nur zur Information: die gesetzlichen Feiertage${state} ${scope}. moto richtet sie nach dem Bundesland der Schule. Stimmt das Bundesland nicht, wenden Sie sich an das moto-Team.`;
}

export function SchoolHolidaysList({
  periods,
  periodsLoading,
}: {
  readonly periods: readonly CalendarPeriod[];
  /** Solange die Zeiträume laden, steht das Fenster noch nicht fest. */
  readonly periodsLoading: boolean;
}) {
  const window = useMemo(
    () => holidayWindow(periods, berlinTodayISO()),
    [periods],
  );
  const { data, error, isLoading, mutate } = useSWRAuth<SchoolHolidays>(
    periodsLoading ? null : `school-holidays-${window.from}-${window.to}`,
    () => getSchoolHolidays(window.from, window.to),
  );
  const loadError = useSwrLoadError(error, "die Liste der Feiertage", () =>
    mutate(),
  );
  const failedWithoutData = Boolean(error) && !data;

  return (
    <SectionCard
      title="Gesetzliche Feiertage"
      description={holidaysDescription(data, window)}
    >
      <LoadErrorAlert
        error={loadError}
        className={failedWithoutData ? undefined : "mb-4"}
      />
      {failedWithoutData && loadError ? null : (
        <DataTable
          columns={columns}
          rows={[...(data?.holidays ?? [])]}
          getRowKey={(holiday) => holiday.date}
          defaultSortKey="date"
          defaultSortDirection="asc"
          isLoading={periodsLoading || isLoading || failedWithoutData}
          emptyState={
            <EmptyState
              title="Keine Feiertage"
              description="In diesem Zeitraum gibt es keinen gesetzlichen Feiertag."
            />
          }
        />
      )}
    </SectionCard>
  );
}
