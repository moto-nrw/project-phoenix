import { describe, it, expect, vi, beforeEach } from "vitest";

const mockApiGet = vi.fn();

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: (...args: unknown[]) => mockApiGet(...args),
}));

vi.mock("~/lib/route-wrapper.server", () => ({
  createGetHandler: (handler: Function) => handler,
}));

const { GET } = await import("./route");

describe("GET /api/settings/values/[key]/reveal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls apiGet with correct endpoint and token", async () => {
    mockApiGet.mockResolvedValue({ data: { value: "1234" } });

    const result = await (GET as Function)({}, "test-token", {
      key: "security.ogs_device_pin",
    });

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/settings/values/security.ogs_device_pin/reveal",
      "test-token",
    );
    expect(result).toEqual({ data: { value: "1234" } });
  });

  it("rejects invalid key format", async () => {
    await expect(
      (GET as Function)({}, "test-token", {
        key: "../../../etc/passwd",
      }),
    ).rejects.toThrow("Invalid settings key format");
  });

  it("accepts valid key with dots and underscores", async () => {
    mockApiGet.mockResolvedValue({ data: { value: "val" } });

    await (GET as Function)({}, "test-token", {
      key: "security.ogs_device_pin",
    });

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/settings/values/security.ogs_device_pin/reveal",
      "test-token",
    );
  });
});
