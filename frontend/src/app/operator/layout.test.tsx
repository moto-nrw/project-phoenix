/**
 * Tests for Operator Auth Guard
 * Tests the conditional rendering based on auth state and pathname.
 * (The layout itself is now a thin server component wrapping OperatorAuthGuard.)
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Hoisted mocks
const { mockUsePathname, mockUseSession, mockRedirect } = vi.hoisted(() => ({
  mockUsePathname: vi.fn(),
  mockUseSession: vi.fn(),
  mockRedirect: vi.fn(),
}));

// Mock navigation
vi.mock("next/navigation", () => ({
  usePathname: mockUsePathname,
  redirect: mockRedirect,
}));

// Mock next-auth
vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

// Mock contexts
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

// Mock operator-url to avoid NEXT_PUBLIC_OPERATOR_HOSTNAME requirement
vi.mock("~/lib/operator-url", () => ({
  operatorPath: (path: string) => path,
  isOperatorSubdomain: () => false,
}));

// Mock AppShell component
vi.mock("~/components/dashboard/app-shell", () => ({
  AppShell: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="app-shell">{children}</div>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

import { OperatorAuthGuard } from "./auth-guard";

describe("OperatorAuthGuard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders children directly on login page", () => {
    mockUsePathname.mockReturnValue("/operator/login");
    mockUseSession.mockReturnValue({
      data: null,
      status: "unauthenticated",
    });

    render(
      <OperatorAuthGuard>
        <div data-testid="test-content">Login Content</div>
      </OperatorAuthGuard>,
    );

    expect(screen.getByTestId("test-content")).toBeInTheDocument();
    expect(
      screen.queryByTestId("operator-shell-provider"),
    ).not.toBeInTheDocument();
    expect(screen.queryByTestId("app-shell")).not.toBeInTheDocument();
  });

  it("wraps children in shell and breadcrumb providers when not on login page", () => {
    mockUsePathname.mockReturnValue("/operator/suggestions");
    mockUseSession.mockReturnValue({
      data: { user: { scope: "platform", name: "Op" } },
      status: "authenticated",
    });

    render(
      <OperatorAuthGuard>
        <div data-testid="test-content">App Content</div>
      </OperatorAuthGuard>,
    );

    expect(screen.getByTestId("operator-shell-provider")).toBeInTheDocument();
    expect(screen.getByTestId("breadcrumb-provider")).toBeInTheDocument();
    expect(screen.getByTestId("app-shell")).toBeInTheDocument();
    expect(screen.getByTestId("test-content")).toBeInTheDocument();
  });

  it("handles announcements page", () => {
    mockUsePathname.mockReturnValue("/operator/announcements");
    mockUseSession.mockReturnValue({
      data: { user: { scope: "platform", name: "Op" } },
      status: "authenticated",
    });

    render(
      <OperatorAuthGuard>
        <div data-testid="test-content">Announcements</div>
      </OperatorAuthGuard>,
    );

    expect(screen.getByTestId("app-shell")).toBeInTheDocument();
    expect(screen.getByTestId("test-content")).toBeInTheDocument();
  });

  it("handles settings page", () => {
    mockUsePathname.mockReturnValue("/operator/settings");
    mockUseSession.mockReturnValue({
      data: { user: { scope: "platform", name: "Op" } },
      status: "authenticated",
    });

    render(
      <OperatorAuthGuard>
        <div data-testid="test-content">Settings</div>
      </OperatorAuthGuard>,
    );

    expect(screen.getByTestId("app-shell")).toBeInTheDocument();
    expect(screen.getByTestId("test-content")).toBeInTheDocument();
  });

  it("shows loading state while session is loading", () => {
    mockUsePathname.mockReturnValue("/operator/suggestions");
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
    });

    render(
      <OperatorAuthGuard>
        <div data-testid="test-content">Content</div>
      </OperatorAuthGuard>,
    );

    expect(screen.getByTestId("loading")).toBeInTheDocument();
    expect(screen.queryByTestId("app-shell")).not.toBeInTheDocument();
  });

  it("redirects to login when unauthenticated", () => {
    mockUsePathname.mockReturnValue("/operator/suggestions");
    mockUseSession.mockReturnValue({
      data: null,
      status: "unauthenticated",
    });

    render(
      <OperatorAuthGuard>
        <div data-testid="test-content">Content</div>
      </OperatorAuthGuard>,
    );

    expect(mockRedirect).toHaveBeenCalledWith("/operator/login");
  });

  it("redirects when authenticated but using the wrong scope", () => {
    mockUsePathname.mockReturnValue("/operator/suggestions");
    mockUseSession.mockReturnValue({
      data: { user: { scope: "teacher", name: "Teacher" } },
      status: "authenticated",
    });

    render(
      <OperatorAuthGuard>
        <div data-testid="test-content">Content</div>
      </OperatorAuthGuard>,
    );

    // Should redirect non-operator users away
    expect(mockRedirect).toHaveBeenCalledWith("/");
  });
});
