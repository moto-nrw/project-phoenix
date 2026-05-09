/**
 * Tests for MobileFilterPanel Component
 * Tests rendering and functionality of mobile filter panel
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { MobileFilterPanel } from "./MobileFilterPanel";
import type { FilterConfig } from "./types";

describe("MobileFilterPanel", () => {
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

  it("renders nothing when isOpen is false", () => {
    const { container } = render(
      <MobileFilterPanel
        isOpen={false}
        onClose={mockOnClose}
        filters={sampleFilters}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders panel when isOpen is true", () => {
    render(
      <MobileFilterPanel
        isOpen={true}
        onClose={mockOnClose}
        filters={sampleFilters}
      />,
    );

    expect(screen.getByText("Status")).toBeInTheDocument();
  });

  describe("Button type filters", () => {
    it("renders button options", () => {
      render(
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
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
      const { container } = render(
        <MobileFilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={gridFilters}
        />,
      );

      const gridContainer = container.querySelector(".grid-cols-2");
      expect(gridContainer).toBeInTheDocument();
    });

    it("renders grid options with icons", () => {
      render(
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={dropdownFilters}
        />,
      );

      expect(screen.getByTestId("filter-room")).toHaveValue("all");
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
        <MobileFilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={dropdownFilters}
        />,
      );

      expect(
        screen.getByRole("option", { name: "Raum 101 (5)" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Raum 102 (3)" }),
      ).toBeInTheDocument();
    });

    it("calls onChange when the single-select dropdown changes", () => {
      render(
        <MobileFilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={dropdownFilters}
        />,
      );

      fireEvent.change(screen.getByTestId("filter-room"), {
        target: { value: "101" },
      });

      expect(mockOnChange).toHaveBeenCalledWith("101");
    });

    it("renders multi-select dropdowns as compact buttons with counts", () => {
      const multiDropdownFilters: FilterConfig[] = [
        {
          ...dropdownFilters[0]!,
          value: ["101"],
          multiSelect: true,
        },
      ];

      render(
        <MobileFilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={multiDropdownFilters}
        />,
      );

      expect(screen.queryByTestId("filter-room")).not.toBeInTheDocument();
      expect(screen.getByText("Raum 101")).toHaveClass(
        "bg-gray-900",
        "text-white",
      );
      expect(screen.getByText("(5)")).toHaveClass("text-gray-300");
      expect(screen.getByText("(3)")).toHaveClass("text-gray-500");
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
        <MobileFilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={multiDropdownFilters}
        />,
      );

      fireEvent.click(screen.getByText("Raum 102"));

      expect(mockOnChange).toHaveBeenCalledWith(["101", "102"]);
    });
  });

  describe("Panel dismissal", () => {
    it("closes when Escape is pressed", () => {
      render(
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
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
        <MobileFilterPanel
          isOpen={true}
          onClose={mockOnClose}
          filters={sampleFilters}
        />,
      );

      expect(screen.queryByText("Zurücksetzen")).not.toBeInTheDocument();
      expect(screen.queryByText("Anwenden")).not.toBeInTheDocument();
    });
  });
});
