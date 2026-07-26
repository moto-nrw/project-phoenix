import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("react-day-picker", () => ({
  DayPicker: () => <div data-testid="range-day-picker" />,
}));

import { DateRangePicker } from "./date-range-picker";

describe("DateRangePicker", () => {
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
