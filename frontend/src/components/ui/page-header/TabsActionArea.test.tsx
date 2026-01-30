/**
 * Tests for TabsActionArea Components
 * Tests rendering of action areas alongside navigation tabs
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { DesktopTabsActionArea, MobileTabsActionArea } from "./TabsActionArea";

// Mock sub-components
vi.mock("./StatusIndicator", () => ({
  StatusIndicator: ({ color }: { color: string }) => (
    <div data-testid="status-indicator" data-color={color}>
      Status
    </div>
  ),
}));

vi.mock("./BadgeDisplay", () => ({
  BadgeDisplay: ({ count }: { count: number | string }) => (
    <div data-testid="badge-display">{count}</div>
  ),
  BadgeDisplayCompact: ({ count }: { count: number | string }) => (
    <div data-testid="badge-display-compact">{count}</div>
  ),
}));

describe("DesktopTabsActionArea", () => {
  const mockActionButton = <button data-testid="action-btn">Action</button>;

  it("renders nothing when hasTitle is true", () => {
    const { container } = render(
      <DesktopTabsActionArea hasTitle={true} actionButton={mockActionButton} />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when no content provided", () => {
    const { container } = render(<DesktopTabsActionArea hasTitle={false} />);

    expect(container.firstChild).toBeNull();
  });

  it("renders action button when provided", () => {
    render(
      <DesktopTabsActionArea
        hasTitle={false}
        actionButton={mockActionButton}
      />,
    );

    expect(screen.getByTestId("action-btn")).toBeInTheDocument();
  });

  it("renders status indicator when provided", () => {
    render(
      <DesktopTabsActionArea
        hasTitle={false}
        statusIndicator={{ color: "green", tooltip: "Active" }}
      />,
    );

    expect(screen.getByTestId("status-indicator")).toBeInTheDocument();
  });

  it("renders badge when provided", () => {
    render(
      <DesktopTabsActionArea
        hasTitle={false}
        badge={{ count: 42, label: "Items" }}
      />,
    );

    expect(screen.getByTestId("badge-display")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("prioritizes action button over status and badge", () => {
    render(
      <DesktopTabsActionArea
        hasTitle={false}
        actionButton={mockActionButton}
        statusIndicator={{ color: "green" }}
        badge={{ count: 10 }}
      />,
    );

    expect(screen.getByTestId("action-btn")).toBeInTheDocument();
    expect(screen.queryByTestId("status-indicator")).not.toBeInTheDocument();
    expect(screen.queryByTestId("badge-display")).not.toBeInTheDocument();
  });

  it("renders both status and badge when no action button", () => {
    render(
      <DesktopTabsActionArea
        hasTitle={false}
        statusIndicator={{ color: "yellow" }}
        badge={{ count: 5 }}
      />,
    );

    expect(screen.getByTestId("status-indicator")).toBeInTheDocument();
    expect(screen.getByTestId("badge-display")).toBeInTheDocument();
  });

  it("has desktop visibility classes", () => {
    const { container } = render(
      <DesktopTabsActionArea hasTitle={false} badge={{ count: 1 }} />,
    );

    expect(container.firstChild).toHaveClass("hidden", "md:flex");
  });
});

describe("MobileTabsActionArea", () => {
  const mockActionButton = (
    <button data-testid="mobile-action-btn">Mobile Action</button>
  );

  it("renders nothing when hasTitle is true", () => {
    const { container } = render(
      <MobileTabsActionArea hasTitle={true} actionButton={mockActionButton} />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when no content provided", () => {
    const { container } = render(<MobileTabsActionArea hasTitle={false} />);

    expect(container.firstChild).toBeNull();
  });

  it("renders action button when provided", () => {
    render(
      <MobileTabsActionArea hasTitle={false} actionButton={mockActionButton} />,
    );

    expect(screen.getByTestId("mobile-action-btn")).toBeInTheDocument();
  });

  it("renders status indicator when provided", () => {
    render(
      <MobileTabsActionArea
        hasTitle={false}
        statusIndicator={{ color: "red", tooltip: "Error" }}
      />,
    );

    expect(screen.getByTestId("status-indicator")).toBeInTheDocument();
    expect(screen.getByTestId("status-indicator")).toHaveAttribute(
      "data-color",
      "red",
    );
  });

  it("renders compact badge when provided", () => {
    render(
      <MobileTabsActionArea
        hasTitle={false}
        badge={{ count: 99, label: "Items" }}
      />,
    );

    expect(screen.getByTestId("badge-display-compact")).toBeInTheDocument();
    expect(screen.getByText("99")).toBeInTheDocument();
  });

  it("prioritizes action button over status and badge", () => {
    render(
      <MobileTabsActionArea
        hasTitle={false}
        actionButton={mockActionButton}
        statusIndicator={{ color: "green" }}
        badge={{ count: 10 }}
      />,
    );

    expect(screen.getByTestId("mobile-action-btn")).toBeInTheDocument();
    expect(screen.queryByTestId("status-indicator")).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("badge-display-compact"),
    ).not.toBeInTheDocument();
  });

  it("has mobile visibility classes", () => {
    const { container } = render(
      <MobileTabsActionArea hasTitle={false} badge={{ count: 1 }} />,
    );

    expect(container.firstChild).toHaveClass("md:hidden");
  });
});
