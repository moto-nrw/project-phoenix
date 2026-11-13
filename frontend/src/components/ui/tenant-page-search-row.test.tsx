/**
 * Tests for TenantPageSearchRow, the search and filter row of TenantPage.
 * Ported from the PageHeaderWithSearch tests (#6578): every statement about a
 * part the row still has keeps its assertion; the title row, tabs,
 * `actionButton`, `statusIndicator`, `className` and `compactOnScroll` are gone
 * with the layer that carried them.
 */
import { setViewportWidth as resizeViewport } from "~/test/helpers/viewport";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  TenantPageSearchRow,
  type TenantPageSearchRowProps,
} from "./tenant-page-search-row";

// Mock sub-components
vi.mock("./page-header/SearchBar", () => ({
  SearchBarDraftProvider: ({ children }: { children: React.ReactNode }) =>
    children,
  SharedSearchBar: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (v: string) => void;
  }) => (
    <input
      data-testid="search-bar"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));

vi.mock("./page-header/DesktopFilters", () => ({
  DesktopFilters: () => <div data-testid="desktop-filters">Filters</div>,
}));

vi.mock("./page-header/FilterButton", () => ({
  FilterButton: ({
    onClick,
    testId,
    hasActiveFilters,
  }: {
    onClick: () => void;
    testId?: string;
    hasActiveFilters?: boolean;
  }) => (
    <button
      type="button"
      data-testid={testId ?? "mobile-filter-button"}
      data-has-active-filters={String(Boolean(hasActiveFilters))}
      onClick={onClick}
    >
      Filter
    </button>
  ),
}));

vi.mock("./page-header/FilterPanel", () => ({
  FilterPanel: ({ isOpen, testId }: { isOpen: boolean; testId?: string }) =>
    isOpen ? (
      <div data-testid={testId ?? "mobile-filter-panel"}>Panel</div>
    ) : null,
}));

vi.mock("./page-header/ActiveFilterChips", () => ({
  ActiveFilterChips: ({ filters }: { filters: unknown[] }) => (
    <div data-testid="active-filter-chips">{filters.length} chips</div>
  ),
}));

vi.mock("./page-header/BadgeDisplay", () => ({
  BadgeDisplay: ({
    count,
    label,
  }: {
    count: number | string;
    label?: string;
  }) => (
    <div data-testid="badge-display">
      {count} {label}
    </div>
  ),
  BadgeDisplayCompact: ({ count }: { count: number | string }) => (
    <div data-testid="badge-display-compact">{count}</div>
  ),
}));

vi.mock("./page-header/OverflowMenu", () => ({
  OverflowMenu: ({ items }: { items: { label: string }[] }) => (
    <div data-testid="overflow-menu">
      {items.map((it) => (
        <span key={it.label}>{it.label}</span>
      ))}
    </div>
  ),
}));

describe("TenantPageSearchRow", () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    setViewportWidth(1024);
  });

  const baseProps: TenantPageSearchRowProps = {
    search: {
      value: "",
      onChange: mockOnChange,
    },
    filters: [],
    activeFilters: [],
  };

  it("renders search bar", () => {
    render(<TenantPageSearchRow {...baseProps} />);
    // Component renders two search bars (mobile and desktop)
    const searchBars = screen.getAllByTestId("search-bar");
    expect(searchBars.length).toBeGreaterThan(0);
  });

  it("does not render search bar when search prop not provided", () => {
    const propsWithoutSearch = { ...baseProps, search: undefined };
    render(
      <TenantPageSearchRow {...propsWithoutSearch} badge={{ count: 3 }} />,
    );
    // Anker: die Zeile ist gerendert, sie trägt die Plakette.
    expect(screen.getByTestId("badge-display")).toHaveTextContent("3");
    expect(screen.queryByTestId("search-bar")).not.toBeInTheDocument();
  });

  it("renders desktop filters when filters provided", () => {
    const propsWithFilters: TenantPageSearchRowProps = {
      ...baseProps,
      filters: [
        {
          id: "status",
          label: "Status",
          type: "buttons",
          value: "all",
          onChange: vi.fn(),
          options: [{ value: "all", label: "Alle" }],
        },
      ],
    };

    render(<TenantPageSearchRow {...propsWithFilters} />);
    expect(screen.getByTestId("desktop-filters")).toBeInTheDocument();
  });

  it("renders mobile filter button when filters provided", () => {
    const propsWithFilters: TenantPageSearchRowProps = {
      ...baseProps,
      filters: [
        {
          id: "status",
          label: "Status",
          type: "buttons",
          value: "all",
          onChange: vi.fn(),
          options: [{ value: "all", label: "Alle" }],
        },
      ],
    };

    render(<TenantPageSearchRow {...propsWithFilters} />);
    expect(screen.getByTestId("mobile-filter-button")).toBeInTheDocument();
  });

  it("opens mobile filter panel when button clicked", () => {
    const propsWithFilters: TenantPageSearchRowProps = {
      ...baseProps,
      filters: [
        {
          id: "status",
          label: "Status",
          type: "buttons",
          value: "all",
          onChange: vi.fn(),
          options: [{ value: "all", label: "Alle" }],
        },
      ],
    };

    render(<TenantPageSearchRow {...propsWithFilters} />);

    expect(screen.queryByTestId("mobile-filter-panel")).not.toBeInTheDocument();

    const filterButton = screen.getByTestId("mobile-filter-button");
    fireEvent.click(filterButton);

    expect(screen.getByTestId("mobile-filter-panel")).toBeInTheDocument();
  });

  it("renders active filter chips when provided", () => {
    const propsWithActiveFilters: TenantPageSearchRowProps = {
      ...baseProps,
      activeFilters: [
        { id: "filter1", label: "Status: Active", onRemove: vi.fn() },
        { id: "filter2", label: "Type: Group", onRemove: vi.fn() },
      ],
    };

    render(<TenantPageSearchRow {...propsWithActiveFilters} />);
    // Component renders chips for both mobile and desktop
    const chips = screen.getAllByText("2 chips");
    expect(chips.length).toBeGreaterThan(0);
  });

  it("detects active filters correctly", () => {
    const propsWithActiveFilter: TenantPageSearchRowProps = {
      ...baseProps,
      filters: [
        {
          id: "status",
          label: "Status",
          type: "buttons",
          value: "active", // Non-default value
          onChange: vi.fn(),
          options: [
            { value: "all", label: "Alle" },
            { value: "active", label: "Aktiv" },
          ],
        },
      ],
    };

    render(<TenantPageSearchRow {...propsWithActiveFilter} />);
    // When filter has non-default value, button should indicate active state
    expect(screen.getByTestId("mobile-filter-button")).toBeInTheDocument();
    expect(screen.getByTestId("mobile-filter-button")).toHaveAttribute(
      "data-has-active-filters",
      "true",
    );
  });

  it("treats an empty multi-select selection as inactive", () => {
    const multiSelectFilters: TenantPageSearchRowProps["filters"] = [
      {
        id: "schoolClass",
        label: "Klasse",
        type: "dropdown",
        multiSelect: true,
        value: [],
        onChange: vi.fn(),
        options: [
          { value: "3a", label: "3a" },
          { value: "4b", label: "4b" },
        ],
      },
    ];

    const { rerender } = render(
      <TenantPageSearchRow {...baseProps} filters={multiSelectFilters} />,
    );
    // An empty selection means "alle" — the filter button must stay neutral
    // even though [] never equals the first option's value.
    expect(screen.getByTestId("mobile-filter-button")).toHaveAttribute(
      "data-has-active-filters",
      "false",
    );

    rerender(
      <TenantPageSearchRow
        {...baseProps}
        filters={[{ ...multiSelectFilters[0]!, value: ["3a", "4b"] }]}
      />,
    );
    expect(screen.getByTestId("mobile-filter-button")).toHaveAttribute(
      "data-has-active-filters",
      "true",
    );
  });

  it("handles empty filters array", () => {
    render(<TenantPageSearchRow {...baseProps} filters={[]} />);
    // Anker: die Suchzeile, in der die Filter stünden, ist da.
    expect(screen.getAllByTestId("search-bar").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("desktop-filters")).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("mobile-filter-button"),
    ).not.toBeInTheDocument();
  });

  it("offers the mobile filter button without a search field", () => {
    const { search: _search, ...withoutSearch } = baseProps;
    render(
      <TenantPageSearchRow
        {...withoutSearch}
        filters={[
          {
            id: "status",
            label: "Status",
            type: "dropdown",
            value: "all",
            onChange: vi.fn(),
            options: [{ value: "all", label: "Alle Status" }],
          },
        ]}
      />,
    );
    expect(screen.queryByTestId("search-bar")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("mobile-filter-button"));
    expect(screen.getByTestId("mobile-filter-panel")).toBeInTheDocument();
  });

  it("renders badge when provided", () => {
    const propsWithBadge: TenantPageSearchRowProps = {
      ...baseProps,
      badge: { count: 42, label: "Items" },
    };

    render(<TenantPageSearchRow {...propsWithBadge} />);
    // Telefon: kompakt ohne Label; Desktop: mit Label.
    expect(screen.getByTestId("badge-display-compact")).toHaveTextContent("42");
    expect(screen.getByTestId("badge-display")).toHaveTextContent("42 Items");
    expect(
      screen.getByTestId("badge-display-compact").parentElement,
    ).toHaveClass("flex", "flex-shrink-0", "items-center", "gap-2");
    expect(screen.getByTestId("badge-display").parentElement).toHaveClass(
      "ml-auto",
      "flex",
      "flex-shrink-0",
      "items-center",
      "gap-3",
    );
  });

  describe("overflowMenu prop", () => {
    it("renders nothing when prop is omitted (default behaviour)", () => {
      render(<TenantPageSearchRow {...baseProps} />);
      // Anker: die Suchzeile ist gerendert.
      expect(screen.getAllByTestId("search-bar").length).toBeGreaterThan(0);
      expect(screen.queryByTestId("overflow-menu")).toBeNull();
    });

    it("renders nothing when items array is empty", () => {
      render(<TenantPageSearchRow {...baseProps} overflowMenu={[]} />);
      // Anker: die Suchzeile ist gerendert.
      expect(screen.getAllByTestId("search-bar").length).toBeGreaterThan(0);
      expect(screen.queryByTestId("overflow-menu")).toBeNull();
    });

    // The TenantPage header has no other row that carries the kebab. It
    // sits in the mobile search row as well as the desktop one; before,
    // phones had no way to reach the menu at all.
    it("offers the kebab in the mobile search row", () => {
      render(
        <TenantPageSearchRow
          search={{ value: "", onChange: vi.fn() }}
          overflowMenu={[
            { label: "Alle als gelesen markieren", onClick: vi.fn() },
          ]}
        />,
      );
      expect(screen.getAllByTestId("overflow-menu")).toHaveLength(2);
    });
  });

  describe("primaryAction + kebab row placement", () => {
    const quietWithFilters: TenantPageSearchRowProps = {
      ...baseProps,
      filterPanel: true,
      filters: [
        {
          id: "status",
          label: "Status",
          type: "buttons",
          value: "all",
          onChange: vi.fn(),
          options: [{ value: "all", label: "Alle" }],
        },
      ],
    };
    const primaryAction = <button data-testid="primary-action">Aktion</button>;
    const overflowMenu = [{ label: "Exportieren", onClick: vi.fn() }];

    it("keeps the kebab beside the primary action when the second row would be empty", () => {
      // Quiet filters live in the popover and there is no inline action, so
      // the kebab would otherwise sit alone on its own line.
      render(
        <TenantPageSearchRow
          {...quietWithFilters}
          primaryAction={primaryAction}
          overflowMenu={overflowMenu}
        />,
      );

      // Without a title row the mobile search row carries a kebab as well;
      // the desktop one is the last.
      const kebab = screen.getAllByTestId("overflow-menu").at(-1);
      expect(kebab?.parentElement).toBe(
        screen.getByTestId("primary-action").parentElement,
      );
    });

    it("leaves the kebab on the second row when inline filters share it", () => {
      render(
        <TenantPageSearchRow
          {...quietWithFilters}
          filterPanel={undefined}
          primaryAction={primaryAction}
          overflowMenu={overflowMenu}
        />,
      );

      expect(screen.getByTestId("desktop-filters")).toBeInTheDocument();
      expect(
        screen.getAllByTestId("overflow-menu").at(-1)?.parentElement,
      ).not.toBe(screen.getByTestId("primary-action").parentElement);
    });
  });

  describe("active filter display", () => {
    const filtersWithActive: TenantPageSearchRowProps = {
      ...baseProps,
      filters: [
        {
          id: "status",
          label: "Status",
          type: "buttons",
          value: "active",
          onChange: vi.fn(),
          options: [
            { value: "all", label: "Alle" },
            { value: "active", label: "Aktiv" },
          ],
        },
      ],
      activeFilters: [
        { id: "status", label: "Status: Aktiv", onRemove: vi.fn() },
      ],
    };

    it("renders chips by default (rückwärtskompatibel)", () => {
      render(<TenantPageSearchRow {...filtersWithActive} />);
      expect(
        screen.getAllByTestId("active-filter-chips").length,
      ).toBeGreaterThan(0);
    });

    it("suppresses chips with the filter panel", () => {
      render(<TenantPageSearchRow {...filtersWithActive} filterPanel />);
      // Anker: die Filter selbst sind geladen, nur die Chips fehlen.
      expect(screen.getByTestId("desktop-filter-button")).toBeInTheDocument();
      expect(screen.queryByTestId("active-filter-chips")).toBeNull();
    });
  });

  describe("filterPanel (popover layout)", () => {
    const propsWithFilters: TenantPageSearchRowProps = {
      ...baseProps,
      filters: [
        {
          id: "status",
          label: "Status",
          type: "buttons",
          value: "all",
          onChange: vi.fn(),
          options: [{ value: "all", label: "Alle" }],
        },
      ],
    };

    it("keeps desktop filters inline by default", () => {
      render(<TenantPageSearchRow {...propsWithFilters} />);

      expect(screen.getByTestId("desktop-filters")).toBeInTheDocument();
    });

    it("uses the shared filter panel when set to popover", () => {
      render(<TenantPageSearchRow {...propsWithFilters} filterPanel />);

      expect(screen.queryByTestId("desktop-filters")).not.toBeInTheDocument();

      fireEvent.click(screen.getByTestId("desktop-filter-button"));

      expect(screen.getByTestId("desktop-filter-panel")).toBeInTheDocument();
    });

    // The filter panel moves to its desktop layout at `xl` (1280 px).
    it("transfers an open mobile filter panel to the desktop popover on breakpoint resize", async () => {
      setViewportWidth(500);
      render(<TenantPageSearchRow {...propsWithFilters} filterPanel />);

      fireEvent.click(screen.getByTestId("mobile-filter-button"));
      expect(screen.getByTestId("mobile-filter-panel")).toBeInTheDocument();

      setViewportWidth(1280);

      await waitFor(() => {
        expect(
          screen.queryByTestId("mobile-filter-panel"),
        ).not.toBeInTheDocument();
        expect(screen.getByTestId("desktop-filter-panel")).toBeInTheDocument();
      });
    });

    it("closes an open mobile panel when resizing to inline desktop filters", async () => {
      setViewportWidth(500);
      render(<TenantPageSearchRow {...propsWithFilters} />);

      fireEvent.click(screen.getByTestId("mobile-filter-button"));
      expect(screen.getByTestId("mobile-filter-panel")).toBeInTheDocument();

      setViewportWidth(1200);

      await waitFor(() => {
        expect(
          screen.queryByTestId("mobile-filter-panel"),
        ).not.toBeInTheDocument();
        expect(screen.getByTestId("desktop-filters")).toBeInTheDocument();
      });
    });

    it("transfers an open desktop popover back to the mobile panel on breakpoint resize", async () => {
      setViewportWidth(1300);
      render(<TenantPageSearchRow {...propsWithFilters} filterPanel />);
      // Happy DOM startet einen `change`-Listener bei `matches === false`;
      // ein Resize auf der Desktop-Seite gleicht ihn an den echten Wert an,
      // damit das Verlassen der Grenze wie im Browser ein `change` meldet.
      setViewportWidth(1299);

      fireEvent.click(screen.getByTestId("desktop-filter-button"));
      expect(screen.getByTestId("desktop-filter-panel")).toBeInTheDocument();

      setViewportWidth(500);

      await waitFor(() => {
        expect(
          screen.queryByTestId("desktop-filter-panel"),
        ).not.toBeInTheDocument();
        expect(screen.getByTestId("mobile-filter-panel")).toBeInTheDocument();
      });
    });
  });
});

function setViewportWidth(width: number) {
  resizeViewport(width);
  fireEvent.resize(window);
}
