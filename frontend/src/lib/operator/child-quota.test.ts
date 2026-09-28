import { describe, expect, it } from "vitest";
import {
  childQuotaLimit,
  CHILD_QUOTA_DEFAULT_BUNDLE_SIZE,
  childQuotaWarning,
  validateChildQuotaDraft,
} from "./child-quota";

describe("validateChildQuotaDraft", () => {
  it("accepts whole bundles and computes the Kinderkontingent", () => {
    expect(validateChildQuotaDraft({ bundles: "2", bundleSize: "50" })).toEqual(
      { ok: true, bundles: 2, bundleSize: 50, limit: 100 },
    );
  });

  it("tolerates surrounding spaces", () => {
    expect(
      validateChildQuotaDraft({ bundles: " 3 ", bundleSize: " 40 " }),
    ).toEqual({ ok: true, bundles: 3, bundleSize: 40, limit: 120 });
  });

  it.each(["", "0", "-1", "1.5", "1,5", "2a", "1e2", "1001"])(
    "rejects %j bundles with a form error",
    (bundles) => {
      const result = validateChildQuotaDraft({ bundles, bundleSize: "50" });
      expect(result).toEqual({
        ok: false,
        field: "bundles",
        error: "Bitte geben Sie eine ganze Zahl von 1 bis 1000 ein.",
      });
    },
  );

  it.each(["", "0", "12.5", "1001"])(
    "rejects %j as bundle size with a form error",
    (bundleSize) => {
      const result = validateChildQuotaDraft({ bundles: "2", bundleSize });
      expect(result).toEqual({
        ok: false,
        field: "bundleSize",
        error: "Bitte geben Sie eine ganze Zahl von 1 bis 1000 ein.",
      });
    },
  );
});

describe("childQuotaLimit", () => {
  it("is bundles times bundle size", () => {
    expect(childQuotaLimit({ bundles: 3, bundleSize: 50 })).toBe(150);
  });

  it("is null without a Kinderkontingent", () => {
    expect(childQuotaLimit({ bundles: null, bundleSize: 50 })).toBeNull();
  });
});

describe("childQuotaWarning", () => {
  it("names both numbers when the Kinderkontingent is below the Kontingentzahl", () => {
    expect(childQuotaWarning(100, 120)).toBe(
      "120 Kinder zählen, Kontingent 100. Speichern ist trotzdem möglich. Danach kann die Schule keine weiteren Kinder aufnehmen, bis weniger als 100 Kinder zählen.",
    );
  });

  it("stays silent when the Kontingentzahl fits", () => {
    expect(childQuotaWarning(100, 100)).toBeNull();
    expect(childQuotaWarning(100, 20)).toBeNull();
  });
});

it("defaults the bundle size to 50", () => {
  expect(CHILD_QUOTA_DEFAULT_BUNDLE_SIZE).toBe(50);
});
