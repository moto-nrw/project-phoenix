import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import { CoverageIndicator } from "./coverage-indicator";

describe("CoverageIndicator", () => {
  it('renders the Ist/Soll pair in the "2/3" format', () => {
    render(<CoverageIndicator state="covered" current={2} total={3} />);

    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("/3")).toBeInTheDocument();
  });

  it("uses the neutral gray dot and gray-600 number text when covered", () => {
    render(<CoverageIndicator state="covered" current={3} total={3} />);

    const dot = document.querySelector('[aria-hidden="true"]');
    expect(dot).toHaveStyle({ backgroundColor: "#9CA3AF" });
    expect(screen.getByText("3").parentElement).toHaveClass("text-gray-600");
  });

  it("uses the orange dot and colors only the understaffed Ist number red when a gap is open", () => {
    render(<CoverageIndicator state="gap" current={1} total={3} />);

    const dot = document.querySelector('[aria-hidden="true"]');
    expect(dot).toHaveStyle({ backgroundColor: "#F78C10" });

    const istNumber = screen.getByText("1");
    expect(istNumber).toHaveStyle({ color: "#DC2626" });
    // The "/3" part stays the default bold gray, not red.
    expect(screen.getByText("/3")).not.toHaveStyle({ color: "#DC2626" });
  });

  it("does not color the Ist number red when the pair is not actually understaffed", () => {
    render(<CoverageIndicator state="gap" current={3} total={3} />);

    const istNumber = screen.getByText("3");
    expect(istNumber).not.toHaveStyle({ color: "#DC2626" });
  });

  it('renders the "bewusst unbesetzt" note for the acknowledged state', () => {
    render(
      <CoverageIndicator
        state="acknowledged"
        current={0}
        total={1}
        note="bewusst unbesetzt"
      />,
    );

    const dot = document.querySelector('[aria-hidden="true"]');
    expect(dot).toHaveStyle({ backgroundColor: "#6B7280" });
    expect(screen.getByText("bewusst unbesetzt")).toHaveClass("text-gray-500");
  });

  it("renders free-form label text instead of a numeric pair when label is given", () => {
    render(<CoverageIndicator state="covered" label="19,5/20,25 h" />);

    expect(screen.getByText("19,5/20,25 h")).toBeInTheDocument();
    expect(screen.queryByText("/undefined")).not.toBeInTheDocument();
  });

  it("exposes a written-out description via title and aria-label", () => {
    render(<CoverageIndicator state="gap" current={1} total={3} />);

    const wrapper = screen.getByLabelText(
      "1 von 3 Positionen besetzt, 2 offen",
    );
    expect(wrapper).toHaveAttribute(
      "title",
      "1 von 3 Positionen besetzt, 2 offen",
    );
  });

  it('renders a smaller 6px dot and text-[11px] numbers for size="sm"', () => {
    render(
      <CoverageIndicator state="covered" current={2} total={3} size="sm" />,
    );

    const dot = document.querySelector('[aria-hidden="true"]');
    expect(dot).toHaveStyle({ width: "6px", height: "6px" });
    expect(screen.getByText("2").parentElement).toHaveClass("text-[11px]");
  });

  it("colors a free-form label red for tone=under (Unterdeckung, text only)", () => {
    render(
      <CoverageIndicator state="covered" label="18/20,25 h" tone="under" />,
    );

    expect(screen.getByText("18/20,25 h")).toHaveStyle({ color: "#DC2626" });
  });

  it("colors a free-form label amber for tone=over (Überdeckung, text only)", () => {
    render(
      <CoverageIndicator state="covered" label="22/20,25 h" tone="over" />,
    );

    expect(screen.getByText("22/20,25 h")).toHaveStyle({ color: "#EAB308" });
  });

  it("leaves the free-form label its default gray for tone=neutral", () => {
    render(
      <CoverageIndicator state="covered" label="20/20,25 h" tone="neutral" />,
    );

    const value = screen.getByText("20/20,25 h");
    expect(value).toHaveClass("text-gray-600");
    expect(value.style.color).toBe("");
  });
});
