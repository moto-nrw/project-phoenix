import { describe, expect, it } from "vitest";

import { SUPPORTED_LOCALES } from "~/i18n/locales";
import { portalNotBookedExplanation } from "./portal-not-booked";

describe("portalNotBookedExplanation", () => {
  it("names the school, not the moto team, in every parent language", () => {
    for (const { code } of SUPPORTED_LOCALES) {
      const text = portalNotBookedExplanation("parent", code);
      expect(text, code).not.toBe("");
      expect(text, code).not.toMatch(/moto-Team|moto team/i);
    }
    expect(portalNotBookedExplanation("parent", "de")).toBe(
      "Ihre Schule nutzt das Elternportal zurzeit nicht. Bei Fragen wenden Sie sich bitte an Ihre Schule.",
    );
  });

  it("explains the school portal in German", () => {
    expect(portalNotBookedExplanation("school", "en")).toBe(
      "Ihre Schule nutzt moto schule zurzeit nicht. Bei Fragen wenden Sie sich bitte an Ihre OGS-Verwaltung.",
    );
  });
});
