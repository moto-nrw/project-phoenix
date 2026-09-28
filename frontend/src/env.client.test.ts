import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.unmock("~/env.client");

const requiredEnv = {
  NEXT_PUBLIC_API_URL: "https://api.moto-app.de",
  NEXT_PUBLIC_TENANT_DOMAIN: "moto-app.de",
  NEXT_PUBLIC_OPERATOR_HOSTNAME: "operator.moto-app.de",
  NEXT_PUBLIC_PARENTS_HOSTNAME: "eltern.moto-app.de",
  NEXT_PUBLIC_SCHOOL_HOSTNAME: "schule.moto-app.de",
};

describe("clientEnv", () => {
  beforeEach(() => {
    vi.resetModules();
    for (const [name, value] of Object.entries(requiredEnv)) {
      vi.stubEnv(name, value);
    }
    vi.stubEnv("NEXT_PUBLIC_POSTHOG_KEY", "");
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("reads public values without the Zod-backed server environment", async () => {
    const { clientEnv } = await import("./env.client");

    expect(clientEnv.NEXT_PUBLIC_API_URL).toBe(requiredEnv.NEXT_PUBLIC_API_URL);
    expect(clientEnv.NEXT_PUBLIC_POSTHOG_KEY).toBeUndefined();
  });

  it("fails fast when a required public value is missing", async () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", "");

    await expect(import("./env.client")).rejects.toThrow(
      "NEXT_PUBLIC_API_URL is not set",
    );
  });

  it("fails fast when a portal hostname is missing", async () => {
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", "");

    await expect(import("./env.client")).rejects.toThrow(
      "NEXT_PUBLIC_OPERATOR_HOSTNAME is not set",
    );
  });

  // The analytics runs behind the same-origin /ingest proxy (#3601): the key
  // alone switches it on.
  it("enables PostHog with the key alone", async () => {
    vi.stubEnv("NEXT_PUBLIC_POSTHOG_KEY", "phc_test_key_123");
    const { clientEnv } = await import("./env.client");

    expect(clientEnv.NEXT_PUBLIC_POSTHOG_KEY).toBe("phc_test_key_123");
  });
});
