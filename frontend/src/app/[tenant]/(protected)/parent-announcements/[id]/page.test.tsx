import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Announcement } from "~/lib/parent-announcements-api";
import AnnouncementDetailPage from "./page";

const {
  searchParams,
  pushMock,
  tenantMutateMock,
  swrState,
  detailMutateMock,
  deleteMock,
  setBreadcrumbMock,
} = vi.hoisted(() => ({
  searchParams: new URLSearchParams(),
  pushMock: vi.fn(),
  tenantMutateMock: vi.fn(() => Promise.resolve()),
  swrState: {
    data: undefined as Announcement | undefined,
    isLoading: false,
    error: null as Error | null,
  },
  detailMutateMock: vi.fn(() => Promise.resolve()),
  deleteMock: vi.fn(() => Promise.resolve()),
  setBreadcrumbMock: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7" }),
  useSearchParams: () => searchParams,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: pushMock, replace: vi.fn(), back: vi.fn() }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => null,
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: () => "subdomain",
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: (data: unknown) => setBreadcrumbMock(data),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) =>
    key?.startsWith("parent-announcement-")
      ? { ...swrState, mutate: detailMutateMock }
      : { data: [], isLoading: false, error: null, mutate: vi.fn() },
  useTenantMutate: () => tenantMutateMock,
}));

vi.mock("~/lib/parent-announcements-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/parent-announcements-api")>();
  return {
    ...actual,
    fetchAnnouncement: vi.fn(),
    deleteAnnouncement: deleteMock,
  };
});

vi.mock("~/components/announcements/announcement-detail", () => ({
  AnnouncementDetail: ({
    announcement,
    onReminded,
  }: {
    announcement: Announcement;
    onReminded: (count: number) => void;
  }) => (
    <div data-testid="announcement-detail">
      {announcement.body}
      <button type="button" onClick={() => onReminded(3)}>
        Erinnern
      </button>
    </div>
  ),
  AnnouncementDetailSkeleton: () => (
    <div data-testid="announcement-detail-skeleton" />
  ),
}));

vi.mock("~/components/ui/confirm-delete-modal", () => ({
  ConfirmDeleteModal: ({
    isOpen,
    onConfirm,
  }: {
    isOpen: boolean;
    onConfirm: () => Promise<void>;
  }) =>
    isOpen ? (
      <button type="button" onClick={() => void onConfirm()}>
        Endgültig löschen
      </button>
    ) : null,
}));

const announcement: Announcement = {
  id: "7",
  title: "Sommerfest am Freitag",
  body: "Bitte Kuchen mitbringen.",
  priority: "important",
  requires_acknowledgement: true,
  send_email: false,
  status: "draft",
  active: true,
  created_at: "2026-09-01T10:00:00Z",
  updated_at: "2026-09-01T10:00:00Z",
  targets: [{ target_type: "school_all" }],
  response_type: "none",
  options: [],
  delivery_mode: "standard",
  email_audience: "portal_only",
};

describe("AnnouncementDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParams.delete("from");
    swrState.data = announcement;
    swrState.isLoading = false;
    swrState.error = null;
  });

  it("shows the announcement as a page with status, badge and actions", () => {
    render(<AnnouncementDetailPage />);

    expect(
      screen.getByRole("heading", { name: "Sommerfest am Freitag" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Noch nicht veröffentlicht")).toBeInTheDocument();
    expect(screen.getByText("Entwurf")).toBeInTheDocument();
    expect(screen.getByText("Wichtig")).toBeInTheDocument();
    expect(screen.getByTestId("announcement-detail")).toHaveTextContent(
      "Bitte Kuchen mitbringen.",
    );
    expect(setBreadcrumbMock).toHaveBeenCalledWith(
      expect.objectContaining({
        announcementTitle: "Sommerfest am Freitag",
        referrerPage: "/parent-announcements?art=mitteilungen",
      }),
    );
  });

  it("leads back to the tab it was opened from", () => {
    searchParams.set("from", "/parent-announcements?art=umfragen");
    render(<AnnouncementDetailPage />);

    // Der Rückweg ist der Knopf der Kopfzeile (auf dem Telefon sichtbar);
    // er trägt die Beschriftung der Art und führt auf den Herkunftsreiter.
    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zu den Mitteilungen" }),
    );
    expect(pushMock).toHaveBeenCalledWith("/parent-announcements?art=umfragen");
  });

  it("falls back to the collection for an external referrer", () => {
    searchParams.set("from", "//attacker.example");
    render(<AnnouncementDetailPage />);

    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zu den Mitteilungen" }),
    );

    expect(pushMock).toHaveBeenCalledWith(
      "/parent-announcements?art=mitteilungen",
    );
  });

  it("sends Bearbeiten to the list with the wizard request", () => {
    render(<AnnouncementDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Bearbeiten" }));

    expect(pushMock).toHaveBeenCalledWith(
      "/parent-announcements?art=mitteilungen&bearbeiten=7",
    );
  });

  it("deletes after confirmation and returns to the referrer", async () => {
    searchParams.set("from", "/parent-announcements?art=mitteilungen");
    render(<AnnouncementDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));

    await waitFor(() => expect(deleteMock).toHaveBeenCalledWith("7"));
    await waitFor(() =>
      expect(pushMock).toHaveBeenCalledWith(
        "/parent-announcements?art=mitteilungen",
      ),
    );
    expect(tenantMutateMock).toHaveBeenCalledWith("parent-announcements-list");
  });

  it("keeps system rows read-only", () => {
    swrState.data = {
      ...announcement,
      status: "published",
      system_kind: "care_cancellation",
      priority: "info",
    };
    render(<AnnouncementDetailPage />);

    expect(screen.getByText("Ausfall")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
  });

  it("shows the reminder confirmation on the page after a poll reminder", () => {
    swrState.data = {
      ...announcement,
      status: "published",
      response_type: "single_choice",
      options: [{ id: "1", label: "Ja" }],
    };
    render(<AnnouncementDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Erinnern" }));

    expect(
      screen.getByText("3 Eltern wurden an die offene Umfrage erinnert."),
    ).toBeInTheDocument();
  });

  it("renders the loading page while the announcement loads", () => {
    swrState.data = undefined;
    swrState.isLoading = true;
    render(<AnnouncementDetailPage />);

    expect(
      screen.getByTestId("announcement-detail-skeleton"),
    ).toBeInTheDocument();
  });

  it("renders the error state when loading fails", () => {
    swrState.data = undefined;
    swrState.error = new Error("boom");
    render(<AnnouncementDetailPage />);

    expect(
      screen.getByText("Elternmitteilung konnte nicht geladen werden."),
    ).toBeInTheDocument();
  });

  it("renders not found when the backend has no such announcement", () => {
    swrState.data = undefined;
    render(<AnnouncementDetailPage />);

    expect(
      screen.getByText("Elternmitteilung nicht gefunden."),
    ).toBeInTheDocument();
  });
});
