import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import DatabaseLayout from "./layout";

const mockUseSession = vi.fn();
vi.mock("next-auth/react", () => ({
  useSession: (...args: unknown[]) => mockUseSession(...args),
}));

const mockPathname = vi.fn(() => "/test-tenant/database/students");
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  usePathname: () => mockPathname(),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: (session: { user?: { isAdmin?: boolean } } | null) =>
    session?.user?.isAdmin ?? false,
  hasPermission: (
    session: { user?: { permissions?: string[] } } | null,
    permission: string,
  ) => session?.user?.permissions?.includes(permission) ?? false,
  hasEffectiveAdminScope: (session: { user?: { isAdmin?: boolean } } | null) =>
    session?.user?.isAdmin ?? false,
  hasRole: (
    session: { user?: { roles?: string[]; isAdmin?: boolean } } | null,
    role: string,
  ) => {
    if (role === "admin") return session?.user?.isAdmin ?? false;
    if (role === "user") return !(session?.user?.isAdmin ?? false);
    return false;
  },
}));

describe("DatabaseLayout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders children for admin users", () => {
    mockUseSession.mockReturnValue({
      data: { user: { isAdmin: true, token: "tok" } },
      status: "authenticated",
    });

    render(
      <DatabaseLayout>
        <div data-testid="database-content">Database Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByTestId("database-content")).toBeInTheDocument();
  });

  it("shows ForbiddenPage for non-admin users", () => {
    mockUseSession.mockReturnValue({
      data: { user: { isAdmin: false, token: "tok" } },
      status: "authenticated",
    });

    render(
      <DatabaseLayout>
        <div data-testid="database-content">Database Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Du verfügst nicht über die notwendigen Berechtigungen, um die Datenverwaltung aufzurufen.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("database-content")).not.toBeInTheDocument();
  });

  it("lets staff:manage without admin scope reach the personnel page (#2906)", () => {
    mockPathname.mockReturnValue("/test-tenant/database/personal");
    mockUseSession.mockReturnValue({
      data: {
        user: { isAdmin: false, token: "tok", permissions: ["staff:manage"] },
      },
      status: "authenticated",
    });

    render(
      <DatabaseLayout>
        <div data-testid="database-content">Database Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByTestId("database-content")).toBeInTheDocument();
  });

  it("lets staff:stammdaten without admin scope reach the personnel page (#2906)", () => {
    mockPathname.mockReturnValue("/test-tenant/database/personal");
    mockUseSession.mockReturnValue({
      data: {
        user: {
          isAdmin: false,
          token: "tok",
          permissions: ["staff:stammdaten"],
        },
      },
      status: "authenticated",
    });

    render(
      <DatabaseLayout>
        <div data-testid="database-content">Database Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByTestId("database-content")).toBeInTheDocument();
  });

  it("lets users:create reach the staff import (#2906)", () => {
    mockPathname.mockReturnValue("/test-tenant/database/personal/import");
    mockUseSession.mockReturnValue({
      data: {
        user: {
          isAdmin: false,
          token: "tok",
          permissions: ["staff:manage", "users:create"],
        },
      },
      status: "authenticated",
    });

    render(
      <DatabaseLayout>
        <div data-testid="database-content">Database Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByTestId("database-content")).toBeInTheDocument();
  });

  it("keeps the staff import closed without users:create (#2906)", () => {
    mockPathname.mockReturnValue("/test-tenant/database/personal/import");
    mockUseSession.mockReturnValue({
      data: {
        user: { isAdmin: false, token: "tok", permissions: ["staff:manage"] },
      },
      status: "authenticated",
    });

    render(
      <DatabaseLayout>
        <div data-testid="database-content">Database Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
    expect(screen.queryByTestId("database-content")).not.toBeInTheDocument();
  });

  it("lets time_tracking:manage reach the opening-balance import (#2906)", () => {
    mockPathname.mockReturnValue(
      "/test-tenant/database/personal/opening-balances",
    );
    mockUseSession.mockReturnValue({
      data: {
        user: {
          isAdmin: false,
          token: "tok",
          permissions: ["staff:manage", "time_tracking:manage"],
        },
      },
      status: "authenticated",
    });

    render(
      <DatabaseLayout>
        <div data-testid="database-content">Database Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByTestId("database-content")).toBeInTheDocument();
  });

  it("keeps the opening-balance import closed without time_tracking:manage (#2906)", () => {
    mockPathname.mockReturnValue(
      "/test-tenant/database/personal/opening-balances",
    );
    mockUseSession.mockReturnValue({
      data: {
        user: { isAdmin: false, token: "tok", permissions: ["staff:manage"] },
      },
      status: "authenticated",
    });

    render(
      <DatabaseLayout>
        <div data-testid="database-content">Database Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
    expect(screen.queryByTestId("database-content")).not.toBeInTheDocument();
  });

  it("keeps the rest of the database area closed for staff:manage (#2906)", () => {
    mockPathname.mockReturnValue("/test-tenant/database/students");
    mockUseSession.mockReturnValue({
      data: {
        user: { isAdmin: false, token: "tok", permissions: ["staff:manage"] },
      },
      status: "authenticated",
    });

    render(
      <DatabaseLayout>
        <div data-testid="database-content">Database Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
    expect(screen.queryByTestId("database-content")).not.toBeInTheDocument();
  });

  it("shows neutral progress while the session loads", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
    });

    render(
      <DatabaseLayout>
        <div>Content</div>
      </DatabaseLayout>,
    );

    const loading = screen.getByRole("status", {
      name: "Berechtigungen werden geprüft…",
    });
    expect(loading).toBeVisible();
    expect(loading).not.toHaveClass("fixed");
    expect(loading).toHaveClass("min-h-40");
  });
});
