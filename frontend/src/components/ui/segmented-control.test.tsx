import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import { SegmentedControl } from "./segmented-control";

const OPTIONS = [
  { value: "today", label: "Heute" },
  { value: "three-days", label: "3 Tage" },
  { value: "week", label: "1 Woche" },
  { value: "custom", label: "Individuell" },
] as const;

function TestSegmentedControl() {
  const [value, setValue] = useState("today");

  return (
    <SegmentedControl
      options={OPTIONS}
      value={value}
      onChange={setValue}
      ariaLabel="Dauer"
    />
  );
}

describe("SegmentedControl", () => {
  it("uses one tab stop and selects options with radio-group navigation keys", () => {
    render(<TestSegmentedControl />);

    const today = screen.getByRole("radio", { name: "Heute" });
    const threeDays = screen.getByRole("radio", { name: "3 Tage" });
    const custom = screen.getByRole("radio", { name: "Individuell" });

    expect(today).toHaveAttribute("tabIndex", "0");
    expect(threeDays).toHaveAttribute("tabIndex", "-1");
    expect(custom).toHaveAttribute("tabIndex", "-1");

    today.focus();
    expect(fireEvent.keyDown(today, { key: "ArrowRight" })).toBe(false);
    expect(threeDays).toHaveFocus();
    expect(threeDays).toHaveAttribute("aria-checked", "true");
    expect(threeDays).toHaveAttribute("tabIndex", "0");

    fireEvent.keyDown(threeDays, { key: "End" });
    expect(custom).toHaveFocus();
    expect(custom).toHaveAttribute("aria-checked", "true");

    fireEvent.keyDown(custom, { key: "Home" });
    expect(today).toHaveFocus();
    expect(today).toHaveAttribute("aria-checked", "true");

    fireEvent.keyDown(today, { key: "ArrowLeft" });
    expect(custom).toHaveFocus();
    expect(custom).toHaveAttribute("aria-checked", "true");

    fireEvent.keyDown(custom, { key: "ArrowDown" });
    expect(today).toHaveFocus();
    expect(today).toHaveAttribute("aria-checked", "true");

    fireEvent.keyDown(today, { key: "ArrowUp" });
    expect(custom).toHaveFocus();
    expect(custom).toHaveAttribute("aria-checked", "true");
  });
});
