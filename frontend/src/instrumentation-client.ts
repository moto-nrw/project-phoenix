import "./sentry.client.config";
import posthog from "posthog-js";
import { sanitizePostHogEvent } from "~/lib/posthog-privacy";

const postHogKey = process.env.NEXT_PUBLIC_POSTHOG_KEY;

if (postHogKey) {
  const postHogHost = process.env.NEXT_PUBLIC_POSTHOG_HOST;
  if (!postHogHost) {
    throw new Error(
      "NEXT_PUBLIC_POSTHOG_HOST is required when NEXT_PUBLIC_POSTHOG_KEY is set.",
    );
  }

  posthog.init(postHogKey, {
    api_host: postHogHost,
    defaults: "2026-01-30",
    autocapture: false,
    rageclick: false,
    capture_pageview: false,
    capture_pageleave: false,
    capture_performance: false,
    capture_heatmaps: false,
    capture_dead_clicks: false,
    capture_exceptions: false,
    disable_scroll_properties: true,
    disable_session_recording: true,
    // Select memory storage during construction so the SDK never reads a
    // distinct ID persisted by an older deployment.
    persistence: "memory",
    disable_persistence: true,
    person_profiles: "never",
    save_referrer: false,
    save_campaign_params: false,
    disable_surveys: true,
    advanced_disable_flags: true,
    before_send: sanitizePostHogEvent,
  });
}
