import { describe, expect, it } from "vitest";
import {
  BUILT_IN_PAYMENT_TEXT,
  paymentTextPreview,
  paymentTextProblem,
} from "./traeger-payment-text";

// The four placeholders and the rules of the backend registry (#5622):
// parent_billing.payment_text in backend/modules/settings/internal/defaults.
describe("paymentTextProblem", () => {
  it("takes the four placeholders, the built-in sentence and an empty text", () => {
    expect(
      paymentTextProblem(
        "{Betrag} {Zahlungsfrist} {Rechnungsnummer} {Debitorennummer} {Betrag}",
      ),
    ).toBeNull();
    expect(paymentTextProblem(BUILT_IN_PAYMENT_TEXT)).toBeNull();
    expect(paymentTextProblem("")).toBeNull();
    expect(paymentTextProblem("ü".repeat(600))).toBeNull();
  });

  it.each([
    ["{Kind}", "„{Kind}“ ist kein Platzhalter."],
    ["{betrag}", "„{betrag}“ ist kein Platzhalter."],
    ["{}", "„{}“ ist kein Platzhalter."],
    [
      "Bitte {Betrag bis {Zahlungsfrist}",
      "Eine geschweifte Klammer steht allein.",
    ],
    ["Betrag}", "Eine geschweifte Klammer steht allein."],
    ["ü".repeat(601), "Der Text ist zu lang. Erlaubt sind 600 Zeichen."],
  ])("refuses %s", (text, start) => {
    expect(paymentTextProblem(text)).toMatch(
      new RegExp(`^${start.replaceAll(/[{}.]/g, "\\$&")}`),
    );
  });
});

describe("paymentTextPreview", () => {
  it("fills in the example values as often as they stand", () => {
    expect(
      paymentTextPreview(
        "{Betrag}, {Betrag}, {Zahlungsfrist}, {Rechnungsnummer}, {Debitorennummer}",
        false,
      ),
    ).toBe("124,00 €, 124,00 €, 15.12.2026, FE-2026-0007, 10 4711");
  });

  it("shows the built-in sentence for an empty text, with the debtor number when the Träger asks for it", () => {
    expect(paymentTextPreview("  ", false)).toBe(
      "Bitte überweisen Sie 124,00 € bis zum 15.12.2026 auf das Konto unten. Geben Sie als Verwendungszweck die Rechnungsnummer FE-2026-0007 an.",
    );
    expect(paymentTextPreview("", true)).toBe(
      "Bitte überweisen Sie 124,00 € bis zum 15.12.2026 auf das Konto unten. Geben Sie als Verwendungszweck Ihre Debitorennummer 10 4711 an.",
    );
  });
});
