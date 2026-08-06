import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockApiPost } = vi.hoisted(() => ({
  mockApiPost: vi.fn(),
}));

vi.mock("~/lib/api-helpers.server", () => ({
  apiPost: mockApiPost,
}));
vi.mock("~/lib/route-wrapper.server", () => ({
  createPostHandler: (handler: Function) => handler,
}));

const { POST } = await import("./route");

describe("/api/notifications/test proxy", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("posts to the backend route with the current tenant token", async () => {
    await (POST as Function)({}, {}, "test-token");

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/notifications/test",
      "test-token",
    );
  });
});
