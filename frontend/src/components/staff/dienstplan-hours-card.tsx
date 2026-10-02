"use client";

import { useMemo } from "react";

import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { SectionCard } from "~/components/ui/section-card";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import {
  formatDeltaHours,
  formatPlannedHours,
  summaryTone,
  type StaffScheduleStaff,
  type StaffWeeklySummary,
} from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

// Stunden je Schichtart pro Person und Woche (#3819): die Liste, die Schulen
// neben dem Dienstplan von Hand führen. Die Zahlen sind dieselben
// Wochensummen wie im Zeilenkopf des Rasters ("18/20 h"), nur nach
// Schichtart aufgeteilt. Wochenend-Dienste derselben Woche zählen mit.

const UNTYPED_KEY = "untyped";
const UNTYPED_LABEL = "Ohne Schichtart";
const UNKNOWN_TYPE_LABEL = "Unbekannte Schichtart";
const EMPTY_VALUE = "–";

const DELTA_COLOR = {
  under: MOTO_COLOR_PALETTE.red.base,
  over: MOTO_COLOR_PALETTE.amber.base,
} as const;

interface HoursRow {
  readonly member: StaffScheduleStaff;
  readonly summary: StaffWeeklySummary;
  /** Schichtart key ("untyped" for shifts without one) -> minutes. */
  readonly minutesByType: ReadonlyMap<string, number>;
}

interface TypeColumn {
  readonly key: string;
  readonly label: string;
}

interface DienstplanHoursCardProps {
  /** Staff in grid order; only people with a summary this week get a row. */
  readonly staff: readonly StaffScheduleStaff[];
  readonly summaryByStaff: ReadonlyMap<string, StaffWeeklySummary>;
  /** Every Schichtart of the school, in the order the legend uses. */
  readonly shiftTypes: readonly ShiftType[];
}

function typeKey(shiftTypeId: string | null): string {
  return shiftTypeId ?? UNTYPED_KEY;
}

// Eine Spalte je Schichtart, die in der Woche vorkommt: in der Reihenfolge
// der Legende, Unbekannte danach, "Ohne Schichtart" zuletzt. Leere Spalten
// für ungenutzte Schichtarten kosten auf dem Telefon nur Platz.
function typeColumnsFor(
  rows: readonly HoursRow[],
  shiftTypes: readonly ShiftType[],
): TypeColumn[] {
  const used = new Set<string>();
  for (const row of rows) {
    for (const key of row.minutesByType.keys()) used.add(key);
  }
  const columns: TypeColumn[] = [];
  for (const type of shiftTypes) {
    if (used.delete(type.id)) columns.push({ key: type.id, label: type.name });
  }
  const hasUntyped = used.delete(UNTYPED_KEY);
  for (const key of used) {
    columns.push({ key, label: UNKNOWN_TYPE_LABEL });
  }
  if (hasUntyped) columns.push({ key: UNTYPED_KEY, label: UNTYPED_LABEL });
  return columns;
}

function DeltaValue({ summary }: { readonly summary: StaffWeeklySummary }) {
  if (summary.deltaMinutes === null) return <>{EMPTY_VALUE}</>;
  const tone = summaryTone(summary);
  return (
    <span
      className="font-medium"
      style={tone === "neutral" ? undefined : { color: DELTA_COLOR[tone] }}
    >
      {formatDeltaHours(summary.deltaMinutes)}
    </span>
  );
}

export function DienstplanHoursCard({
  staff,
  summaryByStaff,
  shiftTypes,
}: DienstplanHoursCardProps) {
  const rows = useMemo<HoursRow[]>(() => {
    const out: HoursRow[] = [];
    for (const member of staff) {
      const summary = summaryByStaff.get(member.id);
      if (!summary) continue;
      const minutesByType = new Map<string, number>();
      for (const entry of summary.plannedByShiftType) {
        const key = typeKey(entry.shiftTypeId);
        minutesByType.set(
          key,
          (minutesByType.get(key) ?? 0) + entry.plannedMinutes,
        );
      }
      out.push({ member, summary, minutesByType });
    }
    return out;
  }, [staff, summaryByStaff]);

  const columns = useMemo<DataTableColumn<HoursRow>[]>(() => {
    const typeColumns = typeColumnsFor(rows, shiftTypes);
    return [
      {
        key: "person",
        header: "Person",
        stacked: "title",
        sortValue: (row) => `${row.member.lastName}, ${row.member.firstName}`,
        render: (row) => (
          <span className="font-medium text-gray-900">
            {row.member.lastName}, {row.member.firstName}
          </span>
        ),
      },
      ...typeColumns.map((column): DataTableColumn<HoursRow> => ({
        key: `type-${column.key}`,
        header: column.label,
        align: "right",
        className: "tabular-nums",
        render: (row) => {
          const minutes = row.minutesByType.get(column.key) ?? 0;
          return minutes > 0 ? formatPlannedHours(minutes) : EMPTY_VALUE;
        },
      })),
      {
        key: "total",
        header: "Gesamt",
        align: "right",
        stacked: "meta",
        className: "tabular-nums font-semibold text-gray-900",
        sortValue: (row) => row.summary.plannedMinutes,
        render: (row) => formatPlannedHours(row.summary.plannedMinutes),
      },
      {
        key: "target",
        header: "Soll",
        align: "right",
        className: "tabular-nums",
        render: (row) =>
          row.summary.targetMinutes === null
            ? EMPTY_VALUE
            : formatPlannedHours(row.summary.targetMinutes),
      },
      {
        key: "delta",
        header: "Differenz",
        align: "right",
        className: "tabular-nums",
        // Ohne Soll gibt es keine Differenz; solche Zeilen stehen am Ende.
        sortValue: (row) =>
          row.summary.deltaMinutes ?? Number.POSITIVE_INFINITY,
        render: (row) => <DeltaValue summary={row.summary} />,
      },
    ];
  }, [rows, shiftTypes]);

  if (rows.length === 0) return null;

  return (
    <SectionCard
      title="Stunden der Woche"
      description="Geplante Stunden je Schichtart, ohne Pausen. Ausgefallene Dienste zählen nicht. Das Soll kommt aus dem Arbeitszeitmodell."
      testId="dienstplan-hours-card"
    >
      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(row) => row.member.id}
        stackedOnMobile
      />
    </SectionCard>
  );
}
