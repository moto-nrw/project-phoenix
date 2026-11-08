import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type {
  CareInvoice,
  CareLetterMailing,
} from "~/lib/traeger-care-billing-api";

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));

import {
  type CareInvoiceLetterActions,
  CareRunLetterInvoices,
} from "./care-run-letter-invoices";

const invoice = (overrides: Partial<CareInvoice> = {}): CareInvoice => ({
  id: "41",
  number: "MB-2027-0001",
  issuedOn: "2027-01-04",
  contractId: "11",
  schoolId: "3",
  schoolName: "OGS Am Lindenhof",
  childFirstName: "Mia",
  childLastName: "Beispiel",
  recipient: { firstName: "Eva", lastName: "Beispiel", email: "eva@x.org" },
  positions: [],
  reductions: [],
  costShares: [],
  totalCents: 9600,
  ...overrides,
});

const letter = (
  overrides: Partial<CareLetterMailing> = {},
): CareLetterMailing => ({
  invoiceId: "41",
  kind: "invoice",
  number: "MB-2027-0001",
  invoiceNumber: "MB-2027-0001",
  issuedOn: "2027-01-04",
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
  inInbox: true,
  status: "not_sent",
  sends: 0,
  attempts: 0,
  requestedAt: null,
  sentAt: null,
  ...overrides,
});

function actions(
  letters: CareLetterMailing[],
  overrides: Partial<CareInvoiceLetterActions> = {},
): CareInvoiceLetterActions {
  return {
    letters,
    created: true,
    sendingKey: null,
    onShowLetter: vi.fn(),
    onSend: vi.fn(),
    onCancel: vi.fn(),
    ...overrides,
  };
}

function openMenu(name: RegExp) {
  // The table renders its rows for the desk and the stacked phone list.
  fireEvent.click(screen.getAllByRole("button", { name })[0]!);
  return screen.getByRole("menu");
}

describe("CareRunLetterInvoices (#4684)", () => {
  it("keeps issued apart from what reached the family", () => {
    render(
      <CareRunLetterInvoices
        invoices={[invoice()]}
        month="2027-01-01"
        letterActions={actions([letter()])}
      />,
    );
    expect(
      screen.getAllByText("Ausgestellt am 04.01.2027").length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("An die Familie").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Nicht gesendet").length).toBeGreaterThan(0);
  });

  it("says when a letter stays out of the parents app", () => {
    render(
      <CareRunLetterInvoices
        invoices={[invoice()]}
        month="2027-01-01"
        letterActions={actions([letter({ inInbox: false })])}
      />,
    );
    expect(
      screen.getAllByText(
        "Nicht in der Eltern-App: Das Kind hat noch keinen Schülerdatensatz.",
      ).length,
    ).toBeGreaterThan(0);
  });

  it("offers send and Storno for an invoice not sent yet", () => {
    const onSend = vi.fn();
    const onCancel = vi.fn();
    const mailing = letter();
    render(
      <CareRunLetterInvoices
        invoices={[invoice()]}
        month="2027-01-01"
        letterActions={actions([mailing], { onSend, onCancel })}
      />,
    );
    const menu = within(openMenu(/Aktionen für die Rechnung von Mia/));
    expect(
      menu.getByRole("menuitem", { name: /Schreiben ansehen/ }),
    ).toBeInTheDocument();
    fireEvent.click(menu.getByRole("menuitem", { name: /^Senden/ }));
    expect(onSend).toHaveBeenCalledWith(mailing);
  });

  it("offers a retry after a failed mail", () => {
    render(
      <CareRunLetterInvoices
        invoices={[invoice()]}
        month="2027-01-01"
        letterActions={actions([letter({ status: "failed", sends: 1 })])}
      />,
    );
    expect(screen.getAllByText("Nicht angekommen").length).toBeGreaterThan(0);
    const menu = within(openMenu(/Aktionen für die Rechnung von Mia/));
    expect(
      menu.getByRole("menuitem", { name: /Erneut senden/ }),
    ).toBeInTheDocument();
  });

  it("shows the Storno as its own row below its invoice", () => {
    render(
      <CareRunLetterInvoices
        invoices={[invoice()]}
        month="2027-01-01"
        letterActions={actions([
          letter({
            cancelled: true,
            cancellationNumber: "MB-2027-0009",
            status: "sent",
            sends: 1,
            sentAt: "2027-01-05T08:01:00Z",
          }),
          letter({
            kind: "cancellation",
            number: "MB-2027-0009",
            totalCents: -9600,
            reason: "Falscher Empfänger",
          }),
        ])}
      />,
    );
    expect(screen.getAllByText("Storniert").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Grund: Falscher Empfänger").length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText(/-96,00\s€/).length).toBeGreaterThan(0);
    // A cancelled invoice is not sent again and not cancelled twice.
    const invoiceMenu = within(openMenu(/Aktionen für die Rechnung von Mia/));
    expect(
      invoiceMenu.getByRole("menuitem", { name: /Schreiben ansehen/ }),
    ).toBeInTheDocument();
    expect(
      invoiceMenu.queryByRole("menuitem", { name: /Senden|Stornieren/ }),
    ).toBeNull();
    fireEvent.keyDown(document, { key: "Escape" });
    const stornoMenu = within(openMenu(/Aktionen für den Storno von Mia/));
    expect(
      stornoMenu.getByRole("menuitem", { name: /^Senden/ }),
    ).toBeInTheDocument();
  });

  it("says that the family gets nothing for a Storno of an invoice never sent", () => {
    render(
      <CareRunLetterInvoices
        invoices={[invoice()]}
        month="2027-01-01"
        letterActions={actions([
          letter({ cancelled: true, cancellationNumber: "MB-2027-0009" }),
          letter({
            kind: "cancellation",
            number: "MB-2027-0009",
            totalCents: -9600,
            hasLetter: false,
            inInbox: true,
          }),
        ])}
      />,
    );
    expect(
      screen.getAllByText("Nicht gesendet. Die Familie bekommt nichts.").length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByText(
        "Kein Schreiben. Die Familie hatte die Rechnung noch nicht.",
      ).length,
    ).toBeGreaterThan(0);
  });

  it("keeps the paper cancellation notice visible after printing", () => {
    render(
      <CareRunLetterInvoices
        invoices={[invoice()]}
        month="2027-01-01"
        letterActions={actions([
          letter({
            cancelled: true,
            printed: true,
            cancellationNumber: "MB-2027-0009",
          }),
          letter({
            kind: "cancellation",
            number: "MB-2027-0009",
            totalCents: -9600,
            recipientEmail: "",
          }),
        ])}
      />,
    );
    expect(
      screen.getAllByText(
        "Zum Drucken heruntergeladen. Geben Sie der Familie das Storno-Schreiben.",
      ).length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByText("Nicht gesendet. Die Familie bekommt nichts."),
    ).not.toBeInTheDocument();
  });

  it("asks to print for a family without an address", () => {
    render(
      <CareRunLetterInvoices
        invoices={[invoice()]}
        month="2027-01-01"
        letterActions={actions([letter({ recipientEmail: "" })])}
      />,
    );
    expect(screen.getAllByText("Ohne E-Mail").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Bitte drucken Sie das Schreiben.").length,
    ).toBeGreaterThan(0);
    const menu = within(openMenu(/Aktionen für die Rechnung von Mia/));
    expect(
      menu.getByRole("menuitem", { name: /Stornieren/ }),
    ).toBeInTheDocument();
    expect(menu.queryByRole("menuitem", { name: /Senden/ })).toBeNull();
  });
});
