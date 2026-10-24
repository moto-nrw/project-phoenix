import { describe, expect, it, vi } from "vitest";

const proxies = vi.hoisted(() => ({
  tenant: { proxyGet: vi.fn(() => vi.fn()), proxyPut: vi.fn(() => vi.fn()) },
  parent: { proxyGet: vi.fn(() => vi.fn()), proxyPut: vi.fn(() => vi.fn()) },
  school: { proxyGet: vi.fn(() => vi.fn()), proxyPut: vi.fn(() => vi.fn()) },
}));

vi.mock("~/lib/route-proxy.server", () => proxies.tenant);
vi.mock("~/lib/parent/route-wrapper.server", () => proxies.parent);
vi.mock("~/lib/school/route-wrapper.server", () => proxies.school);

await import("./route");
await import("../parent/analytics-consent/route");
await import("../school/analytics-consent/route");

describe("analytics consent proxies", () => {
  it.each(["tenant", "parent", "school"] as const)(
    "%s portal reads and writes the account's one backend route",
    (portal) => {
      // The trailing slash is part of the mounted backend route.
      expect(proxies[portal].proxyGet).toHaveBeenCalledExactlyOnceWith(
        "/analytics-consent/",
      );
      expect(proxies[portal].proxyPut).toHaveBeenCalledExactlyOnceWith(
        "/analytics-consent/",
      );
    },
  );
});
