import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Announcement } from "~/lib/parent-announcements-api";
import {
  buildAnnouncementMenuItems,
  DeleteAnnouncementDialog,
  PublishAnnouncementDialog,
  UnpublishAnnouncementDialog,
} from "./announcement-lifecycle-dialogs";

const { publishMock, unpublishMock, deleteMock } = vi.hoisted(() => ({
  publishMock: vi.fn(),
  unpublishMock: vi.fn(),
  deleteMock: vi.fn(),
}));

vi.mock("~/lib/parent-announcements-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/parent-announcements-api")>();
  return {
    ...actual,
    publishAnnouncement: publishMock,
    unpublishAnnouncement: unpublishMock,
    deleteAnnouncement: deleteMock,
  };
});

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    title,
    children,
    onConfirm,
    onClose,
  }: {
    title: string;
    children: React.ReactNode;
    onConfirm: () => void;
    onClose: () => void;
  }) => (
    <div role="dialog" aria-label={title}>
      {children}
      <button type="button" onClick={onConfirm}>
        Bestätigen
      </button>
      <button type="button" onClick={onClose}>
        Abbrechen
      </button>
    </div>
  ),
}));

vi.mock("~/components/ui/confirm-delete-modal", () => ({
  ConfirmDeleteModal: ({
    title,
    onConfirm,
    error,
  }: {
    title: string;
    onConfirm: () => Promise<void>;
    error?: string;
  }) => (
    <div role="dialog" aria-label={title}>
      {error ? <p role="alert">{error}</p> : null}
      <button type="button" onClick={() => void onConfirm()}>
        Endgültig löschen
      </button>
    </div>
  ),
}));

const announcement: Announcement = {
  id: "7",
  title: "Sommerfest",
  body: "Am Freitag.",
  priority: "info",
  requires_acknowledgement: false,
  send_email: true,
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

describe("lifecycle dialogs", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    publishMock.mockResolvedValue(announcement);
    unpublishMock.mockResolvedValue(announcement);
    deleteMock.mockResolvedValue(undefined);
  });

  it("publishes, refreshes and closes", async () => {
    const onDone = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    render(
      <PublishAnnouncementDialog
        announcement={announcement}
        onClose={onClose}
        onDone={onDone}
      />,
    );

    expect(screen.getByText(/Ganze Schule/)).toBeInTheDocument();
    expect(
      screen.getByText(/zusätzlich per E-Mail benachrichtigt/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(publishMock).toHaveBeenCalledWith("7");
    expect(onDone).toHaveBeenCalled();
  });

  it("shows the backend error and stays open when publishing fails", async () => {
    publishMock.mockRejectedValue(new Error("Keine Empfänger"));
    const onClose = vi.fn();
    render(
      <PublishAnnouncementDialog
        announcement={announcement}
        onClose={onClose}
        onDone={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() =>
      expect(screen.getByText("Keine Empfänger")).toBeInTheDocument(),
    );
    expect(onClose).not.toHaveBeenCalled();
  });

  it("unpublishes through the service", async () => {
    const onDone = vi.fn();
    render(
      <UnpublishAnnouncementDialog
        announcement={{ ...announcement, status: "published" }}
        onClose={vi.fn()}
        onDone={onDone}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => expect(unpublishMock).toHaveBeenCalledWith("7"));
    expect(onDone).toHaveBeenCalled();
  });

  it("deletes through the service and surfaces errors in the dialog", async () => {
    deleteMock.mockRejectedValueOnce(new Error("Nicht erlaubt"));
    const onDone = vi.fn();
    render(
      <DeleteAnnouncementDialog
        announcement={announcement}
        onClose={vi.fn()}
        onDone={onDone}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("Nicht erlaubt"),
    );
    expect(onDone).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));
    await waitFor(() => expect(onDone).toHaveBeenCalled());
    expect(deleteMock).toHaveBeenCalledWith("7");
  });
});

describe("buildAnnouncementMenuItems", () => {
  const handlers = {
    onPublish: vi.fn(),
    onEdit: vi.fn(),
    onUnpublish: vi.fn(),
    onDelete: vi.fn(),
  };

  it("offers publish, edit and delete for a draft (plus Anzeigen in the list)", () => {
    expect(
      buildAnnouncementMenuItems(announcement, {
        ...handlers,
        onView: vi.fn(),
      }).map((item) => item.label),
    ).toEqual(["Veröffentlichen", "Anzeigen", "Bearbeiten", "Löschen"]);
    expect(
      buildAnnouncementMenuItems(announcement, handlers).map(
        (item) => item.label,
      ),
    ).toEqual(["Veröffentlichen", "Bearbeiten", "Löschen"]);
  });

  it("offers unpublish and delete for a published announcement", () => {
    expect(
      buildAnnouncementMenuItems(
        { ...announcement, status: "published" },
        handlers,
      ).map((item) => item.label),
    ).toEqual(["Zurückziehen", "Löschen"]);
  });

  it("keeps system rows read-only", () => {
    expect(
      buildAnnouncementMenuItems(
        {
          ...announcement,
          status: "published",
          system_kind: "care_cancellation",
        },
        handlers,
      ),
    ).toEqual([]);
  });
});
