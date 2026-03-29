import { describe, it, expect, vi, beforeEach } from "vitest";

const mockApiPut = vi.fn();
const mockApiDelete = vi.fn();

vi.mock("~/lib/api-helpers", () => ({
  apiPut: (...args: unknown[]) => mockApiPut(...args),
  apiDelete: (...args: unknown[]) => mockApiDelete(...args),
}));

vi.mock("~/lib/route-wrapper", () => ({
  createPutHandler: (handler: Function) => handler,
  createDeleteHandler: (handler: Function) => handler,
}));

const { PUT, DELETE } = await import("./route");

describe("PUT /api/settings/values/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls apiPut with correct endpoint, token, and body", async () => {
    const body = { value: "16:30" };
    mockApiPut.mockResolvedValue({ status: "success" });

    const mockRequest = { json: () => Promise.resolve(body) };
    const result = await (PUT as Function)(mockRequest, body, "test-token", {
      key: "operations.session_end_time",
    });

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/settings/values/operations.session_end_time",
      "test-token",
      body,
    );
    expect(result).toEqual({ status: "success" });
  });
});

describe("DELETE /api/settings/values/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls apiDelete with correct endpoint", async () => {
    mockApiDelete.mockResolvedValue(undefined);

    await (DELETE as Function)({}, "test-token", {
      key: "operations.session_end_time",
    });

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/settings/values/operations.session_end_time",
      "test-token",
    );
  });
});
