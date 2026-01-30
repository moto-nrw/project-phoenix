/**
 * Tests for MobileFilterButton Component
 * Tests rendering and functionality of mobile filter toggle button
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { MobileFilterButton } from "./MobileFilterButton";

describe("MobileFilterButton", () => {
  const mockOnClick = vi.fn();

  it("renders button", () => {
    render(
      <MobileFilterButton
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
      <MobileFilterButton
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
      <MobileFilterButton
        isOpen={true}
        onClick={mockOnClick}
        hasActiveFilters={false}
      />,
    );

    const button = screen.getByRole("button");
    expect(button).toHaveClass("bg-blue-500", "text-white");
  });

  it("applies closed state styles when isOpen is false", () => {
    render(
      <MobileFilterButton
        isOpen={false}
        onClick={mockOnClick}
        hasActiveFilters={false}
      />,
    );

    const button = screen.getByRole("button");
    expect(button).toHaveClass("bg-white", "text-gray-600");
  });

  it("shows ring when hasActiveFilters is true and not open", () => {
    render(
      <MobileFilterButton
        isOpen={false}
        onClick={mockOnClick}
        hasActiveFilters={true}
      />,
    );

    const button = screen.getByRole("button");
    expect(button).toHaveClass("ring-2", "ring-blue-500");
  });

  it("does not show ring when isOpen is true even with active filters", () => {
    render(
      <MobileFilterButton
        isOpen={true}
        onClick={mockOnClick}
        hasActiveFilters={true}
      />,
    );

    const button = screen.getByRole("button");
    expect(button).not.toHaveClass("ring-2");
  });

  it("applies custom className", () => {
    render(
      <MobileFilterButton
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
