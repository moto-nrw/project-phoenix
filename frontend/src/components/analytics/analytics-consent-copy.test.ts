import { describe, expect, it, vi } from "vitest";
import en from "~/i18n/messages/en.json";
import {
  germanAnalyticsConsentCopy,
  useParentAnalyticsConsentCopy,
} from "./analytics-consent-copy";

vi.mock("next-intl", () => ({
  useTranslations: (namespace: keyof typeof en) => (key: string) =>
    (en[namespace] as Record<string, string>)[key],
}));

describe("analytics consent copy", () => {
  it("names the place where the answer changes later", () => {
    expect(germanAnalyticsConsentCopy("Profil").promptChangeLater).toBe(
      "Sie können das jederzeit unter „Profil“ ändern.",
    );
    expect(germanAnalyticsConsentCopy("Einstellungen").promptChangeLater).toBe(
      "Sie können das jederzeit unter „Einstellungen“ ändern.",
    );
  });

  it("translates every parent text and names no recording there", () => {
    const copy = useParentAnalyticsConsentCopy();
    const { promptRecording, recording, ...texts } = copy;

    expect(copy.promptTitle).toBe("Allow usage analytics?");
    for (const [key, text] of Object.entries(texts)) {
      expect(text, key).toEqual(expect.stringMatching(/\S/));
    }
    // Recordings exist only in the OGS portal with Analyse-Freigabe.
    expect(promptRecording).toBe("");
    expect(recording).toBe("");
  });
});
