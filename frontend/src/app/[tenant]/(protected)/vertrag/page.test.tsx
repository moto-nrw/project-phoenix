import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ContractPage from "./page";
import type { ContractOverview } from "~/lib/contract-api";

const { swrState, permissionState } = vi.hoisted(() => ({
  swrState: { data: undefined as unknown, error: undefined as unknown },
  permissionState: { isReady: true, isLoading: false },
}));

vi.mock("swr", () => ({
  default: () => ({ data: swrState.data, error: swrState.error }),
}));

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: () => permissionState,
}));

function overview(overrides: Partial<ContractOverview> = {}): ContractOverview {
  return {
    tier: "plus",
    tierLabel: "Plus",
    bookedChildren: 150,
    activeChildren: 140,
    pricePerChildCents: 200,
    billingCycle: "monatlich",
    billingCycleLabel: "Monatlich",
    termStart: "2026-01-01",
    termEnd: "2026-12-31",
    invoiceRecipient: "buchhaltung@schule.test",
    customerNumber: "K-10023",
    supportEmail: "rechnung@moto.test",
    note: "",
    configured: true,
    referenceDate: "2026-02-15",
    invoices: [],
    openAmountCents: 0,
    nextDue: null,
    ...overrides,
  };
}

beforeEach(() => {
  swrState.data = overview();
  swrState.error = undefined;
  permissionState.isReady = true;
  permissionState.isLoading = false;
});

describe("ContractPage", () => {
  it('is titled "Vertrag", never "Abrechnung"', () => {
    render(<ContractPage />);

    expect(screen.getAllByText("Vertrag").length).toBeGreaterThan(0);
    // "Abrechnung" ist die Lohnabrechnung unter /payroll. Zwei Seiten mit
    // demselben Wortstamm liest man als Dopplung
    // (.claude/rules/verstaendlichkeit.md).
    expect(screen.queryByText(/Abrechnung/)).toBeNull();
  });

  // Der eine Satz, der die ganze Seite erklärt.
  it("states that the school cannot change anything here", () => {
    render(<ContractPage />);

    expect(
      screen.getByText(/Diese Angaben pflegt das moto-Team/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Sie können hier nichts ändern/),
    ).toBeInTheDocument();
  });

  it("says who acts next when no contract is stored", () => {
    swrState.data = overview({ configured: false });

    render(<ContractPage />);

    expect(
      screen.getByText(/Das moto-Team trägt die Daten ein/),
    ).toBeInTheDocument();
  });

  it("hides the not-configured hint once a contract exists", () => {
    render(<ContractPage />);

    expect(screen.queryByText(/noch kein Vertrag hinterlegt/)).toBeNull();
  });

  it("announces the next payment with amount and date", () => {
    swrState.data = overview({
      nextDue: {
        id: "1",
        periodLabel: "März 2026",
        invoiceNumber: "",
        amountCents: 30000,
        dueDate: "2026-03-31",
        status: "offen",
        overdue: false,
        paidOn: null,
        note: "",
      },
    });

    render(<ContractPage />);

    expect(screen.getByText(/Nächste Zahlung/)).toBeInTheDocument();
    expect(screen.getByText(/31\.03\.2026/)).toBeInTheDocument();
  });

  it("asks the school to check the payment when an invoice is overdue", () => {
    swrState.data = overview({
      nextDue: {
        id: "1",
        periodLabel: "Januar 2026",
        invoiceNumber: "",
        amountCents: 19900,
        dueDate: "2026-01-31",
        status: "offen",
        overdue: true,
        paidOn: null,
        note: "",
      },
    });

    render(<ContractPage />);

    expect(screen.getByText(/noch offen/)).toBeInTheDocument();
    expect(
      screen.getByText(/Bitte prüfen Sie die Zahlung/),
    ).toBeInTheDocument();
  });

  it("shows the moto note only when there is one", () => {
    render(<ContractPage />);
    expect(screen.queryByText("Hinweis von moto")).toBeNull();

    swrState.data = overview({ note: "Preis gilt bis Schuljahresende." });
    render(<ContractPage />);
    expect(screen.getByText("Hinweis von moto")).toBeInTheDocument();
    expect(
      screen.getByText("Preis gilt bis Schuljahresende."),
    ).toBeInTheDocument();
  });

  it("names the open total when something is unpaid", () => {
    swrState.data = overview({ openAmountCents: 19900 });

    render(<ContractPage />);

    expect(screen.getByText(/Offen sind zurzeit/)).toBeInTheDocument();
  });

  it("renders an error message instead of an empty contract", () => {
    swrState.data = undefined;
    swrState.error = new Error("boom");

    render(<ContractPage />);

    expect(
      screen.getByText("Die Vertragsdaten konnten nicht geladen werden."),
    ).toBeInTheDocument();
  });

  it("skeletonizes while the permission check is still running", () => {
    permissionState.isReady = false;
    permissionState.isLoading = true;
    swrState.data = undefined;

    render(<ContractPage />);

    expect(
      screen.getByText("Vertragsdaten werden geladen"),
    ).toBeInTheDocument();
  });
});
