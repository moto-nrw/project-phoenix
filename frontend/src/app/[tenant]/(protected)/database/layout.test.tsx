import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import DatabaseLayout from "./layout";

const mockUseSession = vi.fn();
vi.mock("next-auth/react", () => ({
  useSession: (...args: unknown[]) => mockUseSession(...args),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: (session: { user?: { isAdmin?: boolean } } | null) =>
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
