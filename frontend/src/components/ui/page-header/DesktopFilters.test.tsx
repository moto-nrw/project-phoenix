/**
 * Tests for DesktopFilters Component
 * Tests rendering and functionality of desktop filter controls
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { DesktopFilters } from "./DesktopFilters";
import type { FilterConfig } from "./types";

describe("DesktopFilters", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("Button type filters", () => {
    const mockOnChange = vi.fn();
    const buttonFilters: FilterConfig[] = [
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

    it("renders button filters", () => {
      render(<DesktopFilters filters={buttonFilters} />);

      expect(screen.getByText("Alle")).toBeInTheDocument();
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
      expect(screen.getByText("Inaktiv")).toBeInTheDocument();
    });

    it("highlights selected button", () => {
      render(<DesktopFilters filters={buttonFilters} />);

      const activeButton = screen.getByText("Aktiv");
      expect(activeButton).toHaveClass("bg-gray-900", "text-white");
    });

    it("calls onChange when button clicked (single select)", () => {
      render(<DesktopFilters filters={buttonFilters} />);

      const alleButton = screen.getByText("Alle");
      fireEvent.click(alleButton);

      expect(mockOnChange).toHaveBeenCalledWith("all");
    });

    it("handles multi-select button filters", () => {
      const multiSelectFilters: FilterConfig[] = [
        {
          ...buttonFilters[0]!,
          value: ["active"],
          multiSelect: true,
        },
      ];

      render(<DesktopFilters filters={multiSelectFilters} />);

      const inactiveButton = screen.getByText("Inaktiv");
      fireEvent.click(inactiveButton);

      expect(mockOnChange).toHaveBeenCalledWith(["active", "inactive"]);
    });

    it("deselects multi-select option when clicked again", () => {
      const multiSelectFilters: FilterConfig[] = [
        {
          ...buttonFilters[0]!,
          value: ["active", "inactive"],
          multiSelect: true,
        },
      ];

      render(<DesktopFilters filters={multiSelectFilters} />);

      const activeButton = screen.getByText("Aktiv");
      fireEvent.click(activeButton);

      expect(mockOnChange).toHaveBeenCalledWith(["inactive"]);
    });
  });

  describe("Dropdown type filters", () => {
    const mockOnChange = vi.fn();
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

    it("renders dropdown filter button", () => {
      render(<DesktopFilters filters={dropdownFilters} />);
      expect(screen.getByText("Alle Räume")).toBeInTheDocument();
    });

    it("opens dropdown when button clicked", async () => {
      render(<DesktopFilters filters={dropdownFilters} />);

      const button = screen.getByText("Alle Räume");
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("Raum 101")).toBeInTheDocument();
        expect(screen.getByText("Raum 102")).toBeInTheDocument();
      });
    });

    it("closes dropdown when option selected", async () => {
      render(<DesktopFilters filters={dropdownFilters} />);

      const button = screen.getByText("Alle Räume");
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("Raum 101")).toBeInTheDocument();
      });

      const option = screen.getByText("Raum 101");
      fireEvent.click(option);

      expect(mockOnChange).toHaveBeenCalledWith("101");

      await waitFor(() => {
        expect(screen.queryByText("Raum 102")).not.toBeInTheDocument();
      });
    });

    it("shows count when provided", async () => {
      render(<DesktopFilters filters={dropdownFilters} />);

      const button = screen.getByText("Alle Räume");
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("(5)")).toBeInTheDocument();
        expect(screen.getByText("(3)")).toBeInTheDocument();
      });
    });

    it("closes dropdown when clicking outside", async () => {
      render(
        <div>
          <div data-testid="outside">Outside</div>
          <DesktopFilters filters={dropdownFilters} />
        </div>,
      );

      const button = screen.getByText("Alle Räume");
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("Raum 101")).toBeInTheDocument();
      });

      const outside = screen.getByTestId("outside");
      fireEvent.mouseDown(outside);

      await waitFor(() => {
        expect(screen.queryByText("Raum 102")).not.toBeInTheDocument();
      });
    });

    it("handles multi-select dropdown", async () => {
      const multiDropdownFilters: FilterConfig[] = [
        {
          ...dropdownFilters[0]!,
          value: ["101"],
          multiSelect: true,
        },
      ];

      render(<DesktopFilters filters={multiDropdownFilters} />);

      const button = screen.getByText("Raum");
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("Raum 102")).toBeInTheDocument();
      });

      const option = screen.getByText("Raum 102");
      fireEvent.click(option);

      expect(mockOnChange).toHaveBeenCalledWith(["101", "102"]);
    });

    it("shows ring when non-default value selected", () => {
      const selectedFilters: FilterConfig[] = [
        {
          ...dropdownFilters[0]!,
          value: "101",
        },
      ];

      render(<DesktopFilters filters={selectedFilters} />);

      // Find the button that contains the text "Raum 101"
      const button = screen.getByText("Raum 101").closest("button");
      expect(button).toHaveClass("ring-2", "ring-blue-500");
    });
  });

  describe("Grid type filters", () => {
    const mockOnChange = vi.fn();
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
            icon: "M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z",
          },
          {
            value: "room",
            label: "Raum",
            icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6",
          },
        ],
      },
    ];

    it("renders grid filter as dropdown with icons", async () => {
      render(<DesktopFilters filters={gridFilters} />);

      const button = screen.getByText("Gruppe");
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("Raum")).toBeInTheDocument();
      });
    });
  });

  describe("Multiple filters", () => {
    const mockOnChange1 = vi.fn();
    const mockOnChange2 = vi.fn();

    const multipleFilters: FilterConfig[] = [
      {
        id: "status",
        label: "Status",
        type: "buttons",
        value: "active",
        onChange: mockOnChange1,
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
        onChange: mockOnChange2,
        options: [
          { value: "all", label: "Alle Räume" },
          { value: "101", label: "Raum 101" },
        ],
      },
    ];

    it("renders multiple filters side by side", () => {
      render(<DesktopFilters filters={multipleFilters} />);

      expect(screen.getByText("Aktiv")).toBeInTheDocument();
      expect(screen.getByText("Alle Räume")).toBeInTheDocument();
    });

    it("applies custom className", () => {
      const { container } = render(
        <DesktopFilters filters={multipleFilters} className="custom-class" />,
      );

      expect(container.firstChild).toHaveClass("custom-class");
    });
  });
});
