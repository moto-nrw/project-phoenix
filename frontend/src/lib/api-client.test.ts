/**
 * Tests for api-client.ts
 *
 * Tests all HTTP method wrappers:
 * - apiGet
 * - apiPost
 * - apiPut
 * - apiDelete
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { apiGet, apiPost, apiPut, apiDelete } from "./api-client";
import api from "./api";
import axios from "axios";

// Mock the api module
vi.mock("./api", () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

// Mock axios.isAxiosError
vi.mock("axios", () => ({
  default: {
    isAxiosError: vi.fn(),
  },
  isAxiosError: vi.fn(),
}));

describe("api-client", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("apiGet", () => {
    it("makes successful GET request", async () => {
      const mockResponse = { data: { id: 1, name: "Test" }, status: 200 };
      vi.mocked(api.get).mockResolvedValue(mockResponse);

      const result = await apiGet("/test");

      expect(api.get).toHaveBeenCalledWith("/test", { headers: {} });
      expect(result).toEqual(mockResponse);
    });

    it("includes Cookie header when provided", async () => {
      const mockResponse = { data: {}, status: 200 };
      vi.mocked(api.get).mockResolvedValue(mockResponse);

      await apiGet("/test", "session=abc123");

      expect(api.get).toHaveBeenCalledWith("/test", {
        headers: { Cookie: "session=abc123" },
      });
    });

    it("merges custom config with headers", async () => {
      const mockResponse = { data: {}, status: 200 };
      vi.mocked(api.get).mockResolvedValue(mockResponse);

      await apiGet("/test", "session=abc", {
        headers: { "X-Custom": "value" },
        timeout: 5000,
      });

      expect(api.get).toHaveBeenCalledWith("/test", {
        headers: { "X-Custom": "value", Cookie: "session=abc" },
        timeout: 5000,
      });
    });

    it("throws specific error for 401 response", async () => {
      const error = { response: { status: 401 } };
      vi.mocked(api.get).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(true);

      await expect(apiGet("/test")).rejects.toThrow(
        "API error (401): Unauthorized",
      );
    });

    it("re-throws non-401 axios errors", async () => {
      const error = { response: { status: 500 }, message: "Server error" };
      vi.mocked(api.get).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(true);

      await expect(apiGet("/test")).rejects.toEqual(error);
    });

    it("re-throws non-axios errors", async () => {
      const error = new Error("Network error");
      vi.mocked(api.get).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(false);

      await expect(apiGet("/test")).rejects.toThrow("Network error");
    });

    it("handles undefined cookie header", async () => {
      const mockResponse = { data: {}, status: 200 };
      vi.mocked(api.get).mockResolvedValue(mockResponse);

      await apiGet("/test", undefined);

      expect(api.get).toHaveBeenCalledWith("/test", { headers: {} });
    });
  });

  describe("apiPost", () => {
    it("makes successful POST request with data", async () => {
      const mockResponse = { data: { created: true }, status: 201 };
      vi.mocked(api.post).mockResolvedValue(mockResponse);

      const result = await apiPost("/test", { name: "Test" });

      expect(api.post).toHaveBeenCalledWith(
        "/test",
        { name: "Test" },
        { headers: {} },
      );
      expect(result).toEqual(mockResponse);
    });

    it("includes Cookie header when provided", async () => {
      const mockResponse = { data: {}, status: 201 };
      vi.mocked(api.post).mockResolvedValue(mockResponse);

      await apiPost("/test", { data: 1 }, "session=xyz789");

      expect(api.post).toHaveBeenCalledWith(
        "/test",
        { data: 1 },
        {
          headers: { Cookie: "session=xyz789" },
        },
      );
    });

    it("merges custom config with headers", async () => {
      const mockResponse = { data: {}, status: 201 };
      vi.mocked(api.post).mockResolvedValue(mockResponse);

      await apiPost("/test", { body: true }, "session=abc", {
        headers: { "Content-Type": "application/json" },
        timeout: 10000,
      });

      expect(api.post).toHaveBeenCalledWith(
        "/test",
        { body: true },
        {
          headers: {
            "Content-Type": "application/json",
            Cookie: "session=abc",
          },
          timeout: 10000,
        },
      );
    });

    it("throws specific error for 401 response", async () => {
      const error = { response: { status: 401 } };
      vi.mocked(api.post).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(true);

      await expect(apiPost("/test", {})).rejects.toThrow(
        "API error (401): Unauthorized",
      );
    });

    it("re-throws non-401 axios errors", async () => {
      const error = { response: { status: 403 }, message: "Forbidden" };
      vi.mocked(api.post).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(true);

      await expect(apiPost("/test", {})).rejects.toEqual(error);
    });

    it("re-throws non-axios errors", async () => {
      const error = new Error("Connection refused");
      vi.mocked(api.post).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(false);

      await expect(apiPost("/test", {})).rejects.toThrow("Connection refused");
    });

    it("handles undefined data parameter", async () => {
      const mockResponse = { data: {}, status: 201 };
      vi.mocked(api.post).mockResolvedValue(mockResponse);

      await apiPost("/test", undefined);

      expect(api.post).toHaveBeenCalledWith("/test", undefined, {
        headers: {},
      });
    });
  });

  describe("apiPut", () => {
    it("makes successful PUT request with data", async () => {
      const mockResponse = { data: { updated: true }, status: 200 };
      vi.mocked(api.put).mockResolvedValue(mockResponse);

      const result = await apiPut("/test/1", { name: "Updated" });

      expect(api.put).toHaveBeenCalledWith(
        "/test/1",
        { name: "Updated" },
        { headers: {} },
      );
      expect(result).toEqual(mockResponse);
    });

    it("includes Cookie header when provided", async () => {
      const mockResponse = { data: {}, status: 200 };
      vi.mocked(api.put).mockResolvedValue(mockResponse);

      await apiPut("/test/1", { data: "new" }, "session=put123");

      expect(api.put).toHaveBeenCalledWith(
        "/test/1",
        { data: "new" },
        {
          headers: { Cookie: "session=put123" },
        },
      );
    });

    it("merges custom config with headers", async () => {
      const mockResponse = { data: {}, status: 200 };
      vi.mocked(api.put).mockResolvedValue(mockResponse);

      await apiPut("/test/1", { update: true }, "auth=token", {
        headers: { Accept: "application/json" },
        validateStatus: () => true,
      });

      expect(api.put).toHaveBeenCalledWith(
        "/test/1",
        { update: true },
        {
          headers: { Accept: "application/json", Cookie: "auth=token" },
          validateStatus: expect.any(Function),
        },
      );
    });

    it("throws specific error for 401 response", async () => {
      const error = { response: { status: 401 } };
      vi.mocked(api.put).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(true);

      await expect(apiPut("/test/1", {})).rejects.toThrow(
        "API error (401): Unauthorized",
      );
    });

    it("re-throws non-401 axios errors", async () => {
      const error = { response: { status: 404 }, message: "Not found" };
      vi.mocked(api.put).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(true);

      await expect(apiPut("/test/1", {})).rejects.toEqual(error);
    });

    it("re-throws non-axios errors", async () => {
      const error = new TypeError("Invalid data");
      vi.mocked(api.put).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(false);

      await expect(apiPut("/test/1", {})).rejects.toThrow("Invalid data");
    });
  });

  describe("apiDelete", () => {
    it("makes successful DELETE request", async () => {
      const mockResponse = { data: null, status: 204 };
      vi.mocked(api.delete).mockResolvedValue(mockResponse);

      const result = await apiDelete("/test/1");

      expect(api.delete).toHaveBeenCalledWith("/test/1", { headers: {} });
      expect(result).toEqual(mockResponse);
    });

    it("includes Cookie header when provided", async () => {
      const mockResponse = { data: null, status: 204 };
      vi.mocked(api.delete).mockResolvedValue(mockResponse);

      await apiDelete("/test/1", "session=delete456");

      expect(api.delete).toHaveBeenCalledWith("/test/1", {
        headers: { Cookie: "session=delete456" },
      });
    });

    it("merges custom config with headers", async () => {
      const mockResponse = { data: null, status: 204 };
      vi.mocked(api.delete).mockResolvedValue(mockResponse);

      await apiDelete("/test/1", "auth=bearer", {
        headers: { "X-Reason": "cleanup" },
        params: { force: true },
      });

      expect(api.delete).toHaveBeenCalledWith("/test/1", {
        headers: { "X-Reason": "cleanup", Cookie: "auth=bearer" },
        params: { force: true },
      });
    });

    it("throws specific error for 401 response", async () => {
      const error = { response: { status: 401 } };
      vi.mocked(api.delete).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(true);

      await expect(apiDelete("/test/1")).rejects.toThrow(
        "API error (401): Unauthorized",
      );
    });

    it("re-throws non-401 axios errors", async () => {
      const error = { response: { status: 409 }, message: "Conflict" };
      vi.mocked(api.delete).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(true);

      await expect(apiDelete("/test/1")).rejects.toEqual(error);
    });

    it("re-throws non-axios errors", async () => {
      const error = new Error("Timeout exceeded");
      vi.mocked(api.delete).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(false);

      await expect(apiDelete("/test/1")).rejects.toThrow("Timeout exceeded");
    });

    it("handles missing cookie header gracefully", async () => {
      const mockResponse = { data: null, status: 204 };
      vi.mocked(api.delete).mockResolvedValue(mockResponse);

      await apiDelete("/test/1", undefined, { timeout: 3000 });

      expect(api.delete).toHaveBeenCalledWith("/test/1", {
        headers: {},
        timeout: 3000,
      });
    });
  });

  describe("edge cases", () => {
    it("handles axios error without response property", async () => {
      const error = { code: "ECONNREFUSED" };
      vi.mocked(api.get).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(true);

      // Should re-throw since error.response is undefined
      await expect(apiGet("/test")).rejects.toEqual(error);
    });

    it("handles axios error with response but no status", async () => {
      const error = { response: {} };
      vi.mocked(api.post).mockRejectedValue(error);
      vi.mocked(axios.isAxiosError).mockReturnValue(true);

      // Should re-throw since status is not 401
      await expect(apiPost("/test", {})).rejects.toEqual(error);
    });

    it("preserves existing headers when adding Cookie", async () => {
      const mockResponse = { data: {}, status: 200 };
      vi.mocked(api.get).mockResolvedValue(mockResponse);

      await apiGet("/test", "auth=test", {
        headers: {
          Authorization: "Bearer existing",
          "X-Request-Id": "12345",
        },
      });

      expect(api.get).toHaveBeenCalledWith("/test", {
        headers: {
          Authorization: "Bearer existing",
          "X-Request-Id": "12345",
          Cookie: "auth=test",
        },
      });
    });
  });
});
