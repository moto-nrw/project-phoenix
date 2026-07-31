import { describe, it, expect, vi, beforeEach } from "vitest";

const mockCapture = vi.fn();
const mockEnv = vi.hoisted(() => ({
  NEXT_PUBLIC_POSTHOG_KEY: undefined as string | undefined,
  NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
}));

vi.mock("posthog-js", () => ({
  default: {
    capture: (...args: unknown[]) => mockCapture(...args) as void,
  },
}));

vi.mock("~/env", () => ({ env: mockEnv }));

import { trackEvent, trackPageView, trackTenantEvent } from "./analytics";

describe("trackEvent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockEnv.NEXT_PUBLIC_POSTHOG_KEY = undefined;
  });

  it("forwards event name and props to posthog.capture when the key is set", () => {
    mockEnv.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";

    trackEvent("suggestion_created", { direction: "up" });

    expect(mockCapture).toHaveBeenCalledWith("suggestion_created", {
      direction: "up",
    });
  });

  it("is a no-op when NEXT_PUBLIC_POSTHOG_KEY is not set", () => {
    trackEvent("login_success");

    expect(mockCapture).not.toHaveBeenCalled();
  });

  it("swallows capture errors instead of breaking the caller", () => {
    mockEnv.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";
    mockCapture.mockImplementation(() => {
      throw new Error("boom");
    });

    expect(() => trackEvent("login_success")).not.toThrow();
  });

  it("logs a warning when capture throws", () => {
    mockEnv.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";
    mockCapture.mockImplementation(() => {
      throw new Error("boom");
    });
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {
      // suppress test output
    });

    trackEvent("login_success");

    expect(warnSpy).toHaveBeenCalledWith("analytics_capture_failed", {
      event: "login_success",
      error: "boom",
    });
    warnSpy.mockRestore();
  });

  it("captures allowlisted page views without an account identifier", () => {
    mockEnv.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";

    trackPageView("/students/:id", "42");

    expect(mockCapture).toHaveBeenCalledWith("page_viewed", {
      view_id: "/students/:id",
      portal: "tenant",
      deployment: "localhost",
      school_id: "42",
      $groups: { school: "42" },
      $geoip_disable: true,
      $process_person_profile: false,
    });
  });

  it("attaches trusted school context directly to pre-session events", () => {
    mockEnv.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";

    trackTenantEvent("login_failed", "42", {
      reason: "invalid_credentials",
    });

    expect(mockCapture).toHaveBeenCalledWith("login_failed", {
      reason: "invalid_credentials",
      deployment: "localhost",
      school_id: "42",
      $groups: { school: "42" },
    });
  });

  it("rejects tenant events without a numeric school ID", () => {
    mockEnv.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";

    trackTenantEvent("login_success", "school-a");

    expect(mockCapture).not.toHaveBeenCalled();
  });
});
