import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { CareLetterMailing } from "~/lib/traeger-care-billing-api";
import { catalogText } from "~/test/error-catalog-text";

const { cancelCareInvoice } = vi.hoisted(() => ({
  cancelCareInvoice: vi.fn(),
}));
vi.mock("~/lib/traeger-care-billing-api", () => ({ cancelCareInvoice }));

import { CareCancellationModal } from "./care-cancellation-modal";

const INVOICE: CareLetterMailing = {
  invoiceId: "41",
  kind: "invoice",
  number: "MB-2026-0001",
  invoiceNumber: "MB-2026-0001",
  issuedOn: "2026-09-01",
  childFirstName: "Mia",
  childLastName: "Beispiel",
  schoolName: "OGS Am Lindenhof",
  recipientFirstName: "Eva",
  recipientLastName: "Beispiel",
  recipientEmail: "eva@x.org",
  totalCents: 9600,
  reason: "",
  cancelled: false,
  cancellationNumber: "",
  hasLetter: true,
  status: "not_sent",
  sends: 0,
  attempts: 0,
  requestedAt: null,
  sentAt: null,
};

function renderModal(invoice: CareLetterMailing, onCancelled = vi.fn()) {
  render(
    <ToastProvider>
      <CareCancellationModal
        runId="7"
        invoice={invoice}
        onClose={vi.fn()}
        onCancelled={onCancelled}
      />
    </ToastProvider>,
  );
  return onCancelled;
}

describe("CareCancellationModal (#4684)", () => {
  beforeEach(() => {
    cancelCareInvoice.mockReset();
  });

  it("says the family gets nothing for an invoice never sent", () => {
    renderModal(INVOICE);
    expect(
      screen.getByText(
        "Die Familie hat die Rechnung noch nicht bekommen. Sie bekommt auch kein Storno.",
      ),
    ).toBeInTheDocument();
  });

  it("says the family gets a Storno letter for a sent invoice", () => {
    renderModal({ ...INVOICE, status: "sent", sends: 1 });
    expect(
      screen.getByText(
        "Die Familie bekommt ein Storno-Schreiben. Senden Sie es danach.",
      ),
    ).toBeInTheDocument();
  });

  it("requires a reason", async () => {
    renderModal(INVOICE);
    fireEvent.click(screen.getByRole("button", { name: "Stornieren" }));
    expect(
      await screen.findByText("Bitte geben Sie eine Begründung an."),
    ).toBeInTheDocument();
    expect(cancelCareInvoice).not.toHaveBeenCalled();
  });

  it("cancels with the reason and hands the letters back", async () => {
    const letters = { letters: null, documents: [], summary: {} };
    cancelCareInvoice.mockResolvedValue(letters);
    const onCancelled = renderModal(INVOICE);
    fireEvent.change(screen.getByLabelText(/Begründung/), {
      target: { value: "  Falscher Empfänger " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stornieren" }));
    await waitFor(() => expect(onCancelled).toHaveBeenCalledWith(letters));
    expect(cancelCareInvoice).toHaveBeenCalledWith(
      "7",
      "41",
      "Falscher Empfänger",
    );
  });

  it("shows the refusal of an invoice already cancelled", async () => {
    cancelCareInvoice.mockRejectedValue(
      new ApiError("cancelled", 409, {
        code: "parent_billing.care_invoice_cancelled",
      }),
    );
    renderModal(INVOICE);
    fireEvent.change(screen.getByLabelText(/Begründung/), {
      target: { value: "Doppelt" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stornieren" }));
    expect(
      await screen.findByText(
        catalogText("parent_billing.care_invoice_cancelled", "der Storno"),
      ),
    ).toBeInTheDocument();
  });
});
