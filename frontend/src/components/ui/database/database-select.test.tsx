import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { DatabaseSelect, GroupSelect } from "./database-select";

// =============================================================================
// DatabaseSelect Tests
// =============================================================================

describe("DatabaseSelect", () => {
  const defaultProps = {
    name: "test-select",
    value: "",
    onChange: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("Basic Rendering", () => {
    it("renders a select element", () => {
      render(<DatabaseSelect {...defaultProps} />);
      expect(screen.getByRole("combobox")).toBeInTheDocument();
    });

    it("renders with correct name attribute", () => {
      render(<DatabaseSelect {...defaultProps} />);
      expect(screen.getByRole("combobox")).toHaveAttribute(
        "name",
        "test-select",
      );
    });

    it("renders label when provided", () => {
      render(<DatabaseSelect {...defaultProps} label="Auswahl" />);
      expect(screen.getByText("Auswahl")).toBeInTheDocument();
    });

    it("shows required indicator when required", () => {
      render(<DatabaseSelect {...defaultProps} label="Auswahl" required />);
      expect(screen.getByText("Auswahl*")).toBeInTheDocument();
    });

    it("uses id prop for label association", () => {
      render(
        <DatabaseSelect {...defaultProps} id="custom-id" label="Test Label" />,
      );
      expect(screen.getByRole("combobox")).toHaveAttribute("id", "custom-id");
      expect(screen.getByLabelText("Test Label")).toBeInTheDocument();
    });

    it("uses name as id when id is not provided", () => {
      render(<DatabaseSelect {...defaultProps} label="Test Label" />);
      expect(screen.getByRole("combobox")).toHaveAttribute("id", "test-select");
    });
  });

  describe("Options Rendering", () => {
    it("renders static options", () => {
      const options = [
        { value: "a", label: "Option A" },
        { value: "b", label: "Option B" },
      ];
      render(<DatabaseSelect {...defaultProps} options={options} />);

      expect(screen.getByText("Option A")).toBeInTheDocument();
      expect(screen.getByText("Option B")).toBeInTheDocument();
    });

    it("renders empty option by default", () => {
      render(<DatabaseSelect {...defaultProps} />);
      expect(screen.getByText("Bitte wählen")).toBeInTheDocument();
    });

    it("renders custom placeholder as empty option", () => {
      render(
        <DatabaseSelect {...defaultProps} placeholder="Wähle eine Option" />,
      );
      expect(screen.getByText("Wähle eine Option")).toBeInTheDocument();
    });

    it("renders custom empty option label", () => {
      render(
        <DatabaseSelect {...defaultProps} emptyOptionLabel="Keine Auswahl" />,
      );
      expect(screen.getByText("Keine Auswahl")).toBeInTheDocument();
    });

    it("does not render empty option when includeEmpty is false", () => {
      const options = [{ value: "a", label: "Option A" }];
      render(
        <DatabaseSelect
          {...defaultProps}
          options={options}
          includeEmpty={false}
        />,
      );
      expect(screen.queryByText("Bitte wählen")).not.toBeInTheDocument();
    });

    it("renders disabled options correctly", () => {
      const options = [
        { value: "a", label: "Active", disabled: false },
        { value: "b", label: "Disabled", disabled: true },
      ];
      render(<DatabaseSelect {...defaultProps} options={options} />);

      const disabledOption = screen.getByText("Disabled");
      expect(disabledOption).toBeDisabled();
    });
  });

  describe("Value and Change Handling", () => {
    it("displays selected value", () => {
      const options = [
        { value: "a", label: "Option A" },
        { value: "b", label: "Option B" },
      ];
      render(<DatabaseSelect {...defaultProps} options={options} value="b" />);

      expect(screen.getByRole("combobox")).toHaveValue("b");
    });

    it("calls onChange when selection changes", () => {
      const onChange = vi.fn();
      const options = [
        { value: "a", label: "Option A" },
        { value: "b", label: "Option B" },
      ];
      render(
        <DatabaseSelect
          {...defaultProps}
          options={options}
          onChange={onChange}
        />,
      );

      fireEvent.change(screen.getByRole("combobox"), {
        target: { value: "b" },
      });
      expect(onChange).toHaveBeenCalledWith("b");
    });
  });

  describe("Loading State", () => {
    it("shows loading text in empty option when loading", () => {
      render(<DatabaseSelect {...defaultProps} loading />);
      expect(screen.getByText("Lädt...")).toBeInTheDocument();
    });

    it("disables select when loading", () => {
      render(<DatabaseSelect {...defaultProps} loading />);
      expect(screen.getByRole("combobox")).toBeDisabled();
    });

    it("applies loading styles", () => {
      render(<DatabaseSelect {...defaultProps} loading />);
      expect(screen.getByRole("combobox")).toHaveClass("opacity-50");
      expect(screen.getByRole("combobox")).toHaveClass("cursor-wait");
    });
  });

  describe("Disabled State", () => {
    it("disables select when disabled prop is true", () => {
      render(<DatabaseSelect {...defaultProps} disabled />);
      expect(screen.getByRole("combobox")).toBeDisabled();
    });

    it("applies disabled styles", () => {
      render(<DatabaseSelect {...defaultProps} disabled />);
      expect(screen.getByRole("combobox")).toHaveClass("bg-gray-50");
      expect(screen.getByRole("combobox")).toHaveClass("cursor-not-allowed");
    });
  });

  describe("Error State", () => {
    it("displays external error message", () => {
      render(<DatabaseSelect {...defaultProps} error="Auswahl erforderlich" />);
      expect(screen.getByText("Auswahl erforderlich")).toBeInTheDocument();
    });

    it("applies error styles to select", () => {
      render(<DatabaseSelect {...defaultProps} error="Error" />);
      expect(screen.getByRole("combobox")).toHaveClass("border-red-300");
    });
  });

  describe("Helper Text", () => {
    it("displays helper text when provided", () => {
      render(
        <DatabaseSelect {...defaultProps} helperText="Wähle eine Option aus" />,
      );
      expect(screen.getByText("Wähle eine Option aus")).toBeInTheDocument();
    });

    it("hides helper text when error is present", () => {
      render(
        <DatabaseSelect
          {...defaultProps}
          helperText="Helper"
          error="Error message"
        />,
      );
      expect(screen.queryByText("Helper")).not.toBeInTheDocument();
      expect(screen.getByText("Error message")).toBeInTheDocument();
    });
  });

  describe("Async Options Loading", () => {
    it("loads options from loadOptions function", async () => {
      const loadOptions = vi.fn().mockResolvedValue([
        { value: "1", label: "Loaded Option 1" },
        { value: "2", label: "Loaded Option 2" },
      ]);

      render(<DatabaseSelect {...defaultProps} loadOptions={loadOptions} />);

      // Should show loading state initially
      expect(screen.getByText("Lädt...")).toBeInTheDocument();

      await waitFor(() => {
        expect(screen.getByText("Loaded Option 1")).toBeInTheDocument();
        expect(screen.getByText("Loaded Option 2")).toBeInTheDocument();
      });

      expect(loadOptions).toHaveBeenCalledTimes(1);
    });

    it("handles loadOptions error gracefully", async () => {
      const loadOptions = vi.fn().mockRejectedValue(new Error("Network error"));

      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      render(<DatabaseSelect {...defaultProps} loadOptions={loadOptions} />);

      await waitFor(() => {
        expect(
          screen.getByText("Fehler beim Laden der Optionen"),
        ).toBeInTheDocument();
      });

      consoleSpy.mockRestore();
    });

    it("does not call loadOptions if staticOptions are provided", () => {
      const loadOptions = vi.fn();
      const staticOptions = [{ value: "static", label: "Static Option" }];

      render(
        <DatabaseSelect
          {...defaultProps}
          options={staticOptions}
          loadOptions={loadOptions}
        />,
      );

      expect(loadOptions).not.toHaveBeenCalled();
      expect(screen.getByText("Static Option")).toBeInTheDocument();
    });
  });

  describe("Custom Styling", () => {
    it("applies custom className", () => {
      render(<DatabaseSelect {...defaultProps} className="custom-class" />);
      expect(screen.getByRole("combobox")).toHaveClass("custom-class");
    });

    it("applies custom focus ring color", () => {
      render(
        <DatabaseSelect
          {...defaultProps}
          focusRingColor="focus:ring-green-500"
        />,
      );
      expect(screen.getByRole("combobox")).toHaveClass("focus:ring-green-500");
    });

    it("uses default blue focus ring color", () => {
      render(<DatabaseSelect {...defaultProps} />);
      expect(screen.getByRole("combobox")).toHaveClass("focus:ring-blue-500");
    });
  });

  describe("Required Attribute", () => {
    it("sets required attribute on select", () => {
      render(<DatabaseSelect {...defaultProps} required />);
      expect(screen.getByRole("combobox")).toBeRequired();
    });

    it("does not set required attribute by default", () => {
      render(<DatabaseSelect {...defaultProps} />);
      expect(screen.getByRole("combobox")).not.toBeRequired();
    });
  });
});

// =============================================================================
// GroupSelect Tests
// =============================================================================

describe("GroupSelect", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  const defaultProps = {
    name: "group-select",
    value: "",
    onChange: vi.fn(),
  };

  it("renders with default 'Gruppe' label", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: [] }),
    });

    render(<GroupSelect {...defaultProps} />);
    expect(screen.getByText("Gruppe")).toBeInTheDocument();

    // Wait for async fetch to complete to avoid act() warning
    await waitFor(() => {
      expect(screen.getByText("Bitte wählen")).toBeInTheDocument();
    });
  });

  it("uses custom label when provided", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: [] }),
    });

    render(<GroupSelect {...defaultProps} label="Klassengruppe" />);
    expect(screen.getByText("Klassengruppe")).toBeInTheDocument();

    // Wait for async fetch to complete to avoid act() warning
    await waitFor(() => {
      expect(screen.getByText("Bitte wählen")).toBeInTheDocument();
    });
  });

  it("fetches options from groups API", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: [
            { id: "1", name: "Klasse 1a" },
            { id: "2", name: "Klasse 2b" },
          ],
        }),
    });

    render(<GroupSelect {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("Klasse 1a")).toBeInTheDocument();
      expect(screen.getByText("Klasse 2b")).toBeInTheDocument();
    });

    expect(global.fetch).toHaveBeenCalledWith("/api/groups");
  });

  it("handles API error gracefully", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 500,
    });

    const consoleSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    render(<GroupSelect {...defaultProps} />);

    await waitFor(() => {
      expect(
        screen.getByText("Fehler beim Laden der Optionen"),
      ).toBeInTheDocument();
    });

    consoleSpy.mockRestore();
  });

  it("includes filters in API request", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: [] }),
    });

    render(<GroupSelect {...defaultProps} filters={{ active: true }} />);

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith("/api/groups?active=true");
    });
  });

  it("handles array response format", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve([
          { id: 1, name: "Group 1" },
          { id: 2, name: "Group 2" },
        ]),
    });

    render(<GroupSelect {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("Group 1")).toBeInTheDocument();
      expect(screen.getByText("Group 2")).toBeInTheDocument();
    });
  });

  it("handles numeric IDs in response", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: [{ id: 123, name: "Numeric ID Group" }],
        }),
    });

    render(<GroupSelect {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("Numeric ID Group")).toBeInTheDocument();
    });
  });

  it("converts filter values to strings correctly", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: [] }),
    });

    render(
      <GroupSelect
        {...defaultProps}
        filters={{ count: 5, enabled: false, name: "test" }}
      />,
    );

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/groups?count=5&enabled=false&name=test",
      );
    });
  });

  it("skips null and undefined filter values", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: [] }),
    });

    render(
      <GroupSelect
        {...defaultProps}
        filters={{ valid: "yes", invalid: null, missing: undefined }}
      />,
    );

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith("/api/groups?valid=yes");
    });
  });
});
