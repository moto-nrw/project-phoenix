import { describe, it, expect, vi, beforeEach } from "vitest";
import axios from "axios";
import { performImmediateCheckin } from "./checkin-api";

// Mock axios
vi.mock("axios", () => {
  const mockAxiosInstance = {
    post: vi.fn(),
    get: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  };
  return {
    default: {
      create: vi.fn(() => mockAxiosInstance),
    },
  };
});

describe("checkin-api", () => {
  let mockAxiosInstance: {
    post: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockAxiosInstance = axios.create() as unknown as {
      post: ReturnType<typeof vi.fn>;
    };
  });

  describe("performImmediateCheckin", () => {
    it("calls the correct endpoint with student ID and active group ID", async () => {
      mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

      await performImmediateCheckin(123, 456);

      expect(mockAxiosInstance.post).toHaveBeenCalledWith(
        "/api/active/visits/student/123/checkin",
        { active_group_id: 456 },
        { headers: {} },
      );
    });

    it("includes authorization header when token is provided", async () => {
      mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

      await performImmediateCheckin(123, 456, "test-token");

      expect(mockAxiosInstance.post).toHaveBeenCalledWith(
        "/api/active/visits/student/123/checkin",
        { active_group_id: 456 },
        { headers: { Authorization: "Bearer test-token" } },
      );
    });

    it("does not include authorization header when token is undefined", async () => {
      mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

      await performImmediateCheckin(789, 101);

      expect(mockAxiosInstance.post).toHaveBeenCalledWith(
        "/api/active/visits/student/789/checkin",
        { active_group_id: 101 },
        { headers: {} },
      );
    });

    it("propagates errors from axios", async () => {
      const error = new Error("Network error");
      mockAxiosInstance.post.mockRejectedValueOnce(error);

      await expect(performImmediateCheckin(123, 456)).rejects.toThrow(
        "Network error",
      );
    });

    it("handles different student IDs correctly", async () => {
      mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

      await performImmediateCheckin(999, 111);

      expect(mockAxiosInstance.post).toHaveBeenCalledWith(
        "/api/active/visits/student/999/checkin",
        { active_group_id: 111 },
        { headers: {} },
      );
    });

    it("handles different active group IDs correctly", async () => {
      mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

      await performImmediateCheckin(1, 9999);

      expect(mockAxiosInstance.post).toHaveBeenCalledWith(
        "/api/active/visits/student/1/checkin",
        { active_group_id: 9999 },
        { headers: {} },
      );
    });
  });
});
