import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import { PlanLegend } from "./plan-legend";

describe("PlanLegend", () => {
  it("renders each entry label in the muted legend text style", () => {
    render(
      <PlanLegend
        entries={[
          { key: "care", label: "Arbeit am Kind", color: "#5A8E1F" },
          { key: "prep", label: "Vorbereitung", color: "#4A6FA5" },
        ]}
      />,
    );

    const label = screen.getByText("Arbeit am Kind");
    expect(label).toHaveClass("text-[11px]", "text-gray-600");
    expect(screen.getByText("Vorbereitung")).toBeInTheDocument();
  });

  it("renders a colored edge-bar swatch from the hex color prop", () => {
    const { container } = render(
      <PlanLegend
        entries={[{ key: "care", label: "Arbeit am Kind", color: "#5A8E1F" }]}
      />,
    );

    const swatch = container.querySelector('[aria-hidden="true"]');
    expect(swatch).toHaveStyle({ backgroundColor: "#5A8E1F" });
  });

  it("falls back to the neutral swatch color when no color is given", () => {
    const { container } = render(
      <PlanLegend entries={[{ key: "untyped", label: "Ohne Schichtart" }]} />,
    );

    const swatch = container.querySelector('[aria-hidden="true"]');
    expect(swatch).toHaveStyle({ backgroundColor: "#D1D5DB" });
  });

  it("falls back to the neutral swatch color for an invalid color", () => {
    const { container } = render(
      <PlanLegend
        entries={[{ key: "invalid", label: "Ungültig", color: "red" }]}
      />,
    );

    const swatch = container.querySelector('[aria-hidden="true"]');
    expect(swatch).toHaveStyle({ backgroundColor: "#D1D5DB" });
  });

  it("renders state entries as an SVG glyph, not a gradient", () => {
    const { container } = render(
      <PlanLegend
        entries={[
          { key: "absent", label: "Abwesend", variant: "hatched" },
          { key: "cancelled", label: "Fällt aus", variant: "cancelled" },
        ]}
      />,
    );

    expect(container.querySelectorAll("svg")).toHaveLength(2);
    expect(container.innerHTML).not.toContain("repeating-linear-gradient");
    expect(screen.getByText("Abwesend")).toBeInTheDocument();
    expect(screen.getByText("Fällt aus")).toBeInTheDocument();
  });
});
