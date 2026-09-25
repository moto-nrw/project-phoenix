import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const mockInit = vi.fn();
vi.mock("@sentry/nextjs", () => ({ init: mockInit }));

// Each test loads the config afresh; the shared module it compares against
// must come from the same module graph.
async function initOptionsOf(config: "client" | "server" | "edge") {
  if (config === "client") await import("./sentry.client.config");
  if (config === "server") await import("./sentry.server.config");
  if (config === "edge") await import("./sentry.edge.config");
  expect(mockInit).toHaveBeenCalledTimes(1);
  return {
    options: mockInit.mock.calls[0]?.[0] as Record<string, unknown>,
    shared: await import("./sentry.shared"),
  };
}

describe("Sentry runtime configs", () => {
  beforeEach(() => {
    vi.resetModules();
    mockInit.mockClear();
    vi.stubEnv("NEXT_PUBLIC_SENTRY_DSN", "https://key@sentry.test/1");
    vi.stubEnv("NEXT_PUBLIC_SENTRY_ENVIRONMENT", "staging");
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it.each(["client", "server", "edge"] as const)(
    "the %s config sets the shared data boundary and scrubs every event",
    async (config) => {
      const { options, shared } = await initOptionsOf(config);

      expect(options.dataCollection).toBe(shared.sentryDataCollection);
      expect(options.beforeSend).toBe(shared.scrubEvent);
      expect(options).not.toHaveProperty("sendDefaultPii");
      expect(options.environment).toBe("staging");
    },
  );

  it("the client config scrubs spans and samples page views in the browser", async () => {
    const { options, shared } = await initOptionsOf("client");

    expect(options.beforeSendSpan).toBe(shared.scrubSpan);
    expect(options.tracesSampler).toBe(shared.sampleBrowserTrace);
    // v11 streams spans and never calls beforeSendTransaction.
    expect(options).not.toHaveProperty("beforeSendTransaction");
  });

  it.each(["server", "edge"] as const)(
    "the %s config records no spans",
    async (config) => {
      const { options, shared } = await initOptionsOf(config);

      expect(options.tracesSampler).toBe(shared.sampleNoTrace);
    },
  );

  it("does not start Sentry without a DSN", async () => {
    vi.stubEnv("NEXT_PUBLIC_SENTRY_DSN", "");

    await import("./sentry.server.config");

    expect(mockInit).not.toHaveBeenCalled();
  });
});
