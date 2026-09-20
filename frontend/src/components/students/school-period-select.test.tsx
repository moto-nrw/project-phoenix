import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { SchoolPeriodSelect } from "./school-period-select";

const periods = [
  { period: 5, end_time: "12:35" },
  { period: 6, end_time: "13:20" },
];

describe("SchoolPeriodSelect", () => {
  it("renders nothing while the school maintains no lesson end times", () => {
    const { container } = render(
      <SchoolPeriodSelect
        id="period"
        periods={[]}
        time=""
        ariaLabel="Montag: Ankunft nach Schulstunde"
        onSelect={vi.fn()}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("hands over the end time of the chosen lesson", () => {
    const onSelect = vi.fn();
    render(
      <SchoolPeriodSelect
        id="period"
        periods={periods}
        time=""
        ariaLabel="Montag: Ankunft nach Schulstunde"
        onSelect={onSelect}
      />,
    );

    const trigger = screen.getByRole("combobox", {
      name: "Montag: Ankunft nach Schulstunde",
    });
    expect(trigger).toHaveTextContent("Nach Schulstunde");

    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("option", { name: "6. Stunde (13:20)" }));

    expect(onSelect).toHaveBeenCalledWith("13:20");
  });

  it("shows the lesson whose end time is in the time field", () => {
    render(
      <SchoolPeriodSelect
        id="period"
        periods={periods}
        time="12:35"
        ariaLabel="Montag: Ankunft nach Schulstunde"
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByRole("combobox")).toHaveTextContent("5. Stunde (12:35)");
  });

  it("falls back to the placeholder for a time typed by hand", () => {
    render(
      <SchoolPeriodSelect
        id="period"
        periods={periods}
        time="12:50"
        ariaLabel="Montag: Ankunft nach Schulstunde"
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByRole("combobox")).toHaveTextContent("Nach Schulstunde");
  });

  it("is locked together with the time field", () => {
    render(
      <SchoolPeriodSelect
        id="period"
        periods={periods}
        time=""
        ariaLabel="Montag: Ankunft nach Schulstunde"
        disabled
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByRole("combobox")).toBeDisabled();
  });
});
