/**
 * Product analytics wrapper around PostHog.
 *
 * All custom event capture goes through trackEvent — never import
 * posthog.capture directly in components. Events must not contain
 * PII; student IDs are forbidden entirely (GDPR).
 */

import type { Properties } from "posthog-js";
import { clientEnv } from "~/env.client";
import { capturePostHog, resetAndCapturePostHog } from "~/lib/posthog-client";
import {
  isAnalyticsViewId,
  type AnalyticsViewId,
} from "~/lib/analytics-routes";

export type AnalyticsEvent =
  | "login_success"
  | "login_failed"
  | "tenant_switched"
  | "group_created"
  | "group_updated"
  | "user_invited"
  | "data_exported"
  | "pwa_install_prompt_shown"
  | "pwa_install_prompt_accepted"
  | "pwa_install_prompt_dismissed"
  | "pwa_installed";

function captureEvent(event: AnalyticsEvent, props?: Properties): void {
  capturePostHog(event, props);
}

export function trackEvent(
  event: AnalyticsEvent,
  props?: Record<string, string | number | boolean>,
): void {
  captureEvent(event, props);
}

export function trackTenantEvent(
  event: AnalyticsEvent,
  schoolId: string,
  props?: Record<string, string | number | boolean>,
): void {
  if (!/^\d+$/.test(schoolId)) return;

  const eventProperties = {
    ...props,
    deployment: clientEnv.NEXT_PUBLIC_TENANT_DOMAIN,
    school_id: schoolId,
    $groups: { school: schoolId },
  };

  // A completed tenant switch belongs to the target school and must not share
  // the previous school's anonymous runtime identity.
  if (event === "tenant_switched") {
    resetAndCapturePostHog(event, eventProperties);
    return;
  }

  captureEvent(event, eventProperties);
}

export function trackPageView(viewId: AnalyticsViewId, schoolId: string): void {
  if (!isAnalyticsViewId(viewId) || !/^\d+$/.test(schoolId)) return;

  capturePostHog("page_viewed", {
    view_id: viewId,
    portal: "tenant",
    deployment: clientEnv.NEXT_PUBLIC_TENANT_DOMAIN,
    school_id: schoolId,
    $groups: { school: schoolId },
    $geoip_disable: true,
    $process_person_profile: false,
  });
}
