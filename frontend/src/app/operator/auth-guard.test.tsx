import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

const { mockRedirect, mockUseSession, mockUsePathname } = vi.hoisted(() => ({
  mockRedirect: vi.fn(),
  mockUseSession: vi.fn(),
  mockUsePathname: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
  usePathname: mockUsePathname,
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("~/lib/operator-url", () => ({
  operatorPath: (path: string) => path,
}));

vi.mock("~/lib/shell-auth-context", () => ({
  OperatorShellProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="shell-provider">{children}</div>
  ),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="breadcrumb-provider">{children}</div>
  ),
}));

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

  it("renders children directly on public pages (/operator/login)", () => {
    mockUsePathname.mockReturnValue("/operator/login");
    mockUseSession.mockReturnValue({ data: null, status: "unauthenticated" });

    render(
      <OperatorAuthGuard>
        <div>Child</div>
      </OperatorAuthGuard>,
    );

    expect(screen.getByText("Child")).toBeInTheDocument();
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("renders children directly on /operator/email-confirm", () => {
    mockUsePathname.mockReturnValue("/operator/email-confirm");
    mockUseSession.mockReturnValue({ data: null, status: "unauthenticated" });

    render(
      <OperatorAuthGuard>
        <div>Child</div>
      </OperatorAuthGuard>,
    );

    expect(screen.getByText("Child")).toBeInTheDocument();
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("redirects unauthenticated users to login on protected pages", async () => {
    mockUsePathname.mockReturnValue("/operator/settings");
    mockUseSession.mockReturnValue({ data: null, status: "unauthenticated" });

    render(
      <OperatorAuthGuard>
        <div>Protected Content</div>
      </OperatorAuthGuard>,
    );

    await waitFor(() => {
      expect(mockRedirect).toHaveBeenCalledWith("/operator/login");
    });
  });

  it("redirects non-platform users to root", async () => {
    mockUsePathname.mockReturnValue("/operator/settings");
    mockUseSession.mockReturnValue({
      data: { user: { scope: "tenant" } },
      status: "authenticated",
    });

    render(
      <OperatorAuthGuard>
        <div>Protected Content</div>
      </OperatorAuthGuard>,
    );

    await waitFor(() => {
      expect(mockRedirect).toHaveBeenCalledWith("/");
    });
  });

  it("renders full shell for authenticated platform users", () => {
    mockUsePathname.mockReturnValue("/operator/settings");
    mockUseSession.mockReturnValue({
      data: { user: { scope: "platform" } },
      status: "authenticated",
    });

    render(
      <OperatorAuthGuard>
        <div>Dashboard Content</div>
      </OperatorAuthGuard>,
    );

    expect(screen.getByText("Dashboard Content")).toBeInTheDocument();
    expect(screen.getByTestId("shell-provider")).toBeInTheDocument();
    expect(screen.getByTestId("breadcrumb-provider")).toBeInTheDocument();
    expect(screen.getByTestId("app-shell")).toBeInTheDocument();
  });

  it("shows loading spinner while session is loading", () => {
    mockUsePathname.mockReturnValue("/operator/settings");
    mockUseSession.mockReturnValue({ data: null, status: "loading" });

    render(
      <OperatorAuthGuard>
        <div>Protected Content</div>
      </OperatorAuthGuard>,
    );

    expect(screen.getByTestId("loading")).toBeInTheDocument();
    expect(mockRedirect).not.toHaveBeenCalled();
    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
  });
});
