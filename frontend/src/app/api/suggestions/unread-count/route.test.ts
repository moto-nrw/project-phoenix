import { describe, it, expect, vi, beforeEach } from "vitest";
import type { NextRequest } from "next/server";

// Use vi.hoisted for mock values referenced in vi.mock
const { mockApiGet, mockCreateGetHandler } = vi.hoisted(() => ({
  mockApiGet: vi.fn(),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-return -- Mock factory requires any for generic handler
  mockCreateGetHandler: vi.fn((handler: any) => handler),
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
}));

vi.mock("~/lib/route-wrapper", () => ({
  createGetHandler: mockCreateGetHandler,
}));

// GET is the handler itself since mockCreateGetHandler returns the handler as-is
import { GET } from "./route";

type InnerHandler = (
  request: NextRequest,
  token: string,
) => Promise<{ unread_count: number }>;

describe("GET /api/suggestions/unread-count", () => {
  beforeEach(() => {
    mockApiGet.mockReset();
  });

  it("returns unread count from backend", async () => {
    const mockRequest = {} as NextRequest;
    const mockToken = "test-token-123";

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: { unread_count: 5 },
    });

    // GET is the inner handler function (createGetHandler mock returns it as-is)
    const result = await (GET as unknown as InnerHandler)(
      mockRequest,
      mockToken,
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/suggestions/unread-count",
      mockToken,
    );
    expect(result).toEqual({ unread_count: 5 });
  });

  it("extracts data from backend response", async () => {
    const mockRequest = {} as NextRequest;
    const mockToken = "token-456";

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: { unread_count: 0 },
    });

    const result = await (GET as unknown as InnerHandler)(
      mockRequest,
      mockToken,
    );

    expect(result).toEqual({ unread_count: 0 });
  });

  it("handles large unread counts", async () => {
    const mockRequest = {} as NextRequest;
    const mockToken = "token-789";

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: { unread_count: 999 },
    });

    const result = await (GET as unknown as InnerHandler)(
      mockRequest,
      mockToken,
    );

    expect(result).toEqual({ unread_count: 999 });
  });

  it("wraps handler with createGetHandler", () => {
    expect(mockCreateGetHandler).toHaveBeenCalledWith(expect.any(Function));
  });

  it("passes token to apiGet", async () => {
    const mockRequest = {} as NextRequest;
    const customToken = "custom-jwt-token";

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: { unread_count: 3 },
    });

    await (GET as unknown as InnerHandler)(mockRequest, customToken);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/suggestions/unread-count",
      customToken,
    );
  });

  it("propagates errors from apiGet", async () => {
    const mockRequest = {} as NextRequest;
    const mockToken = "token";

    mockApiGet.mockRejectedValueOnce(new Error("Backend error"));

    await expect(
      (GET as unknown as InnerHandler)(mockRequest, mockToken),
    ).rejects.toThrow("Backend error");
  });
});
