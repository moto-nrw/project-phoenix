import { describe, expect, it, vi } from "vitest";

const { mockParentApiDelete, mockProxyGet } = vi.hoisted(() => ({
  mockParentApiDelete: vi.fn(),
  mockProxyGet: vi.fn(() => vi.fn()),
}));

vi.mock("~/lib/parent/route-wrapper.server", () => ({
  createParentDeleteHandler: (handler: Function) => handler,
  parentApiDelete: mockParentApiDelete,
  proxyGet: mockProxyGet,
}));

const { DELETE } = await import("./route");

describe("/api/parent/me/notification-preferences proxy", () => {
  it("uses the backend collection route with its required trailing slash", async () => {
    expect(mockProxyGet).toHaveBeenCalledWith(
      "/parent/me/notification-preferences/",
    );

    await (DELETE as Function)({}, "test-token");

    expect(mockParentApiDelete).toHaveBeenCalledWith(
      "/parent/me/notification-preferences/",
      "test-token",
    );
  });
});
