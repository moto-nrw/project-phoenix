"use client";

import { useMemo } from "react";

import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { SectionCard } from "~/components/ui/section-card";
import { formatPlannedHours, type StaffShift } from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";
import { sumsByType } from "~/components/staff/dienstplan-person-week-grid";

// Wochensummen je Schichtart der eigenen Woche (#3821). Das Raster zeigt die
// Stunden je Tag; hier steht, was über die ganze Woche je Art zusammenkommt.
// Dieselbe Rechnung wie die Tagessummen im Raster (sumsByType), damit beide
// Zahlen nie auseinanderlaufen.

const TOTAL_KEY = "__total__";

interface HoursRow {
  readonly key: string;
  readonly label: string;
  readonly color: string | undefined;
  readonly minutes: number;
}

interface OwnWeekHoursCardProps {
  /** Alle Tage der Woche (Mo–So), auch die, die das Raster nicht zeigt. */
  readonly weekDays: readonly string[];
  readonly shiftsByDate: ReadonlyMap<string, readonly StaffShift[]>;
  readonly typesById: Map<string, ShiftType>;
  readonly shiftTypes: readonly ShiftType[];
}

export function OwnWeekHoursCard({
  weekDays,
  shiftsByDate,
  typesById,
  shiftTypes,
}: OwnWeekHoursCardProps) {
  const rows = useMemo<HoursRow[]>(() => {
    const sums = sumsByType(weekDays, shiftsByDate, typesById, shiftTypes);
    if (sums.rows.length === 0) return [];
    return [
      ...sums.rows.map((row) => ({
        key: row.key,
        label: row.label,
        color: row.color,
        minutes: row.total,
      })),
      {
        key: TOTAL_KEY,
        label: "Gesamt",
        color: undefined,
        minutes: sums.weekTotal,
      },
    ];
  }, [weekDays, shiftsByDate, typesById, shiftTypes]);

  const columns = useMemo<DataTableColumn<HoursRow>[]>(
    () => [
      {
        key: "type",
        header: "Schichtart",
        render: (row) =>
          row.key === TOTAL_KEY ? (
            <span className="font-semibold text-gray-900">{row.label}</span>
          ) : (
            <span className="inline-flex items-center gap-2 text-gray-900">
              <span
                aria-hidden
                className={`h-3.5 w-1 shrink-0 rounded-full ${
                  row.color ? "" : "bg-gray-300"
                }`}
                style={row.color ? { backgroundColor: row.color } : undefined}
              />
              {row.label}
            </span>
          ),
      },
      {
        key: "hours",
        header: "Stunden",
        align: "right",
        className: "tabular-nums",
        render: (row) => (
          <span
            className={
              row.key === TOTAL_KEY ? "font-semibold text-gray-900" : undefined
            }
          >
            {formatPlannedHours(row.minutes)}
          </span>
        ),
      },
    ],
    [],
  );

  if (rows.length === 0) return null;

  return (
    <SectionCard
      title="Stunden der Woche"
      description="Geplante Stunden je Schichtart, ohne Pausen. Ausgefallene Dienste zählen nicht."
      testId="own-week-hours-card"
    >
      <DataTable columns={columns} rows={rows} getRowKey={(row) => row.key} />
    </SectionCard>
  );
}
