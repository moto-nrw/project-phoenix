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

    fireEvent.click(screen.getByRole("button", { name: "Periode anlegen" }));

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
    fireEvent.click(screen.getByRole("button", { name: /Neue Periode/ }));
    expect(onCreate).toHaveBeenCalledOnce();
  });

  it("uses selected period in series view and hides loading state", () => {
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

    expect(container).toBeEmptyDOMElement();

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
});
