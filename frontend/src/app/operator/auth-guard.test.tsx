import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { useState } from "react";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { resumeSWRSession } from "~/lib/swr/scope";
import { render, screen, waitFor, act } from "@testing-library/react";

vi.unmock("swr");
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: vi.fn() }));

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

afterEach(() => resumeSWRSession());

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
      data: { user: { id: "operator-a", scope: "platform" } },
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

  it.each(["account", "other-tab-account", "other-tab-logout"])(
    "discards local modal data on %s before navigation completes",
    (change) => {
      mockUsePathname.mockReturnValue("/operator/accounts");
      mockUseSession.mockReturnValue({
        data: { user: { id: "operator-a", scope: "platform" } },
        status: "authenticated",
      });
      function LocalModal() {
        const [owner] = useState(() => mockUseSession().data.user.id);
        return <p>Dialog von {owner}</p>;
      }
      function Page() {
        return (
          <>
            <SWRScopeBoundary />
            <OperatorAuthGuard>
              <LocalModal />
            </OperatorAuthGuard>
          </>
        );
      }
      const view = render(<Page />);
      expect(screen.getByText("Dialog von operator-a")).toBeInTheDocument();
      act(() => {
        if (change === "account") {
          mockUseSession.mockReturnValue({
            data: { user: { id: "operator-b", scope: "platform" } },
            status: "authenticated",
          });
          view.rerender(<Page />);
        } else {
          globalThis.dispatchEvent(
            new StorageEvent("storage", {
              key: "moto:personal-storage-owner",
              newValue: change.endsWith("logout")
                ? null
                : "platform:operator-b",
            }),
          );
        }
      });
      // Anker: either the new account's dialog or the tab's explicit loading state.
      expect(
        screen.getByText(
          change === "account" ? "Dialog von operator-b" : "Loading...",
        ),
      ).toBeInTheDocument();
      expect(
        screen.queryByText("Dialog von operator-a"),
      ).not.toBeInTheDocument();
    },
  );

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
