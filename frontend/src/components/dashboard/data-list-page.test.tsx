/**
 * Tests for DataListPage Component
 * Tests generic list page with search and selection functionality
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { DataListPage } from "./data-list-page";

// Mock next/link
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    className,
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

// Mock dashboard components
vi.mock("@/components/dashboard", () => ({
  PageHeader: ({ title, backUrl }: { title: string; backUrl: string }) => (
    <div data-testid="page-header">
      <a href={backUrl}>Back</a>
      <h1>{title}</h1>
    </div>
  ),
  SectionTitle: ({ title }: { title: string }) => (
    <h2 data-testid="section-title">{title}</h2>
  ),
}));

interface TestEntity {
  id: string;
  name: string;
}

describe("DataListPage", () => {
  const mockData: TestEntity[] = [
    { id: "1", name: "Entity One" },
    { id: "2", name: "Entity Two" },
    { id: "3", name: "Another Entity" },
  ];

  const defaultProps = {
    title: "Select Entity",
    backUrl: "/dashboard",
    newEntityLabel: "Create New",
    newEntityUrl: "/create",
    data: mockData,
    onSelectEntityAction: vi.fn(),
  };

  it("renders page header with title", () => {
    render(<DataListPage {...defaultProps} />);

    expect(screen.getByText("Select Entity")).toBeInTheDocument();
  });

  it("renders section title", () => {
    render(<DataListPage {...defaultProps} sectionTitle="Choose Item" />);

    expect(screen.getByText("Choose Item")).toBeInTheDocument();
  });

  it("renders search input", () => {
    render(<DataListPage {...defaultProps} />);

    const searchInput = screen.getByPlaceholderText("Suchen...");
    expect(searchInput).toBeInTheDocument();
  });

  it("renders new entity button", () => {
    render(<DataListPage {...defaultProps} />);

    expect(screen.getByText("Create New")).toBeInTheDocument();
  });

  it("renders all entities", () => {
    render(<DataListPage {...defaultProps} />);

    expect(screen.getByText("Entity One")).toBeInTheDocument();
    expect(screen.getByText("Entity Two")).toBeInTheDocument();
    expect(screen.getByText("Another Entity")).toBeInTheDocument();
  });

  it("filters entities based on search term", () => {
    render(<DataListPage {...defaultProps} />);

    const searchInput = screen.getByPlaceholderText("Suchen...");
    fireEvent.change(searchInput, { target: { value: "Another" } });

    expect(screen.getByText("Another Entity")).toBeInTheDocument();
    expect(screen.queryByText("Entity One")).not.toBeInTheDocument();
    expect(screen.queryByText("Entity Two")).not.toBeInTheDocument();
  });

  it("calls onSelectEntityAction when entity clicked", () => {
    const onSelect = vi.fn();
    render(<DataListPage {...defaultProps} onSelectEntityAction={onSelect} />);

    fireEvent.click(screen.getByText("Entity One"));
    expect(onSelect).toHaveBeenCalledWith(mockData[0]);
  });

  it("shows no results message when search has no matches", () => {
    render(<DataListPage {...defaultProps} />);

    const searchInput = screen.getByPlaceholderText("Suchen...");
    fireEvent.change(searchInput, { target: { value: "NonExistent" } });

    expect(screen.getByText("Keine Ergebnisse gefunden.")).toBeInTheDocument();
  });

  it("shows empty state message when no data", () => {
    render(<DataListPage {...defaultProps} data={[]} />);

    expect(screen.getByText("Keine Einträge vorhanden.")).toBeInTheDocument();
  });

  it("uses external search term when provided", () => {
    const onSearchChange = vi.fn();
    render(
      <DataListPage
        {...defaultProps}
        searchTerm="External"
        onSearchChange={onSearchChange}
      />,
    );

    const searchInput = screen.getByPlaceholderText("Suchen...");
    expect(searchInput).toHaveValue("External");
  });

  it("calls onSearchChange when search input changes", () => {
    const onSearchChange = vi.fn();
    render(
      <DataListPage
        {...defaultProps}
        searchTerm=""
        onSearchChange={onSearchChange}
      />,
    );

    const searchInput = screen.getByPlaceholderText("Suchen...");
    fireEvent.change(searchInput, { target: { value: "test" } });

    expect(onSearchChange).toHaveBeenCalledWith("test");
  });

  it("uses custom entity renderer when provided", () => {
    const customRenderer = (entity: TestEntity) => (
      <div data-testid={`custom-${entity.id}`}>{entity.name} Custom</div>
    );

    render(<DataListPage {...defaultProps} renderEntity={customRenderer} />);

    expect(screen.getByTestId("custom-1")).toBeInTheDocument();
    expect(screen.getByText("Entity One Custom")).toBeInTheDocument();
  });

  it("links to new entity URL", () => {
    render(<DataListPage {...defaultProps} />);

    const createLink = screen.getByText("Create New").closest("a");
    expect(createLink).toHaveAttribute("href", "/create");
  });
});
