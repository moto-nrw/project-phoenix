/**
 * Product analytics wrapper around PostHog.
 *
 * All custom event capture goes through trackEvent — never import
 * posthog.capture directly in components. Events must not contain
 * PII; student IDs are forbidden entirely (GDPR).
 */

import type { Properties } from "posthog-js";
import { analyticsDeployment } from "~/lib/analytics-deployment";
import type { AnalyticsRole, AnalyticsSurface } from "~/lib/analytics-policy";
import type { DemoVisit } from "~/lib/demo-access";
import {
  capturePostHog,
  clearPostHogContext,
  resetAndCapturePostHog,
  setAnalyticsSurface,
  setPostHogContext,
} from "~/lib/posthog-client";

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
    deployment: analyticsDeployment(),
    school_id: schoolId,
  };

  // A completed tenant switch belongs to the target school and must not share
  // the previous school's anonymous runtime identity.
  if (event === "tenant_switched") {
    resetAndCapturePostHog(event, eventProperties);
    return;
  }

  captureEvent(event, eventProperties);
}

export interface PortalAnalyticsSession {
  readonly surface: AnalyticsSurface;
  /** The school of the session; null where the token has none (parents). */
  readonly schoolId: string | null;
  readonly role: AnalyticsRole;
}

/**
 * Registers the signed-in portal session as analytics context: the surface
 * for the filter, `school_id` and `role` for every later event. Page views,
 * clicks, and heatmaps come from the SDK itself. Never an account or person
 * identifier.
 */
export function registerPortalSession(
  session: PortalAnalyticsSession,
  resetFirst: boolean,
): void {
  setAnalyticsSurface(session.surface);
  setPostHogContext(
    {
      ...(session.schoolId && /^\d+$/.test(session.schoolId)
        ? { school_id: session.schoolId }
        : {}),
      role: session.role,
    },
    resetFirst,
  );
}

/** Logout or school change: back to an anonymous visit of the host. */
export function clearPortalSession(): void {
  clearPostHogContext();
}

/**
 * The OGS portal's role property. Role names are school data (a school can
 * create its own roles), so only the admin flag reaches the analytics. The
 * read-only staff preview (#2893) is an admin at work, not the staff member.
 */
export function ogsAnalyticsRole(user: {
  readonly isAdmin?: boolean;
  readonly isPreview?: boolean;
}): AnalyticsRole {
  return user.isAdmin === true || user.isPreview === true ? "admin" : "staff";
}

export type DemoAnalyticsEvent =
  | "demo_entered"
  | "demo_role_switched"
  | "demo_restarted"
  | "demo_start_clicked";

// The visitor of the public demo is the demo access (#3467): its ID is the
// distinct_id, so the team can follow one visit without a name or address.
// An unknown visit (no storage) still counts, as the demo without identity.
function demoProperties(visit: DemoVisit | null): Properties {
  if (!visit) return { deployment: "demo" };
  return {
    ...(visit.accessId ? { distinct_id: visit.accessId } : {}),
    deployment: "demo",
    demo_role: visit.role,
    ...(visit.src ? { src: visit.src } : {}),
  };
}

/** Makes every later event of this page belong to the demo visit. */
export function registerDemoVisit(visit: DemoVisit): void {
  setPostHogContext(demoProperties(visit), false);
}

export function trackDemoEvent(
  event: DemoAnalyticsEvent,
  visit: DemoVisit | null,
): void {
  capturePostHog(event, demoProperties(visit));
}
