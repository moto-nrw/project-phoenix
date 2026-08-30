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
      data: { user: { roles: ["admin"], token: "tok" } },
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
      data: { user: { roles: ["user"], token: "tok" } },
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

  it("renders children for caregiver on staffOnly", () => {
    mockUseSession.mockReturnValue({
      data: { user: { roles: ["user"], token: "tok" } },
      status: "authenticated",
    });

    render(
      <RoleGuard variant="staffOnly">
        <div>Staff Content</div>
      </RoleGuard>,
    );

    expect(screen.getByText("Staff Content")).toBeInTheDocument();
  });

  it("renders children for teacher-only account on staffOnly", () => {
    mockUseSession.mockReturnValue({
      data: { user: { roles: ["teacher"], token: "tok" } },
      status: "authenticated",
    });

    render(
      <RoleGuard variant="staffOnly">
        <div>Staff Content</div>
      </RoleGuard>,
    );

    expect(screen.getByText("Staff Content")).toBeInTheDocument();
  });

  it("renders children for dual-role account on staffOnly", () => {
    mockUseSession.mockReturnValue({
      data: { user: { roles: ["admin", "user"], token: "tok" } },
      status: "authenticated",
    });

    render(
      <RoleGuard variant="staffOnly">
        <div>Staff Content</div>
      </RoleGuard>,
    );

    expect(screen.getByText("Staff Content")).toBeInTheDocument();
  });

  it("shows ForbiddenPage for admin-only account on staffOnly", () => {
    mockUseSession.mockReturnValue({
      data: { user: { roles: ["admin"], token: "tok" } },
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

  it("renders children for an admin-only account on staffOrAdmin", () => {
    mockUseSession.mockReturnValue({
      data: { user: { roles: ["admin"], token: "tok" } },
      status: "authenticated",
    });

    render(
      <RoleGuard variant="staffOrAdmin">
        <div>Group Content</div>
      </RoleGuard>,
    );

    expect(screen.getByText("Group Content")).toBeInTheDocument();
  });

  it("shows custom message on ForbiddenPage", () => {
    mockUseSession.mockReturnValue({
      data: { user: { roles: ["user"], token: "tok" } },
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
