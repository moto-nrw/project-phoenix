import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { LetterStatusPanel } from "./letter-status-panel";
import type { LetterStatus } from "~/lib/parent-announcements-api";

const { fetchLetterStatus } = vi.hoisted(() => ({
  fetchLetterStatus: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
  useApiErrorDisplay: () => ({ show: vi.fn() }),
}));

vi.mock("~/lib/parent-announcements-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-announcements-api")>()),
  fetchLetterStatus,
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
