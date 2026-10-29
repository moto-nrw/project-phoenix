"use client";

// Belegung nach Uhrzeit (#4764): wie viele Kinder in jeder halben Stunde da
// sind, geplant und tatsächlich. Ein Tag zeigt seine zwei Zahlen je halbe
// Stunde, ein Zeitraum Durchschnitt und Höchstwert je Wochentag. Die Seite
// lädt die Zahlen; diese Karte zeigt sie nur an und ändert nichts.

import { useMemo, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { formatDate } from "~/lib/date-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import type {
  StatisticsOccupancy,
  StatisticsOccupancySlot,
} from "~/lib/statistics-api";

const WEEKDAY_LABELS: Readonly<Record<number, string>> = {
  1: "Montag",
  2: "Dienstag",
  3: "Mittwoch",
  4: "Donnerstag",
  5: "Freitag",
  6: "Samstag",
  7: "Sonntag",
};

const WEEKDAY_SHORT: Readonly<Record<number, string>> = {
  1: "Mo",
  2: "Di",
  3: "Mi",
  4: "Do",
  5: "Fr",
  6: "Sa",
  7: "So",
};

/** "07:30" plus Rasterbreite → "07:30–08:00". */
function slotRange(start: string, minutes: number): string {
  const [h = 0, m = 0] = start.split(":").map(Number);
  const end = h * 60 + m + minutes;
  const hh = String(Math.floor(end / 60)).padStart(2, "0");
  const mm = String(end % 60).padStart(2, "0");
  return `${start}–${hh}:${mm}`;
}

function formatAverage(value: number): string {
  return value.toLocaleString("de-DE", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  });
}

/**
 * Zahl mit schmalem Balken. Der Balken ist reine Anzeige: er macht die
 * vollsten halben Stunden auf einen Blick sichtbar.
 */
function CountBar({
  value,
  label,
  max,
  color,
}: Readonly<{ value: number; label: string; max: number; color: string }>) {
  const width = max > 0 ? Math.round((value / max) * 100) : 0;
  return (
    <div className="flex items-center justify-end gap-3">
      <span className="tabular-nums">{label}</span>
      <span
        aria-hidden
        className="hidden h-2 w-24 overflow-hidden rounded-full bg-gray-100 sm:block"
      >
        <span
          className="block h-full rounded-full"
          style={{ width: `${width}%`, backgroundColor: color }}
        />
      </span>
    </div>
  );
}

export function OccupancySection({
  occupancy,
  fromISO,
  toISO,
}: Readonly<{
  occupancy: StatisticsOccupancy;
  fromISO: string;
  toISO: string;
}>) {
  const singleDay = fromISO === toISO;
  const weekdays = occupancy.weekdays;
  const [chosen, setChosen] = useState<number | null>(null);
  // Fällt der gewählte Wochentag im neuen Zeitraum weg, gilt der erste.
  const current =
    weekdays.find((day) => day.weekday === chosen) ?? weekdays[0] ?? null;

  const peak = useMemo(() => {
    let value = 0;
    for (const slot of current?.slots ?? []) {
      value = Math.max(value, slot.planned_max, slot.actual_max ?? 0);
    }
    return value;
  }, [current]);

  if (!current) {
    return (
      <EmptyState
        title="Kein Betreuungstag im Zeitraum"
        description="An Wochenenden, Feiertagen, Schließtagen und in den Ferien gibt es keine Zahlen. Wählen Sie einen anderen Zeitraum."
      />
    );
  }

  const noActual = current.actual_days === 0;
  const range = (row: StatisticsOccupancySlot) =>
    slotRange(row.start, occupancy.slot_minutes);
  const plannedColor = LOCATION_COLORS.OTHER_ROOM;
  const actualColor = LOCATION_COLORS.GROUP_ROOM;

  const columns: DataTableColumn<StatisticsOccupancySlot>[] = singleDay
    ? [
        {
          key: "slot",
          header: "Uhrzeit",
          render: (row) => (
            <span className="font-medium whitespace-nowrap text-gray-900 tabular-nums">
              {range(row)}
            </span>
          ),
        },
        {
          key: "planned",
          header: "Geplant",
          align: "right",
          render: (row) => (
            <CountBar
              value={row.planned_max}
              label={String(row.planned_max)}
              max={peak}
              color={plannedColor}
            />
          ),
        },
        {
          key: "actual",
          header: "Da",
          align: "right",
          render: (row) =>
            row.actual_max === null ? (
              "–"
            ) : (
              <CountBar
                value={row.actual_max}
                label={String(row.actual_max)}
                max={peak}
                color={actualColor}
              />
            ),
        },
      ]
    : [
        {
          key: "slot",
          header: "Uhrzeit",
          render: (row) => (
            <span className="font-medium whitespace-nowrap text-gray-900 tabular-nums">
              {range(row)}
            </span>
          ),
        },
        {
          key: "planned-average",
          header: "Geplant Ø",
          align: "right",
          render: (row) => (
            <CountBar
              value={row.planned_average}
              label={formatAverage(row.planned_average)}
              max={peak}
              color={plannedColor}
            />
          ),
        },
        {
          key: "planned-max",
          header: "Geplant höchstens",
          align: "right",
          render: (row) => (
            <span className="tabular-nums">{row.planned_max}</span>
          ),
        },
        {
          key: "actual-average",
          header: "Da Ø",
          align: "right",
          render: (row) =>
            row.actual_average === null ? (
              "–"
            ) : (
              <CountBar
                value={row.actual_average}
                label={formatAverage(row.actual_average)}
                max={peak}
                color={actualColor}
              />
            ),
        },
        {
          key: "actual-max",
          header: "Da höchstens",
          align: "right",
          render: (row) => (
            <span className="tabular-nums">{row.actual_max ?? "–"}</span>
          ),
        },
      ];

  const actualStartsInsideWindow = occupancy.actual_data_from > fromISO;

  return (
    <div className="space-y-3">
      {!singleDay && weekdays.length > 1 && (
        // Sieben Tage passen auf dem Telefon nicht nebeneinander; die Leiste
        // scrollt dann waagerecht wie die Tabelle darunter.
        <div className="max-w-full overflow-x-auto">
          <SegmentedControl
            items={weekdays.map((day) => ({
              value: String(day.weekday),
              label: WEEKDAY_SHORT[day.weekday] ?? String(day.weekday),
            }))}
            value={String(current.weekday)}
            onChange={(next) => setChosen(Number(next))}
            variant="pills"
            ariaLabel="Wochentag wählen"
          />
        </div>
      )}
      {!singleDay && (
        <p className="text-sm text-gray-600">
          {WEEKDAY_LABELS[current.weekday] ?? ""}: Durchschnitt aus{" "}
          {current.plan_days} Betreuungstagen
        </p>
      )}
      {actualStartsInsideWindow && (
        <Alert
          type="info"
          message={
            noActual
              ? `Anwesenheiten werden ${occupancy.actual_data_days} Tage aufbewahrt. Für diese Tage gibt es nur noch die geplanten Zahlen.`
              : `Anwesenheiten werden ${occupancy.actual_data_days} Tage aufbewahrt. „Da“ zählt erst ab ${formatDate(occupancy.actual_data_from)}.`
          }
        />
      )}
      {occupancy.untimed_plan_days > 0 && (
        <Alert
          type="info"
          message={`${occupancy.untimed_plan_days}-mal fehlte bei einem Kind die geplante Ankunft oder Abholung. Dann zählt das Kind nicht bei „Geplant“.`}
        />
      )}
      <DataTable
        columns={columns}
        rows={current.slots}
        getRowKey={(row) => row.start}
      />
    </div>
  );
}
