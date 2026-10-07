import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { DatabaseSelect, GroupSelect } from "./database-select";
import { useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

type SelectOptions = ReadonlyArray<{ value: string; label: string }>;

/** An owner with the shared load path, as a real caller wires it (#2517). */
function SelectOwner({
  loadOptions,
}: {
  readonly loadOptions: () => Promise<SelectOptions>;
}) {
  const { error, show, clear } = useApiLoadError();
  return (
    <DatabaseSelect
      name="room"
      label="Raum"
      value=""
      onChange={() => undefined}
      loadOptions={loadOptions}
      loadError={error}
      onLoadError={(loadError, retry) =>
        void show(loadError, {
          object: "die Liste der Räume",
          retry: () => {
            clear();
            retry();
          },
        })
      }
    />
  );
}

function GroupSelectOwner() {
  const { error, show, clear } = useApiLoadError();
  return (
    <GroupSelect
      name="group_id"
      value=""
      onChange={() => undefined}
      loadError={error}
      onLoadError={(loadError, retry) =>
        void show(loadError, {
          object: "die Liste der Gruppen",
          retry: () => {
            clear();
            retry();
          },
        })
      }
    />
  );
}

// The select renders as the kit listbox (CustomSelect): a combobox trigger
// button plus an option list that only exists while the menu is open.
function openMenu() {
  fireEvent.click(screen.getByRole("combobox"));
}

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
      const { container } = render(<DatabaseSelect {...defaultProps} />);
      expect(
        container.querySelector('input[name="test-select"]'),
      ).toBeInTheDocument();
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
      openMenu();

      expect(
        screen.getByRole("option", { name: "Option A" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Option B" }),
      ).toBeInTheDocument();
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
      openMenu();
      expect(
        screen.queryByRole("option", { name: "Bitte wählen" }),
      ).not.toBeInTheDocument();
    });

    it("renders disabled options correctly", () => {
      const options = [
        { value: "a", label: "Active", disabled: false },
        { value: "b", label: "Disabled", disabled: true },
      ];
      render(<DatabaseSelect {...defaultProps} options={options} />);
      openMenu();

      const disabledOption = screen.getByRole("option", { name: "Disabled" });
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

      expect(screen.getByRole("combobox")).toHaveTextContent("Option B");
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

      openMenu();
      fireEvent.click(screen.getByRole("option", { name: "Option B" }));
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
      expect(screen.getByRole("combobox")).toHaveClass("disabled:bg-gray-100");
      expect(screen.getByRole("combobox")).toHaveClass(
        "disabled:cursor-not-allowed",
      );
    });
  });

  describe("Error State", () => {
    it("displays external error message", () => {
      render(<DatabaseSelect {...defaultProps} error="Auswahl erforderlich" />);
      expect(screen.getByText("Auswahl erforderlich")).toBeInTheDocument();
    });

    it("applies error styles to select", () => {
      render(<DatabaseSelect {...defaultProps} error="Error" />);
      expect(screen.getByRole("combobox")).toHaveClass("border-moto-red");
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

      render(
        <DatabaseSelect
          {...defaultProps}
          loadOptions={loadOptions}
          loadError={null}
          onLoadError={vi.fn()}
        />,
      );

      // Should show loading state initially
      expect(screen.getByText("Lädt...")).toBeInTheDocument();

      // The trigger is disabled while loading; open once loading finished
      await waitFor(() => {
        expect(screen.getByRole("combobox")).not.toBeDisabled();
      });
      openMenu();

      await waitFor(() => {
        expect(
          screen.getByRole("option", { name: "Loaded Option 1" }),
        ).toBeInTheDocument();
        expect(
          screen.getByRole("option", { name: "Loaded Option 2" }),
        ).toBeInTheDocument();
      });

      expect(loadOptions).toHaveBeenCalledTimes(1);
    });

    it("shows a failed load with the owner's catalog text instead of an empty select and retries it", async () => {
      const loadOptions = vi
        .fn<() => Promise<SelectOptions>>()
        .mockRejectedValueOnce(
          new ApiError("down", 503, { code: "general.unavailable" }),
        )
        .mockResolvedValueOnce([{ value: "1", label: "Raum Blau" }]);

      render(<SelectOwner loadOptions={loadOptions} />);

      expect(
        await screen.findByText(
          catalogText("general.unavailable", "die Liste der Räume"),
        ),
      ).toBeInTheDocument();
      expect(screen.queryByRole("combobox")).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

      await waitFor(() => {
        expect(screen.getByRole("combobox")).not.toBeDisabled();
      });
      expect(loadOptions).toHaveBeenCalledTimes(2);
      expect(
        screen.queryByText(
          catalogText("general.unavailable", "die Liste der Räume"),
        ),
      ).not.toBeInTheDocument();
      openMenu();
      expect(
        screen.getByRole("option", { name: "Raum Blau" }),
      ).toBeInTheDocument();
    });

    it("hands a failed load to the owner with the error and a retry", async () => {
      const failure = new ApiError("down", 503, {
        code: "general.unavailable",
      });
      const loadOptions = vi.fn().mockRejectedValue(failure);
      const onLoadError = vi.fn();

      render(
        <DatabaseSelect
          {...defaultProps}
          loadOptions={loadOptions}
          loadError={null}
          onLoadError={onLoadError}
        />,
      );

      await waitFor(() => {
        expect(onLoadError).toHaveBeenCalledWith(failure, expect.any(Function));
      });
    });

    it("does not call loadOptions if staticOptions are provided", () => {
      const loadOptions = vi.fn();
      const staticOptions = [{ value: "static", label: "Static Option" }];

      render(
        // @ts-expect-error The type takes one source; the runtime still
        // prefers static options when a caller passes both.
        <DatabaseSelect
          {...defaultProps}
          options={staticOptions}
          loadOptions={loadOptions}
        />,
      );

      openMenu();
      expect(loadOptions).not.toHaveBeenCalled();
      expect(
        screen.getByRole("option", { name: "Static Option" }),
      ).toBeInTheDocument();
    });
  });

  describe("Custom Styling", () => {
    it("applies custom className", () => {
      render(<DatabaseSelect {...defaultProps} className="custom-class" />);
      expect(screen.getByRole("combobox")).toHaveClass("custom-class");
    });

    it("uses the standard kit focus styling", () => {
      render(<DatabaseSelect {...defaultProps} />);
      expect(screen.getByRole("combobox")).toHaveClass(
        "focus-visible:ring-gray-400",
      );
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
    loadError: null,
    onLoadError: vi.fn(),
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
      expect(screen.getByRole("combobox")).not.toBeDisabled();
    });
    openMenu();

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: "Klasse 1a" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Klasse 2b" }),
      ).toBeInTheDocument();
    });

    expect(global.fetch).toHaveBeenCalledWith("/api/groups");
  });

  it("shows a failed group load with the catalog text and retry", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 500,
      text: () =>
        Promise.resolve(
          JSON.stringify({ status: "error", error: "db down", code: "" }),
        ),
    });

    render(<GroupSelectOwner />);

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Gruppen"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Wiederholen" }),
    ).toBeInTheDocument();
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
      expect(screen.getByRole("combobox")).not.toBeDisabled();
    });
    openMenu();

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: "Group 1" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Group 2" }),
      ).toBeInTheDocument();
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
      expect(screen.getByRole("combobox")).not.toBeDisabled();
    });
    openMenu();

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: "Numeric ID Group" }),
      ).toBeInTheDocument();
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
