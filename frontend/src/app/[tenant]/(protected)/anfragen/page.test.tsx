import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import AnfragenPage from "./page";
import { canOpenRequestsPage } from "~/lib/change-request-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));

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

vi.mock("~/components/students/request-history-list", () => ({
  MasterDataHistoryList: () => <div>master-data-history-list</div>,
  CareRequestHistoryList: () => <div>care-request-history-list</div>,
  OfferingRequestHistoryList: () => <div>offering-request-history-list</div>,
  ExcusedRequestHistoryList: () => <div>excused-request-history-list</div>,
}));

// NavigationTabs beobachtet seine Scroll-Breite; jsdom kennt ResizeObserver
// nicht (gleiches Muster wie die Master-Detail-Tests).
class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal("ResizeObserver", MockResizeObserver);

const mockUseRequirePermission = vi.mocked(useRequirePermission);

/** Sitzung mit den angegebenen Rechten, ohne Admin-Rolle. */
function sessionWith(permissions: readonly string[]) {
  return {
    data: { user: { id: "7", roles: ["user"], permissions } },
    status: "authenticated" as const,
  };
}

const PLACEHOLDER_TITLE = "Anträge von Mitarbeitenden ziehen bald hierhin um";

describe("AnfragenPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseRequirePermission.mockReturnValue({
      isReady: true,
      isLoading: false,
    });
    mockUseSession.mockReturnValue(sessionWith(["users:update"]));
  });

  it("shows a loading state until the permission check is ready", () => {
    mockUseRequirePermission.mockReturnValue({
      isReady: false,
      isLoading: true,
    });

    render(<AnfragenPage />);

    expect(
      screen.getByLabelText("Anfragen werden geladen…"),
    ).toBeInTheDocument();
  });

  // Der Zugriff hängt an der geteilten Regel, nicht an einer zweiten
  // Aufzählung in der Seite — dieselbe Regel tragen Sidebar-Eintrag, mobile
  // Navigation und Zähler-Badge.
  it("öffnet die Seite über canOpenRequestsPage", () => {
    render(<AnfragenPage />);

    expect(mockUseRequirePermission).toHaveBeenCalledWith(canOpenRequestsPage);
  });

  it("renders both review queues once access is ready", () => {
    render(<AnfragenPage />);

    expect(
      screen.getByRole("heading", { name: "Anfragen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("master-data-review-list")).toBeInTheDocument();
    expect(screen.getByText("care-request-review-list")).toBeInTheDocument();
  });

  it("zeigt einer Person mit users:absence nur die Entschuldigungen", () => {
    mockUseSession.mockReturnValue(
      sessionWith(["users:read", "users:absence"]),
    );

    render(<AnfragenPage />);

    expect(screen.getByText("excused-request-review-list")).toBeInTheDocument();
    expect(screen.queryByText("master-data-review-list")).toBeNull();
    expect(screen.queryByText("care-request-review-list")).toBeNull();
    expect(screen.queryByText("offering-request-review-list")).toBeNull();
  });

  it("tauscht per Historie-Schalter alle vier Sektionen gegen die Historie", () => {
    render(<AnfragenPage />);

    fireEvent.click(screen.getByRole("button", { name: "Historie" }));

    expect(screen.getByText("master-data-history-list")).toBeInTheDocument();
    expect(screen.getByText("care-request-history-list")).toBeInTheDocument();
    expect(
      screen.getByText("offering-request-history-list"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("excused-request-history-list"),
    ).toBeInTheDocument();
    expect(screen.queryByText("master-data-review-list")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Offen" }));
    expect(screen.getByText("master-data-review-list")).toBeInTheDocument();
    expect(screen.queryByText("master-data-history-list")).toBeNull();
  });

  // Reiter-Sichtbarkeit nach Berechtigung (#2429): der Mitarbeitende-Reiter
  // hängt an vacation:approve, der Eltern-Reiter an canReviewChangeRequests.
  it("versteckt den Mitarbeitende-Reiter ohne vacation:approve", () => {
    render(<AnfragenPage />);

    expect(screen.queryByRole("button", { name: "Mitarbeitende" })).toBeNull();
    expect(screen.queryByText(PLACEHOLDER_TITLE)).toBeNull();
  });

  it("zeigt mit vacation:approve beide Reiter und wechselt zum Platzhalter", () => {
    mockUseSession.mockReturnValue(
      sessionWith(["users:update", "vacation:approve"]),
    );

    render(<AnfragenPage />);

    // Eltern-Reiter ist voreingestellt aktiv.
    expect(screen.getByText("master-data-review-list")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Mitarbeitende" }));

    expect(screen.getByText(PLACEHOLDER_TITLE)).toBeInTheDocument();
    expect(screen.queryByText("master-data-review-list")).toBeNull();
  });

  it("zeigt mit nur vacation:approve den Platzhalter ohne Reiterleiste", () => {
    mockUseSession.mockReturnValue(sessionWith(["vacation:approve"]));

    render(<AnfragenPage />);

    expect(screen.getByText(PLACEHOLDER_TITLE)).toBeInTheDocument();
    // Nur ein sichtbarer Reiter → keine Reiterleiste, kein Eltern-Inhalt.
    expect(screen.queryByRole("button", { name: "Eltern" })).toBeNull();
    expect(screen.queryByText("excused-request-review-list")).toBeNull();
    expect(screen.queryByRole("button", { name: "Historie" })).toBeNull();
  });

  it("zeigt Admins beide Reiter", () => {
    mockUseSession.mockReturnValue({
      data: { user: { id: "1", roles: ["admin"], permissions: ["admin:*"] } },
      status: "authenticated" as const,
    });

    render(<AnfragenPage />);

    expect(
      screen.getByRole("button", { name: "Mitarbeitende" }),
    ).toBeInTheDocument();
    expect(screen.getByText("master-data-review-list")).toBeInTheDocument();
  });
});
