import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import { ApiError } from "~/lib/api-error";
import type { Announcement } from "~/lib/parent-announcements-api";
import { catalogText } from "~/test/error-catalog-text";
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
    error?: FormErrorInput;
  }) => (
    // Wie der echte Dialog: der Fehler steht im Dialog.
    <div role="dialog" aria-label={title}>
      <FormErrorAlert message={error} />
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

  // #2517: Katalogtext statt Serversatz, Wiederholen im Dialog.
  it("shows the catalog text and stays open when publishing fails", async () => {
    publishMock
      .mockRejectedValueOnce(
        new ApiError("publish exploded", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce(announcement);
    const onClose = vi.fn();
    render(
      <PublishAnnouncementDialog
        announcement={announcement}
        onClose={onClose}
        onDone={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    expect(
      await screen.findByText(
        catalogText("general.server", "das Veröffentlichen der Mitteilung"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/publish exploded/)).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(publishMock).toHaveBeenCalledTimes(2);
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
    deleteMock.mockRejectedValueOnce(
      new ApiError("not allowed", 403, { code: "general.permission" }),
    );
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
      expect(screen.getByRole("alert")).toHaveTextContent(
        catalogText("general.permission", "das Löschen der Mitteilung"),
      ),
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

describe("buildAnnouncementMenuItems: scheduled reminder (#3162)", () => {
  const handlers = {
    onPublish: vi.fn(),
    onEdit: vi.fn(),
    onUnpublish: vi.fn(),
    onDelete: vi.fn(),
    onReminder: vi.fn(),
  };

  it("offers to plan a reminder on a published Mitteilung", () => {
    expect(
      buildAnnouncementMenuItems(
        { ...announcement, status: "published" },
        handlers,
      ).map((item) => item.label),
    ).toEqual(["Erinnerung planen", "Zurückziehen", "Löschen"]);
  });

  it("offers to change an unsent reminder and hides it once sent", () => {
    expect(
      buildAnnouncementMenuItems(
        {
          ...announcement,
          status: "published",
          reminder_at: "2026-09-24T06:00:00Z",
        },
        handlers,
      ).map((item) => item.label),
    ).toEqual(["Erinnerung ändern", "Zurückziehen", "Löschen"]);
    expect(
      buildAnnouncementMenuItems(
        {
          ...announcement,
          status: "published",
          reminder_at: "2026-09-08T06:00:00Z",
          reminder_sent_at: "2026-09-08T06:02:00Z",
        },
        handlers,
      ).map((item) => item.label),
    ).toEqual(["Zurückziehen", "Löschen"]);
  });

  it("never offers it for drafts, polls or system rows", () => {
    expect(
      buildAnnouncementMenuItems(announcement, handlers).map(
        (item) => item.label,
      ),
    ).toEqual(["Veröffentlichen", "Bearbeiten", "Löschen"]);
    expect(
      buildAnnouncementMenuItems(
        {
          ...announcement,
          status: "published",
          response_type: "single_choice",
        },
        handlers,
      ).map((item) => item.label),
    ).toEqual(["Zurückziehen", "Löschen"]);
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

describe("deleting an Einverständnis with answers (#3430)", () => {
  it("explains that it must stay and that withdrawing still works", async () => {
    deleteMock.mockRejectedValue(
      new ApiError("declaration has submissions", 409, {
        code: "communication.declaration_has_submissions",
      }),
    );
    const onClose = vi.fn();
    render(
      <DeleteAnnouncementDialog
        announcement={{ ...announcement, delivery_mode: "declaration" }}
        onClose={onClose}
        onDone={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));

    // Der eigene Katalogtext des Codes, nicht der Serversatz.
    expect(await screen.findByRole("alert")).toHaveTextContent(
      catalogText(
        "communication.declaration_has_submissions",
        "das Löschen der Mitteilung",
      ),
    );
    expect(
      screen.queryByText(/declaration has submissions/),
    ).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });
});
