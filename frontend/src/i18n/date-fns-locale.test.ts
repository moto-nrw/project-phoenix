import { format } from "date-fns";
import { describe, expect, it } from "vitest";

import { dateFnsLocaleFor } from "./date-fns-locale";
import { SUPPORTED_LOCALES } from "./locales";

describe("dateFnsLocaleFor", () => {
  it("gives every registered locale its own date-fns locale", () => {
    const codes = SUPPORTED_LOCALES.map(
      ({ code }) => dateFnsLocaleFor(code).code,
    );
    expect(new Set(codes).size).toBe(SUPPORTED_LOCALES.length);
  });

  it("renders calendar month names in the new languages", () => {
    const september = new Date(2026, 8, 14);
    expect(format(september, "LLLL", { locale: dateFnsLocaleFor("pl") })).toBe(
      "wrzesień",
    );
    expect(format(september, "LLLL", { locale: dateFnsLocaleFor("tr") })).toBe(
      "Eylül",
    );
    expect(format(september, "LLLL", { locale: dateFnsLocaleFor("uk") })).toBe(
      "вересень",
    );
  });

  it("falls back to German for an unknown locale", () => {
    expect(dateFnsLocaleFor("xx").code).toBe("de");
  });
});
