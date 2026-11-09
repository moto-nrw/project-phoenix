import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  CareInvoice,
  CareRunDetail,
} from "~/lib/traeger-care-billing-api";

// The exclusion dialog of the review (#6346): it excludes exactly the chosen
// invoice and names the other invoice of the contract that stays in the run.

const { review } = vi.hoisted(() => ({
  review: { excludeCareInvoice: vi.fn(), setCareRunInvoiceDate: vi.fn() },
}));
vi.mock("~/lib/traeger-care-review-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-review-api")>()),
  ...review,
}));

import { CareExcludeModal } from "./care-run-review-modals";

function invoice(service: "care" | "meal", totalCents: number): CareInvoice {
  return {
    id: "",
    number: "",
    issuedOn: null,
    contractId: "22",
    service,
    schoolId: "3",
    schoolName: "Grundschule Ispringen",
    childFirstName: "Mia",
    childLastName: "Beispiel",
    recipient: { firstName: "Eva", lastName: "Beispiel", email: "" },
    positions: [],
    reductions: [],
    costShares: [],
    totalCents,
    exclusion: null,
    mealPositions: [],
  };
}

const CARE = invoice("care", 7750);
const MEAL = invoice("meal", 960);

describe("CareExcludeModal", () => {
  beforeEach(() => vi.clearAllMocks());

  it("excludes only the Mensa invoice and names the care invoice that stays", async () => {
    const next = { id: "7" } as CareRunDetail;
    review.excludeCareInvoice.mockResolvedValue(next);
    const onExcluded = vi.fn();
    render(
      <ToastProvider>
        <CareExcludeModal
          runId="7"
          invoice={MEAL}
          invoices={[CARE, MEAL]}
          onClose={vi.fn()}
          onExcluded={onExcluded}
        />
      </ToastProvider>,
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Mensa-Rechnung ausschließen",
    });
    const shown = within(within(dialog).getByTestId("care-exclusion-invoice"));
    expect(
      shown.getByText("Mensa-Rechnung · Grundschule Ispringen"),
    ).toBeInTheDocument();
    expect(shown.getByText("9,60 €")).toBeInTheDocument();
    expect(
      shown.getByText(
        "Die Betreuungsrechnung von Mia über 77,50 € bleibt im Lauf.",
      ),
    ).toBeInTheDocument();

    fireEvent.change(within(dialog).getByLabelText(/Grund/), {
      target: { value: "Essenszahl wird geprüft" },
    });
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Ausschließen" }),
    );
    await vi.waitFor(() => expect(onExcluded).toHaveBeenCalledWith(next, MEAL));
    expect(review.excludeCareInvoice).toHaveBeenCalledWith(
      "7",
      expect.objectContaining({ contractId: "22", service: "meal" }),
      "Essenszahl wird geprüft",
    );
  });
});
