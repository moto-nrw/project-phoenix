import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const mockInit = vi.fn();
const ANDROID_UA =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36";
const DESKTOP_CHROME_UA =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36";

vi.mock("posthog-js", () => ({
  default: {
    init: (...args: unknown[]) => mockInit(...args) as void,
  },
}));

describe("instrumentation-client", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    vi.stubEnv("NEXT_PUBLIC_TENANT_DOMAIN", "moto-app.de");
    vi.stubGlobal("navigator", { userAgent: ANDROID_UA });
    window.location.href = "https://school-a.moto-app.de/dashboard";
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
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

  it("captures the install prompt before React hydration", async () => {
    await import("./instrumentation-client");
    const { canPromptInstall } = await import("./lib/pwa-install-prompt");
    const event = Object.assign(
      new Event("beforeinstallprompt", { cancelable: true }),
      {
        prompt: vi.fn().mockResolvedValue(undefined),
        userChoice: Promise.resolve({ outcome: "accepted" as const }),
      },
    );

    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(true);
    expect(canPromptInstall()).toBe(true);
  });

  it("leaves Chrome's native install prompt enabled on desktop tenant hosts", async () => {
    vi.stubGlobal("navigator", { userAgent: DESKTOP_CHROME_UA });
    await import("./instrumentation-client");
    const { canPromptInstall } = await import("./lib/pwa-install-prompt");
    const event = Object.assign(
      new Event("beforeinstallprompt", { cancelable: true }),
      {
        prompt: vi.fn().mockResolvedValue(undefined),
        userChoice: Promise.resolve({ outcome: "accepted" as const }),
      },
    );

    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(false);
    expect(canPromptInstall()).toBe(false);
  });

  it.each([
    "https://operator.moto-app.de/",
    "https://eltern.moto-app.de/",
    "https://moto-app.de/",
    "https://help.moto-app.de/",
  ])("leaves Chrome's native install prompt enabled on %s", async (url) => {
    window.location.href = url;
    await import("./instrumentation-client");
    const { canPromptInstall } = await import("./lib/pwa-install-prompt");
    const event = Object.assign(
      new Event("beforeinstallprompt", { cancelable: true }),
      {
        prompt: vi.fn().mockResolvedValue(undefined),
        userChoice: Promise.resolve({ outcome: "accepted" as const }),
      },
    );

    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(false);
    expect(canPromptInstall()).toBe(false);
  });
});
