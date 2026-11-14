import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  BillingPosition,
  BillingRunDetail,
} from "~/lib/traeger-billing-api";

const { mockDownload, mockSave } = vi.hoisted(() => ({
  mockDownload: vi.fn(),
  mockSave: vi.fn(),
}));

vi.mock("~/lib/traeger-billing-api", () => ({
  downloadCostTakeoverList: (id: string, bearer: string, format: string) =>
    mockDownload(id, bearer, format),
}));
vi.mock("~/lib/file-download", () => ({
  downloadBlob: (blob: Blob, name: string) => mockSave(blob, name),
}));

import {
  BillingCostTakeoverCard,
  runCostTakeovers,
} from "./billing-cost-takeover-card";

const OFFICE = "Fachdienst Jugend des Landkreises Musterstadt";

function position(
  kind: BillingPosition["kind"],
  amountCents: number,
  costBearerId: string | null = null,
): BillingPosition {
  return {
    kind,
    amountCents,
    weekStartsOn: null,
    weekEndsOn: null,
    timeModelName: "bis 16 Uhr",
    childRank: null,
    discountBasisPoints: null,
    discountCents: null,
    note: "",
    costBearerId,
    costBearerName: costBearerId ? OFFICE : "",
  };
}

function child(id: string, positions: BillingPosition[]) {
  return {
    childId: id,
    firstName: "Lia",
    lastName: "Muster",
    totalCents: 0,
    corrections: [],
    carriedCorrections: [],
    positions,
  };
}

const RUN: BillingRunDetail = {
  id: "9",
  holidayPeriodId: "1",
  periodName: "Sommerferien 2027",
  serviceFrom: "2027-07-05",
  serviceUntil: "2027-07-16",
  status: "released",
  createdAt: "2026-09-01T08:00:00Z",
  releasedAt: "2026-09-02T08:00:00Z",
  issuedOn: "2026-09-02",
  documentCount: 2,
  totalCents: 4500,
  previewCount: 0,
  previewCents: 0,
  unbilled: 0,
  missingDebtorNumbers: 0,
  paymentTextMissingDebtorNumbers: 0,
  sharedChildDebtorNumbers: 0,
  numberPattern: "FE-{NNNN}",
  numberRangeSet: true,
  fingerprint: "",
  letters: null,
  documents: [
    {
      id: "1",
      number: "FE-0001",
      kind: "invoice",
      referenceNumber: "",
      reason: "",
      history: [],
      replacedCredits: [],
      issuedOn: "2026-09-02",
      registrationId: "31",
      recipientFirstName: "Eva",
      recipientLastName: "Muster",
      recipientEmail: "eva@example.org",
      debtorNumber: "",
      serviceFrom: "2027-07-05",
      serviceUntil: "2027-07-16",
      totalCents: 0,
      children: [
        child("41", [
          position("base", 10000),
          position("cost_takeover", -10000, "4"),
        ]),
        child("42", [
          position("base", 10000),
          position("sibling", -5000),
          position("cost_takeover", -5000, "4"),
        ]),
      ],
    },
    {
      id: "2",
      number: "FE-0002",
      kind: "invoice",
      referenceNumber: "",
      reason: "",
      history: [],
      replacedCredits: [],
      issuedOn: "2026-09-02",
      registrationId: "32",
      recipientFirstName: "Tom",
      recipientLastName: "Beispiel",
      recipientEmail: "tom@example.org",
      debtorNumber: "",
      serviceFrom: "2027-07-05",
      serviceUntil: "2027-07-09",
      totalCents: 4500,
      children: [child("43", [position("base", 4500)])],
    },
  ],
};

function renderCard(run: BillingRunDetail) {
  render(
    <ToastProvider>
      <BillingCostTakeoverCard run={run} />
    </ToastProvider>,
  );
}

describe("BillingCostTakeoverCard", () => {
  beforeEach(() => {
    mockDownload.mockReset();
    mockSave.mockReset();
  });

  it("sums what each cost bearer pays, once per invoice", () => {
    expect(runCostTakeovers(RUN)).toEqual([
      { id: "4", name: OFFICE, documents: 1, cents: 15000 },
    ]);
  });

  it("takes an invoice the Storno cancels out of the sum and the count", () => {
    const invoice = RUN.documents[0]!;
    const storno = {
      ...invoice,
      id: "3",
      number: "FE-0003",
      kind: "cancellation" as const,
      referenceNumber: "FE-0001",
      reason: "Falsch bestätigt",
      children: invoice.children.map((entry) => ({
        ...entry,
        positions: entry.positions.map((line) => ({
          ...line,
          amountCents: -line.amountCents,
        })),
      })),
    };

    expect(
      runCostTakeovers({ ...RUN, documents: [...RUN.documents, storno] }),
    ).toEqual([]);
    expect(runCostTakeovers(RUN)).toHaveLength(1);
  });

  it("downloads the list for the reimbursement of a released run", async () => {
    const blob = new Blob(["csv"]);
    mockDownload.mockResolvedValue({ blob, filename: "liste.csv" });
    renderCard(RUN);

    expect(
      screen.getByText(
        "Diese Beträge bezahlen Kostenträger statt der Familien. Die Liste für die Erstattung steht im Menü der Zeile.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText(OFFICE)[0]).toHaveClass("wrap-anywhere");
    fireEvent.click(
      screen.getAllByRole("button", {
        name: `Liste für die Erstattung durch ${OFFICE} herunterladen`,
      })[0]!,
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: /CSV/ }));

    await waitFor(() =>
      expect(mockSave).toHaveBeenCalledWith(blob, "liste.csv"),
    );
    expect(mockDownload).toHaveBeenCalledWith("9", "4", "csv");
  });

  it("names the release as the precondition of the list in a preview", () => {
    renderCard({ ...RUN, status: "calculated" });

    expect(
      screen.getByText(
        "Diese Beträge bezahlen Kostenträger statt der Familien. Die Liste für die Erstattung gibt es nach der Freigabe.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Liste für die Erstattung durch/ }),
    ).toBeNull();
  });

  it("shows nothing when no cost bearer pays in the run", () => {
    const { container } = render(
      <ToastProvider>
        <span>Lauf</span>
        <BillingCostTakeoverCard
          run={{ ...RUN, documents: [RUN.documents[1]!] }}
        />
      </ToastProvider>,
    );
    expect(screen.getByText("Lauf")).toBeInTheDocument();
    expect(container.querySelector("section")).toBeNull();
  });
});
