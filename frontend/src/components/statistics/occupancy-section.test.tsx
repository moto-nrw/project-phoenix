import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OccupancySection } from "./occupancy-section";
import type {
  StatisticsOccupancy,
  StatisticsOccupancyWeekday,
} from "~/lib/statistics-api";

function weekday(
  day: number,
  planned: number,
  actual: number | null,
): StatisticsOccupancyWeekday {
  return {
    weekday: day,
    plan_days: 2,
    actual_days: actual === null ? 0 : 2,
    slots: [
      {
        start: "13:00",
        end: "13:30",
        planned_average: planned,
        planned_max: Math.ceil(planned),
        actual_average: actual,
        actual_max: actual === null ? null : Math.ceil(actual),
      },
    ],
  };
}

function occupancy(
  overrides: Partial<StatisticsOccupancy> = {},
): StatisticsOccupancy {
  return {
    slot_minutes: 30,
    untimed_plan_days: 0,
    actual_data_days: 365,
    actual_data_from: "2025-01-01",
    plan_data_days: 365,
    plan_data_from: "2025-01-01",
    weekdays: [weekday(1, 12, 10)],
    ...overrides,
  };
}

describe("OccupancySection", () => {
  it("zeigt für einen Tag geplant und da je halbe Stunde", () => {
    render(
      <OccupancySection
        occupancy={occupancy()}
        fromISO="2026-09-07"
        toISO="2026-09-07"
      />,
    );
    expect(screen.getByText("13:00–13:30")).toBeVisible();
    expect(screen.getByRole("columnheader", { name: /Geplant/ })).toBeVisible();
    expect(screen.getByText("12")).toBeVisible();
    expect(screen.getByText("10")).toBeVisible();
    // Ein Tag hat keine Durchschnittsspalten und keinen Wochentagswechsel.
    expect(screen.queryByText("Geplant Ø")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Di" }),
    ).not.toBeInTheDocument();
  });

  it("wechselt im Zeitraum zwischen den Wochentagen", () => {
    render(
      <OccupancySection
        occupancy={occupancy({
          weekdays: [weekday(1, 12.5, 10), weekday(2, 4.5, 3)],
        })}
        fromISO="2026-08-31"
        toISO="2026-09-08"
      />,
    );
    expect(screen.getByText("12,5")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Di" }));
    expect(screen.getByText("4,5")).toBeVisible();
    expect(
      screen.getByText("Dienstag: Durchschnitt aus 2 Betreuungstagen"),
    ).toBeVisible();
  });

  it("nennt die Aufbewahrungsfrist und fehlende Uhrzeiten", () => {
    render(
      <OccupancySection
        occupancy={occupancy({
          weekdays: [weekday(1, 5, null)],
          actual_data_days: 30,
          actual_data_from: "2026-08-10",
          untimed_plan_days: 3,
        })}
        fromISO="2026-06-01"
        toISO="2026-06-30"
      />,
    );
    expect(
      screen.getByText(
        "Anwesenheiten werden 30 Tage aufbewahrt. Für diese Tage gibt es keine Zahlen bei „Da“ mehr.",
      ),
    ).toBeVisible();
    expect(
      screen.getByText(
        "3-mal fehlte bei einem Kind die geplante Ankunft oder Abholung. Dann zählt das Kind nicht bei „Geplant“.",
      ),
    ).toBeVisible();
    expect(screen.getAllByText("–").length).toBeGreaterThan(0);
  });

  it("nennt beide Tageszahlen, wenn „Da“ aus weniger Tagen stammt", () => {
    render(
      <OccupancySection
        occupancy={occupancy({
          weekdays: [{ ...weekday(1, 6, 4), plan_days: 3, actual_days: 2 }],
          plan_data_days: 30,
          plan_data_from: "2026-08-10",
        })}
        fromISO="2026-08-01"
        toISO="2026-08-31"
      />,
    );
    expect(
      screen.getByText(
        "Montag: Durchschnitt aus 3 Betreuungstagen, bei „Da“ aus 2",
      ),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Krankmeldungen und Entschuldigungen werden 30 Tage aufbewahrt. „Geplant“ zählt erst ab 10.08.2026.",
      ),
    ).toBeVisible();
  });

  it("erklärt einen Zeitraum ohne Betreuungstag", () => {
    render(
      <OccupancySection
        occupancy={occupancy({ weekdays: [] })}
        fromISO="2026-09-05"
        toISO="2026-09-05"
      />,
    );
    expect(screen.getByText("Kein Betreuungstag im Zeitraum")).toBeVisible();
  });
});
