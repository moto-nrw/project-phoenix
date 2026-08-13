/**
 * Tests for FilterPanel Component
 * Tests rendering and functionality of mobile filter panel
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { FilterPanel } from "./FilterPanel";
import type { FilterConfig } from "./types";

describe("FilterPanel", () => {
  const mockOnClose = vi.fn();
  const mockOnApply = vi.fn();
  const mockOnReset = vi.fn();
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  const sampleFilters: FilterConfig[] = [
    {
      id: "status",
      label: "Status",
      type: "buttons",
      value: "active",
      onChange: mockOnChange,
      options: [
        { value: "all", label: "Alle" },
        { value: "active", label: "Aktiv" },
        { value: "inactive", label: "Inaktiv" },
      ],
    },
  ];

  // Grouping is now consumer-supplied via the `sections` prop (the panel no
  // longer knows which filter ids belong together). Mirrors the student-search
  // section map so the grouping assertions below stay meaningful.
  const sampleSections = [
    { title: "Organisation", filterIds: ["year", "group", "room"] },
    {
      title: "Anwesenheit",
      filterIds: ["dayStatus", "attendance", "tracking"],
    },
    {
      title: "Zeiten & Abholung",
      filterIds: [
        "arrivalTime",
        "pickupTime",
        "pickupStatus",
        "bus",
        "photoConsent",
      ],
    },
    { title: "Darstellung", filterIds: ["sort", "groupMode"] },
  ];

  it("renders nothing when isOpen is false", () => {
    const { container } = render(
      <FilterPanel
        isOpen={false}
        onClose={mockOnClose}
        filters={sampleFilters}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders panel when isOpen is true", () => {
    render(
      <FilterPanel
        isOpen={true}
        onClose={mockOnClose}
        filters={sampleFilters}
      />,
    );

    expect(screen.getByText("Status")).toBeInTheDocument();
  });

  it("sizes anchored mobile panels from the actual panel top", async () => {
    const trigger = document.createElement("div");
    trigger.getBoundingClientRect = () =>
      ({
        left: 12,
        bottom: 160,
        right: 52,
        width: 40,
      }) as DOMRect;

    render(
      <FilterPanel
        isOpen={true}
        onClose={mockOnClose}
        anchorRect={{ left: 12, bottom: 80, right: 52 }}
        anchorRef={{ current: trigger }}
        filters={sampleFilters}
      />,
    );

    await waitFor(() => {
      const panel = screen.getByRole("region", { name: "Filter" });
      expect(panel).toHaveStyle({ top: "168px" });
      expect(panel).toHaveStyle({ position: "fixed" });
      expect(panel.style.maxHeight).toBe(
        "min(80vh, calc(100dvh - 168px - calc(6rem + env(safe-area-inset-bottom))))",
      );
    });
  });

  it("updates anchored mobile panel height continuously while the background scrolls", async () => {
    let triggerBottom = 160;
    const trigger = document.createElement("div");
    trigger.getBoundingClientRect = () =>
      ({
        left: 12,
        bottom: triggerBottom,
        right: 52,
        width: 40,
      }) as DOMRect;

    render(
      <FilterPanel
        isOpen={true}
        onClose={mockOnClose}
        anchorRect={{ left: 12, bottom: 160, right: 52 }}
        anchorRef={{ current: trigger }}
        filters={sampleFilters}
      />,
    );

    await waitFor(() =>
      expect(screen.getByRole("region", { name: "Filter" })).toHaveStyle({
        top: "168px",
      }),
    );

    triggerBottom = 140;
    fireEvent.scroll(window);

    await waitFor(() => {
      const panel = screen.getByRole("region", { name: "Filter" });
      expect(panel).toHaveStyle({ top: "148px" });
      expect(panel.style.maxHeight).toBe(
        "min(80vh, calc(100dvh - 148px - calc(6rem + env(safe-area-inset-bottom))))",
      );
    });
  });

  describe("Button type filters", () => {
    it("renders button options", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
        />,
      );

      expect(screen.getByText("Alle")).toBeInTheDocument();
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
      expect(screen.getByText("Inaktiv")).toBeInTheDocument();
    });

    it("highlights selected button", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
        />,
      );

      const activeButton = screen.getByText("Aktiv");
      expect(activeButton).toHaveClass("bg-gray-900", "text-white");
    });

    it("handles single-select button click", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
        />,
      );

      const alleButton = screen.getByText("Alle");
      fireEvent.click(alleButton);

      expect(mockOnChange).toHaveBeenCalledWith("all");
    });

    it("handles multi-select button click", () => {
      const multiSelectFilters: FilterConfig[] = [
        {
          ...sampleFilters[0]!,
          value: ["active"],
          multiSelect: true,
        },
      ];

      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={multiSelectFilters}
        />,
      );

      const inactiveButton = screen.getByText("Inaktiv");
      fireEvent.click(inactiveButton);

      expect(mockOnChange).toHaveBeenCalledWith(["active", "inactive"]);
    });

    it("removes a selected multi-select button when clicked again", () => {
      const multiSelectFilters: FilterConfig[] = [
        {
          ...sampleFilters[0]!,
          value: ["active", "inactive"],
          multiSelect: true,
        },
      ];

      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={multiSelectFilters}
        />,
      );

      fireEvent.click(screen.getByText("Aktiv"));

      expect(mockOnChange).toHaveBeenCalledWith(["inactive"]);
    });
  });

  describe("Grid type filters", () => {
    const gridFilters: FilterConfig[] = [
      {
        id: "type",
        label: "Typ",
        type: "grid",
        value: "group",
        onChange: mockOnChange,
        options: [
          {
            value: "group",
            label: "Gruppe",
            icon: "M17 20h5v-2a3 3 0 00-5.356-1.857",
          },
          { value: "room", label: "Raum", icon: "M3 12l2-2m0 0l7-7" },
        ],
      },
    ];

    it("renders grid options in 2 columns", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={gridFilters}
        />,
      );

      // Panel is portaled to document.body, so query the document rather than
      // the local render container.
      const gridContainer = document.querySelector(".grid-cols-2");
      expect(gridContainer).toBeInTheDocument();
    });

    it("renders grid options with icons", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={gridFilters}
        />,
      );

      expect(screen.getByText("Gruppe")).toBeInTheDocument();
      expect(screen.getByText("Raum")).toBeInTheDocument();
    });

    it("handles single-select grid option clicks", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={gridFilters}
        />,
      );

      fireEvent.click(screen.getByText("Raum"));

      expect(mockOnChange).toHaveBeenCalledWith("room");
    });

    it("handles multi-select grid option clicks", () => {
      const multiGridFilters: FilterConfig[] = [
        {
          ...gridFilters[0]!,
          value: ["group"],
          multiSelect: true,
        },
      ];

      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={multiGridFilters}
        />,
      );

      fireEvent.click(screen.getByText("Raum"));

      expect(mockOnChange).toHaveBeenCalledWith(["group", "room"]);
    });
  });

  describe("Dropdown type filters", () => {
    const dropdownFilters: FilterConfig[] = [
      {
        id: "room",
        label: "Raum",
        type: "dropdown",
        value: "all",
        onChange: mockOnChange,
        options: [
          { value: "all", label: "Alle Räume" },
          { value: "101", label: "Raum 101", count: 5 },
          { value: "102", label: "Raum 102", count: 3 },
        ],
      },
    ];

    it("renders single-select dropdown options inside a select", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={dropdownFilters}
        />,
      );

      expect(screen.getByTestId("filter-room")).toHaveTextContent("Alle Räume");
      fireEvent.click(screen.getByTestId("filter-room"));
      expect(
        screen.getByRole("option", { name: "Alle Räume" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Raum 101 (5)" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Raum 102 (3)" }),
      ).toBeInTheDocument();
    });

    it("includes counts in single-select option labels", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={dropdownFilters}
        />,
      );

      fireEvent.click(screen.getByTestId("filter-room"));
      expect(
        screen.getByRole("option", { name: "Raum 101 (5)" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Raum 102 (3)" }),
      ).toBeInTheDocument();
    });

    it("calls onChange when the single-select dropdown changes", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={dropdownFilters}
        />,
      );

      fireEvent.click(screen.getByTestId("filter-room"));
      fireEvent.click(screen.getByRole("option", { name: "Raum 101 (5)" }));

      expect(mockOnChange).toHaveBeenCalledWith("101");
    });

    // A multi-select is a dropdown too, not a wall of toggle chips: a school
    // with ten groups and eight classes would otherwise get two unreadable
    // blocks of buttons that grow with every class it adds.
    it("renders multi-select dropdowns as a dropdown naming the selection", () => {
      const multiDropdownFilters: FilterConfig[] = [
        {
          ...dropdownFilters[0]!,
          value: ["101"],
          multiSelect: true,
        },
      ];

      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={multiDropdownFilters}
        />,
      );

      expect(screen.getByTestId("filter-room")).toHaveTextContent("Raum 101");
      fireEvent.click(screen.getByTestId("filter-room"));
      expect(
        screen.getByRole("option", { name: "Raum 101 (5)" }),
      ).toHaveAttribute("aria-selected", "true");
      expect(
        screen.getByRole("option", { name: "Raum 102 (3)" }),
      ).toHaveAttribute("aria-selected", "false");
    });

    it("names an empty multi-select selection after the filter", () => {
      const multiDropdownFilters: FilterConfig[] = [
        {
          ...dropdownFilters[0]!,
          value: [],
          multiSelect: true,
        },
      ];

      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={multiDropdownFilters}
        />,
      );

      // No neutral "Alle …" option to check — an empty selection IS "alle",
      // and the trigger has to say so.
      expect(screen.getByTestId("filter-room")).toHaveTextContent("Alle Raum");
    });

    it("toggles multi-select dropdown options", () => {
      const multiDropdownFilters: FilterConfig[] = [
        {
          ...dropdownFilters[0]!,
          value: ["101"],
          multiSelect: true,
        },
      ];

      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={multiDropdownFilters}
        />,
      );

      fireEvent.click(screen.getByTestId("filter-room"));
      fireEvent.click(screen.getByRole("option", { name: "Raum 102 (3)" }));

      expect(mockOnChange).toHaveBeenCalledWith(["101", "102"]);
      // The menu stays open so a second value can be picked without
      // reopening it.
      expect(
        screen.getByRole("option", { name: "Raum 101 (5)" }),
      ).toBeInTheDocument();
    });
  });

  describe("Panel dismissal", () => {
    it("closes when Escape is pressed", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
        />,
      );

      fireEvent.keyDown(document, { key: "Escape" });

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it("ignores non-Escape key presses", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
        />,
      );

      fireEvent.keyDown(document, { key: "Enter" });

      expect(mockOnClose).not.toHaveBeenCalled();
    });

    it("closes when the backdrop is clicked", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
        />,
      );

      fireEvent.click(screen.getByRole("button", { name: "Filter schließen" }));

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it("renders nothing for unsupported filter types", () => {
      const unknownFilter = {
        ...sampleFilters[0]!,
        type: "unknown",
      } as unknown as FilterConfig;

      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={[unknownFilter]}
        />,
      );

      expect(screen.getByText("Status")).toBeInTheDocument();
      expect(screen.queryByText("Alle")).not.toBeInTheDocument();
    });
  });

  describe("Action buttons", () => {
    it("renders reset button when onReset provided", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
          onReset={mockOnReset}
        />,
      );

      expect(screen.getByText("Zurücksetzen")).toBeInTheDocument();
    });

    it("renders apply button when onApply provided", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
          onApply={mockOnApply}
        />,
      );

      expect(screen.getByText("Anwenden")).toBeInTheDocument();
    });

    it("calls onReset when reset button clicked", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
          onReset={mockOnReset}
        />,
      );

      const resetButton = screen.getByText("Zurücksetzen");
      fireEvent.click(resetButton);

      expect(mockOnReset).toHaveBeenCalledTimes(1);
    });

    it("calls onApply and onClose when apply button clicked", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
          onApply={mockOnApply}
        />,
      );

      const applyButton = screen.getByText("Anwenden");
      fireEvent.click(applyButton);

      expect(mockOnApply).toHaveBeenCalledTimes(1);
      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it("does not render action section when no actions provided", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
        />,
      );

      expect(screen.queryByText("Zurücksetzen")).not.toBeInTheDocument();
      expect(screen.queryByText("Anwenden")).not.toBeInTheDocument();
    });
  });

  describe("Desktop placement", () => {
    it("groups filters into desktop sections", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          placement="desktop"
          anchorRect={{ left: 24, bottom: 80, right: 624 }}
          sections={sampleSections}
          filters={[
            {
              id: "year",
              label: "Stufe",
              type: "dropdown",
              value: "all",
              onChange: mockOnChange,
              options: [{ value: "all", label: "Alle Stufen" }],
            },
            {
              id: "attendance",
              label: "Status",
              type: "dropdown",
              value: "all",
              onChange: mockOnChange,
              options: [{ value: "all", label: "Alle Status" }],
            },
            {
              id: "pickupTime",
              label: "Abholzeit",
              type: "dropdown",
              value: "all",
              onChange: mockOnChange,
              options: [{ value: "all", label: "Alle Abholzeiten" }],
            },
            {
              id: "sort",
              label: "Sortierung",
              type: "dropdown",
              value: "name",
              onChange: mockOnChange,
              options: [{ value: "name", label: "Name A-Z" }],
            },
          ]}
        />,
      );

      expect(screen.getByText("Organisation")).toBeInTheDocument();
      expect(screen.getByText("Anwesenheit")).toBeInTheDocument();
      expect(screen.getByText("Zeiten & Abholung")).toBeInTheDocument();
      expect(screen.getByText("Darstellung")).toBeInTheDocument();
    });

    it("aligns the desktop panel to the measured anchor edges", () => {
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          placement="desktop"
          anchorRect={{ left: 24, bottom: 80, right: 624 }}
          filters={sampleFilters}
        />,
      );

      const panel = screen.getByRole("region", { name: "Filter" });
      expect(panel).toHaveStyle({
        left: "24px",
        top: "88px",
      });
    });

    it("positions the first anchored open after mounting closed", () => {
      const { rerender } = render(
        <FilterPanel
          isOpen={false}
          onClose={mockOnClose}
          placement="desktop"
          anchorRect={null}
          filters={sampleFilters}
        />,
      );

      rerender(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          placement="desktop"
          anchorRect={{ left: 48, bottom: 96, right: 448 }}
          filters={sampleFilters}
        />,
      );

      expect(screen.getByRole("region", { name: "Filter" })).toHaveStyle({
        position: "fixed",
        left: "48px",
        top: "104px",
      });
    });

    it("pins to an 8px viewport inset when no app topbar is present", () => {
      // Trigger scrolled above the viewport (negative bottom): the panel must
      // stop tracking it. With no [data-app-header] in the DOM it falls back to
      // an 8px top inset.
      render(
        <FilterPanel
          isOpen={true}
          onClose={mockOnClose}
          placement="desktop"
          anchorRect={{ left: 24, bottom: -200, right: 624 }}
          filters={sampleFilters}
        />,
      );

      expect(screen.getByRole("region", { name: "Filter" })).toHaveStyle({
        top: "8px",
      });
    });

    it("pins just below the sticky app topbar when the trigger scrolls away", () => {
      // Inject a fake sticky topbar whose bottom edge sits at 64px.
      const header = document.createElement("header");
      header.setAttribute("data-app-header", "");
      header.getBoundingClientRect = () => ({ bottom: 64 }) as DOMRect;
      document.body.appendChild(header);

      try {
        render(
          <FilterPanel
            isOpen={true}
            onClose={mockOnClose}
            placement="desktop"
            anchorRect={{ left: 24, bottom: -200, right: 624 }}
            filters={sampleFilters}
          />,
        );

        // 64px topbar bottom + 8px gap = 72px, not the trigger's negative top.
        expect(screen.getByRole("region", { name: "Filter" })).toHaveStyle({
          top: "72px",
        });
      } finally {
        header.remove();
      }
    });
  });

  it("renders the mobile sheet as a non-modal filter region", () => {
    render(
      <FilterPanel
        isOpen={true}
        onClose={mockOnClose}
        filters={sampleFilters}
      />,
    );

    // Transparent backdrop, page stays interactive — non-modal, no aria-modal.
    expect(screen.getByRole("region", { name: "Filter" })).not.toHaveAttribute(
      "aria-modal",
    );
  });

  describe("Quiet variant", () => {
    const quietFilters: FilterConfig[] = [
      {
        id: "year",
        label: "Stufe",
        type: "buttons",
        value: "active",
        onChange: mockOnChange,
        options: [
          { value: "all", label: "Alle" },
          { value: "active", label: "Aktiv" },
        ],
      },
      {
        id: "room",
        label: "Raum",
        type: "dropdown",
        value: "all",
        onChange: mockOnChange,
        options: [{ value: "all", label: "Alle Räume" }],
      },
    ];

    it("groups filters into sectioned cards even on mobile placement", () => {
      render(
        <FilterPanel
          isOpen
          onClose={mockOnClose}
          variant="quiet"
          sections={sampleSections}
          filters={quietFilters}
        />,
      );

      // Both "year" and "room" belong to the Organisation section.
      expect(screen.getByText("Organisation")).toBeInTheDocument();
    });
  });
});
