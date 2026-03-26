import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { RoleGuard } from "./role-guard";

const mockUseSession = vi.fn();
vi.mock("next-auth/react", () => ({
  useSession: (...args: unknown[]) => mockUseSession(...args),
}));

const mockRedirect = vi.fn();
vi.mock("next/navigation", () => ({
  redirect: (url: string) => mockRedirect(url),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: (session: { user?: { isAdmin?: boolean } } | null) =>
    session?.user?.isAdmin ?? false,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

describe("RoleGuard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading state while session loads", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
    });

    render(
      <RoleGuard variant="adminOnly">
        <div>Content</div>
      </RoleGuard>,
    );

    expect(screen.getByTestId("loading")).toBeInTheDocument();
    expect(screen.queryByText("Content")).not.toBeInTheDocument();
  });

  it("renders children for admin on adminOnly", () => {
    mockUseSession.mockReturnValue({
      data: { user: { isAdmin: true, token: "tok" } },
      status: "authenticated",
    });

    render(
      <RoleGuard variant="adminOnly">
        <div>Admin Content</div>
      </RoleGuard>,
    );

    expect(screen.getByText("Admin Content")).toBeInTheDocument();
  });

  it("shows ForbiddenPage for non-admin on adminOnly", () => {
    mockUseSession.mockReturnValue({
      data: { user: { isAdmin: false, token: "tok" } },
      status: "authenticated",
    });

    render(
      <RoleGuard variant="adminOnly">
        <div>Admin Content</div>
      </RoleGuard>,
    );

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
    expect(screen.queryByText("Admin Content")).not.toBeInTheDocument();
  });

  it("renders children for non-admin on staffOnly", () => {
    mockUseSession.mockReturnValue({
      data: { user: { isAdmin: false, token: "tok" } },
      status: "authenticated",
    });

    render(
      <RoleGuard variant="staffOnly">
        <div>Staff Content</div>
      </RoleGuard>,
    );

    expect(screen.getByText("Staff Content")).toBeInTheDocument();
  });

  it("shows ForbiddenPage for admin on staffOnly", () => {
    mockUseSession.mockReturnValue({
      data: { user: { isAdmin: true, token: "tok" } },
      status: "authenticated",
    });

    render(
      <RoleGuard variant="staffOnly">
        <div>Staff Content</div>
      </RoleGuard>,
    );

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
    expect(screen.queryByText("Staff Content")).not.toBeInTheDocument();
  });

  it("shows custom message on ForbiddenPage", () => {
    mockUseSession.mockReturnValue({
      data: { user: { isAdmin: false, token: "tok" } },
      status: "authenticated",
    });

    render(
      <RoleGuard variant="adminOnly" message="Nur für Administratoren.">
        <div>Content</div>
      </RoleGuard>,
    );

    expect(screen.getByText("Nur für Administratoren.")).toBeInTheDocument();
  });
});
