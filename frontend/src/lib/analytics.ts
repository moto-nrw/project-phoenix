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
