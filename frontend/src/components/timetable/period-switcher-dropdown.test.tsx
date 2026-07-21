import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PeriodSwitcherDropdown } from "./period-switcher-dropdown";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";

const basePeriods: CalendarPeriod[] = [
  {
    id: "1",
    tenantId: "1",
    name: "Schuljahr 2026/2027",
    periodType: "school_year",
    startDate: "2026-08-01",
    endDate: "2027-07-31",
    weekCycleLength: 1,
    weekCycleAnchor: "2026-08-03",
    isActive: true,
    createdAt: "2026-05-01T00:00:00Z",
    updatedAt: "2026-05-01T00:00:00Z",
  },
  {
    id: "2",
    tenantId: "1",
    name: "Sommerferien",
    periodType: "holiday",
    startDate: "2026-07-01",
    endDate: "2026-07-31",
    weekCycleLength: 1,
    weekCycleAnchor: null,
    isActive: false,
    createdAt: "2026-05-01T00:00:00Z",
    updatedAt: "2026-05-01T00:00:00Z",
  },
];

const weekDays = [
  new Date("2026-08-03T00:00:00"),
  new Date("2026-08-04T00:00:00"),
  new Date("2026-08-05T00:00:00"),
  new Date("2026-08-06T00:00:00"),
  new Date("2026-08-07T00:00:00"),
];

describe("PeriodSwitcherDropdown", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-08-05T12:00:00"));
  });

  it("opens create directly when no periods exist", () => {
    const onCreate = vi.fn();

    render(
      <PeriodSwitcherDropdown
        periods={[]}
        weekDays={weekDays}
        onCreate={onCreate}
        onEdit={vi.fn()}
        onSelect={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zeitraum anlegen" }));

    expect(onCreate).toHaveBeenCalledOnce();
  });

  it("shows week assignments and selects or edits periods", () => {
    const onCreate = vi.fn();
    const onEdit = vi.fn();
    const onSelect = vi.fn();

    render(
      <PeriodSwitcherDropdown
        periods={basePeriods}
        weekDays={weekDays}
        onCreate={onCreate}
        onEdit={onEdit}
        onSelect={onSelect}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Schuljahr/ }));

    expect(screen.getByText("Diese Woche")).toBeInTheDocument();
    expect(screen.getAllByText("Schuljahr 2026/2027").length).toBeGreaterThan(
      1,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Schuljahr 2026\/2027 bearbeiten/ }),
    );
    expect(onEdit).toHaveBeenCalledWith(basePeriods[0]);

    fireEvent.click(screen.getByRole("button", { name: /Schuljahr/ }));
    fireEvent.click(screen.getByText("Sommerferien"));
    expect(onSelect).toHaveBeenCalledWith(basePeriods[1]);

    fireEvent.click(screen.getByRole("button", { name: /Schuljahr/ }));
    fireEvent.click(screen.getByRole("button", { name: /Neuen Zeitraum/ }));
    expect(onCreate).toHaveBeenCalledOnce();
  });

  it("uses selected period in series view and shows a loading skeleton", () => {
    const { container } = render(
      <PeriodSwitcherDropdown
        periods={basePeriods}
        weekDays={weekDays}
        view="series"
        selectedPeriodId="2"
        isLoading
        onCreate={vi.fn()}
        onEdit={vi.fn()}
        onSelect={vi.fn()}
      />,
    );

    // Loading now renders a skeleton placeholder (sized like the trigger
    // pill) instead of nothing, so the header keeps its place.
    expect(container.querySelector(".animate-pulse")).not.toBeNull();

    render(
      <PeriodSwitcherDropdown
        periods={basePeriods}
        weekDays={weekDays}
        view="series"
        selectedPeriodId="2"
        onCreate={vi.fn()}
        onEdit={vi.fn()}
        onSelect={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: /Sommerferien/ })).toBeVisible();
  });

  it("labels multi-week cycles with their actual length and slot", () => {
    // Vier-Wochen-Zyklus, Anker Mo 2026-08-03: die Woche ab dem 17.08. ist
    // Slot 3 (Woche C). Die Liste nennt den echten Zyklus statt pauschal A/B.
    const fourWeekPeriod: CalendarPeriod = {
      ...basePeriods[0]!,
      weekCycleLength: 4,
      weekCycleAnchor: "2026-08-03",
    };
    const slotCWeek = [
      new Date("2026-08-17T00:00:00"),
      new Date("2026-08-18T00:00:00"),
      new Date("2026-08-19T00:00:00"),
      new Date("2026-08-20T00:00:00"),
      new Date("2026-08-21T00:00:00"),
    ];

    render(
      <PeriodSwitcherDropdown
        periods={[fourWeekPeriod]}
        weekDays={slotCWeek}
        onCreate={vi.fn()}
        onEdit={vi.fn()}
        onSelect={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Schuljahr/ }));

    // Exakter String-Match trifft nur die inneren Slot-Spans (keine Eltern).
    expect(screen.getAllByText("· Woche C")).toHaveLength(5);
    expect(screen.getAllByText(/A\/B\/C\/D-Wochen/).length).toBeGreaterThan(0);
    expect(screen.queryByText(/· A\/B-Wochen/)).not.toBeInTheDocument();
  });

  it("keeps the A/B labels for two-week cycles", () => {
    const twoWeekPeriod: CalendarPeriod = {
      ...basePeriods[0]!,
      weekCycleLength: 2,
      weekCycleAnchor: "2026-08-03",
    };

    render(
      <PeriodSwitcherDropdown
        periods={[twoWeekPeriod]}
        weekDays={weekDays}
        onCreate={vi.fn()}
        onEdit={vi.fn()}
        onSelect={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Schuljahr/ }));

    expect(screen.getAllByText("· Woche A")).toHaveLength(5);
    expect(screen.getAllByText(/A\/B-Wochen/).length).toBeGreaterThan(0);
  });

  it("summarizes month period coverage for full, missing, and mixed months", () => {
    const fullMonthDays = [
      new Date("2026-08-03T00:00:00"),
      new Date("2026-08-10T00:00:00"),
    ];

    const { rerender } = render(
      <PeriodSwitcherDropdown
        periods={basePeriods}
        weekDays={fullMonthDays}
        view="month"
        onCreate={vi.fn()}
        onEdit={vi.fn()}
        onSelect={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Schuljahr/ }));
    expect(screen.getByText(/Liegt komplett in/)).toBeInTheDocument();

    fireEvent.mouseDown(document.body);
    rerender(
      <PeriodSwitcherDropdown
        periods={basePeriods}
        weekDays={[new Date("2026-06-01T00:00:00")]}
        view="month"
        onCreate={vi.fn()}
        onEdit={vi.fn()}
        onSelect={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Zeitraum anlegen" }));
    expect(
      screen.getByText(/Für diesen Monat ist kein aktiver Zeitraum hinterlegt/),
    ).toBeInTheDocument();

    fireEvent.mouseDown(document.body);
    const schoolYearPeriod = basePeriods[0]!;
    const holidayPeriod = basePeriods[1]!;
    rerender(
      <PeriodSwitcherDropdown
        periods={[schoolYearPeriod, { ...holidayPeriod, isActive: true }]}
        weekDays={[
          new Date("2026-07-31T00:00:00"),
          new Date("2026-08-03T00:00:00"),
        ]}
        view="month"
        selectedPeriodId="1"
        onCreate={vi.fn()}
        onEdit={vi.fn()}
        onSelect={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /Mehrere Zeiträume/ }));
    expect(screen.getByText("Umfasst mehrere Zeiträume.")).toBeInTheDocument();
    expect(screen.getAllByTestId("selected-period-check")).toHaveLength(1);
  });
});
