import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { InvoicePayment } from "~/lib/traeger-billing-terms-api";
import type { CareLetters } from "~/lib/traeger-care-billing-api";

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));
vi.mock("~/lib/traeger-care-billing-api", () => ({
  createCareLetters: vi.fn(),
  previewCareLetter: vi.fn(),
}));

import {
  CareLettersCard,
  lettersToSend,
  withoutInboxNote,
} from "./care-letters-card";

const CREATED: CareLetters = {
  letters: {
    paymentDueOn: "2026-09-23",
    notes: "",
    createdAt: "2026-09-09T08:00:00Z",
  },
  documents: [],
  summary: {
    letters: 10,
    sent: 4,
    queued: 1,
    failed: 1,
    notSent: 4,
    withoutEmail: 2,
    withoutInbox: 0,
  },
};

function renderCard(
  letters: CareLetters | null,
  onSendAll = vi.fn(async () => undefined),
  onPrint = vi.fn(),
  payments: readonly (InvoicePayment | null)[] = [null],
) {
  render(
    <ToastProvider>
      <CareLettersCard
        runId="7"
        payments={payments}
        letters={letters}
        loadError={null}
        sendingAll={false}
        printing={null}
        onCreated={vi.fn()}
        onSendAll={onSendAll}
        onPrint={onPrint}
      />
    </ToastProvider>,
  );
  return { onSendAll, onPrint };
}

describe("CareLettersCard (#4684)", () => {
  it("says that creating sends nothing yet", () => {
    renderCard({ ...CREATED, letters: null });
    expect(
      screen.getByText(/Erstellen sendet noch nichts/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Schreiben erstellen" }),
    ).toBeInTheDocument();
  });

  it("counts every letter once and sends the open ones with an address", () => {
    expect(lettersToSend(CREATED.summary)).toBe(2);
    renderCard(CREATED);
    expect(screen.getByText("4 von 10 Schreiben gesendet")).toBeInTheDocument();
    expect(screen.getByText("Noch nicht gesendet")).toBeInTheDocument();
    expect(screen.getByText("Ohne E-Mail")).toBeInTheDocument();
  });

  it("shows the frozen deadlines instead of the unused fallback date", () => {
    renderCard(CREATED, undefined, undefined, [
      { method: "direct_debit", dueOn: "2026-09-15", dueSource: "family" },
      { method: "transfer", dueOn: "2026-09-20", dueSource: "run" },
    ]);
    expect(
      screen.getByText("Überweisung am 20.09.2026 · Lastschrift am 15.09.2026"),
    ).toBeInTheDocument();
    expect(screen.queryByText("23.09.2026")).toBeNull();
  });

  it("confirms the send with the count and the families to print", async () => {
    const { onSendAll } = renderCard(CREATED);
    fireEvent.click(screen.getByRole("button", { name: "An alle senden" }));
    expect(
      await screen.findByText(
        /2 Schreiben gehen jetzt per E-Mail an die Familien/,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/2 Familien ohne E-Mail-Adresse bekommen nichts/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "2 Schreiben senden" }));
    await waitFor(() => expect(onSendAll).toHaveBeenCalledTimes(1));
  });

  it("closes the send confirmation when letters are discarded and keeps it closed after recovery", async () => {
    const props = {
      runId: "7",
      payments: [null],
      loadError: null,
      sendingAll: false,
      printing: null,
      onCreated: vi.fn(),
      onSendAll: vi.fn(),
      onPrint: vi.fn(),
    };
    const view = render(
      <ToastProvider>
        <CareLettersCard {...props} letters={CREATED} />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "An alle senden" }));
    expect(await screen.findByRole("dialog")).toBeVisible();
    view.rerender(
      <ToastProvider>
        <CareLettersCard {...props} letters={null} />
      </ToastProvider>,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    view.rerender(
      <ToastProvider>
        <CareLettersCard {...props} letters={CREATED} />
      </ToastProvider>,
    );
    expect(
      screen.getByRole("button", { name: "An alle senden" }),
    ).toBeVisible();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("prints only the families without an address", () => {
    const { onPrint } = renderCard(CREATED);
    fireEvent.click(screen.getByRole("button", { name: "Schreiben drucken" }));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Nur Familien ohne E-Mail/ }),
    );
    expect(onPrint).toHaveBeenCalledWith(true);
  });

  it("has nothing to send when every letter with an address went out", () => {
    renderCard({
      ...CREATED,
      summary: {
        letters: 3,
        sent: 3,
        queued: 0,
        failed: 0,
        notSent: 0,
        withoutEmail: 0,
        withoutInbox: 0,
      },
    });
    expect(
      screen.getByRole("button", { name: "An alle senden" }),
    ).toBeDisabled();
    expect(
      screen.getByText(/Alle Schreiben mit E-Mail-Adresse sind gesendet/),
    ).toBeInTheDocument();
  });
});

describe("letters outside the parents app (#4684)", () => {
  it("tells the office which letters stay out of the parents app and why", () => {
    renderCard({
      ...CREATED,
      summary: { ...CREATED.summary, withoutInbox: 2 },
    });
    expect(
      screen.getByText(
        "Nicht in der Eltern-App: 2 Schreiben sind nicht unter „Dokumente“. Das Kind hat noch keinen Schülerdatensatz. Die Familie bekommt das Schreiben per E-Mail oder gedruckt.",
      ),
    ).toBeInTheDocument();
    expect(withoutInboxNote(1)).toContain("1 Schreiben ist nicht");
  });

  it("promises nothing extra when every letter reaches the parents app", () => {
    renderCard(CREATED);
    expect(screen.getByText("4 von 10 Schreiben gesendet")).toBeInTheDocument();
    expect(screen.queryByText(/Nicht in der Eltern-App/)).toBeNull();
  });
});
