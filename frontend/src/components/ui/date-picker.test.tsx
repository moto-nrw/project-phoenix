import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { DatePicker } from "./date-picker";

// Mock react-day-picker
vi.mock("react-day-picker", () => ({
  DayPicker: ({
    selected,
    onSelect,
    required,
  }: {
    selected?: Date | Date[];
    onSelect: (date: Date | undefined) => void;
    required?: boolean;
  }) => (
    <div data-testid="day-picker" data-required={required}>
      <button
        type="button"
        onClick={() => onSelect(new Date("2024-01-15T00:00:00Z"))}
        data-testid="select-date"
      >
        Select 15.01.2024
      </button>
      {selected instanceof Date && (
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

  it("closes calendar on pointer down outside", async () => {
    render(
      <div>
        <DatePicker value={null} onChange={mockOnChange} />
        <div data-testid="outside">Outside</div>
      </div>,
    );

    const button = screen.getByRole("button", { name: /datum auswählen/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    });

    fireEvent.pointerDown(screen.getByTestId("outside"));

    await waitFor(() => {
      expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
    });
  });

  it("does not close inline calendar when pressing inside it", async () => {
    render(
      <div>
        <DatePicker
          mode="multiple"
          values={[new Date("2024-01-15T00:00:00Z")]}
          onChangeDates={mockOnChange}
          calendarLayout="inline"
        />
        <div data-testid="outside">Outside</div>
      </div>,
    );

    const button = screen.getByRole("button", { name: /15\.01\.2024/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    });

    fireEvent.pointerDown(screen.getByTestId("day-picker"));

    expect(screen.getByTestId("day-picker")).toBeInTheDocument();
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

  it("keeps required dates non-clearable", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} required />);

    expect(
      screen.queryByRole("button", { name: /datum löschen/i }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /15\.01\.2024/i }));

    expect(screen.getByTestId("day-picker")).toHaveAttribute(
      "data-required",
      "true",
    );
  });

  it("clears date when clear button is clicked", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button", { name: /datum löschen/i });
    fireEvent.click(clearButton);

    expect(mockOnChange).toHaveBeenCalledWith(null);
  });

  it("clears date when Enter key is pressed on clear button", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button", { name: /datum löschen/i });
    fireEvent.keyDown(clearButton, { key: "Enter" });

    expect(mockOnChange).toHaveBeenCalledWith(null);
  });

  it("clears date when Space key is pressed on clear button", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button", { name: /datum löschen/i });
    fireEvent.keyDown(clearButton, { key: " " });

    expect(mockOnChange).toHaveBeenCalledWith(null);
  });

  it("does not clear date on other key presses", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button", { name: /datum löschen/i });
    fireEvent.keyDown(clearButton, { key: "Tab" });

    expect(mockOnChange).not.toHaveBeenCalled();
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

  it("renders the popover calendar already positioned on its first paint", () => {
    render(
      <DatePicker
        value={null}
        onChange={mockOnChange}
        calendarLayout="popover"
      />,
    );

    const trigger = screen.getByRole("button", { name: /datum auswählen/i });
    // jsdom reports an all-zero rect by default, which would make a top-left
    // render indistinguishable from a correctly positioned one.
    vi.spyOn(trigger.parentElement!, "getBoundingClientRect").mockReturnValue({
      top: 200,
      bottom: 240,
      left: 120,
      right: 320,
      width: 200,
      height: 40,
      x: 120,
      y: 200,
      toJSON: () => ({}),
    });

    fireEvent.click(trigger);

    // The position is measured in the click handler and the portal is gated on
    // it, so opening commits the calendar already in place instead of painting
    // it at the viewport origin first. jsdom cannot observe the intermediate
    // frame; this pins the resulting placement.
    const portal = screen.getByTestId("day-picker").closest(".fixed");
    expect(portal).toHaveStyle({ top: "244px", left: "120px" });
  });

  it("portals the calendar into the surrounding modal focus scope", () => {
    render(
      <div data-modal-focus-scope="true">
        <DatePicker
          value={null}
          onChange={mockOnChange}
          calendarLayout="popover"
        />
      </div>,
    );

    fireEvent.click(screen.getByRole("button", { name: /datum auswählen/i }));

    expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    expect(
      screen.getByTestId("day-picker").closest("[data-modal-focus-scope]"),
    ).not.toBeNull();
  });

  it("keeps month menus inside the calendar panel", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(
      <DatePicker
        value={testDate}
        onChange={mockOnChange}
        calendarLayout="popover"
        monthYearNavigation
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /15\.01\.2024/i }));
    fireEvent.click(screen.getByRole("combobox", { name: "Monat" }));

    expect(screen.getByRole("listbox", { name: "Monat" })).toBeInTheDocument();
    expect(
      screen
        .getByRole("listbox", { name: "Monat" })
        .closest("[data-date-picker-panel]"),
    ).not.toBeNull();
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
