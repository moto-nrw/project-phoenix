import { describe, expect, it } from "vitest";
import type { FamilyBilling } from "~/lib/traeger-billing-terms-api";
import {
  bundleLabel,
  dueDaysLabel,
  dueSourceHint,
  findFamilyBilling,
  paymentMethodsText,
  recipientKindLabel,
  recipientLine,
  runDueDatesText,
} from "./billing-payment-format";

function family(patch: Partial<FamilyBilling>): FamilyBilling {
  return {
    id: "1",
    familyEmail: "anna@example.org",
    contractId: "",
    recipientKind: "applicant",
    secondFirstName: "",
    secondLastName: "",
    payerFirstName: "",
    payerLastName: "",
    payerEmail: "",
    payerAddress: null,
    paymentMethod: "",
    dueDay: 15,
    updatedAt: "2026-09-01T10:00:00Z",
    ...patch,
  };
}

describe("billing payment format", () => {
  it("names the payment term and the invoice a service travels on", () => {
    expect(dueDaysLabel(0)).toBe("Sofort");
    expect(dueDaysLabel(1)).toBe("1 Tag");
    expect(dueDaysLabel(14)).toBe("14 Tage");
    expect(bundleLabel("care", ["care"])).toBe(
      "Auf der Rechnung der Betreuung",
    );
    expect(bundleLabel("", ["care"])).toBe("Eigene Rechnung");
    expect(bundleLabel("", [])).toBe("Immer eigene Rechnung");
    expect(bundleLabel("", [], true)).toBe("Eigene Rechnung");
  });

  it("names both parents and the payer, not the contract party", () => {
    expect(recipientKindLabel("applicant")).toBeNull();
    expect(recipientKindLabel("")).toBeNull();
    expect(recipientKindLabel("both")).toBe("Beide Eltern");
    expect(recipientKindLabel("payer")).toBe("Abweichender Zahler");
    expect(
      recipientLine({
        firstName: "Anna",
        lastName: "Muster",
        kind: "both",
        secondName: "Ben Muster",
      }),
    ).toBe("An Anna Muster und Ben Muster");
    expect(recipientLine({ firstName: "", lastName: "" })).toBe(
      "Ohne Empfänger",
    );
  });

  it("lists the run's due dates in form order and hints a family's own day", () => {
    expect(runDueDatesText([])).toBeNull();
    expect(
      runDueDatesText([
        { paymentMethod: "direct_debit", dueOn: "2026-10-01" },
        { paymentMethod: "transfer", dueOn: "2026-10-15" },
      ]),
    ).toBe("Überweisung am 15.10.2026 · Lastschrift am 01.10.2026");
    expect(
      dueSourceHint({
        method: "transfer",
        dueOn: "2026-10-15",
        dueSource: "family",
      }),
    ).toBe("Zahltag der Familie");
    expect(
      dueSourceHint({
        method: "transfer",
        dueOn: "2026-10-15",
        dueSource: "run",
      }),
    ).toBeNull();
    expect(paymentMethodsText(["direct_debit", "transfer"])).toBe(
      "Überweisung, Lastschrift",
    );
    expect(paymentMethodsText([])).toBeNull();
  });

  it("finds a family's deviation by contract, else by its address", () => {
    const byContract = family({ id: "2", contractId: "42" });
    const byEmail = family({ id: "3", familyEmail: "anna@example.org" });
    const payer = family({
      id: "4",
      familyEmail: "carla@example.org",
      recipientKind: "payer",
      payerEmail: "Amt@Example.org",
    });
    const families = [byEmail, byContract, payer];

    expect(
      findFamilyBilling(families, {
        contractId: "42",
        recipientEmail: "other@example.org",
      })?.id,
    ).toBe("2");
    expect(
      findFamilyBilling(families, {
        contractId: "7",
        recipientEmail: " Anna@Example.org ",
      })?.id,
    ).toBe("3");
    // A payer deviation sends the invoice to the payer's address.
    expect(
      findFamilyBilling(families, {
        contractId: "8",
        recipientEmail: "amt@example.org",
        recipientKind: "payer",
      })?.id,
    ).toBe("4");
    expect(
      findFamilyBilling(families, {
        contractId: "9",
        recipientEmail: "nobody@example.org",
      }),
    ).toBeNull();
  });
});
