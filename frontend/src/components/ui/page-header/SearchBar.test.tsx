/**
 * Tests for SearchBar Component
 * Tests rendering and functionality of search input with clear button
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { SearchBar } from "./SearchBar";

describe("SearchBar", () => {
  const mockOnChange = vi.fn();
  const mockOnClear = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders search input", () => {
    render(<SearchBar value="" onChange={mockOnChange} />);
    const input = screen.getByPlaceholderText("Name suchen...");
    expect(input).toBeInTheDocument();
  });

  it("renders with custom placeholder", () => {
    render(
      <SearchBar
        value=""
        onChange={mockOnChange}
        placeholder="Suche Schüler..."
      />,
    );

    expect(screen.getByPlaceholderText("Suche Schüler...")).toBeInTheDocument();
  });

  it("displays current value", () => {
    render(<SearchBar value="Test Query" onChange={mockOnChange} />);
    const input = screen.getByDisplayValue("Test Query");
    expect(input).toBeInTheDocument();
  });

  it("calls onChange when typing", () => {
    render(<SearchBar value="" onChange={mockOnChange} />);
    const input = screen.getByPlaceholderText("Name suchen...");

    fireEvent.change(input, { target: { value: "New Text" } });

    expect(mockOnChange).toHaveBeenCalledWith("New Text");
  });

  it("shows clear button when value is not empty", () => {
    render(<SearchBar value="Some text" onChange={mockOnChange} />);
    const clearButton = screen.getByRole("button");
    expect(clearButton).toBeInTheDocument();
  });

  it("does not show clear button when value is empty", () => {
    render(<SearchBar value="" onChange={mockOnChange} />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("clears value when clear button clicked", () => {
    render(<SearchBar value="Some text" onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button");
    fireEvent.click(clearButton);

    expect(mockOnChange).toHaveBeenCalledWith("");
  });

  it("calls onClear callback when clear button clicked", () => {
    render(
      <SearchBar
        value="Some text"
        onChange={mockOnChange}
        onClear={mockOnClear}
      />,
    );

    const clearButton = screen.getByRole("button");
    fireEvent.click(clearButton);

    expect(mockOnClear).toHaveBeenCalledTimes(1);
  });

  it("does not call onClear when not provided", () => {
    render(<SearchBar value="Some text" onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button");
    fireEvent.click(clearButton);

    // Should not throw error
    expect(mockOnChange).toHaveBeenCalledWith("");
  });

  it("applies small size classes", () => {
    const { container } = render(
      <SearchBar value="" onChange={mockOnChange} size="sm" />,
    );
    const input = container.querySelector("input");
    expect(input).toHaveClass("py-2", "pl-9", "pr-3", "text-sm");
  });

  it("applies medium size classes", () => {
    const { container } = render(
      <SearchBar value="" onChange={mockOnChange} size="md" />,
    );
    const input = container.querySelector("input");
    expect(input).toHaveClass("py-2.5", "pl-9", "pr-3", "text-sm");
  });

  it("applies large size classes", () => {
    const { container } = render(
      <SearchBar value="" onChange={mockOnChange} size="lg" />,
    );
    const input = container.querySelector("input");
    expect(input).toHaveClass("py-3", "pl-10", "pr-10", "text-base");
  });

  it("applies custom className", () => {
    const { container } = render(
      <SearchBar value="" onChange={mockOnChange} className="custom-class" />,
    );
    expect(container.firstChild).toHaveClass("custom-class");
  });

  it("renders search icon", () => {
    const { container } = render(
      <SearchBar value="" onChange={mockOnChange} />,
    );
    const icon = container.querySelector("svg");
    expect(icon).toBeInTheDocument();
  });
});
