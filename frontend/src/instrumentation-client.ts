import "./sentry.client.config";
import posthog from "posthog-js";
import { sanitizePostHogEvent } from "~/lib/posthog-privacy";

if (process.env.NEXT_PUBLIC_POSTHOG_KEY) {
  posthog.init(process.env.NEXT_PUBLIC_POSTHOG_KEY, {
    api_host: process.env.NEXT_PUBLIC_POSTHOG_HOST,
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
    advanced_disable_feature_flags: true,
    advanced_disable_feature_flags_on_first_load: true,
    before_send: sanitizePostHogEvent,
  });
}
