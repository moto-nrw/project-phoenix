import { schoolDateCases, setSchoolDateClock } from "~/test/school-date-cases";
import { afterEach as restoreZoneAfterEach, vi as zoneVi } from "vitest";
restoreZoneAfterEach(() => zoneVi.unstubAllEnvs());
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("react-day-picker", () => ({
  DayPicker: ({ month }: { month: Date }) => (
    <div
      data-testid="range-day-picker"
      data-month={`${month.getFullYear()}-${month.getMonth() + 1}`}
    />
  ),
}));

import { DateRangePicker, RangeCalendarInline } from "./date-range-picker";

describe("DateRangePicker", () => {
  it("collapses the label when both dates share month and year", () => {
    // Kurz genug für den Chip zwischen den Pfeilen; vorher rutschte auf dem
    // Telefon das Jahr in eine zweite Zeile.
    const { rerender } = render(
      <DateRangePicker
        value={{ from: new Date(2026, 7, 1), to: new Date(2026, 7, 30) }}
        onChange={() => undefined}
      />,
    );
    expect(screen.getByRole("button", { name: /Aug/ })).toHaveTextContent(
      "1.–30. Aug. 2026",
    );

    rerender(
      <DateRangePicker
        value={{ from: new Date(2026, 6, 20), to: new Date(2026, 7, 30) }}
        onChange={() => undefined}
      />,
    );
    expect(screen.getByRole("button", { name: /Aug/ })).toHaveTextContent(
      "20. Juli – 30. Aug. 2026",
    );
  });

  it("shows a single day as one date", () => {
    render(
      <DateRangePicker
        value={{ from: new Date(2026, 9, 6), to: new Date(2026, 9, 6) }}
        onChange={() => undefined}
      />,
    );
    const trigger = screen.getByRole("button", { name: /Okt/ });
    expect(trigger).toHaveTextContent("6. Okt. 2026");
    expect(trigger).not.toHaveTextContent("6.–6.");
  });

  it("keeps its own panel open when another range picker is also open", async () => {
    render(
      <>
        <DateRangePicker value={undefined} onChange={vi.fn()} />
        <DateRangePicker value={undefined} onChange={vi.fn()} />
      </>,
    );

    const triggers = screen.getAllByRole("button", {
      name: /zeitraum wählen/i,
    });
    fireEvent.click(triggers[0]!);
    fireEvent.click(triggers[1]!);

    await waitFor(() => {
      expect(document.querySelectorAll("[data-range-panel]")).toHaveLength(2);
    });

    const panels = document.querySelectorAll("[data-range-panel]");
    fireEvent.mouseDown(panels[1]!);

    expect(document.querySelectorAll("[data-range-panel]")).toHaveLength(1);
    expect(document.querySelector("[data-range-panel]")).toBe(panels[1]!);
  });
});

it.each(schoolDateCases("2026-06-01"))(
  "opens the Berlin month in %s",
  (zone, instant) => {
    setSchoolDateClock(zone, instant, "2026-06-01");
    render(<RangeCalendarInline value={undefined} onChange={vi.fn()} />);
    for (const picker of screen.getAllByTestId("range-day-picker"))
      expect(picker).toHaveAttribute("data-month", "2026-6");
  },
);
