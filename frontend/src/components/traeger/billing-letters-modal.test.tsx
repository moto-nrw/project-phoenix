import { schoolDateCases, setSchoolDateClock } from "~/test/school-date-cases";
import { afterEach as restoreZoneAfterEach, vi as zoneVi } from "vitest";
restoreZoneAfterEach(() => zoneVi.unstubAllEnvs());
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { BillingLettersModal } from "./billing-letters-modal";

const { createBillingLetters } = vi.hoisted(() => ({
  createBillingLetters: vi.fn(),
}));
vi.mock("~/lib/traeger-billing-api", () => ({
  createBillingLetters,
  previewBillingLetter: vi.fn(),
}));

it.each(schoolDateCases("2026-05-27"))(
  "names the failed creation with a singular subject in %s",
  async (zone, instant) => {
    setSchoolDateClock(zone, instant, "2026-05-27");

    createBillingLetters.mockRejectedValue(
      new ApiError("Internal error", 500, { code: "general.server" }),
    );
    render(
      <ToastProvider>
        <BillingLettersModal
          runId="9"
          isOpen
          onClose={vi.fn()}
          onCreated={vi.fn()}
        />
      </ToastProvider>,
    );
    expect(screen.getByLabelText("Zahlen bis")).toHaveValue("10.06.2026");
    fireEvent.click(
      screen.getByRole("button", { name: "Schreiben erstellen" }),
    );
    expect(
      await screen.findByText(/Das Erstellen der Schreiben konnte nicht/),
    ).toBeInTheDocument();
  },
);

// A payment text with the debtor number refuses letters of a run with
// invoices without one (#5622); the form names how many and what to do.
it("names the invoices without a debtor number the payment text needs", async () => {
  createBillingLetters.mockRejectedValue(
    new ApiError("missing", 409, {
      code: "parent_billing.payment_text_debtor_number_missing",
      details: { count: 3, invoice_numbers: ["FE-0002", "FE-0003", "FE-0004"] },
    }),
  );
  render(
    <ToastProvider>
      <BillingLettersModal
        runId="9"
        isOpen
        onClose={vi.fn()}
        onCreated={vi.fn()}
      />
    </ToastProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Schreiben erstellen" }));
  expect(
    await screen.findByText(
      /^Rechnungen ohne Debitorennummer: 3\. Ihr Zahlungstext nennt aber die Debitorennummer\./,
    ),
  ).toBeInTheDocument();
});
