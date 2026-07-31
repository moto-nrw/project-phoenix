import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const mockInit = vi.fn();

vi.mock("posthog-js", () => ({
  default: {
    init: (...args: unknown[]) => mockInit(...args) as void,
  },
}));

describe("instrumentation-client", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("initializes PostHog with remote configuration disabled", async () => {
    vi.stubEnv("NEXT_PUBLIC_POSTHOG_KEY", "phc_test_key_123");
    vi.stubEnv("NEXT_PUBLIC_POSTHOG_HOST", "https://eu.i.posthog.com");

    await import("./instrumentation-client");

    expect(mockInit).toHaveBeenCalledOnce();
    expect(mockInit).toHaveBeenCalledWith(
      "phc_test_key_123",
      expect.objectContaining({
        api_host: "https://eu.i.posthog.com",
        defaults: "2026-01-30",
        autocapture: false,
        rageclick: false,
        capture_pageview: false,
        capture_pageleave: false,
        capture_performance: false,
        capture_heatmaps: false,
        capture_dead_clicks: false,
        capture_exceptions: false,
        disable_session_recording: true,
        persistence: "memory",
        disable_persistence: true,
        person_profiles: "never",
        save_referrer: false,
        save_campaign_params: false,
        disable_surveys: true,
        advanced_disable_flags: true,
        before_send: expect.any(Function),
      }),
    );
  });

  it("rejects an analytics key without an explicit ingestion host", async () => {
    vi.stubEnv("NEXT_PUBLIC_POSTHOG_KEY", "phc_test_key_123");
    vi.stubEnv("NEXT_PUBLIC_POSTHOG_HOST", "");

    await expect(import("./instrumentation-client")).rejects.toThrow(
      "NEXT_PUBLIC_POSTHOG_HOST is required when NEXT_PUBLIC_POSTHOG_KEY is set",
    );
    expect(mockInit).not.toHaveBeenCalled();
  });

  it("does not initialize PostHog when NEXT_PUBLIC_POSTHOG_KEY is not set", async () => {
    vi.stubEnv("NEXT_PUBLIC_POSTHOG_KEY", "");

    await import("./instrumentation-client");

    expect(mockInit).not.toHaveBeenCalled();
  });
});
