import { fireEvent, render, screen } from "@testing-library/react";
import { beforeAll, expect, it, vi } from "vitest";
import { preloadDayPicker } from "./lazy-day-picker";
import { RangeCalendarInline } from "./date-range-picker";

beforeAll(async () => {
  await preloadDayPicker();
});

const initialRange = {
  from: new Date(2026, 8, 1),
  to: new Date(2026, 8, 3),
};

it("replaces the accessible selection with the new two-click range", async () => {
  const onChange = vi.fn();
  render(<RangeCalendarInline value={initialRange} onChange={onChange} />);
  const day = (date: number) =>
    screen
      .getByRole("button", {
        name: new RegExp(`, ${date}\\. September 2026`),
      })
      .closest('[role="gridcell"]');
  await screen.findByRole("button", { name: /, 1\. September 2026/ });
  expect(day(1)).toHaveAttribute("aria-selected", "true");

  fireEvent.click(
    screen.getByRole("button", { name: /, 10\. September 2026/ }),
  );
  expect(screen.getByText("Enddatum wählen")).toBeInTheDocument();
  expect(onChange).not.toHaveBeenCalled();
  expect(day(1)).not.toHaveAttribute("aria-selected", "true");
  expect(day(10)).toHaveAttribute("aria-selected", "true");

  // Picking the earlier end still sorts the range and selects its middle.
  fireEvent.click(screen.getByRole("button", { name: /, 8\. September 2026/ }));
  expect(onChange).toHaveBeenCalledExactlyOnceWith({
    from: new Date(2026, 8, 8),
    to: new Date(2026, 8, 10),
  });
  for (const date of [8, 9, 10]) {
    expect(day(date)).toHaveAttribute("aria-selected", "true");
  }
});

it("updates the accessible selection when an inline preset is applied", async () => {
  const onChange = vi.fn();
  render(
    <RangeCalendarInline
      value={initialRange}
      onChange={onChange}
      presets={[
        {
          label: "Andere Woche",
          range: () => ({
            from: new Date(2026, 8, 14),
            to: new Date(2026, 8, 16),
          }),
        },
      ]}
    />,
  );
  const oldDay = (
    await screen.findByRole("button", {
      name: /, 1\. September 2026/,
    })
  ).closest('[role="gridcell"]');
  expect(oldDay).toHaveAttribute("aria-selected", "true");

  fireEvent.click(screen.getByRole("button", { name: "Andere Woche" }));

  expect(onChange).toHaveBeenCalledExactlyOnceWith({
    from: new Date(2026, 8, 14),
    to: new Date(2026, 8, 16),
  });
  expect(oldDay).not.toHaveAttribute("aria-selected", "true");
  for (const date of [14, 15, 16]) {
    expect(
      screen
        .getByRole("button", {
          name: new RegExp(`, ${date}\\. September 2026`),
        })
        .closest('[role="gridcell"]'),
    ).toHaveAttribute("aria-selected", "true");
  }
});
