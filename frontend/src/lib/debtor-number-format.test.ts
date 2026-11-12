import { describe, expect, it } from "vitest";
import {
  NO_DEBTOR_NUMBER_FORMAT,
  debtorNumberExample,
  debtorNumberFormat,
  debtorNumberFormatError,
  debtorNumberFormatHint,
  debtorNumberFormatOf,
} from "./debtor-number-format";

describe("debtor number format (#6675)", () => {
  it("admits every number without the setting", () => {
    expect(debtorNumberFormatError("10 4711-A", NO_DEBTOR_NUMBER_FORMAT)).toBe(
      null,
    );
    expect(debtorNumberFormatHint(NO_DEBTOR_NUMBER_FORMAT)).toBe("");
    expect(
      debtorNumberFormatOf({
        digits_only: false,
        min_digits: 5,
        max_digits: 5,
      }),
    ).toEqual(NO_DEBTOR_NUMBER_FORMAT);
  });

  it("checks digits and length like the backend, and names the format", () => {
    const five = debtorNumberFormat(true, 5, 5);
    expect(debtorNumberFormatError(" 20815 ", five)).toBe(null);
    expect(debtorNumberFormatError("", five)).toBe(null);
    for (const wrong of ["20 815", "2081", "208150", "2081A", "２０８１５"]) {
      expect(debtorNumberFormatError(wrong, five)).toBe(
        "Die Debitorennummer hat 5 Ziffern, zum Beispiel 20815.",
      );
    }
    expect(debtorNumberFormatHint(five)).toBe(
      "Die Nummer hat 5 Ziffern, zum Beispiel 20815.",
    );
  });

  it("takes the two lengths in either order and keeps them in 1 to 35", () => {
    const range = debtorNumberFormat(true, 8, 6);
    expect(range).toEqual({ digitsOnly: true, minDigits: 6, maxDigits: 8 });
    expect(debtorNumberExample(range)).toBe("208154");
    expect(debtorNumberFormatError("20815", range)).toBe(
      "Die Debitorennummer hat 6 bis 8 Ziffern, zum Beispiel 208154.",
    );
    expect(debtorNumberExample(debtorNumberFormat(true, 3, 3))).toBe("208");
    expect(debtorNumberExample(debtorNumberFormat(true, 1, 35))).toBe("20815");
    expect(debtorNumberExample(debtorNumberFormat(true, 35, 99))).toHaveLength(
      35,
    );
    expect(debtorNumberFormatHint(debtorNumberFormat(true, 1, 1))).toBe(
      "Die Nummer hat 1 Ziffer, zum Beispiel 2.",
    );
  });
});
