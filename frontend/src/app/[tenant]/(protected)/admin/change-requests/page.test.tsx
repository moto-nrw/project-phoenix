import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import AdminChangeRequestsPage from "./page";
import { canReviewChangeRequests } from "~/lib/change-request-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));

// Die Seite liest die Sitzung, seit die Stammdaten-Warteschlangen an
// users:update hängen und users:absence allein nur die Entschuldigungen
// freischaltet (#2232). Ohne Provider wirft useSession, also wird es gemockt.
vi.mock("next-auth/react", () => ({
  useSession: (): ReturnType<typeof mockUseSession> => mockUseSession(),
}));

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: vi.fn(),
}));

vi.mock("~/components/students/master-data-review-list", () => ({
  MasterDataReviewList: () => <div>master-data-review-list</div>,
}));

vi.mock("~/components/students/care-request-review-list", () => ({
  CareRequestReviewList: () => <div>care-request-review-list</div>,
}));

vi.mock("~/components/students/offering-request-review-list", () => ({
  OfferingRequestReviewList: () => <div>offering-request-review-list</div>,
}));

vi.mock("~/components/students/excused-request-review-list", () => ({
  ExcusedRequestReviewList: () => <div>excused-request-review-list</div>,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div>loading</div>,
}));

const mockUseRequirePermission = vi.mocked(useRequirePermission);

/** Sitzung mit den angegebenen Rechten, ohne Admin-Rolle. */
function sessionWith(permissions: readonly string[]) {
  return {
    data: { user: { id: "7", roles: ["user"], permissions } },
    status: "authenticated" as const,
  };
}

describe("AdminChangeRequestsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSession.mockReturnValue(sessionWith(["users:update"]));
  });

  it("shows a loading state until the permission check is ready", () => {
    mockUseRequirePermission.mockReturnValue({
      isReady: false,
      isLoading: true,
    });

    render(<AdminChangeRequestsPage />);

    expect(screen.getByText("loading")).toBeInTheDocument();
  });

  it("renders both review queues once access is ready", () => {
    mockUseRequirePermission.mockReturnValue({
      isReady: true,
      isLoading: false,
    });

    render(<AdminChangeRequestsPage />);

    expect(
      screen.getByRole("heading", { name: "Änderungsanfragen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("master-data-review-list")).toBeInTheDocument();
    expect(screen.getByText("care-request-review-list")).toBeInTheDocument();
  });

  it("zeigt einer Person mit users:absence nur die Entschuldigungen", () => {
    mockUseRequirePermission.mockReturnValue({
      isReady: true,
      isLoading: false,
    });
    mockUseSession.mockReturnValue(
      sessionWith(["users:read", "users:absence"]),
    );

    render(<AdminChangeRequestsPage />);

    expect(screen.getByText("excused-request-review-list")).toBeInTheDocument();
    expect(screen.queryByText("master-data-review-list")).toBeNull();
    expect(screen.queryByText("care-request-review-list")).toBeNull();
    expect(screen.queryByText("offering-request-review-list")).toBeNull();
  });

  // Der Zugriff hängt an der geteilten Regel, nicht an einer zweiten
  // Aufzählung in der Seite: users:update ODER das Paar users:absence +
  // users:read, genau wie Sidebar, Eltern-Übersicht und Zähler-Badge.
  it("öffnet die Seite über canReviewChangeRequests", () => {
    mockUseRequirePermission.mockReturnValue({
      isReady: true,
      isLoading: false,
    });

    render(<AdminChangeRequestsPage />);

    expect(mockUseRequirePermission).toHaveBeenCalledWith(
      canReviewChangeRequests,
    );
  });
});
