/**
 * Tests for FilterButton Component
 * Tests rendering and functionality of filter toggle button
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { FilterButton } from "./FilterButton";

describe("FilterButton", () => {
  const mockOnClick = vi.fn();

  it("renders button", () => {
    render(
      <FilterButton
        isOpen={false}
        onClick={mockOnClick}
        hasActiveFilters={false}
      />,
    );

    const button = screen.getByRole("button");
    expect(button).toBeInTheDocument();
  });

  it("calls onClick when button is clicked", () => {
    render(
      <FilterButton
        isOpen={false}
        onClick={mockOnClick}
        hasActiveFilters={false}
      />,
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(mockOnClick).toHaveBeenCalledTimes(1);
  });

  it("applies open state styles when isOpen is true", () => {
    render(
      <FilterButton
        isOpen={true}
        onClick={mockOnClick}
        hasActiveFilters={false}
      />,
    );

    // #6182: Der Offen-Zustand steht in `aria-expanded`; die Kit-Variante
    // `filter` färbt ihn blau.
    const button = screen.getByRole("button", { expanded: true });
    expect(button).toHaveClass(
      "aria-expanded:bg-moto-blue",
      "aria-expanded:text-white",
    );
  });

  it("applies closed state styles when isOpen is false", () => {
    render(
      <FilterButton
        isOpen={false}
        onClick={mockOnClick}
        hasActiveFilters={false}
      />,
    );

    const button = screen.getByRole("button", { expanded: false });
    expect(button).toHaveClass("bg-white", "text-gray-600");
  });

  it("shows ring when hasActiveFilters is true and not open", () => {
    render(
      <FilterButton
        isOpen={false}
        onClick={mockOnClick}
        hasActiveFilters={true}
      />,
    );

    // #6182: Der Ring hängt an `data-filters-active` (Kit-Variante `filter`).
    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("data-filters-active");
    expect(button).toHaveClass(
      "data-filters-active:ring-2",
      "data-filters-active:ring-moto-blue",
    );
  });

  it("does not show ring when isOpen is true even with active filters", () => {
    render(
      <FilterButton
        isOpen={true}
        onClick={mockOnClick}
        hasActiveFilters={true}
      />,
    );

    const button = screen.getByRole("button", { expanded: true });
    expect(button).not.toHaveAttribute("data-filters-active");
  });

  it("applies custom className", () => {
    render(
      <FilterButton
        isOpen={false}
        onClick={mockOnClick}
        hasActiveFilters={false}
        className="custom-class"
      />,
    );

    const button = screen.getByRole("button");
    expect(button).toHaveClass("custom-class");
  });
});

describe("FilterButton count badge", () => {
  // The name carries the count once; the badge is only its picture, so a
  // screen reader does not read it twice and the shown digit is no text
  // outside the name (WCAG 2.5.3).
  it("names the active filters once and hides the badge from assistive tech", () => {
    render(
      <FilterButton
        isOpen={false}
        onClick={vi.fn()}
        hasActiveFilters
        activeCount={2}
      />,
    );

    const button = screen.getByRole("button", { name: "Filter, 2 aktiv" });
    expect(button).toHaveTextContent("2");
    expect(
      screen.getByText("2").closest('[aria-hidden="true"]'),
    ).not.toBeNull();
  });
});
