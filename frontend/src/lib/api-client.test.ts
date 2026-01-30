import { describe, it, expect, vi, beforeEach } from "vitest";
import type { AxiosError } from "axios";
import { apiGet, apiPost, apiPut, apiDelete } from "./api-client";

const mockIsAxiosError = vi.fn();
const mockGet = vi.fn();
const mockPost = vi.fn();
const mockPut = vi.fn();
const mockDelete = vi.fn();

vi.mock("axios", () => ({
  default: {
    isAxiosError: (...args: unknown[]): boolean =>
      mockIsAxiosError(...args) as boolean,
  },
  isAxiosError: (...args: unknown[]): boolean =>
    mockIsAxiosError(...args) as boolean,
}));

vi.mock("./api", () => ({
  default: {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    get: (...args: unknown[]) => mockGet(...args),
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    post: (...args: unknown[]) => mockPost(...args),
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    put: (...args: unknown[]) => mockPut(...args),
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    delete: (...args: unknown[]) => mockDelete(...args),
  },
}));

describe("api-client", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsAxiosError.mockReturnValue(false);
  });

  describe("apiGet", () => {
    it("makes successful GET request with token", async () => {
      const mockResponse = { data: { id: "1", name: "Test" } };
      mockGet.mockResolvedValue(mockResponse);

      const result = await apiGet("/test", "test-token");

      expect(mockGet).toHaveBeenCalledWith("/test", {
        headers: { Authorization: "Bearer test-token" },
      });
      expect(result).toBe(mockResponse);
    });

    it("makes successful GET request without token", async () => {
      const mockResponse = { data: { id: "1", name: "Test" } };
      mockGet.mockResolvedValue(mockResponse);

      const result = await apiGet("/test");

      expect(mockGet).toHaveBeenCalledWith("/test", {
        headers: {},
      });
      expect(result).toBe(mockResponse);
    });

    it("throws specific error for 401 response", async () => {
      const axiosError = {
        response: { status: 401 },
      } as Partial<AxiosError>;
      mockGet.mockRejectedValue(axiosError);
      mockIsAxiosError.mockReturnValue(true);

      await expect(apiGet("/test", "test-token")).rejects.toThrow(
        "API error (401): Unauthorized",
      );
    });

    it("re-throws non-401 errors", async () => {
      const error = new Error("Network error");
      mockGet.mockRejectedValue(error);

      await expect(apiGet("/test", "test-token")).rejects.toThrow(
        "Network error",
      );
    });

    it("merges custom headers with authorization", async () => {
      const mockResponse = { data: {} };
      mockGet.mockResolvedValue(mockResponse);

      await apiGet("/test", "test-token", {
        headers: { "X-Custom": "value" },
      });

      expect(mockGet).toHaveBeenCalledWith("/test", {
        headers: {
          "X-Custom": "value",
          Authorization: "Bearer test-token",
        },
      });
    });
  });

  describe("apiPost", () => {
    it("makes successful POST request with token", async () => {
      const mockResponse = { data: { id: "1" } };
      const postData = { name: "Test" };
      mockPost.mockResolvedValue(mockResponse);

      const result = await apiPost("/test", postData, "test-token");

      expect(mockPost).toHaveBeenCalledWith("/test", postData, {
        headers: { Authorization: "Bearer test-token" },
      });
      expect(result).toBe(mockResponse);
    });

    it("makes successful POST request without token", async () => {
      const mockResponse = { data: { id: "1" } };
      const postData = { name: "Test" };
      mockPost.mockResolvedValue(mockResponse);

      const result = await apiPost("/test", postData);

      expect(mockPost).toHaveBeenCalledWith("/test", postData, {
        headers: {},
      });
      expect(result).toBe(mockResponse);
    });

    it("throws specific error for 401 response", async () => {
      const axiosError = {
        response: { status: 401 },
      } as Partial<AxiosError>;
      mockPost.mockRejectedValue(axiosError);
      mockIsAxiosError.mockReturnValue(true);

      await expect(
        apiPost("/test", { name: "Test" }, "test-token"),
      ).rejects.toThrow("API error (401): Unauthorized");
    });

    it("re-throws non-401 errors", async () => {
      const error = new Error("Validation error");
      mockPost.mockRejectedValue(error);

      await expect(
        apiPost("/test", { name: "Test" }, "test-token"),
      ).rejects.toThrow("Validation error");
    });
  });

  describe("apiPut", () => {
    it("makes successful PUT request with token", async () => {
      const mockResponse = { data: { id: "1" } };
      const putData = { name: "Updated" };
      mockPut.mockResolvedValue(mockResponse);

      const result = await apiPut("/test/1", putData, "test-token");

      expect(mockPut).toHaveBeenCalledWith("/test/1", putData, {
        headers: { Authorization: "Bearer test-token" },
      });
      expect(result).toBe(mockResponse);
    });

    it("throws specific error for 401 response", async () => {
      const axiosError = {
        response: { status: 401 },
      } as Partial<AxiosError>;
      mockPut.mockRejectedValue(axiosError);
      mockIsAxiosError.mockReturnValue(true);

      await expect(
        apiPut("/test/1", { name: "Updated" }, "test-token"),
      ).rejects.toThrow("API error (401): Unauthorized");
    });

    it("re-throws non-401 errors", async () => {
      const error = new Error("Update failed");
      mockPut.mockRejectedValue(error);

      await expect(
        apiPut("/test/1", { name: "Updated" }, "test-token"),
      ).rejects.toThrow("Update failed");
    });
  });

  describe("apiDelete", () => {
    it("makes successful DELETE request with token", async () => {
      const mockResponse = { data: { success: true } };
      mockDelete.mockResolvedValue(mockResponse);

      const result = await apiDelete("/test/1", "test-token");

      expect(mockDelete).toHaveBeenCalledWith("/test/1", {
        headers: { Authorization: "Bearer test-token" },
      });
      expect(result).toBe(mockResponse);
    });

    it("throws specific error for 401 response", async () => {
      const axiosError = {
        response: { status: 401 },
      } as Partial<AxiosError>;
      mockDelete.mockRejectedValue(axiosError);
      mockIsAxiosError.mockReturnValue(true);

      await expect(apiDelete("/test/1", "test-token")).rejects.toThrow(
        "API error (401): Unauthorized",
      );
    });

    it("re-throws non-401 errors", async () => {
      const error = new Error("Delete failed");
      mockDelete.mockRejectedValue(error);

      await expect(apiDelete("/test/1", "test-token")).rejects.toThrow(
        "Delete failed",
      );
    });
  });
});
