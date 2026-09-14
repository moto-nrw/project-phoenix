import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Announcement } from "~/lib/parent-announcements-api";
import {
  AnnouncementReminderDialog,
  reminderError,
} from "./announcement-reminder-dialog";

// The test clock is 2026-09-09 12:00 Berlin (setup-common).

const { updateMock } = vi.hoisted(() => ({
  updateMock: vi.fn(),
}));

vi.mock("~/lib/parent-announcements-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/parent-announcements-api")>();
  return { ...actual, updateAnnouncementReminder: updateMock };
});

vi.mock("~/components/ui/confirm-delete-modal", () => ({
  ConfirmDeleteModal: ({
    isOpen,
    title,
    onConfirm,
    error,
  }: {
    isOpen: boolean;
    title: string;
    onConfirm: () => Promise<void> | void;
    error?: string;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        {error ? <p role="alert">{error}</p> : null}
        <button type="button" onClick={() => void onConfirm()}>
          Entfernen
        </button>
      </div>
    ) : null,
}));

const published: Announcement = {
  id: "7",
  title: "Betreuung endet um 13:00 Uhr",
  body: "Am letzten Schultag endet die Betreuung um 13:00 Uhr.",
  priority: "info",
  requires_acknowledgement: false,
  send_email: true,
  status: "published",
  published_at: "2026-09-02T10:00:00Z",
  active: true,
  created_at: "2026-09-01T10:00:00Z",
  updated_at: "2026-09-01T10:00:00Z",
  targets: [{ target_type: "school_all" }],
  response_type: "none",
  options: [],
  delivery_mode: "standard",
  email_audience: "portal_only",
};

const withReminder: Announcement = {
  ...published,
  reminder_at: "2026-09-24T06:00:00Z",
  reminder_text: "Morgen endet die Betreuung um 13:00 Uhr.",
};

describe("reminderError", () => {
  it("accepts no reminder and a future moment before the expiry", () => {
    expect(reminderError(null, "", null)).toBeNull();
    expect(
      reminderError(new Date(2026, 8, 24), "08:00", new Date(2026, 8, 25)),
    ).toBeNull();
  });

  it("names the rule that was broken", () => {
    expect(reminderError(new Date(2026, 8, 24), "8", null)).toMatch(/Uhrzeit/);
    expect(reminderError(new Date(2026, 8, 1), "08:00", null)).toMatch(
      /in der Zukunft/,
    );
    expect(
      reminderError(new Date(2026, 8, 26), "08:00", new Date(2026, 8, 25)),
    ).toMatch(/Ablaufdatum/);
  });
});

describe("AnnouncementReminderDialog (#3162)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    updateMock.mockResolvedValue(withReminder);
  });

  it("saves the moved reminder as a Berlin instant and keeps the wording", async () => {
    const onSaved = vi.fn();
    const onClose = vi.fn();
    render(
      <AnnouncementReminderDialog
        announcement={withReminder}
        onClose={onClose}
        onSaved={onSaved}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Erinnerung ändern" }),
    ).toBeInTheDocument();
    const time = screen.getByRole("textbox", { name: /Uhrzeit/ });
    expect(time).toHaveValue("08:00");
    fireEvent.change(time, { target: { value: "0930" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith("7", {
        reminder_at: "2026-09-24T07:30:00.000Z",
        reminder_text: "Morgen endet die Betreuung um 13:00 Uhr.",
      }),
    );
    expect(onSaved).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it("refuses a moment in the past before touching the backend", async () => {
    render(
      <AnnouncementReminderDialog
        announcement={{ ...withReminder, reminder_at: "2026-09-01T06:00:00Z" }}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Die Erinnerung muss in der Zukunft liegen.",
    );
    expect(updateMock).not.toHaveBeenCalled();
  });

  it("removes only after the confirmation and sends an explicit null", async () => {
    const onSaved = vi.fn();
    render(
      <AnnouncementReminderDialog
        announcement={withReminder}
        onClose={vi.fn()}
        onSaved={onSaved}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Erinnerung entfernen" }),
    );
    expect(updateMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));

    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith("7", {
        reminder_at: null,
        reminder_text: null,
      }),
    );
    expect(onSaved).toHaveBeenCalled();
  });

  it("asks for a day when planning a new reminder", async () => {
    render(
      <AnnouncementReminderDialog
        announcement={published}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Erinnerung planen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Erinnerung entfernen" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bitte einen Tag für die Erinnerung wählen.",
    );
    expect(updateMock).not.toHaveBeenCalled();
  });

  it("shows the backend error and stays open", async () => {
    updateMock.mockRejectedValueOnce(
      new Error("Die Erinnerung wurde bereits verschickt."),
    );
    const onClose = vi.fn();
    render(
      <AnnouncementReminderDialog
        announcement={withReminder}
        onClose={onClose}
        onSaved={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Die Erinnerung wurde bereits verschickt.",
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});
