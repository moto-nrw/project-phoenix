import { describe, expect, it } from "vitest";
import type { CareInvoice } from "~/lib/traeger-care-billing-api";
import { debitHints } from "./care-run-review";
import {
  creditorIdLooksValid,
  ibanLooksValid,
  mandateEventLabel,
  referenceLooksValid,
} from "./sepa-format";

describe("SEPA checks (#4695)", () => {
  it("checks the IBAN's check digits as people type it", () => {
    expect(ibanLooksValid("de89 3704 0044 0532 0130 00")).toBe(true);
    expect(ibanLooksValid("DE89370400440532013001")).toBe(false);
    expect(ibanLooksValid("DE89A70400440532013000")).toBe(false);
    expect(ibanLooksValid("AT611904300234573201")).toBe(true);
  });

  it("checks the creditor identifier without its business code", () => {
    expect(creditorIdLooksValid("DE98ZZZ09999999999")).toBe(true);
    expect(creditorIdLooksValid("de98 abc0 9999 9999 99")).toBe(true);
    expect(creditorIdLooksValid("DE97ZZZ09999999999")).toBe(false);
  });

  it("keeps the mandate reference to the SEPA characters", () => {
    expect(referenceLooksValid("")).toBe(true);
    expect(referenceLooksValid("SJR-2026-0001")).toBe(true);
    expect(referenceLooksValid("Müller-1")).toBe(false);
    expect(referenceLooksValid("/SJR")).toBe(false);
  });

  it("names a change of the trail by its fields, never a value", () => {
    expect(
      mandateEventLabel({
        id: "1",
        action: "update",
        source: "office",
        fields: ["account_holder", "iban"],
        occurredAt: "2026-11-08T10:00:00+01:00",
      }),
    ).toBe("Geändert: Kontoinhaber, IBAN im Büro");
  });
});

function invoice(payment: CareInvoice["payment"]): CareInvoice {
  return {
    contractId: "3",
    service: "care",
    childFirstName: "Mia",
    childLastName: "Beispiel",
    schoolName: "Grundschule Ispringen",
    totalCents: 7800,
    payment,
  } as CareInvoice;
}

describe("debitHints (#4695)", () => {
  it("lists a family without mandate and a debit sooner than announced", () => {
    const hints = debitHints([
      invoice({
        method: "transfer",
        dueOn: "2027-01-02",
        dueSource: "run",
        debitGap: "mandate_missing",
      }),
      invoice({
        method: "direct_debit",
        dueOn: "2027-01-04",
        dueSource: "run",
        mandate: {
          reference: "SJR-1",
          signedOn: "2026-09-01",
          creditorId: "DE98ZZZ09999999999",
          ibanMasked: "DE•• 30 00",
          sequenceType: "RCUR",
          noticeShort: true,
        },
      }),
      invoice({
        method: "direct_debit",
        dueOn: "2027-01-04",
        dueSource: "run",
      }),
    ]);
    expect(hints.map((hint) => hint.text)).toEqual([
      "Kein gültiges Mandat. Geht als Überweisung, fällig am 02.01.2027.",
      "Abbuchung schon am 04.01.2027. Das ist früher als Ihre Vorabankündigung erlaubt.",
    ]);
  });
});
