"use client";

import { PortalAnalyticsSession } from "~/components/analytics/portal-analytics-session";
import { useParentAnalyticsConsentCopy } from "~/components/analytics/parent-analytics-consent-copy";

/** The parents portal's analytics session with its translated consent texts. */
export function ParentPortalAnalyticsSession() {
  return (
    <PortalAnalyticsSession
      surface="parents"
      consentCopy={useParentAnalyticsConsentCopy()}
    />
  );
}
