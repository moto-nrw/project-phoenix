import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { DatePicker } from "./date-picker";

// Mock react-day-picker
vi.mock("react-day-picker", () => ({
  DayPicker: ({
    selected,
    onSelect,
  }: {
    selected?: Date;
    onSelect: (date: Date | undefined) => void;
  }) => (
    <div data-testid="day-picker">
      <button
        onClick={() => onSelect(new Date("2024-01-15T00:00:00Z"))}
        data-testid="select-date"
      >
        Select 15.01.2024
      </button>
      {selected && (
        <div data-testid="selected-date">{selected.toISOString()}</div>
      )}
    </div>
  ),
}));

// Mock date-fns
vi.mock("date-fns", () => ({
  format: vi.fn((date: Date, formatStr: string) => {
    if (formatStr === "dd.MM.yyyy") {
      return "15.01.2024";
    }
    if (formatStr === "MMMM yyyy") {
      return "Januar 2024";
    }
    return date.toISOString();
  }),
  addMonths: vi.fn((date: Date, amount: number) => {
    const newDate = new Date(date);
    newDate.setMonth(newDate.getMonth() + amount);
    return newDate;
  }),
  subMonths: vi.fn((date: Date, amount: number) => {
    const newDate = new Date(date);
    newDate.setMonth(newDate.getMonth() - amount);
    return newDate;
  }),
}));

// Mock date-fns/locale
vi.mock("date-fns/locale", () => ({
  de: {},
}));

describe("DatePicker", () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders with placeholder when no value is selected", () => {
    render(<DatePicker value={null} onChange={mockOnChange} />);

    expect(screen.getByText("Datum auswählen")).toBeInTheDocument();
  });

  it("renders with custom placeholder", () => {
    render(
      <DatePicker
        value={null}
        onChange={mockOnChange}
        placeholder="Wähle ein Datum"
      />,
    );

    expect(screen.getByText("Wähle ein Datum")).toBeInTheDocument();
  });

  it("displays formatted date when value is provided", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    expect(screen.getByText("15.01.2024")).toBeInTheDocument();
  });

  it("opens calendar when button is clicked", async () => {
    render(<DatePicker value={null} onChange={mockOnChange} />);

    const button = screen.getByRole("button", { name: /datum auswählen/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    });
  });

  it("closes calendar when clicking outside", async () => {
    render(
      <div>
        <DatePicker value={null} onChange={mockOnChange} />
        <div data-testid="outside">Outside</div>
      </div>,
    );

    // Open calendar
    const button = screen.getByRole("button", { name: /datum auswählen/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    });

    // Click outside
    fireEvent.mouseDown(screen.getByTestId("outside"));

    await waitFor(() => {
      expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
    });
  });

  it("calls onChange when a date is selected", async () => {
    render(<DatePicker value={null} onChange={mockOnChange} />);

    // Open calendar
    const button = screen.getByRole("button", { name: /datum auswählen/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    });

    // Select a date
    const selectButton = screen.getByTestId("select-date");
    fireEvent.click(selectButton);

    await waitFor(() => {
      expect(mockOnChange).toHaveBeenCalledWith(expect.any(Date));
    });
  });

  it("closes calendar after selecting a date", async () => {
    render(<DatePicker value={null} onChange={mockOnChange} />);

    // Open calendar
    const button = screen.getByRole("button", { name: /datum auswählen/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    });

    // Select a date
    const selectButton = screen.getByTestId("select-date");
    fireEvent.click(selectButton);

    await waitFor(() => {
      expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
    });
  });

  it("renders clear button when a date is selected", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button", { name: /datum löschen/i });
    expect(clearButton).toBeInTheDocument();
  });

  it("does not render clear button when no date is selected", () => {
    render(<DatePicker value={null} onChange={mockOnChange} />);

    const clearButton = screen.queryByRole("button", {
      name: /datum löschen/i,
    });
    expect(clearButton).not.toBeInTheDocument();
  });

  it("clears date when clear button is clicked", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button", { name: /datum löschen/i });
    fireEvent.click(clearButton);

    expect(mockOnChange).toHaveBeenCalledWith(null);
  });

  it("prevents calendar from opening when clear button is clicked", async () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button", { name: /datum löschen/i });
    fireEvent.click(clearButton);

    // Calendar should not open
    await waitFor(() => {
      expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
    });
  });

  it("toggles calendar open/close state", async () => {
    render(<DatePicker value={null} onChange={mockOnChange} />);

    const button = screen.getByRole("button", { name: /datum auswählen/i });

    // Open
    fireEvent.click(button);
    await waitFor(() => {
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    });

    // Close
    fireEvent.click(button);
    await waitFor(() => {
      expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
    });

    // Open again
    fireEvent.click(button);
    await waitFor(() => {
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    });
  });

  it("applies custom className", () => {
    const { container } = render(
      <DatePicker
        value={null}
        onChange={mockOnChange}
        className="custom-class"
      />,
    );

    const wrapper = container.querySelector(".custom-class");
    expect(wrapper).toBeInTheDocument();
  });

  it("updates button styling when calendar is open", async () => {
    render(<DatePicker value={null} onChange={mockOnChange} />);

    const button = screen.getByRole("button", { name: /datum auswählen/i });

    // Initial state
    expect(button.className).toContain("hover:bg-gray-50");

    // Open calendar
    fireEvent.click(button);

    await waitFor(() => {
      expect(button.className).toContain("border-gray-300");
      expect(button.className).toContain("bg-gray-50");
    });
  });

  it("changes text color when date is selected", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    const { rerender } = render(
      <DatePicker value={null} onChange={mockOnChange} />,
    );

    const button = screen.getByRole("button");
    const span = button.querySelector("span");

    // No date selected - gray text
    expect(span?.className).toContain("text-gray-500");

    // Date selected - dark text
    rerender(<DatePicker value={testDate} onChange={mockOnChange} />);
    expect(span?.className).toContain("text-gray-900");
  });

  it("shows calendar icon", () => {
    render(<DatePicker value={null} onChange={mockOnChange} />);

    const svg = screen.getByRole("button").querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("h-4", "w-4");
  });

  it("does not close calendar when clicking inside the calendar container", async () => {
    render(<DatePicker value={null} onChange={mockOnChange} />);

    // Open calendar
    const button = screen.getByRole("button", { name: /datum auswählen/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    });

    // Click inside calendar
    const calendar = screen.getByTestId("day-picker");
    fireEvent.mouseDown(calendar);

    // Calendar should still be open
    expect(screen.getByTestId("day-picker")).toBeInTheDocument();
  });
});
