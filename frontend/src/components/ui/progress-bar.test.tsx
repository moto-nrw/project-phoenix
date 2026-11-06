import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import { ProgressBar } from "./progress-bar";

describe("ProgressBar", () => {
  it("announces its label and value", () => {
    render(
      <ProgressBar value={3} max={6} label="3 von 6 Schritten erledigt" />,
    );

    const bar = screen.getByRole("progressbar", {
      name: "3 von 6 Schritten erledigt",
    });
    expect(bar).toHaveAttribute("aria-valuenow", "3");
    expect(bar).toHaveAttribute("aria-valuemax", "6");
  });

  it("keeps the announced value inside the range", () => {
    render(<ProgressBar value={120} max={100} label="Stunden" />);

    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "100",
    );
  });

  it("fills in proportion to the value", () => {
    render(<ProgressBar value={1} max={4} label="Schritte" />);

    expect(screen.getByRole("progressbar").firstElementChild).toHaveStyle({
      width: "25%",
    });
  });

  it("uses a colour from the data over the tone", () => {
    render(
      <ProgressBar
        value={1}
        max={4}
        label="Belegung"
        tone="gray"
        color="#5080D8"
      />,
    );

    expect(screen.getByRole("progressbar").firstElementChild).toHaveStyle({
      backgroundColor: "#5080D8",
    });
  });
});
