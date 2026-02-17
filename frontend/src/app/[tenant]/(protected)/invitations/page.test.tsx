/* eslint-disable @typescript-eslint/no-unsafe-return */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import InvitationsPage from "./page";

const mockUseSession = vi.fn();
vi.mock("next-auth/react", () => ({
  useSession: (...args: unknown[]) => mockUseSession(...args),
}));

const mockRedirect = vi.fn();
vi.mock("next/navigation", () => ({
  redirect: (url: string) => mockRedirect(url),
}));

vi.mock("~/components/admin/invitation-form", () => ({
  InvitationForm: ({ onCreated }: { onCreated: () => void }) => (
    <div data-testid="invitation-form">
      <button onClick={onCreated}>Create</button>
    </div>
  ),
}));

vi.mock("~/components/admin/pending-invitations-list", () => ({
  PendingInvitationsList: ({ refreshKey }: { refreshKey: number }) => (
    <div data-testid="pending-list" data-refresh-key={refreshKey}>
      Pending
    </div>
  ),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: (session: { user?: { roles?: string[] } }) =>
    session?.user?.roles?.includes("admin") ?? false,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

describe("InvitationsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading state", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
    });

    render(<InvitationsPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("shows access denied for non-admin users", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "1",
          name: "User",
          email: "user@test.com",
          token: "tok",
          roles: ["teacher"],
        },
      },
      status: "authenticated",
    });

    render(<InvitationsPage />);

    expect(screen.getByText("Keine Berechtigung")).toBeInTheDocument();
  });

  it("renders invitation form and pending list for admin users", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "1",
          name: "Admin",
          email: "admin@test.com",
          token: "tok",
          roles: ["admin"],
        },
      },
      status: "authenticated",
    });

    render(<InvitationsPage />);

    expect(screen.getByTestId("invitation-form")).toBeInTheDocument();
    expect(screen.getByTestId("pending-list")).toBeInTheDocument();
  });

  it("shows no session access denied", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "authenticated",
    });

    render(<InvitationsPage />);

    expect(screen.getByText("Keine Berechtigung")).toBeInTheDocument();
  });
});
