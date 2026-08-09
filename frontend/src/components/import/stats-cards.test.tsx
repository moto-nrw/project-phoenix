import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { StatsCards } from "./stats-cards";

describe("StatsCards", () => {
  it("renders all four stat cards with correct labels", () => {
    render(<StatsCards total={10} newCount={5} existing={3} errors={2} />);

    expect(screen.getByText("Gesamt")).toBeInTheDocument();
    expect(screen.getByText("Neu")).toBeInTheDocument();
    expect(screen.getByText("Vorhanden")).toBeInTheDocument();
    expect(screen.getByText("Fehler")).toBeInTheDocument();
  });

  it("displays the correct values for each stat", () => {
    render(<StatsCards total={100} newCount={50} existing={30} errors={20} />);

    expect(screen.getByText("100")).toBeInTheDocument();
    expect(screen.getByText("50")).toBeInTheDocument();
    expect(screen.getByText("30")).toBeInTheDocument();
    expect(screen.getByText("20")).toBeInTheDocument();
  });

  it("handles zero values correctly", () => {
    render(<StatsCards total={0} newCount={0} existing={0} errors={0} />);

    const zeros = screen.getAllByText("0");
    expect(zeros).toHaveLength(4);
  });

  it("renders icons for each stat card", () => {
    const { container } = render(
      <StatsCards total={10} newCount={5} existing={3} errors={2} />,
    );

    // Each card has an SVG icon
    const svgElements = container.querySelectorAll("svg");
    expect(svgElements.length).toBe(4);
  });

  it("renders icon tiles as neutral gray tiles, not colored gradients", () => {
    const { container } = render(
      <StatsCards total={10} newCount={5} existing={3} errors={2} />,
    );

    // Icon tiles use the shared neutral-gray pattern; no gradients behind
    // functional icons.
    const iconTiles = container.querySelectorAll("[class*='bg-gray-100']");
    expect(iconTiles.length).toBe(4);
    expect(
      container.querySelectorAll("[class*='bg-gradient-to-br']").length,
    ).toBe(0);
  });

  it("renders in a responsive grid layout", () => {
    const { container } = render(
      <StatsCards total={10} newCount={5} existing={3} errors={2} />,
    );

    const grid = container.querySelector("[class*='grid']");
    expect(grid).toBeInTheDocument();
    expect(grid).toHaveClass("grid-cols-2");
    expect(grid).toHaveClass("md:grid-cols-4");
  });
});
