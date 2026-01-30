/**
 * Tests for ActiveFilterChips Component
 * Tests rendering and functionality of active filter chips with remove actions
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ActiveFilterChips } from "./ActiveFilterChips";
import type { ActiveFilterChipsProps } from "./types";

describe("ActiveFilterChips", () => {
  const mockOnRemove1 = vi.fn();
  const mockOnRemove2 = vi.fn();
  const mockOnClearAll = vi.fn();

  const defaultFilters: ActiveFilterChipsProps["filters"] = [
    { id: "filter1", label: "Status: Active", onRemove: mockOnRemove1 },
    { id: "filter2", label: "Type: Group", onRemove: mockOnRemove2 },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when filters array is empty", () => {
    const { container } = render(
      <ActiveFilterChips filters={[]} onClearAll={mockOnClearAll} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders filter chips with labels", () => {
    render(<ActiveFilterChips filters={defaultFilters} />);

    expect(screen.getByText("Status: Active")).toBeInTheDocument();
    expect(screen.getByText("Type: Group")).toBeInTheDocument();
  });

  it("calls onRemove when remove button is clicked", () => {
    render(<ActiveFilterChips filters={defaultFilters} />);

    const removeButtons = screen.getAllByRole("button");
    fireEvent.click(removeButtons[0]!);

    expect(mockOnRemove1).toHaveBeenCalledTimes(1);
    expect(mockOnRemove2).not.toHaveBeenCalled();
  });

  it("renders clear all button when multiple filters and onClearAll provided", () => {
    render(
      <ActiveFilterChips
        filters={defaultFilters}
        onClearAll={mockOnClearAll}
      />,
    );

    expect(screen.getByText("Alle löschen")).toBeInTheDocument();
  });

  it("does not render clear all button when only one filter", () => {
    render(
      <ActiveFilterChips
        filters={[defaultFilters[0]!]}
        onClearAll={mockOnClearAll}
      />,
    );

    expect(screen.queryByText("Alle löschen")).not.toBeInTheDocument();
  });

  it("does not render clear all button when onClearAll not provided", () => {
    render(<ActiveFilterChips filters={defaultFilters} />);

    expect(screen.queryByText("Alle löschen")).not.toBeInTheDocument();
  });

  it("calls onClearAll when clear all button is clicked", () => {
    render(
      <ActiveFilterChips
        filters={defaultFilters}
        onClearAll={mockOnClearAll}
      />,
    );

    const clearAllButton = screen.getByText("Alle löschen");
    fireEvent.click(clearAllButton);

    expect(mockOnClearAll).toHaveBeenCalledTimes(1);
  });

  it("applies custom className", () => {
    const { container } = render(
      <ActiveFilterChips filters={defaultFilters} className="custom-class" />,
    );

    expect(container.firstChild).toHaveClass("custom-class");
  });

  it("renders correct number of filter chips", () => {
    render(<ActiveFilterChips filters={defaultFilters} />);

    const chips = screen
      .getAllByRole("button")
      .filter((button) => button.textContent !== "Alle löschen");

    // Each chip has one remove button
    expect(chips).toHaveLength(2);
  });
});
