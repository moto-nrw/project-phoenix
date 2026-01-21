import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ResponsiveLayout from "./responsive-layout";

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
  Header: ({
    userName,
    userEmail,
    userRole,
    customPageTitle,
  }: {
    userName?: string;
    userEmail?: string;
    userRole?: string;
    customPageTitle?: string;
  }) => (
    <header data-testid="header">
      <span data-testid="header-name">{userName}</span>
      <span data-testid="header-email">{userEmail}</span>
      <span data-testid="header-role">{userRole}</span>
      <span data-testid="header-title">{customPageTitle}</span>
    </header>
  ),
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

import { useSession } from "next-auth/react";

describe("ResponsiveLayout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders children content", () => {
    render(
      <ResponsiveLayout>
        <div data-testid="child-content">Page Content</div>
      </ResponsiveLayout>,
    );

    expect(screen.getByTestId("child-content")).toBeInTheDocument();
    expect(screen.getByText("Page Content")).toBeInTheDocument();
  });

  it("renders header component", () => {
    render(
      <ResponsiveLayout>
        <div>Content</div>
      </ResponsiveLayout>,
    );

    expect(screen.getByTestId("header")).toBeInTheDocument();
  });

  it("renders sidebar component", () => {
    render(
      <ResponsiveLayout>
        <div>Content</div>
      </ResponsiveLayout>,
    );

    expect(screen.getByTestId("sidebar")).toBeInTheDocument();
  });

  it("renders mobile navigation", () => {
    render(
      <ResponsiveLayout>
        <div>Content</div>
      </ResponsiveLayout>,
    );

    expect(screen.getByTestId("mobile-nav")).toBeInTheDocument();
  });

  it("passes pageTitle to header", () => {
    render(
      <ResponsiveLayout pageTitle="Custom Title">
        <div>Content</div>
      </ResponsiveLayout>,
    );

    expect(screen.getByTestId("header-title")).toHaveTextContent("Custom Title");
  });

  it("passes user info from session to header", () => {
    render(
      <ResponsiveLayout>
        <div>Content</div>
      </ResponsiveLayout>,
    );

    expect(screen.getByTestId("header-name")).toHaveTextContent("Test User");
    expect(screen.getByTestId("header-email")).toHaveTextContent(
      "test@example.com",
    );
  });

  it("shows Admin role for admin users", () => {
    render(
      <ResponsiveLayout>
        <div>Content</div>
      </ResponsiveLayout>,
    );

    expect(screen.getByTestId("header-role")).toHaveTextContent("Admin");
  });

  it("shows Betreuer role for non-admin users", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "2",
          name: "Regular User",
          email: "user@example.com",
          token: "valid-token",
          roles: ["betreuer"],
        },
        expires: "",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(
      <ResponsiveLayout>
        <div>Content</div>
      </ResponsiveLayout>,
    );

    expect(screen.getByTestId("header-role")).toHaveTextContent("Betreuer");
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
      <ResponsiveLayout>
        <div>Content</div>
      </ResponsiveLayout>,
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
      <ResponsiveLayout>
        <div>Content</div>
      </ResponsiveLayout>,
    );

    expect(mockPush).not.toHaveBeenCalled();
  });

  it("hides sidebar on mobile (using class)", () => {
    render(
      <ResponsiveLayout>
        <div>Content</div>
      </ResponsiveLayout>,
    );

    const sidebar = screen.getByTestId("sidebar");
    expect(sidebar.className).toContain("hidden");
    expect(sidebar.className).toContain("lg:block");
  });
});
