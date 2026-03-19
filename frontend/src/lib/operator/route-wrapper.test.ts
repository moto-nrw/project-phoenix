import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockFetch, mockGetServerApiUrl } = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { operatorApiGet, operatorApiPost } from "./route-wrapper";

describe("operatorServerFetch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns unwrapped data from envelope response", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { id: 1, name: "Test" },
        message: "OK",
      }),
    });

    const result = await operatorApiGet("/api/test", "my-token");

    expect(result).toEqual({ id: 1, name: "Test" });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/test",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer my-token",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("returns undefined for 204 response", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const result = await operatorApiGet("/api/test", "my-token");
    expect(result).toBeUndefined();
  });

  it("returns raw JSON for non-envelope response", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ custom: "response" }),
    });

    const result = await operatorApiGet("/api/test", "my-token");
    expect(result).toEqual({ custom: "response" });
  });

  it("throws on 401 error", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      text: async () => "Unauthorized",
    });

    await expect(operatorApiGet("/api/test", "old-token")).rejects.toThrow(
      "API error (401)",
    );
  });

  it("throws on non-401 error", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => "Internal Server Error",
    });

    await expect(operatorApiGet("/api/test", "my-token")).rejects.toThrow(
      "API error (500)",
    );

    // Only one fetch call (no retry)
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("sends POST body correctly", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { created: true },
      }),
    });

    const result = await operatorApiPost("/api/test", "my-token", {
      name: "test",
    });

    expect(result).toEqual({ created: true });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/test",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ name: "test" }),
      }),
    );
  });
});
