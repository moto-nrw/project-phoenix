import { describe, it, expect, vi, beforeEach } from "vitest";

const mockApiGet = vi.fn();

vi.mock("~/lib/api-helpers", () => ({
  apiGet: (...args: unknown[]) => mockApiGet(...args),
}));

// Mock route-wrapper to just call handler directly
vi.mock("~/lib/route-wrapper", () => ({
  createGetHandler: (handler: Function) => handler,
}));

const { GET } = await import("./route");

describe("GET /api/settings/schema", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls apiGet with correct endpoint and returns data", async () => {
    const schemaData = { tabs: [{ key: "test" }] };
    mockApiGet.mockResolvedValue({ status: "success", data: schemaData });

    const result = await (GET as Function)({}, "test-token");

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/settings/schema",
      "test-token",
    );
    expect(result).toEqual(schemaData);
  });

  it("propagates errors from apiGet", async () => {
    mockApiGet.mockRejectedValue(new Error("API error"));

    await expect((GET as Function)({}, "test-token")).rejects.toThrow(
      "API error",
    );
  });
});
