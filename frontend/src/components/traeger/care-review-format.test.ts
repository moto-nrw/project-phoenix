import { describe, expect, it } from "vitest";
import {
  exclusionScopeText,
  exclusionTitle,
  invoiceKindLabel,
} from "./care-review-format";
import { formatEuro } from "./contribution-format";

// An exclusion leaves out exactly one invoice of a run (#6346): the texts of
// the dialog name the other invoice of the contract that stays, or the Mensa
// that travels on the care invoice and goes with it.

function invoice(
  service: "care" | "meal",
  overrides: Partial<{
    contractId: string;
    totalCents: number;
    meals: number;
    excluded: boolean;
  }> = {},
) {
  return {
    contractId: overrides.contractId ?? "22",
    service,
    childFirstName: "Mia",
    totalCents: overrides.totalCents ?? (service === "meal" ? 960 : 7750),
    mealPositions: Array.from({ length: overrides.meals ?? 0 }, () => ({})),
    exclusion: overrides.excluded ? { reason: "Prüfung" } : null,
  };
}

describe("exclusion of one invoice", () => {
  it("names the care invoice that stays when the Mensa invoice is excluded", () => {
    const care = invoice("care");
    const meal = invoice("meal", { meals: 1 });
    expect(exclusionTitle(meal, [care, meal])).toBe(
      "Mensa-Rechnung ausschließen",
    );
    expect(invoiceKindLabel(meal)).toBe("Mensa-Rechnung");
    expect(exclusionScopeText(meal, [care, meal], formatEuro)).toBe(
      `Die Betreuungsrechnung von Mia über ${formatEuro(7750)} bleibt im Lauf.`,
    );
  });

  it("names the Mensa invoice that stays when the care invoice is excluded", () => {
    const care = invoice("care");
    const meal = invoice("meal", { meals: 1 });
    expect(exclusionTitle(care, [care, meal])).toBe(
      "Betreuungsrechnung ausschließen",
    );
    expect(exclusionScopeText(care, [care, meal], formatEuro)).toBe(
      `Die Mensa-Rechnung von Mia über ${formatEuro(960)} bleibt im Lauf.`,
    );
  });

  it("says when the other invoice is excluded already", () => {
    const care = invoice("care", { excluded: true });
    const meal = invoice("meal", { meals: 1 });
    expect(exclusionScopeText(meal, [care, meal], formatEuro)).toBe(
      "Die Betreuungsrechnung von Mia ist schon ausgeschlossen.",
    );
  });

  it("says that Mensa on the care invoice goes with it", () => {
    const bundled = invoice("care", { meals: 2 });
    expect(invoiceKindLabel(bundled)).toBe("Betreuung mit Mensa");
    expect(exclusionTitle(bundled, [bundled])).toBe("Rechnung ausschließen");
    expect(exclusionScopeText(bundled, [bundled], formatEuro)).toBe(
      "Die Mensa steht auf dieser Rechnung. Sie wird mit ausgeschlossen.",
    );
  });

  it("adds nothing for an invoice that is the contract's only one", () => {
    const care = invoice("care");
    const other = invoice("meal", { contractId: "23", meals: 1 });
    expect(invoiceKindLabel(care)).toBe("Betreuung");
    expect(exclusionTitle(care, [care, other])).toBe("Rechnung ausschließen");
    expect(exclusionScopeText(care, [care, other], formatEuro)).toBeNull();
  });
});
