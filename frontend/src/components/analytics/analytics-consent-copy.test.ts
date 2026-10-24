import { describe, expect, it, vi } from "vitest";
import en from "~/i18n/messages/en.json";
import {
  SCHOOL_ANALYTICS_CONSENT_COPY,
  TENANT_ANALYTICS_CONSENT_COPY,
  useParentAnalyticsConsentCopy,
} from "./analytics-consent-copy";

vi.mock("next-intl", () => ({
  useTranslations: (namespace: keyof typeof en) => (key: string) =>
    (en[namespace] as Record<string, string>)[key],
}));

describe("analytics consent copy", () => {
  it("names the account scope and the place where the answer changes", () => {
    expect(TENANT_ANALYTICS_CONSENT_COPY.promptChangeLater).toBe(
      "Ihre Wahl gilt für Ihr Konto. Sie können sie jederzeit unter „Profil“ ändern.",
    );
    expect(SCHOOL_ANALYTICS_CONSENT_COPY.promptChangeLater).toBe(
      "Ihre Wahl gilt für Ihr Konto. Sie können sie jederzeit unter „Einstellungen“ ändern.",
    );
  });

  it("names recordings only in the OGS portal, before and while they run", () => {
    expect(TENANT_ANALYTICS_CONSENT_COPY.recordingPossible).toMatch(
      /gilt Ihre Wahl auch dafür/,
    );
    expect(TENANT_ANALYTICS_CONSENT_COPY.recordingActive).toMatch(
      /Ihre Schule hat zusätzlich Aufzeichnungen erlaubt/,
    );
    expect(SCHOOL_ANALYTICS_CONSENT_COPY.recordingPossible).toBe("");
    expect(SCHOOL_ANALYTICS_CONSENT_COPY.recordingActive).toBe("");
  });

  it("translates every parent text and names no recording there", () => {
    const copy = useParentAnalyticsConsentCopy();
    const { recordingActive, recordingPossible, ...texts } = copy;

    expect(copy.promptTitle).toBe("Allow usage analytics?");
    for (const [key, text] of Object.entries(texts)) {
      expect(text, key).toEqual(expect.stringMatching(/\S/));
    }
    // Recordings exist only in the OGS portal with Analyse-Freigabe.
    expect(recordingActive).toBe("");
    expect(recordingPossible).toBe("");
  });
});
