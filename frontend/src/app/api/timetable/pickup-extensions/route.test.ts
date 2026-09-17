import { describe, expect, it, vi } from "vitest";

const mockProxyGet = vi.hoisted(() => vi.fn(() => vi.fn()));

vi.mock("~/lib/route-proxy.server", () => ({
  proxyGet: mockProxyGet,
}));

await import("./route");

describe("/api/timetable/pickup-extensions proxy", () => {
  it("uses the backend collection route with its required trailing slash", () => {
    expect(mockProxyGet).toHaveBeenCalledWith(
      "/api/timetable/pickup-extensions/",
    );
  });
});
