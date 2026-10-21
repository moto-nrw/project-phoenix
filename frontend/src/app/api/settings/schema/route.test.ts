import { describe, it, expect, vi, beforeEach } from "vitest";

const mockApiGet = vi.fn();

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: (...args: unknown[]) => mockApiGet(...args),
}));

// Mock route-wrapper to just call handler directly
vi.mock("~/lib/route-wrapper.server", () => ({
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
});
