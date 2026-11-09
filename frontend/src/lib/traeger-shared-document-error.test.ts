import { describe, expect, it } from "vitest";
import { ERROR_CATALOG } from "~/lib/error-catalog.generated";

// A parent opens a document the Träger no longer shares (#4692 review). Every
// locale of the parents portal names that condition and the reload; none
// falls back to the class text that asks to check the input.
describe("traeger.shared_document_not_found", () => {
  it.each(["de", "en", "pl", "tr", "ru", "sq", "uk"] as const)(
    "has its own text in %s",
    (locale) => {
      const catalog: Readonly<Record<string, string>> =
        ERROR_CATALOG[locale].codes;
      const text = catalog["traeger.shared_document_not_found"];
      expect(text).toBeTruthy();
      expect(text).not.toContain("{object}");
    },
  );
});
