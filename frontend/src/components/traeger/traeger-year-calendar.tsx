"use client";

/**
 * Jahresübersicht der Schließtage und Feiertage des Trägers (#4702).
 *
 * Zwölf kleine Monatsblätter zeigen auf einen Blick, wann die Schulen des
 * Trägers zu haben. Der Kit hat Datumsauswahlen, aber keine Jahresansicht,
 * die nur liest; deshalb gibt es diese Komponente. Sie ist reine Anzeige:
 * kein Tag ist anklickbar, der Tooltip und der vorgelesene Text nennen,
 * was an dem Tag ist.
 */

import { useMemo } from "react";

import { PlanLegend } from "~/components/ui/planning/plan-legend";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { cn } from "~/lib/utils";
import type { CalendarDayMark } from "./closing-day-format";

const WEEKDAYS = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"] as const;

const MONTH_FORMAT = new Intl.DateTimeFormat("de-DE", { month: "long" });
const DAY_FORMAT = new Intl.DateTimeFormat("de-DE", {
  weekday: "long",
  day: "numeric",
  month: "long",
});

function pad(value: number): string {
  return String(value).padStart(2, "0");
}

interface MonthSheet {
  readonly month: number;
  readonly name: string;
  /** Leere Felder vor dem 1., Montag zuerst. */
  readonly leading: number;
  readonly days: readonly {
    readonly iso: string;
    readonly day: number;
    readonly weekend: boolean;
    readonly label: string;
  }[];
}

function monthSheets(year: number): MonthSheet[] {
  return Array.from({ length: 12 }, (_, month) => {
    const first = new Date(year, month, 1);
    const length = new Date(year, month + 1, 0).getDate();
    return {
      month,
      name: MONTH_FORMAT.format(first),
      leading: (first.getDay() + 6) % 7,
      days: Array.from({ length }, (_, index) => {
        const date = new Date(year, month, index + 1);
        const weekday = date.getDay();
        return {
          iso: `${year}-${pad(month + 1)}-${pad(index + 1)}`,
          day: index + 1,
          weekend: weekday === 0 || weekday === 6,
          label: DAY_FORMAT.format(date),
        };
      }),
    };
  });
}

export function TraegerYearCalendar({
  year,
  marks,
  todayISO,
}: Readonly<{
  year: number;
  marks: ReadonlyMap<string, CalendarDayMark>;
  todayISO: string;
}>) {
  const sheets = useMemo(() => monthSheets(year), [year]);
  return (
    <div className="space-y-4">
      <PlanLegend
        size="md"
        aria-label="Legende"
        entries={[
          {
            key: "holiday",
            label: "Gesetzlicher Feiertag",
            color: LOCATION_COLORS.OTHER_ROOM,
            variant: "dot",
          },
          {
            key: "closing",
            label: "Schließtag des Trägers",
            color: LOCATION_COLORS.SCHOOLYARD,
            variant: "dot",
          },
        ]}
      />
      <div className="grid grid-cols-1 gap-x-6 gap-y-5 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
        {sheets.map((sheet) => (
          <section key={sheet.month} aria-label={`${sheet.name} ${year}`}>
            <h3 className="mb-2 text-sm font-semibold text-gray-900">
              {sheet.name}
            </h3>
            <div
              className="grid grid-cols-7 gap-1 text-center"
              role="presentation"
            >
              {WEEKDAYS.map((weekday) => (
                <span
                  key={weekday}
                  aria-hidden
                  className="text-xs font-medium text-gray-400"
                >
                  {weekday}
                </span>
              ))}
              {Array.from({ length: sheet.leading }, (_, index) => (
                <span key={`leer-${index}`} aria-hidden />
              ))}
              {sheet.days.map((day) => {
                const mark = marks.get(day.iso);
                const label = mark
                  ? `${day.label}: ${mark.labels.join(", ")}`
                  : undefined;
                return (
                  <span
                    key={day.iso}
                    title={label}
                    aria-label={label}
                    aria-hidden={label ? undefined : true}
                    role={label ? "img" : undefined}
                    className={cn(
                      "flex h-7 items-center justify-center rounded-md text-xs tabular-nums",
                      !mark && day.weekend && "bg-gray-50 text-gray-400",
                      !mark && !day.weekend && "text-gray-700",
                      mark?.kind === "holiday" &&
                        "bg-moto-blue-soft text-moto-blue-strong font-semibold",
                      mark?.kind === "closing" &&
                        "bg-moto-orange-soft text-moto-orange-strong font-semibold",
                      day.iso === todayISO && "ring-1 ring-gray-900",
                    )}
                  >
                    {day.day}
                  </span>
                );
              })}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}
