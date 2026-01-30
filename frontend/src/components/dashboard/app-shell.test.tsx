import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { AppShell } from "./app-shell";

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
  }),
  usePathname: () => "/dashboard",
}));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: {
      user: {
        id: "1",
        name: "Test User",
        email: "test@example.com",
        token: "valid-token",
        roles: ["admin"],
      },
    },
    status: "authenticated",
  })),
}));

vi.mock("./header", () => ({
  Header: () => <header data-testid="header">Header</header>,
}));

vi.mock("./sidebar", () => ({
  Sidebar: ({ className }: { className?: string }) => (
    <nav data-testid="sidebar" className={className}>
      Sidebar
    </nav>
  ),
}));

vi.mock("./mobile-bottom-nav", () => ({
  MobileBottomNav: () => <nav data-testid="mobile-nav">Mobile Nav</nav>,
}));

vi.mock("~/lib/profile-context", () => ({
  useProfile: () => ({
    profile: null,
    loading: false,
    error: null,
  }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  useSetBreadcrumb: vi.fn(),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

import { useSession } from "next-auth/react";

describe("AppShell", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders children content", () => {
    render(
      <AppShell>
        <div data-testid="child-content">Page Content</div>
      </AppShell>,
    );

    expect(screen.getByTestId("child-content")).toBeInTheDocument();
    expect(screen.getByText("Page Content")).toBeInTheDocument();
  });

  it("renders header, sidebar, and mobile nav", () => {
    render(
      <AppShell>
        <div>Content</div>
      </AppShell>,
    );

    expect(screen.getByTestId("header")).toBeInTheDocument();
    expect(screen.getByTestId("sidebar")).toBeInTheDocument();
    expect(screen.getByTestId("mobile-nav")).toBeInTheDocument();
  });

  it("hides sidebar on mobile via CSS class", () => {
    render(
      <AppShell>
        <div>Content</div>
      </AppShell>,
    );

    const sidebar = screen.getByTestId("sidebar");
    expect(sidebar.className).toContain("hidden");
    expect(sidebar.className).toContain("lg:block");
  });

  it("redirects to login when session has no token", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "3",
          name: "User",
          email: "user@example.com",
          token: "",
          roles: [],
        },
        expires: "",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(
      <AppShell>
        <div>Content</div>
      </AppShell>,
    );

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/");
    });
  });

  it("does not redirect when session is loading", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "loading",
      update: vi.fn(),
    });

    render(
      <AppShell>
        <div>Content</div>
      </AppShell>,
    );

    expect(mockPush).not.toHaveBeenCalled();
  });

  it("does not redirect when session has valid token", () => {
    render(
      <AppShell>
        <div>Content</div>
      </AppShell>,
    );

    expect(mockPush).not.toHaveBeenCalled();
  });

  it("renders main content area with correct classes", () => {
    render(
      <AppShell>
        <div data-testid="page">Page</div>
      </AppShell>,
    );

    const main = screen.getByRole("main");
    expect(main).toBeInTheDocument();
    expect(main.className).toContain("flex-1");
    expect(main.className).toContain("pb-24");
  });
});
