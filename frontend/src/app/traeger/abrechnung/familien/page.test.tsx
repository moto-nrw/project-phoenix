import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { BillingFamily } from "~/lib/traeger-billing-families-api";

const { api, mockPush } = vi.hoisted(() => ({
  api: { fetchBillingFamilies: vi.fn() },
  mockPush: vi.fn(),
}));

vi.mock("~/lib/traeger-billing-families-api", () => api);
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  usePathname: () => "/abrechnung/familien",
  useSearchParams: () => new URLSearchParams(),
}));

import TraegerBillingFamiliesPage from "./page";

const SCHNEIDER: BillingFamily = {
  id: "4",
  recipientFirstName: "Sabine",
  recipientLastName: "Schneider",
  recipientEmail: "sabine@example.org",
  debtorNumbers: ["10 4711"],
  payments: [{ kind: "family", costBearerName: "" }],
  invoiceCount: 1,
  openInvoiceCount: 1,
  documentCount: 2,
  lastIssuedOn: "2026-09-02",
  issuedCents: 11250,
  cancelledCents: 0,
  creditedCents: 500,
  openCents: 10750,
};

const BERGER: BillingFamily = {
  ...SCHNEIDER,
  id: "7",
  recipientFirstName: "Tom",
  recipientLastName: "Berger",
  recipientEmail: "tom@example.org",
  debtorNumbers: [],
  payments: [{ kind: "cost_bearer", costBearerName: "Jugendamt Enzkreis" }],
  openInvoiceCount: 0,
  documentCount: 1,
  issuedCents: 0,
  creditedCents: 0,
  openCents: 0,
};

function renderPage() {
  render(
    <ToastProvider>
      <TraegerBillingFamiliesPage />
    </ToastProvider>,
  );
}

describe("TraegerBillingFamiliesPage", () => {
  beforeEach(() => {
    api.fetchBillingFamilies.mockReset();
    mockPush.mockReset();
    api.fetchBillingFamilies.mockResolvedValue([SCHNEIDER, BERGER]);
  });

  it("lists every family with who pays and its sums", async () => {
    renderPage();

    expect(
      (await screen.findAllByText("Sabine Schneider"))[0],
    ).toBeInTheDocument();
    expect(screen.getAllByText("Familie zahlt selbst")[0]).toBeInTheDocument();
    expect(
      screen.getAllByText("Kostenübernahme: Jugendamt Enzkreis")[0],
    ).toBeInTheDocument();
    const tab = screen
      .getAllByText("Familien")
      .map((element) => element.closest("a"))
      .find((link) => link !== null);
    expect(tab).toHaveAttribute("href", "/abrechnung/familien");
    expect(screen.getAllByText("107,50 €").length).toBeGreaterThan(0);
  });

  it("names the open sum a claim before payment", async () => {
    // #6348: moto sees no incoming payment. The open sum is what the
    // invoices still ask for, never a checked arrear.
    renderPage();
    await screen.findAllByText("Sabine Schneider");

    expect(screen.getAllByText("Offene Forderung").length).toBeGreaterThan(0);
    expect(
      screen.getByText(
        "Stand vor Zahlungseingang. Zahlungen sieht moto nicht.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("mit offener Forderung")).toBeInTheDocument();
  });

  it("finds a family by its debtor number", async () => {
    renderPage();
    await screen.findAllByText("Tom Berger");

    fireEvent.change(
      screen.getAllByPlaceholderText("Name, E-Mail oder Debitorennummer")[0]!,
      { target: { value: "4711" } },
    );

    expect(
      (await screen.findAllByText("Sabine Schneider"))[0],
    ).toBeInTheDocument();
    expect(screen.queryByText("Tom Berger")).not.toBeInTheDocument();
  });

  it("opens a family", async () => {
    renderPage();
    const row = (await screen.findAllByText("Sabine Schneider"))[0]!.closest(
      "[role='button'], tr",
    );
    expect(row).not.toBeNull();
    fireEvent.click(row!);

    expect(mockPush).toHaveBeenCalledWith("/abrechnung/familien/4");
  });

  it("says when no run is released yet", async () => {
    api.fetchBillingFamilies.mockResolvedValue([]);
    renderPage();

    expect(
      (await screen.findAllByText("Noch keine Rechnung."))[0],
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Familien stehen hier, sobald ein Abrechnungslauf freigegeben ist.",
      )[0],
    ).toBeInTheDocument();
  });
});
