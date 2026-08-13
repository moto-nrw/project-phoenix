import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatePicker, ISODateInput } from "./date-picker";

// Mock react-day-picker
vi.mock("react-day-picker", () => ({
  DayPicker: ({
    selected,
    onSelect,
    required,
    month,
  }: {
    selected?: Date | Date[];
    onSelect: (date: Date | undefined) => void;
    required?: boolean;
    month?: Date;
  }) => (
    <div
      data-testid="day-picker"
      data-required={required}
      data-month={month?.toISOString()}
    >
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

  it("renders an inline calendar below the trigger controls", () => {
    render(
      <DatePicker
        value={null}
        onChange={mockOnChange}
        calendarLayout="inline"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /datum auswählen/i }));

    const controls = screen
      .getByRole("button", { name: /datum auswählen/i })
      .closest("[data-date-picker-controls]");
    const panel = screen
      .getByTestId("day-picker")
      .closest("[data-date-picker-panel]");
    expect(controls?.nextElementSibling).toBe(panel);
  });

  it("keeps compact sizing and legible day buttons in a narrow modal", () => {
    render(
      <DatePicker
        value={null}
        onChange={mockOnChange}
        calendarLayout="inline"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /datum auswählen/i }));

    const panel = screen
      .getByTestId("day-picker")
      .closest("[data-date-picker-panel]");
    expect(panel).toHaveClass("w-full", "min-w-[222px]", "max-w-full", "p-3");
    expect(panel).not.toHaveClass("p-4", "min-w-[304px]");
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

  it("uses a native button for keyboard-accessible clearing", () => {
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button", { name: /datum löschen/i });
    expect(clearButton.tagName).toBe("BUTTON");
    expect(clearButton).toHaveAttribute("type", "button");
  });

  it.each([
    ["Enter", "{Enter}"],
    ["Space", " "],
  ])("clears the date when %s activates the button", async (_name, key) => {
    const user = userEvent.setup();
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    const clearButton = screen.getByRole("button", { name: /datum löschen/i });
    clearButton.focus();
    await user.keyboard(key);

    expect(mockOnChange).toHaveBeenCalledOnce();
    expect(mockOnChange).toHaveBeenCalledWith(null);
  });

  it("does not clear the date for an unrelated key", async () => {
    const user = userEvent.setup();
    const testDate = new Date("2024-01-15T00:00:00Z");
    render(<DatePicker value={testDate} onChange={mockOnChange} />);

    screen.getByRole("button", { name: /datum löschen/i }).focus();
    await user.keyboard("{ArrowDown}");

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

  it("closes the calendar with Escape and restores trigger focus", () => {
    const parentKeyDown = vi.fn();
    render(
      <div role="group" aria-label="Testgruppe" onKeyDown={parentKeyDown}>
        <DatePicker value={null} onChange={mockOnChange} />
      </div>,
    );

    const trigger = screen.getByRole("button", { name: /datum auswählen/i });
    fireEvent.click(trigger);
    expect(screen.getByTestId("day-picker")).toBeInTheDocument();

    fireEvent.keyDown(screen.getByTestId("day-picker"), { key: "Escape" });

    expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
    expect(parentKeyDown).not.toHaveBeenCalled();
  });

  it("closes an open month listbox before closing the calendar", () => {
    render(
      <DatePicker
        value={new Date("2024-01-15T00:00:00Z")}
        onChange={mockOnChange}
        monthYearNavigation
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /15\.01\.2024/i }));
    const month = screen.getByRole("combobox", { name: "Monat" });
    fireEvent.click(month);
    expect(month).toHaveAttribute("aria-expanded", "true");

    fireEvent.keyDown(month, { key: "Escape" });

    expect(month).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByTestId("day-picker")).toBeInTheDocument();
  });

  it("constrains a long month label inside a narrow header", () => {
    render(
      <DatePicker
        value={new Date("2024-01-15T00:00:00Z")}
        onChange={mockOnChange}
        monthYearNavigation
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /15\.01\.2024/i }));

    const month = screen.getByRole("combobox", { name: "Monat" });
    expect(month).toHaveClass("min-w-0", "overflow-hidden");
    expect(month.querySelector("span")).toHaveClass("min-w-0", "truncate");
    expect(month.querySelector("svg")).toHaveClass("shrink-0");
  });

  it("synchronizes the displayed month when the controlled value changes", () => {
    const { rerender } = render(
      <DatePicker
        value={new Date("2024-01-15T00:00:00Z")}
        onChange={mockOnChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /15\.01\.2024/i }));
    expect(screen.getByTestId("day-picker")).toHaveAttribute(
      "data-month",
      "2024-01-15T00:00:00.000Z",
    );

    rerender(
      <DatePicker
        value={new Date("1982-04-17T00:00:00Z")}
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByTestId("day-picker")).toHaveAttribute(
      "data-month",
      "1982-04-17T00:00:00.000Z",
    );
  });

  it("keeps manual navigation when the controlled day is unchanged", () => {
    const value = new Date("2024-01-15T00:00:00Z");
    const { rerender } = render(
      <DatePicker value={value} onChange={mockOnChange} />,
    );

    fireEvent.click(screen.getByRole("button", { name: /15\.01\.2024/i }));
    fireEvent.click(screen.getByRole("button", { name: "Nächster Monat" }));
    expect(screen.getByTestId("day-picker")).toHaveAttribute(
      "data-month",
      "2024-02-15T00:00:00.000Z",
    );

    rerender(
      <DatePicker value={new Date(value.getTime())} onChange={mockOnChange} />,
    );

    expect(screen.getByTestId("day-picker")).toHaveAttribute(
      "data-month",
      "2024-02-15T00:00:00.000Z",
    );
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

  it("closes an open calendar when the picker becomes disabled", async () => {
    const { rerender } = render(
      <DatePicker value={null} onChange={mockOnChange} />,
    );

    fireEvent.click(screen.getByRole("button", { name: /datum auswählen/i }));
    expect(screen.getByTestId("day-picker")).toBeInTheDocument();

    rerender(<DatePicker value={null} onChange={mockOnChange} disabled />);

    expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
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
    vi.spyOn(
      trigger.parentElement!.parentElement!,
      "getBoundingClientRect",
    ).mockReturnValue({
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
    expect(portal).toHaveClass("max-h-[calc(100dvh-1rem)]", "overflow-y-auto");
  });

  it("keeps the calendar open while its short-viewport panel scrolls", () => {
    render(
      <DatePicker
        value={null}
        onChange={mockOnChange}
        calendarLayout="popover"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /datum auswählen/i }));
    const panel = screen.getByTestId("day-picker").closest(".fixed")!;
    fireEvent.scroll(panel);

    expect(screen.getByTestId("day-picker")).toBeInTheDocument();
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

describe("ISODateInput", () => {
  it("reports an existing date after the configured maximum as invalid", () => {
    const onValidityChange = vi.fn();
    render(
      <ISODateInput
        id="birthday"
        label="Geburtsdatum"
        value="2026-08-13"
        onChange={vi.fn()}
        onValidityChange={onValidityChange}
        max="2026-08-12"
        maxDateError="Das Geburtsdatum darf nicht in der Zukunft liegen."
      />,
    );

    expect(onValidityChange).toHaveBeenLastCalledWith(false);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Das Geburtsdatum darf nicht in der Zukunft liegen.",
    );
  });

  it("converts a valid German date to the ISO calendar format", () => {
    const onChange = vi.fn();
    render(
      <ISODateInput
        id="birthday"
        label="Geburtsdatum"
        value=""
        onChange={onChange}
        max="2026-08-12"
      />,
    );

    fireEvent.change(screen.getByLabelText("Geburtsdatum"), {
      target: { value: "17.04.1982" },
    });

    expect(onChange).toHaveBeenLastCalledWith("1982-04-17");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("rejects an impossible calendar date", () => {
    const onChange = vi.fn();
    render(
      <ISODateInput
        id="birthday"
        label="Geburtsdatum"
        value=""
        onChange={onChange}
      />,
    );

    const input = screen.getByLabelText("Geburtsdatum");
    fireEvent.change(input, { target: { value: "31.02.1982" } });
    fireEvent.blur(input);

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Bitte geben Sie ein gültiges Datum im Format TT.MM.JJJJ ein.",
    );
    expect(input).toHaveAttribute("aria-invalid", "true");
  });

  it("rejects a date after the configured maximum", () => {
    const onChange = vi.fn();
    render(
      <ISODateInput
        id="birthday"
        label="Geburtsdatum"
        value=""
        onChange={onChange}
        max="2026-08-12"
        maxDateError="Das Geburtsdatum darf nicht in der Zukunft liegen."
      />,
    );

    fireEvent.change(screen.getByLabelText("Geburtsdatum"), {
      target: { value: "13.08.2026" },
    });

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Das Geburtsdatum darf nicht in der Zukunft liegen.",
    );
  });

  it("shows and replaces an existing ISO date", () => {
    const onChange = vi.fn();
    render(
      <ISODateInput
        id="birthday"
        label="Geburtsdatum"
        value="1982-04-17"
        onChange={onChange}
      />,
    );

    const input = screen.getByLabelText("Geburtsdatum");
    expect(input).toHaveValue("17.04.1982");

    fireEvent.change(input, { target: { value: "18.04.1982" } });

    expect(onChange).toHaveBeenLastCalledWith("1982-04-18");
  });

  it("keeps calendar selection on a labeled native button", () => {
    const onChange = vi.fn();
    render(
      <ISODateInput
        id="birthday"
        label="Geburtsdatum"
        value=""
        onChange={onChange}
      />,
    );

    const calendarButton = screen.getByRole("button", {
      name: "Geburtsdatum im Kalender auswählen",
    });
    expect(calendarButton.tagName).toBe("BUTTON");
    expect(calendarButton).toHaveAttribute("type", "button");
    fireEvent.click(calendarButton);
    fireEvent.click(screen.getByTestId("select-date"));

    expect(onChange).toHaveBeenLastCalledWith("2024-01-15");
  });
});
