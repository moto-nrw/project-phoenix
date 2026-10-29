"use client";

// Soll und Ist je Kind (#4774): the booked care time of each day next to
// the actual attendance, with overruns marked in colour AND text. Read-only:
// the booked times are changed in the weekly plan above.

import { useEffect, useMemo, useState } from "react";
import type { DateRange } from "react-day-picker";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import {
  buildDefaultPresets,
  DateRangePicker,
} from "~/components/ui/date-range-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiLoadError } from "~/contexts/ToastContext";
import {
  CARE_TIME_DAY_LABELS,
  fetchStudentCareTimes,
  formatClockSpan,
  formatDeviation,
  overrunReason,
  type CareTimeDay,
  type StudentCareTimes,
} from "~/lib/care-times";
import {
  berlinTodayISO,
  formatStatusDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { LOCATION_COLORS, getAccessibleTextColor } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentCareTimesSection" });

/** Laufender Monat bis heute: der Bericht reicht nie in die Zukunft. */
function currentMonthRange(today: Date): DateRange {
  return {
    from: new Date(today.getFullYear(), today.getMonth(), 1),
    to: today,
  };
}

function monthPresets(today: Date) {
  const lastMonthEnd = new Date(today.getFullYear(), today.getMonth(), 0);
  return [
    { label: "Dieser Monat", range: () => currentMonthRange(today) },
    {
      label: "Letzter Monat",
      range: () => ({
        from: new Date(lastMonthEnd.getFullYear(), lastMonthEnd.getMonth(), 1),
        to: lastMonthEnd,
      }),
    },
    ...buildDefaultPresets(
      new Date(today.getFullYear() - 1, today.getMonth(), today.getDate() + 1),
      today,
    ),
  ];
}

const COLUMNS: DataTableColumn<CareTimeDay>[] = [
  {
    key: "date",
    header: "Tag",
    render: (day) => formatStatusDate(day.date),
    sortValue: (day) => day.date,
    stacked: "title",
  },
  {
    key: "planned",
    header: "Gebucht",
    render: (day) => formatClockSpan(day.planned_start, day.planned_end),
  },
  {
    key: "actual",
    header: "Tatsächlich",
    render: (day) => formatClockSpan(day.actual_start, day.actual_end),
  },
  {
    key: "deviation",
    header: "Abweichung",
    align: "right",
    render: (day) => (
      <span className="tabular-nums">
        {formatDeviation(day.deviation_minutes)}
      </span>
    ),
    sortValue: (day) => day.deviation_minutes,
  },
  {
    key: "state",
    header: "Hinweis",
    render: (day) => {
      const { label, tone } = CARE_TIME_DAY_LABELS[day.kind];
      const reason = overrunReason(day);
      return (
        <div className="flex flex-col items-start gap-1">
          <StatusBadge label={label} tone={tone} compact />
          {reason ? (
            <span className="text-xs text-gray-600">{reason}</span>
          ) : null}
        </div>
      );
    },
    sortValue: (day) => (day.overrun ? 0 : 1),
  },
];

export function StudentCareTimesSection({
  studentId,
  active,
}: Readonly<{ studentId: string; active: boolean }>) {
  const todayISO = berlinTodayISO();
  const today = useMemo(() => parseISODate(todayISO), [todayISO]);
  const [range, setRange] = useState<DateRange>(() =>
    currentMonthRange(parseISODate(berlinTodayISO())),
  );
  const [data, setData] = useState<StudentCareTimes | null>(null);
  const [loading, setLoading] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;
  const presets = useMemo(() => monthPresets(today), [today]);

  const fromISO = range.from ? toISODate(range.from) : null;
  const toISO = range.to ? toISODate(range.to) : null;

  useEffect(() => {
    if (!active || !fromISO || !toISO) return;
    let cancelled = false;
    setLoading(true);
    clearLoadError();
    fetchStudentCareTimes(studentId, fromISO, toISO)
      .then((result) => {
        if (!cancelled) setData(result);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setData(null);
        logger.error("student_care_times_fetch_failed", {
          student_id: studentId,
          error: error instanceof Error ? error.message : String(error),
        });
        void showLoadError(error, {
          object: "Soll und Ist",
          retry: () => setReloadKey((key) => key + 1),
        });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [
    active,
    studentId,
    fromISO,
    toISO,
    reloadKey,
    showLoadError,
    clearLoadError,
  ]);

  const summary = data?.summary;
  const tolerance = data?.tolerance_minutes ?? 15;

  return (
    <SectionCard
      title="Soll und Ist"
      description={`Gebucht ist die hinterlegte Betreuungszeit. Tatsächlich ist die Zeit von der ersten Anmeldung bis zur letzten Abmeldung. Mehr als ${tolerance} Minuten früher oder später zählt als Überschreitung, ebenso ein Tag ohne Buchung.`}
      actions={
        <DateRangePicker
          value={range}
          onChange={(next) => {
            if (next?.from && next?.to) setRange(next);
          }}
          presets={presets}
          toMax={today}
        />
      }
    >
      <div className="space-y-4">
        <LoadErrorAlert error={load.error} />
        {summary ? (
          <DataGrid columns={4}>
            <DataField label="Überschreitungen">
              <span
                className="text-lg leading-tight font-semibold tabular-nums"
                style={{
                  color:
                    summary.overrun_count > 0
                      ? getAccessibleTextColor(LOCATION_COLORS.SCHOOLYARD)
                      : undefined,
                }}
              >
                {summary.overrun_count}
              </span>
            </DataField>
            <DataField label="Davon ohne Buchung">
              <span className="text-lg leading-tight font-semibold tabular-nums">
                {summary.unbooked_days}
              </span>
            </DataField>
            <DataField label="Minuten über Buchung">
              <span className="text-lg leading-tight font-semibold tabular-nums">
                {summary.extra_minutes}
              </span>
            </DataField>
            <DataField label="Abweichung gesamt">
              <span className="text-lg leading-tight font-semibold tabular-nums">
                {formatDeviation(summary.deviation_minutes)}
              </span>
            </DataField>
          </DataGrid>
        ) : null}
        <DataTable
          columns={COLUMNS}
          rows={data?.days ?? []}
          getRowKey={(day) => day.date}
          defaultSortKey="date"
          isLoading={loading && data === null}
          pageSize={31}
          paginationResetKey={`${fromISO}-${toISO}`}
          stackedOnMobile
          emptyState={
            <EmptyState
              title="Keine Tage im Zeitraum"
              description="In diesem Zeitraum war keine Betreuung gebucht und das Kind war nicht da."
            />
          }
        />
      </div>
    </SectionCard>
  );
}
