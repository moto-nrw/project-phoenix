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

  it("initializes PostHog when NEXT_PUBLIC_POSTHOG_KEY is set", async () => {
    vi.stubEnv("NEXT_PUBLIC_POSTHOG_KEY", "phc_test_key_123");
    vi.stubEnv("NEXT_PUBLIC_POSTHOG_HOST", "https://eu.i.posthog.com");

    await import("./instrumentation-client");

    expect(mockInit).toHaveBeenCalledWith("phc_test_key_123", {
      api_host: "https://eu.i.posthog.com",
      defaults: "2026-01-30",
    });
  });

  it("does not initialize PostHog when NEXT_PUBLIC_POSTHOG_KEY is not set", async () => {
    vi.stubEnv("NEXT_PUBLIC_POSTHOG_KEY", "");

    await import("./instrumentation-client");

    expect(mockInit).not.toHaveBeenCalled();
  });
});
