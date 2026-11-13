import { describe, expect, it } from "vitest";
import { belegTypeLabel } from "./beleg-retention-api";

describe("belegTypeLabel", () => {
  it("names the Belege", () => {
    expect(belegTypeLabel("care_contract")).toBe("Betreuungsverträge");
  });

  // The deletion log also counts the debtor numbers of families without a
  // care contract (#6692); they are no Beleg type of their own.
  it("names the family debtor numbers of the deletion log", () => {
    expect(belegTypeLabel("family_debtor_number")).toBe(
      "Debitorennummern von Familien ohne Vertrag",
    );
  });
});
