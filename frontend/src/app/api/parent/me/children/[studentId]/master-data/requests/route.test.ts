import { describe, expect, it, vi } from "vitest";

// Capture the endpoint builders the route hands to the parent proxy factories,
// so the test can assert path-segment encoding without real HTTP plumbing.
vi.mock("~/lib/parent/route-wrapper.server", () => ({
  proxyGet: vi.fn((endpoint: unknown) => endpoint),
  proxyPost: vi.fn((endpoint: unknown) => endpoint),
}));

import { GET, POST } from "./route";

type EndpointBuilder = (params: Record<string, unknown>) => string;

describe("parent child master-data requests route", () => {
  it("forwards GET to the encoded backend request list endpoint", () => {
    const buildEndpoint = GET as unknown as EndpointBuilder;

    expect(buildEndpoint({ studentId: "42/unsafe" })).toBe(
      "/parent/me/children/42%2Funsafe/master-data/requests",
    );
  });

  it("forwards POST to the encoded backend request endpoint", () => {
    const buildEndpoint = POST as unknown as EndpointBuilder;

    expect(buildEndpoint({ studentId: "42/unsafe" })).toBe(
      "/parent/me/children/42%2Funsafe/master-data/requests",
    );
  });
});
