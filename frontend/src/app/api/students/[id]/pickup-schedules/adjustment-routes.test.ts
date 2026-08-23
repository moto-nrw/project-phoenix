import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/route-proxy.server", () => ({
  proxyPost: vi.fn((endpoint: unknown) => endpoint),
}));

import { POST as apply } from "./apply/route";
import { POST as preview } from "./preview/route";

type EndpointBuilder = (params: Record<string, unknown>) => string;

describe("pickup adjustment proxy routes", () => {
  it.each([
    ["preview", preview],
    ["apply", apply],
  ])("forwards %s to the encoded backend endpoint", (action, route) => {
    const buildEndpoint = route as unknown as EndpointBuilder;

    expect(buildEndpoint({ id: "42/unsafe" })).toBe(
      `/api/students/42%2Funsafe/pickup-schedules/${action}`,
    );
  });
});
