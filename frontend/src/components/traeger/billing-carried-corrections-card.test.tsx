import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  BillingCarriedCorrection,
  BillingChild,
  BillingDocument,
  BillingRunDetail,
} from "~/lib/traeger-billing-api";

const { mockRemove } = vi.hoisted(() => ({ mockRemove: vi.fn() }));

vi.mock("~/lib/traeger-billing-api", () => ({
  removeCarriedCorrection: (run: string, position: string) =>
    mockRemove(run, position),
}));

import { BillingCarriedCorrectionsCard } from "./billing-carried-corrections-card";

const CARRIED: BillingCarriedCorrection = {
  positionId: "61",
  sourceNumber: "FE-0001",
  amountCents: -2000,
  sourceAmountCents: -2000,
  reason: "Erster Tag krank",
};

function child(
  carried: BillingCarriedCorrection[],
  totalCents = 4750,
): BillingChild {
  return {
    childId: "7",
    firstName: "Lia",
    lastName: "Muster",
    totalCents,
    corrections: [],
    carriedCorrections: carried,
    positions: [],
  };
}

function documentOf(
  carried: BillingCarriedCorrection[],
  replacedCredits: BillingDocument["replacedCredits"] = [],
  childTotalCents = 4750,
): BillingDocument {
  return {
    id: "",
    kind: "invoice",
    referenceNumber: "FE-0001",
    reason: "",
    history: [],
    replacedCredits,
    number: "",
    issuedOn: null,
    registrationId: "3",
    recipientFirstName: "Sabine",
    recipientLastName: "Muster",
    recipientEmail: "sabine@example.org",
    debtorNumber: "",
    serviceFrom: "2027-07-05",
    serviceUntil: "2027-07-16",
    children: [child(carried, childTotalCents)],
    totalCents: 4750,
  };
}

function runOf(documents: BillingDocument[]): BillingRunDetail {
  return {
    id: "9",
    holidayPeriodId: "1",
    periodName: "Sommerferien 2027",
    serviceFrom: "2027-07-05",
    serviceUntil: "2027-07-16",
    status: "calculated",
    createdAt: "2026-09-01T08:00:00Z",
    releasedAt: null,
    issuedOn: null,
    documentCount: null,
    totalCents: null,
    documents,
    previewCount: documents.length,
    previewCents: 4750,
    unbilled: 0,
    missingDebtorNumbers: 0,
    paymentTextMissingDebtorNumbers: 0,
    numberPattern: "FE-{NNNN}",
    numberRangeSet: true,
    letters: null,
  };
}

function renderCard(run: BillingRunDetail) {
  const onChange = vi.fn();
  const onRemoved = vi.fn();
  const view = render(
    <ToastProvider>
      <BillingCarriedCorrectionsCard
        run={run}
        onChange={onChange}
        onRemoved={onRemoved}
      />
    </ToastProvider>,
  );
  return { onChange, onRemoved, ...view };
}

function openMenu() {
  fireEvent.click(
    screen.getAllByRole("button", {
      name: /^Aktionen für die Korrektur von Lia über /,
    })[0]!,
  );
}

describe("BillingCarriedCorrectionsCard", () => {
  beforeEach(() => {
    mockRemove.mockReset();
  });

  it("names amount, origin and reason of a proposal", () => {
    renderCard(runOf([documentOf([CARRIED])]));
    expect(screen.getByText("Übernommene Korrekturen")).toBeInTheDocument();
    expect(screen.getByText(/−?-?20,00.*aus FE-0001/)).toBeInTheDocument();
    expect(screen.getByText("Erster Tag krank")).toBeInTheDocument();
  });

  it("says when a proposal stops at zero", () => {
    renderCard(runOf([documentOf([{ ...CARRIED, amountCents: -1500 }])]));
    expect(screen.getByText(/nicht unter 0 € fällt/)).toBeInTheDocument();
  });

  it("shows nothing for a run without proposals and credits", () => {
    const { container } = renderCard(runOf([documentOf([])]));
    expect(container).toHaveTextContent("");
    expect(screen.queryByText("Übernommene Korrekturen")).toBeNull();
  });

  it("offers the change of a proposal", async () => {
    const { onChange } = renderCard(runOf([documentOf([CARRIED])]));
    openMenu();
    fireEvent.click(await screen.findByRole("menuitem", { name: "Ändern" }));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ childId: "7" }),
      CARRIED,
    );
  });

  it("removes a proposal only after the confirmation", async () => {
    const updated = runOf([documentOf([])]);
    mockRemove.mockResolvedValue(updated);
    const { onRemoved } = renderCard(runOf([documentOf([CARRIED])]));
    openMenu();
    fireEvent.click(await screen.findByRole("menuitem", { name: "Entfernen" }));
    expect(mockRemove).not.toHaveBeenCalled();
    expect(
      await screen.findByText(/Der Betrag wird 67,50/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));
    expect(mockRemove).not.toHaveBeenCalled();
    fireEvent.click(
      await screen.findByRole("button", { name: "Jetzt entfernen" }),
    );
    await waitFor(() => expect(mockRemove).toHaveBeenCalledWith("9", "61"));
    await waitFor(() => expect(onRemoved).toHaveBeenCalledWith(updated));
  });

  it("recalculates a later proposal that was limited at zero", async () => {
    renderCard(
      runOf([
        documentOf(
          [
            {
              ...CARRIED,
              positionId: "61",
              amountCents: -10000,
              sourceAmountCents: -10000,
            },
            {
              ...CARRIED,
              positionId: "62",
              amountCents: 0,
              sourceAmountCents: -5000,
            },
          ],
          [],
          0,
        ),
      ]),
    );

    openMenu();
    fireEvent.click(await screen.findByRole("menuitem", { name: "Entfernen" }));

    expect(
      await screen.findByText(/Der Betrag wird 50,00/),
    ).toBeInTheDocument();
  });

  it("names a credit of the cancelled invoice only as a notice", () => {
    renderCard(
      runOf([
        documentOf(
          [],
          [
            {
              id: "12",
              runId: "8",
              kind: "credit",
              number: "FE-0004",
              issuedOn: "2026-09-03",
              totalCents: -500,
              reason: "Ausflug fiel aus",
            },
          ],
        ),
      ]),
    );
    expect(screen.queryByText("Übernommene Korrekturen")).toBeNull();
    expect(screen.getByText(/Gutschrift FE-0004/)).toHaveTextContent(
      "nicht übernommen",
    );
  });
});
