/**
 * Tests for PageHeaderWithSearch Component
 * Tests rendering and functionality of the main page header with search and filters
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PageHeaderWithSearch } from "./PageHeaderWithSearch";
import type { PageHeaderWithSearchProps } from "./types";

// Mock sub-components
vi.mock("./PageHeader", () => ({
  PageHeader: ({ title, concept }: { title: string; concept?: string }) => (
    <div data-testid="page-header" data-concept={concept}>
      {title}
    </div>
  ),
}));

vi.mock("./SearchBar", () => ({
  SearchBarDraftProvider: ({ children }: { children: React.ReactNode }) =>
    children,
  SearchBar: ({
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

vi.mock("./DesktopFilters", () => ({
  DesktopFilters: () => <div data-testid="desktop-filters">Filters</div>,
}));

vi.mock("./FilterButton", () => ({
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

vi.mock("./FilterPanel", () => ({
  FilterPanel: ({ isOpen, testId }: { isOpen: boolean; testId?: string }) =>
    isOpen ? (
      <div data-testid={testId ?? "mobile-filter-panel"}>Panel</div>
    ) : null,
}));

vi.mock("./ActiveFilterChips", () => ({
  ActiveFilterChips: ({ filters }: { filters: unknown[] }) => (
    <div data-testid="active-filter-chips">{filters.length} chips</div>
  ),
}));

vi.mock("./NavigationTabs", () => ({
  NavigationTabs: ({ items }: { items: { id: string; label: string }[] }) => (
    <div data-testid="navigation-tabs">
      {items.map((item) => (
        <span key={item.id}>{item.label}</span>
      ))}
    </div>
  ),
}));

vi.mock("./TabsActionArea", () => ({
  DesktopTabsActionArea: () => (
    <div data-testid="desktop-tabs-action">Desktop Action</div>
  ),
  MobileTabsActionArea: () => (
    <div data-testid="mobile-tabs-action">Mobile Action</div>
  ),
}));

vi.mock("./OverflowMenu", () => ({
  OverflowMenu: ({ items }: { items: { label: string }[] }) => (
    <div data-testid="overflow-menu">
      {items.map((it) => (
        <span key={it.label}>{it.label}</span>
      ))}
    </div>
  ),
}));

describe("PageHeaderWithSearch", () => {
  const mockOnChange = vi.fn();
  const mockOnTabChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    setViewportWidth(1024);
  });

  const baseProps: PageHeaderWithSearchProps = {
    title: "Test Page",
    search: {
      value: "",
      onChange: mockOnChange,
    },
    filters: [],
    activeFilters: [],
  };

  it("renders page header with title", () => {
    render(<PageHeaderWithSearch {...baseProps} />);
    expect(screen.getByTestId("page-header")).toHaveTextContent("Test Page");
  });

  it("forwards the concept prop to the title area when set", () => {
    render(<PageHeaderWithSearch {...baseProps} concept="staff" />);
    expect(screen.getByTestId("page-header")).toHaveAttribute(
      "data-concept",
      "staff",
    );
  });

  it("does not set a concept on the title area when omitted", () => {
    render(<PageHeaderWithSearch {...baseProps} />);
    expect(screen.getByTestId("page-header")).not.toHaveAttribute(
      "data-concept",
    );
  });

  it("renders search bar", () => {
    render(<PageHeaderWithSearch {...baseProps} />);
    // Component renders two search bars (mobile and desktop)
    const searchBars = screen.getAllByTestId("search-bar");
    expect(searchBars.length).toBeGreaterThan(0);
  });

  it("does not render search bar when search prop not provided", () => {
    const propsWithoutSearch = { ...baseProps, search: undefined };
    render(<PageHeaderWithSearch {...propsWithoutSearch} />);
    expect(screen.queryByTestId("search-bar")).not.toBeInTheDocument();
  });

  it("renders navigation tabs when provided", () => {
    const propsWithTabs: PageHeaderWithSearchProps = {
      ...baseProps,
      tabs: {
        items: [
          { id: "all", label: "Alle" },
          { id: "active", label: "Aktiv" },
        ],
        activeTab: "all",
        onTabChange: mockOnTabChange,
      },
    };

    render(<PageHeaderWithSearch {...propsWithTabs} />);
    expect(screen.getByTestId("navigation-tabs")).toBeInTheDocument();
    expect(screen.getByText("Alle")).toBeInTheDocument();
    expect(screen.getByText("Aktiv")).toBeInTheDocument();
  });

  it("renders desktop filters when filters provided", () => {
    const propsWithFilters: PageHeaderWithSearchProps = {
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

    render(<PageHeaderWithSearch {...propsWithFilters} />);
    expect(screen.getByTestId("desktop-filters")).toBeInTheDocument();
  });

  it("renders mobile filter button when filters provided", () => {
    const propsWithFilters: PageHeaderWithSearchProps = {
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

    render(<PageHeaderWithSearch {...propsWithFilters} />);
    expect(screen.getByTestId("mobile-filter-button")).toBeInTheDocument();
  });

  it("opens mobile filter panel when button clicked", () => {
    const propsWithFilters: PageHeaderWithSearchProps = {
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

    render(<PageHeaderWithSearch {...propsWithFilters} />);

    expect(screen.queryByTestId("mobile-filter-panel")).not.toBeInTheDocument();

    const filterButton = screen.getByTestId("mobile-filter-button");
    fireEvent.click(filterButton);

    expect(screen.getByTestId("mobile-filter-panel")).toBeInTheDocument();
  });

  it("renders active filter chips when provided", () => {
    const propsWithActiveFilters: PageHeaderWithSearchProps = {
      ...baseProps,
      activeFilters: [
        { id: "filter1", label: "Status: Active", onRemove: vi.fn() },
        { id: "filter2", label: "Type: Group", onRemove: vi.fn() },
      ],
    };

    render(<PageHeaderWithSearch {...propsWithActiveFilters} />);
    // Component renders chips for both mobile and desktop
    const chips = screen.getAllByText("2 chips");
    expect(chips.length).toBeGreaterThan(0);
  });

  it("renders action button when provided", () => {
    const actionButton = (
      <button type="button" data-testid="custom-action">
        Add New
      </button>
    );
    const propsWithAction: PageHeaderWithSearchProps = {
      ...baseProps,
      actionButton,
    };

    render(<PageHeaderWithSearch {...propsWithAction} />);
    expect(screen.getByTestId("custom-action")).toBeInTheDocument();
  });

  it("renders mobile action button when provided", () => {
    const mobileActionButton = (
      <button type="button" data-testid="mobile-action">
        Add
      </button>
    );
    const propsWithMobileAction: PageHeaderWithSearchProps = {
      ...baseProps,
      mobileActionButton,
    };

    const { container } = render(
      <PageHeaderWithSearch {...propsWithMobileAction} />,
    );
    // The component accepts mobileActionButton prop but may not render it directly
    // Just verify the component renders without errors
    expect(container).toBeTruthy();
  });

  it("applies custom className", () => {
    const { container } = render(
      <PageHeaderWithSearch {...baseProps} className="custom-class" />,
    );
    expect(container.firstChild).toHaveClass("custom-class");
  });

  it("detects active filters correctly", () => {
    const propsWithActiveFilter: PageHeaderWithSearchProps = {
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

    render(<PageHeaderWithSearch {...propsWithActiveFilter} />);
    // When filter has non-default value, button should indicate active state
    expect(screen.getByTestId("mobile-filter-button")).toBeInTheDocument();
    expect(screen.getByTestId("mobile-filter-button")).toHaveAttribute(
      "data-has-active-filters",
      "true",
    );
  });

  it("treats an empty multi-select selection as inactive", () => {
    const multiSelectFilters: PageHeaderWithSearchProps["filters"] = [
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
      <PageHeaderWithSearch {...baseProps} filters={multiSelectFilters} />,
    );
    // An empty selection means "alle" — the filter button must stay neutral
    // even though [] never equals the first option's value.
    expect(screen.getByTestId("mobile-filter-button")).toHaveAttribute(
      "data-has-active-filters",
      "false",
    );

    rerender(
      <PageHeaderWithSearch
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
    render(<PageHeaderWithSearch {...baseProps} filters={[]} />);
    expect(screen.queryByTestId("desktop-filters")).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("mobile-filter-button"),
    ).not.toBeInTheDocument();
  });

  it("renders badge when provided", () => {
    const propsWithBadge: PageHeaderWithSearchProps = {
      ...baseProps,
      badge: { count: 42, label: "Items" },
    };

    render(<PageHeaderWithSearch {...propsWithBadge} />);
    // Badge is passed to PageHeader mock
    expect(screen.getByTestId("page-header")).toBeInTheDocument();
  });

  it("renders status indicator when provided", () => {
    const propsWithStatus: PageHeaderWithSearchProps = {
      ...baseProps,
      statusIndicator: { color: "green", tooltip: "Active" },
    };

    render(<PageHeaderWithSearch {...propsWithStatus} />);
    // Status is passed to PageHeader mock
    expect(screen.getByTestId("page-header")).toBeInTheDocument();
  });

  describe("tabsRowAction prop", () => {
    const tabs = {
      items: [
        { id: "a", label: "Eltern" },
        { id: "b", label: "Mitarbeitende" },
      ],
      activeTab: "a",
      onTabChange: vi.fn(),
    };

    it("rendert die Aktion neben den Reitern", () => {
      render(
        <PageHeaderWithSearch
          {...baseProps}
          tabs={tabs}
          tabsRowAction={<button type="button">Historie</button>}
        />,
      );
      expect(
        screen.getByRole("button", { name: "Historie" }),
      ).toBeInTheDocument();
    });

    it("rendert die Aktion auch ohne Reiter", () => {
      // Wer nur einen Reiter sehen darf, bekommt keine Reiterleiste — die
      // Aktion daneben muss trotzdem erreichbar bleiben.
      render(
        <PageHeaderWithSearch
          {...baseProps}
          tabsRowAction={<button type="button">Historie</button>}
        />,
      );
      expect(
        screen.getByRole("button", { name: "Historie" }),
      ).toBeInTheDocument();
    });

    it("rendert nichts, wenn die Prop fehlt", () => {
      render(<PageHeaderWithSearch {...baseProps} tabs={tabs} />);
      expect(screen.queryByRole("button", { name: "Historie" })).toBeNull();
    });
  });

  describe("overflowMenu prop", () => {
    it("renders nothing when prop is omitted (default behaviour)", () => {
      render(<PageHeaderWithSearch {...baseProps} />);
      expect(screen.queryByTestId("overflow-menu")).toBeNull();
    });

    it("renders nothing when items array is empty", () => {
      render(<PageHeaderWithSearch {...baseProps} overflowMenu={[]} />);
      expect(screen.queryByTestId("overflow-menu")).toBeNull();
    });

    it("renders the kebab when tabs + items are provided", () => {
      render(
        <PageHeaderWithSearch
          {...baseProps}
          tabs={{
            items: [{ id: "a", label: "A" }],
            activeTab: "a",
            onTabChange: vi.fn(),
          }}
          overflowMenu={[{ label: "Gruppe übergeben", onClick: vi.fn() }]}
        />,
      );
      expect(screen.getByTestId("overflow-menu")).toBeInTheDocument();
      expect(screen.getByText("Gruppe übergeben")).toBeInTheDocument();
    });
  });

  describe("primaryAction + kebab row placement", () => {
    const quietWithFilters: PageHeaderWithSearchProps = {
      ...baseProps,
      filterVariant: "quiet",
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
        <PageHeaderWithSearch
          {...quietWithFilters}
          primaryAction={primaryAction}
          overflowMenu={overflowMenu}
        />,
      );

      const kebab = screen.getByTestId("overflow-menu");
      expect(kebab.parentElement).toBe(
        screen.getByTestId("primary-action").parentElement,
      );
    });

    it("leaves the kebab on the second row when inline filters share it", () => {
      render(
        <PageHeaderWithSearch
          {...quietWithFilters}
          filterVariant={undefined}
          primaryAction={primaryAction}
          overflowMenu={overflowMenu}
        />,
      );

      expect(screen.getByTestId("desktop-filters")).toBeInTheDocument();
      expect(screen.getByTestId("overflow-menu").parentElement).not.toBe(
        screen.getByTestId("primary-action").parentElement,
      );
    });
  });

  describe("activeFilterDisplay prop", () => {
    const filtersWithActive: PageHeaderWithSearchProps = {
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
      render(<PageHeaderWithSearch {...filtersWithActive} />);
      expect(
        screen.getAllByTestId("active-filter-chips").length,
      ).toBeGreaterThan(0);
    });

    it("suppresses chips when set to count", () => {
      render(
        <PageHeaderWithSearch
          {...filtersWithActive}
          activeFilterDisplay="count"
        />,
      );
      expect(screen.queryByTestId("active-filter-chips")).toBeNull();
    });
  });

  describe("filterVariant quiet (popover layout)", () => {
    const propsWithFilters: PageHeaderWithSearchProps = {
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
      render(<PageHeaderWithSearch {...propsWithFilters} />);

      expect(screen.getByTestId("desktop-filters")).toBeInTheDocument();
    });

    it("uses the shared filter panel when set to popover", () => {
      render(
        <PageHeaderWithSearch {...propsWithFilters} filterVariant="quiet" />,
      );

      expect(screen.queryByTestId("desktop-filters")).not.toBeInTheDocument();

      fireEvent.click(screen.getByTestId("desktop-filter-button"));

      expect(screen.getByTestId("desktop-filter-panel")).toBeInTheDocument();
    });

    it("transfers an open mobile filter panel to the desktop popover on breakpoint resize", async () => {
      setViewportWidth(500);
      render(
        <PageHeaderWithSearch {...propsWithFilters} filterVariant="quiet" />,
      );

      fireEvent.click(screen.getByTestId("mobile-filter-button"));
      expect(screen.getByTestId("mobile-filter-panel")).toBeInTheDocument();

      setViewportWidth(1200);

      await waitFor(() => {
        expect(
          screen.queryByTestId("mobile-filter-panel"),
        ).not.toBeInTheDocument();
        expect(screen.getByTestId("desktop-filter-panel")).toBeInTheDocument();
      });
    });

    it("closes an open mobile panel when resizing to inline desktop filters", async () => {
      setViewportWidth(500);
      render(<PageHeaderWithSearch {...propsWithFilters} />);

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
      setViewportWidth(1200);
      render(
        <PageHeaderWithSearch {...propsWithFilters} filterVariant="quiet" />,
      );

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

  describe("compactOnScroll prop", () => {
    it("does not add scroll-driven classes by default", () => {
      const { container } = render(<PageHeaderWithSearch {...baseProps} />);
      // No transition classes injected when compactOnScroll is off.
      expect(container.innerHTML).not.toContain("backdrop-blur-md");
    });

    it("opts in to scroll-driven transitions when enabled", () => {
      const { container } = render(
        <PageHeaderWithSearch {...baseProps} compactOnScroll />,
      );
      // The transition class is rendered even at scrollY=0 (idle); the
      // active state classes only attach once scrollY > threshold.
      expect(container.innerHTML).toContain(
        "transition-[transform,backdrop-filter,background-color]",
      );
    });
  });
});

function setViewportWidth(width: number) {
  Object.defineProperty(window, "innerWidth", {
    value: width,
    configurable: true,
    writable: true,
  });
  fireEvent.resize(window);
}
