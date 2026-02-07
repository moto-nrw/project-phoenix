import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  setOperatorTokens,
  clearOperatorTokens,
  getOperatorToken,
  getOperatorRefreshToken,
} from "./cookies";

// Mock next/headers
const mockSet = vi.fn();
const mockDelete = vi.fn();
const mockGet = vi.fn();

vi.mock("next/headers", () => ({
  cookies: vi.fn(async () => ({
    set: mockSet,
    delete: mockDelete,
    get: mockGet,
  })),
}));

describe("operator/cookies", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("setOperatorTokens", () => {
    it("sets both access and refresh tokens with correct options", async () => {
      await setOperatorTokens("access-token-123", "refresh-token-456");

      expect(mockSet).toHaveBeenCalledTimes(2);

      // Access token call
      expect(mockSet).toHaveBeenNthCalledWith(
        1,
        "phoenix-operator-token",
        "access-token-123",
        {
          httpOnly: true,
          secure: false, // test environment
          sameSite: "lax",
          path: "/",
          maxAge: 15 * 60, // 15 minutes
        },
      );

      // Refresh token call
      expect(mockSet).toHaveBeenNthCalledWith(
        2,
        "phoenix-operator-refresh",
        "refresh-token-456",
        {
          httpOnly: true,
          secure: false,
          sameSite: "lax",
          path: "/",
          maxAge: 60 * 60, // 1 hour
        },
      );
    });

    it("uses secure=false in test environment (COOKIE_OPTIONS evaluated at module load)", async () => {
      // COOKIE_OPTIONS.secure is set at module load time based on NODE_ENV.
      // In test environment, NODE_ENV !== "production", so secure is false.
      await setOperatorTokens("access", "refresh");

      expect(mockSet).toHaveBeenCalledWith(
        "phoenix-operator-token",
        "access",
        expect.objectContaining({ secure: false }),
      );
    });

    it("handles empty token strings", async () => {
      await setOperatorTokens("", "");

      expect(mockSet).toHaveBeenCalledTimes(2);
      expect(mockSet).toHaveBeenNthCalledWith(
        1,
        "phoenix-operator-token",
        "",
        expect.any(Object),
      );
    });
  });

  describe("clearOperatorTokens", () => {
    it("deletes both token cookies", async () => {
      await clearOperatorTokens();

      expect(mockDelete).toHaveBeenCalledTimes(2);
      expect(mockDelete).toHaveBeenNthCalledWith(1, "phoenix-operator-token");
      expect(mockDelete).toHaveBeenNthCalledWith(2, "phoenix-operator-refresh");
    });
  });

  describe("getOperatorToken", () => {
    it("returns token value when cookie exists", async () => {
      mockGet.mockReturnValueOnce({ value: "token-123" });

      const result = await getOperatorToken();

      expect(mockGet).toHaveBeenCalledWith("phoenix-operator-token");
      expect(result).toBe("token-123");
    });

    it("returns undefined when cookie does not exist", async () => {
      mockGet.mockReturnValueOnce(undefined);

      const result = await getOperatorToken();

      expect(result).toBeUndefined();
    });

    it("returns undefined when cookie exists but has no value", async () => {
      mockGet.mockReturnValueOnce({});

      const result = await getOperatorToken();

      expect(result).toBeUndefined();
    });
  });

  describe("getOperatorRefreshToken", () => {
    it("returns refresh token value when cookie exists", async () => {
      mockGet.mockReturnValueOnce({ value: "refresh-456" });

      const result = await getOperatorRefreshToken();

      expect(mockGet).toHaveBeenCalledWith("phoenix-operator-refresh");
      expect(result).toBe("refresh-456");
    });

    it("returns undefined when cookie does not exist", async () => {
      mockGet.mockReturnValueOnce(undefined);

      const result = await getOperatorRefreshToken();

      expect(result).toBeUndefined();
    });

    it("returns undefined when cookie exists but has no value", async () => {
      mockGet.mockReturnValueOnce({});

      const result = await getOperatorRefreshToken();

      expect(result).toBeUndefined();
    });
  });
});
