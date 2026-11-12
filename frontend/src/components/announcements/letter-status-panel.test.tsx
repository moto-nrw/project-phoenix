import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { LetterStatusPanel } from "./letter-status-panel";
import type { LetterStatus } from "~/lib/parent-announcements-api";
import { ApiError } from "~/lib/api-error";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { deferred } from "~/test/helpers/form-modal-pending";
import { catalogText } from "~/test/error-catalog-text";

const { fetchLetterStatus, remindUnanswered, showError } = vi.hoisted(() => ({
  fetchLetterStatus: vi.fn(),
  remindUnanswered: vi.fn(),
  showError: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
  useApiErrorDisplay: () => ({ show: showError }),
}));

vi.mock("~/lib/parent-announcements-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-announcements-api")>()),
  fetchLetterStatus,
  remindUnanswered,
}));

function status(
  overrides: Partial<LetterStatus["summary"]>,
  recipients: LetterStatus["recipients"],
): LetterStatus {
  return {
    recipients,
    children: [],
    summary: {
      children_total: 0,
      children_confirmable: 0,
      children_fulfilled: 0,
      children_open: 0,
      children_without_portal: 0,
      emails_sent: 0,
      emails_pending: 0,
      emails_failed: 0,
      without_email: 0,
      without_portal: 0,
      ...overrides,
    },
  } as LetterStatus;
}

const recipient = (overrides: Partial<LetterStatus["recipients"][number]>) => ({
  first_name: "Ada",
  last_name: "Muster",
  email: "ada@example.com",
  email_status: "sent" as const,
  reachability: "ok" as const,
  ...overrides,
});

function renderPanel() {
  render(
    <LetterStatusPanel announcementId="1" canAct emailAudience="portal_only" />,
  );
}

describe("LetterStatusPanel mail state (#5934)", () => {
  beforeEach(() => fetchLetterStatus.mockReset());

  it("never reads an accepted mail as delivered", async () => {
    fetchLetterStatus.mockResolvedValue(
      status({ emails_sent: 1 }, [recipient({ mail_outcome: "accepted" })]),
    );
    renderPanel();
    expect(await screen.findByText("Versendet")).toBeInTheDocument();
    expect(screen.queryByText("Zugestellt")).not.toBeInTheDocument();
  });

  it("says what is wrong with a blocked address and does not offer a useless resend", async () => {
    fetchLetterStatus.mockResolvedValue(
      status(
        { emails_failed: 1, emails_address_check: 1, emails_resendable: 0 },
        [
          recipient({
            email_status: "failed",
            mail_outcome: "blocked",
            address_mark: "failed",
            last_error: "raw server text",
          }),
        ],
      ),
    );
    renderPanel();
    expect(await screen.findAllByText("Adresse prüfen")).not.toHaveLength(0);
    expect(screen.getByText(/Sie ist deshalb gesperrt/)).toBeInTheDocument();
    expect(screen.getByText(/„Erziehungsberechtigte“/)).toBeInTheDocument();
    expect(screen.queryByText("raw server text")).not.toBeInTheDocument();
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Fehlgeschlagene erneut senden" }),
      ).toBeDisabled(),
    );
  });

  it("keeps a mail's result with the address it went to", async () => {
    fetchLetterStatus.mockResolvedValue(
      status({ emails_delivered: 1 }, [
        recipient({
          email: "neu@example.com",
          mailed_to: "alt@example.com",
          mail_outcome: "delivered",
        }),
      ]),
    );
    renderPanel();
    expect(await screen.findByText("alt@example.com")).toBeInTheDocument();
    expect(
      screen.getByText(/Aktuelle Adresse: neu@example.com/),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Zugestellt").length).toBeGreaterThan(0);
  });

  it.each([
    ["unsubscribed", "Alte Adresse abgemeldet", /alte Adresse war abgemeldet/],
    ["complained", "Alte Adresse gesperrt", /als unerwünscht gemeldet/],
  ] as const)(
    "says a new mail can go to the corrected address after %s",
    async (mailOutcome, label, explanation) => {
      fetchLetterStatus.mockResolvedValue(
        status({ emails_failed: 1, emails_resendable: 1 }, [
          recipient({
            email: "neu@example.com",
            mailed_to: "alt@example.com",
            email_status: "failed",
            mail_outcome: mailOutcome,
          }),
        ]),
      );
      renderPanel();
      expect(await screen.findByText(explanation)).toHaveTextContent(
        /neue Mail an die neue Adresse ist möglich/,
      );
      expect(screen.getByText(label)).toBeInTheDocument();
      expect(screen.queryByText(/direkt an/)).toBeNull();
      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: "Fehlgeschlagene erneut senden" }),
        ).toBeEnabled(),
      );
    },
  );

  it("offers nothing new while the address stays the same", async () => {
    fetchLetterStatus.mockResolvedValue(
      status({ emails_sent: 1, emails_resendable: 0 }, [
        recipient({ mail_outcome: "unsubscribed" }),
      ]),
    );
    renderPanel();
    expect(await screen.findByText(/Sprechen Sie sie direkt an/)).toBeVisible();
    expect(screen.getAllByText("Abgemeldet").length).toBeGreaterThan(0);
    expect(screen.queryByText(/neue Adresse/)).toBeNull();
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Fehlgeschlagene erneut senden" }),
      ).toBeDisabled(),
    );
  });

  it("promises nothing for a corrected address that a hold still stops", async () => {
    fetchLetterStatus.mockResolvedValue(
      status({ emails_address_check: 1, emails_resendable: 0 }, [
        recipient({
          email: "neu@example.com",
          mailed_to: "alt@example.com",
          email_status: "failed",
          mail_outcome: "unsubscribed",
          address_mark: "failed",
        }),
      ]),
    );
    renderPanel();
    expect(
      await screen.findByText(/Aktuelle Adresse: neu@example.com/),
    ).toHaveTextContent(/Adresse prüfen/);
    expect(screen.queryByText(/neue Adresse ist möglich/)).toBeNull();
  });

  it("names the block of a current address that refused an earlier mail", async () => {
    fetchLetterStatus.mockResolvedValue(
      status({ emails_provider_blocked: 1, emails_resendable: 0 }, [
        recipient({
          email: "zurueck@example.com",
          mailed_to: "anders@example.com",
          email_status: "failed",
          mail_outcome: "undeliverable",
          provider_block: "active",
        }),
      ]),
    );
    renderPanel();
    expect(await screen.findByText(/moto-Support/)).toBeInTheDocument();
    expect(screen.queryByText(/neue Adresse ist möglich/)).toBeNull();
  });

  it("names the support for a block at the provider and offers no resend", async () => {
    fetchLetterStatus.mockResolvedValue(
      status(
        { emails_failed: 1, emails_provider_blocked: 1, emails_resendable: 0 },
        [
          recipient({
            email_status: "failed",
            mail_outcome: "provider_blocked",
            provider_block: "active",
          }),
        ],
      ),
    );
    renderPanel();
    expect(await screen.findByText(/moto-Support/)).toBeInTheDocument();
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Fehlgeschlagene erneut senden" }),
      ).toBeDisabled(),
    );
  });

  it("does not count a stop as an address to check", async () => {
    fetchLetterStatus.mockResolvedValue(
      status({ emails_stopped: 1, emails_resendable: 0 }, [
        recipient({ stopped: true }),
      ]),
    );
    renderPanel();
    expect(await screen.findAllByText("Versand gestoppt")).not.toHaveLength(0);
    expect(screen.getByText(/die Schule sie gestoppt hat/)).toBeInTheDocument();
    expect(screen.queryByText("Adresse prüfen")).not.toBeInTheDocument();
    expect(
      screen.queryByText(/„Erziehungsberechtigte“/),
    ).not.toBeInTheDocument();
  });

  it("explains a delay and keeps the resend for what it can change", async () => {
    fetchLetterStatus.mockResolvedValue(
      status(
        {
          emails_sent: 2,
          emails_delayed: 1,
          emails_failed: 1,
          emails_resendable: 1,
        },
        [
          recipient({ mail_outcome: "delayed" }),
          recipient({
            email: "b@example.com",
            email_status: "failed",
            mail_outcome: "send_failed",
          }),
        ],
      ),
    );
    renderPanel();
    expect(await screen.findAllByText("Verzögert")).not.toHaveLength(0);
    expect(
      screen.getByText(/Sie werden nicht erneut gesendet/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Fehlgeschlagene erneut senden" }),
    ).toBeEnabled();
  });
});

// #6652: a write holds once it is through; a failed reload keeps the shown
// status and says the change is saved, and its retry only reloads.
describe("reload after a write (#6652)", () => {
  const reloadText = `${catalogText("general.unavailable", "die Anzeige der Übersicht zum Elternbrief")} ${SAVED_BUT_STALE_SUFFIX}`;

  beforeEach(() => {
    fetchLetterStatus.mockReset();
    remindUnanswered.mockReset();
    showError.mockReset();
  });

  it("keeps reminder requests locked through the reload", async () => {
    const reload = deferred<ReturnType<typeof status>>();
    fetchLetterStatus
      .mockReset()
      .mockResolvedValueOnce(
        status(
          { children_confirmable: 2, children_fulfilled: 1, children_open: 1 },
          [recipient({})],
        ),
      )
      .mockReturnValueOnce(reload.promise);
    remindUnanswered.mockResolvedValueOnce(1);
    renderPanel();
    fireEvent.click(
      await screen.findByRole("button", { name: /Offene erinnern/ }),
    );
    await waitFor(() => expect(fetchLetterStatus).toHaveBeenCalledTimes(2));
    expect(
      screen.getByRole("button", { name: /Wird gesendet/ }),
    ).toBeDisabled();
    await act(async () =>
      reload.resolve(
        status(
          { children_confirmable: 2, children_fulfilled: 1, children_open: 1 },
          [recipient({})],
        ),
      ),
    );
    expect(remindUnanswered).toHaveBeenCalledTimes(1);
  });

  it("keeps a sent reminder sent when the reload fails", async () => {
    fetchLetterStatus
      .mockResolvedValueOnce(
        status(
          { children_confirmable: 2, children_fulfilled: 1, children_open: 1 },
          [recipient({})],
        ),
      )
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce(
        status(
          { children_confirmable: 2, children_fulfilled: 2, children_open: 0 },
          [recipient({})],
        ),
      );
    remindUnanswered.mockResolvedValueOnce(1);
    renderPanel();

    fireEvent.click(
      await screen.findByRole("button", { name: "Offene erinnern" }),
    );

    expect(await screen.findByText(reloadText)).toBeInTheDocument();
    // Anker: der bisher gezeigte Stand steht noch da.
    expect(
      screen.getByText("1 Kind wartet noch auf eine Bestätigung."),
    ).toBeInTheDocument();
    expect(showError).not.toHaveBeenCalled();
    expect(remindUnanswered).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(
      await screen.findByText(
        "Alle Kinder sind bestätigt, bei denen das möglich ist.",
      ),
    ).toBeInTheDocument();
    expect(fetchLetterStatus).toHaveBeenCalledTimes(3);
    expect(remindUnanswered).toHaveBeenCalledTimes(1);
  });
});
