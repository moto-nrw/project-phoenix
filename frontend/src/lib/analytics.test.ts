import { describe, it, expect, vi, beforeEach } from "vitest";

const mockCapture = vi.fn();
const mockEnv = vi.hoisted(() => ({
  NEXT_PUBLIC_POSTHOG_KEY: undefined as string | undefined,
}));

vi.mock("posthog-js", () => ({
  default: {
    capture: (...args: unknown[]) => mockCapture(...args) as void,
  },
}));

vi.mock("~/env", () => ({ env: mockEnv }));

import { trackEvent } from "./analytics";

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
});
