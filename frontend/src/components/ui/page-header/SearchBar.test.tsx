/**
 * Tests for SearchBar Component
 * Tests rendering and functionality of search input with clear button
 */
import { render, screen, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  SearchBar,
  SearchBarDraftProvider,
  SharedSearchBar,
} from "./SearchBar";

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
        placeholder="Suche Kinder..."
      />,
    );

    expect(screen.getByPlaceholderText("Suche Kinder...")).toBeInTheDocument();
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

  // #2975: without a debounce the owning page re-renders per keystroke — on the
  // Kindersuche that meant all 100 Kinderkarten per character.
  describe("debounceMs", () => {
    beforeEach(() => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("shows every keystroke but reports the term only once", () => {
      render(<SearchBar value="" onChange={mockOnChange} debounceMs={300} />);
      const input = screen.getByPlaceholderText("Name suchen...");

      for (const term of ["M", "Ma", "Max", "Maxi"]) {
        fireEvent.change(input, { target: { value: term } });
      }

      expect(input).toHaveValue("Maxi");
      expect(mockOnChange).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(300);
      });

      expect(mockOnChange).toHaveBeenCalledTimes(1);
      expect(mockOnChange).toHaveBeenCalledWith("Maxi");
    });

    it("follows an external value change, e.g. a cleared filter chip", () => {
      const { rerender } = render(
        <SearchBar value="Maxi" onChange={mockOnChange} debounceMs={300} />,
      );
      const input = screen.getByPlaceholderText("Name suchen...");
      fireEvent.change(input, { target: { value: "Maxim" } });

      rerender(<SearchBar value="" onChange={mockOnChange} debounceMs={300} />);

      expect(input).toHaveValue("");

      act(() => {
        vi.advanceTimersByTime(300);
      });

      // The pending keystroke must not resurrect the cleared term.
      expect(mockOnChange).not.toHaveBeenCalled();
    });

    it("uses the current callback when it changes during a pending debounce", () => {
      const initialOnChange = vi.fn();
      const currentOnChange = vi.fn();
      const { rerender } = render(
        <SearchBar value="" onChange={initialOnChange} debounceMs={300} />,
      );
      const input = screen.getByPlaceholderText("Name suchen...");
      fireEvent.change(input, { target: { value: "Maxi" } });

      rerender(
        <SearchBar value="" onChange={currentOnChange} debounceMs={300} />,
      );
      act(() => {
        vi.advanceTimersByTime(300);
      });

      expect(initialOnChange).not.toHaveBeenCalled();
      expect(currentOnChange).toHaveBeenCalledWith("Maxi");
    });

    it("cancels a pending draft when an external reset keeps the same value", () => {
      const { rerender } = render(
        <SearchBar
          value=""
          onChange={mockOnChange}
          debounceMs={300}
          resetKey={0}
        />,
      );
      const input = screen.getByPlaceholderText("Name suchen...");
      fireEvent.change(input, { target: { value: "Maxi" } });

      rerender(
        <SearchBar
          value=""
          onChange={mockOnChange}
          debounceMs={300}
          resetKey={1}
        />,
      );

      expect(input).toHaveValue("");
      act(() => {
        vi.advanceTimersByTime(300);
      });
      expect(mockOnChange).not.toHaveBeenCalled();
    });

    it("shares a pending draft between responsive copies of the same field", () => {
      render(
        <SearchBarDraftProvider
          value=""
          onChange={mockOnChange}
          debounceMs={300}
          resetKey={0}
        >
          <SharedSearchBar
            value=""
            onChange={mockOnChange}
            placeholder="Mobil"
          />
          <SharedSearchBar
            value=""
            onChange={mockOnChange}
            placeholder="Desktop"
          />
        </SearchBarDraftProvider>,
      );

      const mobileInput = screen.getByPlaceholderText("Mobil");
      const desktopInput = screen.getByPlaceholderText("Desktop");
      fireEvent.change(mobileInput, { target: { value: "Max" } });
      expect(desktopInput).toHaveValue("Max");

      fireEvent.change(desktopInput, { target: { value: "Maxi" } });
      expect(mobileInput).toHaveValue("Maxi");
      act(() => {
        vi.advanceTimersByTime(300);
      });
      expect(mockOnChange).toHaveBeenCalledTimes(1);
      expect(mockOnChange).toHaveBeenCalledWith("Maxi");
    });

    it("keeps a nested SearchBar independent from the shared header draft", () => {
      const headerOnChange = vi.fn();
      const nestedOnChange = vi.fn();

      render(
        <SearchBarDraftProvider
          value="Kopfbereich"
          onChange={headerOnChange}
          debounceMs={300}
          resetKey={0}
        >
          <SharedSearchBar
            value="Kopfbereich"
            onChange={headerOnChange}
            placeholder="Kopfbereich"
          />
          <SearchBar
            value="Nebenfeld"
            onChange={nestedOnChange}
            placeholder="Nebenfeld"
          />
        </SearchBarDraftProvider>,
      );

      const nestedInput = screen.getByPlaceholderText("Nebenfeld");
      expect(nestedInput).toHaveValue("Nebenfeld");

      fireEvent.change(nestedInput, { target: { value: "Eigene Suche" } });

      expect(nestedOnChange).toHaveBeenCalledWith("Eigene Suche");
      expect(headerOnChange).not.toHaveBeenCalled();
    });

    it("clears immediately instead of after the debounce window", () => {
      render(
        <SearchBar
          value="Maxi"
          onChange={mockOnChange}
          onClear={mockOnClear}
          debounceMs={300}
        />,
      );

      fireEvent.click(screen.getByLabelText("Suche löschen"));

      expect(mockOnChange).toHaveBeenCalledWith("");
      expect(mockOnClear).toHaveBeenCalled();
    });
  });
});
