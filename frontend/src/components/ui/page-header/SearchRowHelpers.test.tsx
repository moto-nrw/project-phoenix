/**
 * Tests for SearchRowHelpers Component
 * Tests rendering and logic of inline status/badge helpers
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import {
  InlineStatusBadge,
  shouldShowInlineStatusBadge,
  DesktopSearchAction,
} from "./SearchRowHelpers";

// Mock sub-components
vi.mock("./StatusIndicator", () => ({
  StatusIndicator: ({
    color,
    tooltip,
  }: {
    color: string;
    tooltip?: string;
  }) => (
    <div data-testid="status-indicator" data-color={color} title={tooltip}>
      Status
    </div>
  ),
}));

vi.mock("./BadgeDisplay", () => ({
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

describe("InlineStatusBadge", () => {
  it("renders nothing when no status or badge provided", () => {
    const { container } = render(<InlineStatusBadge variant="mobile" />);
    expect(container.firstChild).toBeNull();
  });

  it("renders status indicator when provided", () => {
    render(
      <InlineStatusBadge
        statusIndicator={{ color: "green", tooltip: "Active" }}
        variant="mobile"
      />,
    );

    expect(screen.getByTestId("status-indicator")).toBeInTheDocument();
    expect(screen.getByTestId("status-indicator")).toHaveAttribute(
      "data-color",
      "green",
    );
  });

  it("renders badge when provided (mobile)", () => {
    render(
      <InlineStatusBadge
        badge={{ count: 42, label: "Items" }}
        variant="mobile"
      />,
    );

    expect(screen.getByTestId("badge-display-compact")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("renders badge when provided (desktop)", () => {
    render(
      <InlineStatusBadge
        badge={{ count: 42, label: "Items" }}
        variant="desktop"
      />,
    );

    expect(screen.getByTestId("badge-display")).toBeInTheDocument();
  });

  it("renders both status and badge", () => {
    render(
      <InlineStatusBadge
        statusIndicator={{ color: "yellow" }}
        badge={{ count: 10 }}
        variant="desktop"
      />,
    );

    expect(screen.getByTestId("status-indicator")).toBeInTheDocument();
    expect(screen.getByTestId("badge-display")).toBeInTheDocument();
  });

  it("applies mobile container classes", () => {
    const { container } = render(
      <InlineStatusBadge badge={{ count: 5 }} variant="mobile" />,
    );

    expect(container.firstChild).toHaveClass(
      "flex",
      "flex-shrink-0",
      "items-center",
      "gap-2",
    );
  });

  it("applies desktop container classes", () => {
    const { container } = render(
      <InlineStatusBadge badge={{ count: 5 }} variant="desktop" />,
    );

    expect(container.firstChild).toHaveClass(
      "ml-auto",
      "flex",
      "flex-shrink-0",
      "items-center",
      "gap-3",
    );
  });
});

describe("shouldShowInlineStatusBadge", () => {
  it("returns true when no tabs, no title, no action, with status", () => {
    const result = shouldShowInlineStatusBadge(
      false, // hasTabs
      false, // hasTitle
      false, // hasActionButton
      { color: "green" }, // statusIndicator
      undefined, // badge
    );

    expect(result).toBe(true);
  });

  it("returns true when no tabs, no title, no action, with badge", () => {
    const result = shouldShowInlineStatusBadge(false, false, false, undefined, {
      count: 10,
    });

    expect(result).toBe(true);
  });

  it("returns false when has tabs", () => {
    const result = shouldShowInlineStatusBadge(
      true, // hasTabs
      false,
      false,
      { color: "green" },
      undefined,
    );

    expect(result).toBe(false);
  });

  it("returns false when has title", () => {
    const result = shouldShowInlineStatusBadge(
      false,
      true, // hasTitle
      false,
      { color: "green" },
      undefined,
    );

    expect(result).toBe(false);
  });

  it("returns false when has action button", () => {
    const result = shouldShowInlineStatusBadge(
      false,
      false,
      true, // hasActionButton
      { color: "green" },
      undefined,
    );

    expect(result).toBe(false);
  });

  it("returns false when no status or badge", () => {
    const result = shouldShowInlineStatusBadge(
      false,
      false,
      false,
      undefined,
      undefined,
    );

    expect(result).toBe(false);
  });
});

describe("DesktopSearchAction", () => {
  const mockActionButton = <button data-testid="action-btn">Action</button>;

  it("renders action button when no tabs and action provided", () => {
    render(
      <DesktopSearchAction
        hasTabs={false}
        hasTitle={false}
        actionButton={mockActionButton}
      />,
    );

    expect(screen.getByTestId("action-btn")).toBeInTheDocument();
  });

  it("does not render action button when has tabs", () => {
    render(
      <DesktopSearchAction
        hasTabs={true}
        hasTitle={false}
        actionButton={mockActionButton}
      />,
    );

    expect(screen.queryByTestId("action-btn")).not.toBeInTheDocument();
  });

  it("renders inline status/badge when conditions met", () => {
    render(
      <DesktopSearchAction
        hasTabs={false}
        hasTitle={false}
        actionButton={undefined}
        statusIndicator={{ color: "green" }}
      />,
    );

    expect(screen.getByTestId("status-indicator")).toBeInTheDocument();
  });

  it("returns null when no content to show", () => {
    const { container } = render(
      <DesktopSearchAction
        hasTabs={true}
        hasTitle={true}
        actionButton={undefined}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("prioritizes action button over status/badge", () => {
    render(
      <DesktopSearchAction
        hasTabs={false}
        hasTitle={false}
        actionButton={mockActionButton}
        statusIndicator={{ color: "green" }}
        badge={{ count: 10 }}
      />,
    );

    expect(screen.getByTestId("action-btn")).toBeInTheDocument();
    expect(screen.queryByTestId("status-indicator")).not.toBeInTheDocument();
  });
});
