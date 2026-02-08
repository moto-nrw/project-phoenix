/**
 * Tests for Operator Layout
 * Tests the conditional rendering of layout based on pathname
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Hoisted mocks
const { mockUsePathname } = vi.hoisted(() => ({
  mockUsePathname: vi.fn(),
}));

// Mock navigation
vi.mock("next/navigation", () => ({
  usePathname: mockUsePathname,
}));

// Mock contexts
vi.mock("~/lib/operator/auth-context", () => ({
  OperatorAuthProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="operator-auth-provider">{children}</div>
  ),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  OperatorShellProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="operator-shell-provider">{children}</div>
  ),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="breadcrumb-provider">{children}</div>
  ),
}));

// Mock AppShell component
vi.mock("~/components/dashboard/app-shell", () => ({
  AppShell: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="app-shell">{children}</div>
  ),
}));

import OperatorLayout from "./layout";

describe("OperatorLayout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders children directly on login page", () => {
    mockUsePathname.mockReturnValue("/operator/login");

    render(
      <OperatorLayout>
        <div data-testid="test-content">Login Content</div>
      </OperatorLayout>,
    );

    expect(screen.getByTestId("operator-auth-provider")).toBeInTheDocument();
    expect(screen.getByTestId("test-content")).toBeInTheDocument();
    expect(
      screen.queryByTestId("operator-shell-provider"),
    ).not.toBeInTheDocument();
    expect(screen.queryByTestId("app-shell")).not.toBeInTheDocument();
  });

  it("wraps children in shell and breadcrumb providers when not on login page", () => {
    mockUsePathname.mockReturnValue("/operator/suggestions");

    render(
      <OperatorLayout>
        <div data-testid="test-content">App Content</div>
      </OperatorLayout>,
    );

    expect(screen.getByTestId("operator-auth-provider")).toBeInTheDocument();
    expect(screen.getByTestId("operator-shell-provider")).toBeInTheDocument();
    expect(screen.getByTestId("breadcrumb-provider")).toBeInTheDocument();
    expect(screen.getByTestId("app-shell")).toBeInTheDocument();
    expect(screen.getByTestId("test-content")).toBeInTheDocument();
  });

  it("always wraps in OperatorAuthProvider", () => {
    mockUsePathname.mockReturnValue("/operator/suggestions");

    render(
      <OperatorLayout>
        <div data-testid="test-content">Content</div>
      </OperatorLayout>,
    );

    expect(screen.getByTestId("operator-auth-provider")).toBeInTheDocument();
  });

  it("handles announcements page", () => {
    mockUsePathname.mockReturnValue("/operator/announcements");

    render(
      <OperatorLayout>
        <div data-testid="test-content">Announcements</div>
      </OperatorLayout>,
    );

    expect(screen.getByTestId("app-shell")).toBeInTheDocument();
    expect(screen.getByTestId("test-content")).toBeInTheDocument();
  });

  it("handles settings page", () => {
    mockUsePathname.mockReturnValue("/operator/settings");

    render(
      <OperatorLayout>
        <div data-testid="test-content">Settings</div>
      </OperatorLayout>,
    );

    expect(screen.getByTestId("app-shell")).toBeInTheDocument();
    expect(screen.getByTestId("test-content")).toBeInTheDocument();
  });
});
