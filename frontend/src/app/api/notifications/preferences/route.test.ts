import { describe, expect, it, vi } from "vitest";

const { mockApiDelete, mockProxyGet } = vi.hoisted(() => ({
  mockApiDelete: vi.fn(),
  mockProxyGet: vi.fn(() => vi.fn()),
}));

vi.mock("~/lib/api-helpers.server", () => ({
  apiDelete: mockApiDelete,
}));
vi.mock("~/lib/route-proxy.server", () => ({
  proxyGet: mockProxyGet,
}));
vi.mock("~/lib/route-wrapper.server", () => ({
  createDeleteHandler: (handler: Function) => handler,
}));

const { DELETE } = await import("./route");

describe("/api/notifications/preferences proxy", () => {
  it("uses the backend collection route with its required trailing slash", async () => {
    expect(mockProxyGet).toHaveBeenCalledWith(
      "/api/notifications/preferences/",
    );

    await (DELETE as Function)({}, "test-token");

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/notifications/preferences/",
      "test-token",
    );
  });
});
