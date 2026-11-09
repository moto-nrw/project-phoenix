import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi, afterEach } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { BillingFamilyDetail } from "~/lib/traeger-billing-families-api";

const { api, mockPush } = vi.hoisted(() => ({
  api: { fetchBillingFamily: vi.fn() },
  mockPush: vi.fn(),
}));

vi.mock("~/lib/traeger-billing-families-api", () => api);
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  useParams: () => ({ id: "4" }),
  usePathname: () => "/abrechnung/familien/4",
  useSearchParams: () => new URLSearchParams(),
}));

import TraegerBillingFamilyPage from "./page";
import {
  DESKTOP_WIDTH,
  PHONE_WIDTH,
  setViewportWidth,
} from "~/test/helpers/viewport";

// DataTable renders only the layout for the window width (#6220).
afterEach(() => setViewportWidth(DESKTOP_WIDTH));

const FAMILY: BillingFamilyDetail = {
  id: "4",
  recipientFirstName: "Sabine",
  recipientLastName: "Schneider",
  recipientEmail: "sabine@example.org",
  debtorNumbers: ["10 4711"],
  payments: [{ kind: "family", costBearerName: "" }],
  invoiceCount: 2,
  openInvoiceCount: 1,
  documentCount: 4,
  lastIssuedOn: "2026-09-05",
  issuedCents: 20000,
  cancelledCents: 8000,
  creditedCents: 500,
  openCents: 11500,
  documents: [
    {
      id: "12",
      runId: "3",
      kind: "credit",
      number: "FE-0004",
      issuedOn: "2026-09-05",
      periodName: "Herbstferien 2026",
      serviceFrom: "2026-10-12",
      serviceUntil: "2026-10-23",
      referenceNumber: "FE-0003",
      reason: "Erster Tag krank",
      childNames: ["Lotta"],
      debtorNumber: "10 4711",
      totalCents: -500,
      payment: null,
      status: null,
      openCents: 0,
    },
    {
      id: "11",
      runId: "3",
      kind: "invoice",
      number: "FE-0003",
      issuedOn: "2026-09-04",
      periodName: "Herbstferien 2026",
      serviceFrom: "2026-10-12",
      serviceUntil: "2026-10-23",
      referenceNumber: "FE-0001",
      reason: "",
      childNames: ["Jonas", "Lotta"],
      debtorNumber: "10 4711",
      totalCents: 12000,
      payment: { kind: "family", costBearerName: "" },
      status: "partly_credited",
      openCents: 11500,
    },
    {
      id: "10",
      runId: "2",
      kind: "cancellation",
      number: "FE-0002",
      issuedOn: "2026-09-03",
      periodName: "Herbstferien 2026",
      serviceFrom: "2026-10-12",
      serviceUntil: "2026-10-23",
      referenceNumber: "FE-0001",
      reason: "Anschrift falsch",
      childNames: ["Jonas", "Lotta"],
      debtorNumber: "10 4711",
      totalCents: -8000,
      payment: null,
      status: null,
      openCents: 0,
    },
    {
      id: "9",
      runId: "2",
      kind: "invoice",
      number: "FE-0001",
      issuedOn: "2026-09-02",
      periodName: "Herbstferien 2026",
      serviceFrom: "2026-10-12",
      serviceUntil: "2026-10-23",
      referenceNumber: "",
      reason: "",
      childNames: ["Jonas", "Lotta"],
      debtorNumber: "10 4711",
      totalCents: 8000,
      payment: { kind: "family", costBearerName: "" },
      status: "cancelled",
      openCents: 0,
    },
  ],
};

function renderPage() {
  render(
    <ToastProvider>
      <TraegerBillingFamilyPage />
    </ToastProvider>,
  );
}

describe("TraegerBillingFamilyPage", () => {
  beforeEach(() => {
    api.fetchBillingFamily.mockReset();
    mockPush.mockReset();
    api.fetchBillingFamily.mockResolvedValue(FAMILY);
  });

  it("shows the sums, who pays and every document with its state", async () => {
    renderPage();

    expect(
      await screen.findByRole("heading", { name: "Sabine Schneider" }),
    ).toBeInTheDocument();
    expect(api.fetchBillingFamily).toHaveBeenCalledWith("4");
    expect(screen.getAllByText("115,00 €").length).toBeGreaterThan(0);
    expect(screen.getAllByText("80,00 €").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Teilgutschrift")[0]).toBeVisible();
    expect(screen.getAllByText("Storniert").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Zu FE-0003 · Erster Tag krank")[0],
    ).toBeInTheDocument();
    expect(screen.getAllByText("Ersetzt FE-0001")[0]).toBeInTheDocument();
    expect(screen.getAllByText(/^[-−]5,00\s€$/).length).toBeGreaterThan(0);
  });

  it("keeps all document details in the mobile list", async () => {
    setViewportWidth(PHONE_WIDTH);
    renderPage();
    await screen.findByRole("heading", { name: "Sabine Schneider" });

    const mobileList = within(screen.getByRole("list"));
    const invoice = within(
      mobileList.getByRole("button", { name: /Rechnung FE-0003/ }),
    );
    expect(invoice.getByText("04.09.2026")).toBeInTheDocument();
    expect(invoice.getByText("Herbstferien 2026")).toBeInTheDocument();
    expect(invoice.getByText("12.10.2026 bis 23.10.2026")).toBeInTheDocument();
    expect(invoice.getByText("Jonas, Lotta")).toBeInTheDocument();
    expect(invoice.getByText("Betrag")).toBeInTheDocument();
    expect(invoice.getByText("120,00 €")).toBeInTheDocument();
    expect(invoice.getByText("Teilgutschrift")).toBeInTheDocument();
    expect(invoice.getByText("Offene Forderung")).toBeInTheDocument();
    expect(invoice.getByText("115,00 €")).toBeInTheDocument();

    const cancelled = within(
      mobileList.getByRole("button", { name: /Rechnung FE-0001/ }),
    );
    expect(cancelled.getByText("Storniert")).toBeInTheDocument();
    expect(cancelled.getByText("0,00 €")).toBeInTheDocument();

    const credit = within(
      mobileList.getByRole("button", { name: /Gutschrift FE-0004/ }),
    );
    expect(credit.getByText(/^[-−]5,00\s€$/)).toBeInTheDocument();

    fireEvent.click(invoice.getByText("FE-0003"));
    expect(mockPush).toHaveBeenCalledWith("/abrechnung/3");
  });

  it("opens the run of a document", async () => {
    renderPage();
    const cell = (await screen.findAllByText("FE-0004"))[0]!;
    fireEvent.click(cell.closest("[role='button'], tr")!);

    expect(mockPush).toHaveBeenCalledWith("/abrechnung/3");
  });
});
