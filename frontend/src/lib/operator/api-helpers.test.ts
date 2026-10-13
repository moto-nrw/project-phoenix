import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "../api-error";
import { operatorFetch } from "./api-helpers";

global.fetch = vi.fn();

describe("operatorFetch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("performs successful GET request", async () => {
    const mockData = { id: "1", name: "Test" };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockData,
    });

    const result = await operatorFetch<typeof mockData>("/api/test");
    expect(result).toEqual(mockData);
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/test",
      expect.objectContaining({
        method: "GET",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      }),
    );
  });

  it("performs successful POST request with body", async () => {
    const requestBody = { name: "New Item" };
    const responseData = { id: "2", name: "New Item" };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      status: 201,
      json: async () => responseData,
    });

    const result = await operatorFetch<typeof responseData>("/api/test", {
      method: "POST",
      body: requestBody,
    });
    expect(result).toEqual(responseData);
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/test",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(requestBody),
      }),
    );
  });

  it("handles 204 No Content response", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      status: 204,
    });

    const result = await operatorFetch<void>("/api/test", { method: "DELETE" });
    expect(result).toEqual({});
  });

  it("unwraps proxy response envelope", async () => {
    const actualData = { id: "1", value: 42 };
    const envelope = { success: true, data: actualData, message: "Success" };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => envelope,
    });

    const result = await operatorFetch<typeof actualData>("/api/test");
    expect(result).toEqual(actualData);
  });

  it("returns plain JSON when not wrapped in envelope", async () => {
    const plainData = { id: "1", name: "Test" };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => plainData,
    });

    const result = await operatorFetch<typeof plainData>("/api/test");
    expect(result).toEqual(plainData);
  });

  it("throws an ApiError with the envelope on a non-ok response", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 400,
      statusText: "Bad Request",
      json: async () => ({
        detail: "Invalid input",
        code: "general.input",
        status: 400,
      }),
    });

    await expect(operatorFetch("/api/test")).rejects.toThrow(ApiError);
    await expect(operatorFetch("/api/test")).rejects.toThrow("Bad Request");

    try {
      await operatorFetch("/api/test");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError);
      expect((error as ApiError).status).toBe(400);
      expect((error as ApiError).code).toBe("general.input");
      expect((error as ApiError).detail).toBe("Invalid input");
    }
  });

  it("throws an ApiError with statusText when JSON parsing fails", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      json: async () => {
        throw new Error("Invalid JSON");
      },
    });

    await expect(operatorFetch("/api/test")).rejects.toThrow(
      "Internal Server Error",
    );
  });

  it("includes credentials in all requests", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({}),
    });

    await operatorFetch("/api/test");
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/test",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("handles PUT request", async () => {
    const updateData = { name: "Updated" };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => updateData,
    });

    const result = await operatorFetch<typeof updateData>("/api/test/1", {
      method: "PUT",
      body: updateData,
    });
    expect(result).toEqual(updateData);
  });
});
