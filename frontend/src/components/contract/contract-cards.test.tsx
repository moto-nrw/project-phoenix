import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  BillingContactCard,
  ChildQuotaCard,
  ContractFactsCard,
  InvoiceTable,
} from "./contract-cards";
import type { ContractOverview, Invoice } from "~/lib/contract-api";

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

function invoice(overrides: Partial<Invoice> = {}): Invoice {
  return {
    id: "1",
    periodLabel: "Januar 2026",
    invoiceNumber: "R-2026-001",
    amountCents: 19900,
    dueDate: "2026-01-31",
    status: "offen",
    overdue: false,
    paidOn: null,
    note: "",
    ...overrides,
  };
}

describe("ContractFactsCard", () => {
  it("names what the school booked", () => {
    render(<ContractFactsCard overview={overview()} />);

    expect(screen.getByText("Ihr Tarif")).toBeInTheDocument();
    expect(screen.getByText("Plus")).toBeInTheDocument();
    expect(screen.getByText("Monatlich")).toBeInTheDocument();
    expect(screen.getByText(/pro Kind und Monat/)).toBeInTheDocument();
  });

  // Leere Felder als Satz, nicht als leere Zeile: sonst liest man sie als
  // "hier fehlt etwas, das ich eintragen soll".
  it("says 'noch nicht hinterlegt' instead of leaving blanks", () => {
    render(
      <ContractFactsCard
        overview={overview({
          tierLabel: "Noch nicht hinterlegt",
          pricePerChildCents: 0,
          billingCycleLabel: "Noch nicht hinterlegt",
          termStart: null,
          termEnd: null,
        })}
      />,
    );

    expect(screen.getAllByText("Noch nicht hinterlegt").length).toBe(4);
  });

  it("calls a contract without an end date unbefristet", () => {
    render(<ContractFactsCard overview={overview({ termEnd: null })} />);

    expect(screen.getByText(/unbefristet/)).toBeInTheDocument();
  });
});

describe("ChildQuotaCard", () => {
  it("shows booked against active children", () => {
    render(<ChildQuotaCard overview={overview()} />);

    expect(screen.getByText("150 Kinder")).toBeInTheDocument();
    expect(screen.getByText(/140 Kinder mit Status/)).toBeInTheDocument();
  });

  it("dates the active count so the number is not read as permanent", () => {
    render(<ChildQuotaCard overview={overview()} />);

    expect(screen.getByText(/Aktiv am 15\.02\.2026/)).toBeInTheDocument();
  });

  // Der Satz ist der Kern der Verständlichkeit hier: eine Überschreitung
  // sperrt nichts. Ohne ihn liest man die Zahl als Abschaltungswarnung.
  it("states that exceeding the contingent blocks nothing", () => {
    render(
      <ChildQuotaCard
        overview={overview({ bookedChildren: 100, activeChildren: 120 })}
      />,
    );

    expect(screen.getByText(/bleibt vollständig nutzbar/)).toBeInTheDocument();
  });

  it("stays quiet while the contingent is not exceeded", () => {
    render(<ChildQuotaCard overview={overview()} />);

    expect(screen.queryByText(/bleibt vollständig nutzbar/)).toBeNull();
  });

  it("says nothing about exceeding when no contingent is booked", () => {
    render(
      <ChildQuotaCard
        overview={overview({ bookedChildren: 0, activeChildren: 12 })}
      />,
    );

    expect(screen.getByText("Noch nicht hinterlegt")).toBeInTheDocument();
    expect(screen.queryByText(/bleibt vollständig nutzbar/)).toBeNull();
  });
});

describe("BillingContactCard", () => {
  it("makes the support address mailable", () => {
    render(<BillingContactCard overview={overview()} />);

    const link = screen.getByRole("link", { name: "rechnung@moto.test" });
    expect(link).toHaveAttribute("href", "mailto:rechnung@moto.test");
  });

  it("does not render a link when no support address is set", () => {
    render(<BillingContactCard overview={overview({ supportEmail: "" })} />);

    expect(screen.queryByRole("link")).toBeNull();
  });
});

describe("InvoiceTable", () => {
  it("lists a paid invoice with its payment date", () => {
    render(
      <InvoiceTable
        invoices={[
          invoice({ status: "bezahlt", paidOn: "2026-02-03", overdue: false }),
        ]}
      />,
    );

    expect(screen.getByText("Bezahlt")).toBeInTheDocument();
    expect(screen.getByText("Bezahlt am 03.02.2026")).toBeInTheDocument();
  });

  it("labels an open, late invoice as überfällig", () => {
    render(<InvoiceTable invoices={[invoice({ overdue: true })]} />);

    expect(screen.getByText("Überfällig")).toBeInTheDocument();
  });

  it("says so when an invoice has no number yet", () => {
    render(<InvoiceTable invoices={[invoice({ invoiceNumber: "" })]} />);

    expect(screen.getByText("Noch keine")).toBeInTheDocument();
  });

  it("shows an empty state instead of a bare table", () => {
    render(<InvoiceTable invoices={[]} />);

    expect(screen.getByText("Noch keine Rechnungen")).toBeInTheDocument();
  });

  // Anzeige bleibt Anzeige: keine Zeile darf anklickbar wirken, weil dahinter
  // nichts passiert (.claude/rules/verstaendlichkeit.md).
  it("renders no buttons or links in the schedule", () => {
    render(<InvoiceTable invoices={[invoice()]} />);

    expect(screen.queryAllByRole("link")).toHaveLength(0);
    // Die einzigen Buttons sind die Sortier-Kopfzellen der DataTable.
    const buttons = screen.queryAllByRole("button");
    for (const button of buttons) {
      expect(button.closest("thead")).not.toBeNull();
    }
  });
});
