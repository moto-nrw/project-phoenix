"use client";

import dynamic from "next/dynamic";
import type { AnalyticsConsentCopy } from "~/components/analytics/analytics-consent-copy";
import {
  analyticsConsentApplies,
  useAnalyticsConsent,
  type AnalyticsConsentPortal,
} from "~/lib/analytics-consent";

const AnalyticsConsentPrompt = dynamic(
  () =>
    import("~/components/analytics/analytics-consent-prompt").then(
      (module) => module.AnalyticsConsentPrompt,
    ),
  { ssr: false },
);

/**
 * Mounted on every portal page; loads the consent prompt (#4901) only for a
 * session that still has to answer, so decided sessions never download it.
 */
export function AnalyticsConsentPromptGate(
  props: Readonly<{
    portal: AnalyticsConsentPortal;
    copy: AnalyticsConsentCopy;
    recording?: boolean;
  }>,
) {
  const consent = useAnalyticsConsent();
  const undecided =
    consent.portal === props.portal &&
    consent.status === "ready" &&
    consent.granted === null &&
    consent.schoolAllows &&
    analyticsConsentApplies();
  return undecided ? <AnalyticsConsentPrompt {...props} /> : null;
}
