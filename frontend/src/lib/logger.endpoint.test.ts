import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.unmock("~/lib/logger");

const { createLogger } = await import("./logger");

// Each portal ships client logs to the route that can verify its session
// cookie. Before, the parents portal posted to the tenant-only /api/logs and
// every guardian's log was rejected with 401.
describe("client logger endpoint", () => {
  let originalFetch: typeof globalThis.fetch;
  let originalHref: string;
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    originalHref = window.location.href;
    fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response(null, { status: 200 }));
    globalThis.fetch = fetchMock;
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.moto-app.de");
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    window.location.href = originalHref;
    vi.unstubAllEnvs();
  });

  async function shipOneEntry(): Promise<unknown> {
    const logger = createLogger({ component: "EndpointProbe" });
    logger.error("endpoint_probe");
    await logger.flush();
    return fetchMock.mock.calls[0]?.[0];
  }

  it("ships to the parent route on the parents host", async () => {
    window.location.href = "https://eltern.moto-app.de/news";

    expect(await shipOneEntry()).toBe("/api/parent/logs");
  });

  it("ships to the tenant route on a school subdomain", async () => {
    window.location.href = "https://ogs-wissingen.moto-app.de/home";

    expect(await shipOneEntry()).toBe("/api/logs");
  });
});
