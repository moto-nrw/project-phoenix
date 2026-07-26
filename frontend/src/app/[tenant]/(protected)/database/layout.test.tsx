import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import DatabaseLayout from "./layout";

const mockUseSession = vi.fn();
vi.mock("next-auth/react", () => ({
  useSession: (...args: unknown[]) => mockUseSession(...args),
}));

const mockUseSelectedLayoutSegment = vi.fn(() => null as string | null);
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useSelectedLayoutSegment: () => mockUseSelectedLayoutSegment(),
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

vi.mock("~/components/database/master-detail-skeleton", () => ({
  MasterDetailSkeleton: () => (
    <div data-testid="master-detail-skeleton">Loading...</div>
  ),
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

  it("shows the index card-grid skeleton while session loads on the index page", () => {
    mockUseSelectedLayoutSegment.mockReturnValue(null);
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
    });

    render(
      <DatabaseLayout>
        <div>Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByTestId("database-index-skeleton")).toBeInTheDocument();
  });

  it("shows the master-detail skeleton while session loads on a sub-route", () => {
    mockUseSelectedLayoutSegment.mockReturnValue("students");
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
    });

    render(
      <DatabaseLayout>
        <div>Content</div>
      </DatabaseLayout>,
    );

    expect(screen.getByTestId("master-detail-skeleton")).toBeInTheDocument();
  });
});
