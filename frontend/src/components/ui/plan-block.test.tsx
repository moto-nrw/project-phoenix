import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { PlanBlock } from "./plan-block";

describe("PlanBlock", () => {
  it("renders as a type=button and fires onClick", () => {
    const onClick = vi.fn();
    render(
      <PlanBlock
        timeRange="12:00–14:00"
        label="Mensa"
        onClick={onClick}
        aria-label="Mensa bearbeiten"
      />,
    );

    const block = screen.getByRole("button", { name: "Mensa bearbeiten" });
    expect(block).toHaveAttribute("type", "button");
    fireEvent.click(block);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("applies the 3px edge color and the 10% tint as inline styles", () => {
    render(<PlanBlock timeRange="12:00–14:00" label="Mensa" color="#3F6F12" />);

    const block = screen.getByRole("button");
    expect(block).toHaveStyle({ borderLeft: "3px solid #3F6F12" });
    expect(block).toHaveStyle({ backgroundColor: "#3F6F121A" });
  });

  it("falls back to the neutral edge color when no shift-type color is given", () => {
    render(<PlanBlock timeRange="12:00–14:00" label="Mensa" />);

    expect(screen.getByRole("button")).toHaveStyle({
      borderLeft: "3px solid #D1D5DB",
    });
  });

  it("layers the sanctioned hatch pattern over the tint for status=hatched", () => {
    render(
      <PlanBlock
        timeRange="12:00–14:00"
        label="Mensa"
        color="#3F6F12"
        status="hatched"
      />,
    );

    const block = screen.getByRole("button");
    expect(block).toHaveStyle({
      backgroundImage:
        "repeating-linear-gradient(45deg, transparent 0 4px, rgba(107,114,128,0.15) 4px 8px)",
    });
    expect(block).toHaveStyle({ backgroundColor: "#3F6F121A" });
  });

  it("strikes the label through, flattens the edge to neutral, and drops the tint for status=cancelled", () => {
    render(
      <PlanBlock
        timeRange="12:00–14:00"
        label="Mensa"
        color="#3F6F12"
        status="cancelled"
      />,
    );

    const block = screen.getByRole("button");
    expect(block).toHaveStyle({ borderLeft: "3px solid #9CA3AF" });
    expect(block.style.backgroundColor).toBe("");
    const label = screen.getByText("Mensa");
    expect(label).toHaveClass("line-through", "text-gray-400");
  });

  it("shows the focus/selection ring when selected", () => {
    render(<PlanBlock timeRange="12:00–14:00" label="Mensa" selected />);

    expect(screen.getByRole("button")).toHaveClass(
      "ring-2",
      "ring-gray-900",
      "ring-offset-1",
    );
  });

  it("renders a non-interactive container instead of a button when interactive=false", () => {
    render(
      <PlanBlock
        timeRange="12:00–14:00"
        label="Mensa"
        interactive={false}
        aria-label="Mensa"
      />,
    );

    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Mensa").tagName).toBe("DIV");
  });

  it("renders time and label on a single line and drops the footer slot when compact", () => {
    render(
      <PlanBlock
        timeRange="12:00–14:00"
        label="Mensa"
        size="compact"
        footer={<span>Soll 3, da 1</span>}
      />,
    );

    expect(screen.queryByText("Soll 3, da 1")).not.toBeInTheDocument();
  });

  it("renders the optional footer slot at size=default", () => {
    render(
      <PlanBlock
        timeRange="12:00–14:00"
        label="Mensa"
        footer={<span>Soll 3, da 1</span>}
      />,
    );

    expect(screen.getByText("Soll 3, da 1")).toBeInTheDocument();
  });

  it("renders exactly one status icon", () => {
    render(
      <PlanBlock
        timeRange="12:00–14:00"
        label="Mensa"
        statusIcon={<svg data-testid="status-icon" />}
      />,
    );

    expect(screen.getAllByTestId("status-icon")).toHaveLength(1);
  });
});
