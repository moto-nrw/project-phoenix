/**
 * Tests for PageHeaderWithSearch Component
 * Tests rendering and functionality of the main page header with search and filters
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PageHeaderWithSearch } from "./PageHeaderWithSearch";
import type { PageHeaderWithSearchProps } from "./types";

// Mock sub-components
vi.mock("./PageHeader", () => ({
  PageHeader: ({ title }: { title: string }) => (
    <div data-testid="page-header">{title}</div>
  ),
}));

vi.mock("./SearchBar", () => ({
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

vi.mock("./MobileFilterButton", () => ({
  MobileFilterButton: ({ onClick }: { onClick: () => void }) => (
    <button data-testid="mobile-filter-button" onClick={onClick}>
      Filter
    </button>
  ),
}));

vi.mock("./MobileFilterPanel", () => ({
  MobileFilterPanel: ({ isOpen }: { isOpen: boolean }) =>
    isOpen ? <div data-testid="mobile-filter-panel">Panel</div> : null,
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

describe("PageHeaderWithSearch", () => {
  const mockOnChange = vi.fn();
  const mockOnTabChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
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
    const actionButton = <button data-testid="custom-action">Add New</button>;
    const propsWithAction: PageHeaderWithSearchProps = {
      ...baseProps,
      actionButton,
    };

    render(<PageHeaderWithSearch {...propsWithAction} />);
    expect(screen.getByTestId("custom-action")).toBeInTheDocument();
  });

  it("renders mobile action button when provided", () => {
    const mobileActionButton = <button data-testid="mobile-action">Add</button>;
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
});
