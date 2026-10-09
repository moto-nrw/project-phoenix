import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import WeekChart from "./week-chart";

describe("WeekChart", () => {
  // A failed history load must not draw an empty week: zero bars would read
  // as a week without work (#2514).
  it("shows a failed load in the card instead of the chart", () => {
    const { container } = render(
      <WeekChart
        history={[]}
        weekOffset={0}
        error="Die Wochenübersicht ist gerade nicht erreichbar."
      />,
    );

    expect(screen.getByText("Wochenübersicht")).toBeInTheDocument();
    expect(
      screen.getByText("Die Wochenübersicht ist gerade nicht erreichbar."),
    ).toBeInTheDocument();
    expect(container.querySelector("[data-chart]")).toBeNull();
  });

  it("draws the chart when the history loaded", () => {
    const { container } = render(<WeekChart history={[]} weekOffset={0} />);

    expect(container.querySelector("[data-chart]")).not.toBeNull();
  });
});
