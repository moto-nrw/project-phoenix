import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.unmock("~/lib/logger");

const { createLogger } = await import("./logger");

// Each portal ships client logs to the route that can verify its session
// cookie. On the school and Träger hosts the proxy answered the tenant
// /api/logs with 404, and on the operator host it needed a tenant cookie.
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
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", "operator.moto-app.de");
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.moto-app.de");
    vi.stubEnv("NEXT_PUBLIC_SCHOOL_HOSTNAME", "schule.moto-app.de");
    vi.stubEnv("NEXT_PUBLIC_CATERER_HOSTNAME", "caterer.moto-app.de");
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.moto-app.de");
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

  it.each([
    ["operator.moto-app.de", "/api/operator/logs"],
    ["eltern.moto-app.de", "/api/parent/logs"],
    ["schule.moto-app.de", "/api/school/logs"],
    ["caterer.moto-app.de", "/api/caterer/logs"],
    ["traeger.moto-app.de", "/api/traeger/logs"],
    ["ogs-wissingen.moto-app.de", "/api/logs"],
  ])("ships from %s to %s", async (host, logPath) => {
    window.location.href = `https://${host}/`;

    expect(await shipOneEntry()).toBe(logPath);
  });
});
