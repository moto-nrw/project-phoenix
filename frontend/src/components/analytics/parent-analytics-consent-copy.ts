"use client";

import { useTranslations } from "next-intl";
import type { AnalyticsConsentCopy } from "~/components/analytics/analytics-consent-copy";

/** The consent texts of the parents portal, in the reader's language. */
export function useParentAnalyticsConsentCopy(): AnalyticsConsentCopy {
  const t = useTranslations("analyticsConsent");
  return {
    title: t("title"),
    promptTitle: t("promptTitle"),
    promptIntro: t("promptIntro"),
    promptPrivacy: t("promptPrivacy"),
    promptNoDisadvantage: t("promptNoDisadvantage"),
    promptChangeLater: t("promptChangeLater"),
    allow: t("allow"),
    refuse: t("refuse"),
    saving: t("saving"),
    closeLabel: t("closeLabel"),
    sectionDescription: t("sectionDescription"),
    switchLabel: t("switchLabel"),
    schoolOff: t("schoolOff"),
    // The parents portal is never recorded.
    recordingActive: "",
    recordingPossible: "",
    errorObject: t("errorObject"),
  };
}
