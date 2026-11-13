import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { OccupancyBadges } from "./occupancy-badges";

describe("OccupancyBadges", () => {
  it("shows count and limit without a warning below the limit", () => {
    render(<OccupancyBadges occupancy={{ count: 44, limit: 45 }} nfcEnabled />);
    expect(screen.getByText("44 / 45 Kinder")).toBeInTheDocument();
    expect(screen.queryByText("Überbucht")).not.toBeInTheDocument();
  });

  it("does not flag a full activity as overbooked", () => {
    render(<OccupancyBadges occupancy={{ count: 45, limit: 45 }} nfcEnabled />);
    expect(screen.getByText("45 / 45 Kinder")).toBeInTheDocument();
    expect(screen.queryByText("Überbucht")).not.toBeInTheDocument();
  });

  it("names an overbooked activity in text, with the hint on the badge", () => {
    render(
      <OccupancyBadges
        occupancy={{ count: 66, limit: 45 }}
        nfcEnabled={false}
      />,
    );
    expect(screen.getByText("66 / 45 Kinder")).toBeInTheDocument();
    expect(screen.getByText("Überbucht")).toHaveAttribute(
      "title",
      "Mehr Kinder als erlaubt (höchstens 45).",
    );
  });

  it("shows only the count without a limit", () => {
    render(
      <OccupancyBadges occupancy={{ count: 66, limit: null }} nfcEnabled />,
    );
    expect(screen.getByText("66 Kinder")).toBeInTheDocument();
    expect(screen.queryByText("Überbucht")).not.toBeInTheDocument();
  });
});
