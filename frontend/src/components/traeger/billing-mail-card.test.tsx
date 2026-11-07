import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { LetterMailing } from "~/lib/traeger-billing-api";
import { catalogText } from "~/test/error-catalog-text";

const { api } = vi.hoisted(() => ({
  api: {
    fetchLetterMailings: vi.fn(),
    mailBillingLetters: vi.fn(),
    mailBillingLetter: vi.fn(),
    printBillingLetters: vi.fn(),
  },
}));
const { downloadBlob } = vi.hoisted(() => ({ downloadBlob: vi.fn() }));
vi.mock("~/lib/traeger-billing-api", () => api);
vi.mock("~/lib/file-download", () => ({ downloadBlob }));

import { BillingMailCard } from "./billing-mail-card";

const mailing = (overrides: Partial<LetterMailing>): LetterMailing => ({
  documentId: "51",
  kind: "invoice",
  number: "FE-0001",
  recipientFirstName: "Eva",
  recipientLastName: "Muster",
  recipientEmail: "eva@example.org",
  status: "not_sent",
  sends: 0,
  requestedAt: null,
  sentAt: null,
  ...overrides,
});

const FAMILIES: LetterMailing[] = [
  mailing({}),
  mailing({
    documentId: "52",
    number: "FE-0002",
    recipientFirstName: "Ben",
    recipientLastName: "Kraus",
    recipientEmail: "ben@example.org",
    status: "failed",
    sends: 1,
    requestedAt: "2026-09-09T08:00:00Z",
  }),
  mailing({
    documentId: "53",
    number: "FE-0003",
    recipientFirstName: "Ida",
    recipientLastName: "Lang",
    recipientEmail: "ida@example.org",
    status: "sent",
    sends: 1,
    requestedAt: "2026-09-09T08:00:00Z",
    sentAt: "2026-09-09T08:01:00Z",
  }),
];

function renderCard() {
  render(
    <ToastProvider>
      <BillingMailCard runId="9" />
    </ToastProvider>,
  );
}

describe("BillingMailCard", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    downloadBlob.mockReset();
    api.fetchLetterMailings.mockResolvedValue(FAMILIES);
  });

  it("names a Storno and a Gutschrift by their kind (#4690)", async () => {
    api.fetchLetterMailings.mockResolvedValue([
      mailing({}),
      mailing({ documentId: "54", kind: "cancellation", number: "FE-0004" }),
      mailing({ documentId: "55", kind: "credit", number: "FE-0005" }),
    ]);
    renderCard();

    expect(await screen.findByText("Storno FE-0004")).toBeInTheDocument();
    expect(screen.getByText("Gutschrift FE-0005")).toBeInTheDocument();
    expect(screen.getByText("FE-0001")).toBeInTheDocument();
  });

  it("shows the state of the mail of every family", async () => {
    renderCard();

    expect(await screen.findByText("Nicht gesendet")).toBeInTheDocument();
    expect(screen.getByText("Nicht angekommen")).toBeInTheDocument();
    expect(screen.getByText("Gesendet")).toBeInTheDocument();
    expect(screen.getByText("am 09.09.2026, 10:01 Uhr")).toBeInTheDocument();
    expect(
      screen.getByText(/Kam eine Mail nicht an, senden Sie sie erneut/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Senden" })).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "Erneut senden" }),
    ).toHaveLength(2);
    expect(api.fetchLetterMailings).toHaveBeenCalledWith("9");
  });

  it("reads only a report as delivered and offers no second mail while delayed", async () => {
    api.fetchLetterMailings.mockResolvedValue([
      mailing({ status: "sent", mailOutcome: "accepted", sends: 1 }),
      mailing({
        documentId: "52",
        status: "sent",
        mailOutcome: "delivered",
        sends: 1,
      }),
      mailing({
        documentId: "53",
        status: "sent",
        mailOutcome: "delayed",
        sends: 1,
      }),
      mailing({
        documentId: "54",
        status: "sent",
        mailOutcome: "undeliverable",
        sends: 1,
      }),
    ]);
    renderCard();

    expect(await screen.findByText("Gesendet")).toBeInTheDocument();
    expect(screen.getByText("Zugestellt")).toBeInTheDocument();
    expect(screen.getByText("Verzögert")).toBeInTheDocument();
    expect(screen.getByText("Nicht zustellbar")).toBeInTheDocument();
    expect(screen.getByText(/nicht noch einmal/)).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "Erneut senden" }),
    ).toHaveLength(3);
  });

  it.each([
    ["provider_blocked", "Nicht gesendet"],
    ["unsubscribed", "Abgemeldet"],
    ["complained", "Nicht zustellbar"],
  ] as const)(
    "points to the print and offers no useless resend while the provider holds %s",
    async (mailOutcome, label) => {
      api.fetchLetterMailings.mockResolvedValue([
        mailing({
          documentId: "60",
          status: "sent",
          mailOutcome,
          providerBlock: "active",
          sends: 1,
        }),
      ]);
      renderCard();

      expect(await screen.findByText(label)).toBeInTheDocument();
      expect(screen.queryByText("Gesendet")).toBeNull();
      expect(screen.getByText(/drucken Sie das Schreiben/)).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Erneut senden" }),
      ).toBeNull();
    },
  );

  it("offers the resend again once the provider's block is gone", async () => {
    api.fetchLetterMailings.mockResolvedValue([
      mailing({
        documentId: "62",
        status: "sent",
        mailOutcome: "provider_blocked",
        providerBlock: "none",
        sends: 1,
      }),
    ]);
    renderCard();

    expect(await screen.findByText(/besteht nicht mehr/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Erneut senden" }),
    ).toBeInTheDocument();
  });

  it("offers nothing while the block cannot be read", async () => {
    api.fetchLetterMailings.mockResolvedValue([
      mailing({
        documentId: "63",
        status: "sent",
        mailOutcome: "provider_blocked",
        providerBlock: "unreadable",
        sends: 1,
      }),
    ]);
    renderCard();

    expect(await screen.findByText(/später noch einmal/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Erneut senden" })).toBeNull();
  });

  it("offers the resend again after a delay ran past its limit", async () => {
    api.fetchLetterMailings.mockResolvedValue([
      mailing({
        documentId: "61",
        status: "sent",
        mailOutcome: "overdue",
        sends: 1,
      }),
    ]);
    renderCard();

    expect(await screen.findByText("Überfällig")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Erneut senden" }),
    ).toBeInTheDocument();
  });

  it("offers no send while a mail waits and none without an address", async () => {
    api.fetchLetterMailings.mockResolvedValue([
      mailing({ status: "queued", sends: 1 }),
      mailing({
        documentId: "52",
        recipientLastName: "Kraus",
        recipientEmail: "",
      }),
    ]);
    renderCard();

    expect(await screen.findByText("Wird gesendet")).toBeInTheDocument();
    expect(
      screen.getByText("Keine Mail-Adresse: Bitte drucken Sie das Schreiben."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Alle Schreiben drucken" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Senden" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Erneut senden" })).toBeNull();
  });

  it("keeps the status short and explains a forgotten result beside it", async () => {
    api.fetchLetterMailings.mockResolvedValue([
      mailing({
        status: "unknown",
        sends: 1,
        requestedAt: "2026-09-09T08:00:00Z",
      }),
    ]);
    renderCard();

    expect(await screen.findByText("Ergebnis unbekannt")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Versand angefragt am 09.09.2026, 10:00 Uhr. Ob die Mail ankam, ist nicht bekannt.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Erneut senden" }),
    ).toBeInTheDocument();
  });

  it("mails every family and reports what it sent", async () => {
    api.mailBillingLetters.mockResolvedValue({
      queued: 1,
      already: 2,
      withoutEmail: 1,
    });
    renderCard();

    fireEvent.click(
      await screen.findByRole("button", { name: "An alle Familien senden" }),
    );

    await waitFor(() =>
      expect(api.mailBillingLetters).toHaveBeenCalledWith("9"),
    );
    expect(
      await screen.findByText(
        "1 Mail(s) werden jetzt gesendet. 1 Familie(n) ohne Mail-Adresse: Bitte drucken Sie deren Schreiben.",
      ),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(api.fetchLetterMailings).toHaveBeenCalledTimes(2),
    );
  });

  it("sends a failed mail again", async () => {
    api.mailBillingLetter.mockResolvedValue(
      mailing({ documentId: "52", status: "queued", sends: 2 }),
    );
    renderCard();

    fireEvent.click(
      (await screen.findAllByRole("button", { name: "Erneut senden" }))[0]!,
    );

    expect(
      await screen.findByText("Die Mail an Familie Kraus wird jetzt gesendet."),
    ).toBeInTheDocument();
    expect(api.mailBillingLetter).toHaveBeenCalledWith("9", "52");
  });

  it("names the waiting mail when a send is refused", async () => {
    api.mailBillingLetter.mockRejectedValue(
      new ApiError("pending", 409, {
        code: "parent_billing.letter_mail_pending",
      }),
    );
    renderCard();

    fireEvent.click(await screen.findByRole("button", { name: "Senden" }));

    expect(
      await screen.findByText(
        catalogText("parent_billing.letter_mail_pending", "die Mail"),
      ),
    ).toBeInTheDocument();
  });

  it("prints every letter in one file", async () => {
    const blob = new Blob(["%PDF"], { type: "application/pdf" });
    api.printBillingLetters.mockResolvedValue({
      blob,
      filename: "bestaetigungsschreiben-herbstferien-2026.pdf",
    });
    renderCard();

    fireEvent.click(
      await screen.findByRole("button", { name: "Alle Schreiben drucken" }),
    );

    await waitFor(() =>
      expect(downloadBlob).toHaveBeenCalledWith(
        blob,
        "bestaetigungsschreiben-herbstferien-2026.pdf",
      ),
    );
    expect(api.printBillingLetters).toHaveBeenCalledWith("9");
  });
});
