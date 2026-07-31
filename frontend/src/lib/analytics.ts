/**
 * Product analytics wrapper around PostHog.
 *
 * All custom event capture goes through trackEvent — never import
 * posthog.capture directly in components. Events must not contain
 * PII; student IDs are forbidden entirely (GDPR).
 */

import posthog from "posthog-js";
import { env } from "~/env";
import { createLogger } from "~/lib/logger";
import {
  isAnalyticsViewId,
  type AnalyticsViewId,
} from "~/lib/analytics-routes";

const logger = createLogger({ component: "Analytics" });

export type AnalyticsEvent =
  | "login_success"
  | "login_failed"
  | "tenant_switched"
  | "suggestion_created"
  | "suggestion_voted"
  | "group_created"
  | "group_updated"
  | "user_invited"
  | "data_exported";

export function trackEvent(
  event: AnalyticsEvent,
  props?: Record<string, string | number | boolean>,
): void {
  if (!env.NEXT_PUBLIC_POSTHOG_KEY) {
    return;
  }
  try {
    posthog.capture(event, props);
  } catch (err) {
    // Analytics must never break the calling flow — but a broken SDK
    // should not go dark silently either.
    logger.warn("analytics_capture_failed", {
      event,
      error: err instanceof Error ? err.message : String(err),
    });
  }
}

export function trackPageView(viewId: AnalyticsViewId, schoolId: string): void {
  if (!env.NEXT_PUBLIC_POSTHOG_KEY) return;
  if (!isAnalyticsViewId(viewId) || !/^\d+$/.test(schoolId)) return;

  try {
    posthog.capture("page_viewed", {
      view_id: viewId,
      portal: "tenant",
      deployment: env.NEXT_PUBLIC_TENANT_DOMAIN,
      school_id: schoolId,
      $groups: { school: schoolId },
      $geoip_disable: true,
      $process_person_profile: false,
    });
  } catch (err) {
    logger.warn("analytics_capture_failed", {
      event: "page_viewed",
      error: err instanceof Error ? err.message : String(err),
    });
  }
}
