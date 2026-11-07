import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { WeekdayToggleGroup } from "./weekday-toggle-group";

const DAYS = [
  { value: 1, label: "Mo" },
  { value: 2, label: "Di" },
  { value: 3, label: "Mi", ariaLabel: "Mittwoch" },
];

describe("WeekdayToggleGroup", () => {
  it("names the group and shows which days are on", () => {
    render(
      <WeekdayToggleGroup
        ariaLabel="Wochentage"
        days={DAYS}
        value={[2]}
        onToggle={vi.fn()}
      />,
    );

    const group = within(screen.getByRole("group", { name: "Wochentage" }));
    expect(
      group.getByRole("button", { name: "Mo", pressed: false }),
    ).toBeTruthy();
    expect(
      group.getByRole("button", { name: "Di", pressed: true }),
    ).toBeTruthy();
  });

  it("reports the toggled day in its own value format", () => {
    const onToggle = vi.fn();
    render(
      <WeekdayToggleGroup
        ariaLabel="Wochentage"
        days={DAYS}
        value={[2]}
        onToggle={onToggle}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Di" }));
    expect(onToggle).toHaveBeenCalledWith(2);
  });

  it("reads the long name of a day that has one", () => {
    render(
      <WeekdayToggleGroup
        ariaLabel="Wochentage"
        days={DAYS}
        value={[]}
        onToggle={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Mittwoch" })).toHaveTextContent(
      "Mi",
    );
  });

  it("does not react while disabled", () => {
    const onToggle = vi.fn();
    render(
      <WeekdayToggleGroup
        ariaLabel="Wochentage"
        days={DAYS}
        value={[]}
        onToggle={onToggle}
        disabled
      />,
    );

    const monday = screen.getByRole("button", { name: "Mo" });
    expect(monday).toBeDisabled();
    fireEvent.click(monday);
    expect(onToggle).not.toHaveBeenCalled();
  });
});
