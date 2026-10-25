"use client";

import { useParentAnalyticsConsentCopy } from "~/components/analytics/parent-analytics-consent-copy";
import { AnalyticsConsentSection } from "~/components/settings/analytics-consent-section";

/** Parents settings: the consent card in the reader's language. */
export function ParentAnalyticsConsentSection() {
  return (
    <AnalyticsConsentSection
      portal="parent"
      copy={useParentAnalyticsConsentCopy()}
    />
  );
}
