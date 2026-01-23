import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextResponse } from "next/server";
import type { ApiErrorResponse } from "./api-helpers";

// Mock the auth module before importing the function under test
vi.mock("~/server/auth");

// Import the function under test AFTER setting up the mock
import { checkAuth } from "./api-helpers.server";
import { auth } from "~/server/auth";

// Type the mocked function for proper TypeScript support
const mockedAuth = vi.mocked(auth);

describe("api-helpers.server", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("checkAuth", () => {
    describe("when user is authenticated", () => {
      it("should return null when session has valid user", async () => {
        // Arrange
        mockedAuth.mockResolvedValueOnce({
          user: {
            id: "test-user-id",
            email: "test@example.com",
            name: "Test User",
          },
        });

        // Act
        const result = await checkAuth();

        // Assert
        expect(result).toBeNull();
        expect(mockedAuth).toHaveBeenCalledTimes(1);
      });

      it("should return null when session has user with minimal data", async () => {
        // Arrange - user with only required id field
        mockedAuth.mockResolvedValueOnce({
          user: {
            id: "minimal-user",
            email: "",
            name: null,
          },
        });

        // Act
        const result = await checkAuth();

        // Assert
        expect(result).toBeNull();
      });

      it("should return null when session has user with empty string name", async () => {
        // Arrange
        mockedAuth.mockResolvedValueOnce({
          user: {
            id: "user-123",
            email: "user@example.com",
            name: "",
          },
        });

        // Act
        const result = await checkAuth();

        // Assert
        expect(result).toBeNull();
      });
    });

    describe("when user is not authenticated", () => {
      it("should return 401 when session is null", async () => {
        // Arrange
        mockedAuth.mockResolvedValueOnce(null);

        // Act
        const result = await checkAuth();

        // Assert
        expect(result).toBeInstanceOf(NextResponse);
        expect(result).not.toBeNull();

        // Parse the response to verify content
        const body = (await result!.json()) as ApiErrorResponse;
        expect(body.error).toBe("Unauthorized");
        expect(result!.status).toBe(401);
      });

      it("should return 401 when session has no user property", async () => {
        // Arrange - session exists but user is undefined
        mockedAuth.mockResolvedValueOnce({
          user: undefined,
        } as unknown as Awaited<ReturnType<typeof auth>>);

        // Act
        const result = await checkAuth();

        // Assert
        expect(result).toBeInstanceOf(NextResponse);
        expect(result).not.toBeNull();

        const body = (await result!.json()) as ApiErrorResponse;
        expect(body.error).toBe("Unauthorized");
        expect(result!.status).toBe(401);
      });

      it("should return 401 when session.user is null", async () => {
        // Arrange
        mockedAuth.mockResolvedValueOnce({
          user: null,
        } as unknown as Awaited<ReturnType<typeof auth>>);

        // Act
        const result = await checkAuth();

        // Assert
        expect(result).toBeInstanceOf(NextResponse);
        expect(result).not.toBeNull();

        const body = (await result!.json()) as ApiErrorResponse;
        expect(body.error).toBe("Unauthorized");
        expect(result!.status).toBe(401);
      });
    });

    describe("error handling", () => {
      it("should propagate errors from auth() function", async () => {
        // Arrange
        const authError = new Error("Auth service unavailable");
        mockedAuth.mockRejectedValueOnce(authError);

        // Act & Assert
        await expect(checkAuth()).rejects.toThrow("Auth service unavailable");
      });

      it("should propagate network errors from auth() function", async () => {
        // Arrange
        const networkError = new Error("Network timeout");
        mockedAuth.mockRejectedValueOnce(networkError);

        // Act & Assert
        await expect(checkAuth()).rejects.toThrow("Network timeout");
      });
    });

    describe("response format", () => {
      it("should return JSON response with correct content-type headers", async () => {
        // Arrange
        mockedAuth.mockResolvedValueOnce(null);

        // Act
        const result = await checkAuth();

        // Assert
        expect(result).not.toBeNull();
        expect(result!.headers.get("content-type")).toContain(
          "application/json",
        );
      });

      it("should return response that can be parsed as JSON", async () => {
        // Arrange
        mockedAuth.mockResolvedValueOnce(null);

        // Act
        const result = await checkAuth();

        // Assert - verify the response body is valid JSON
        expect(result).not.toBeNull();
        const body = await result!.json();
        expect(body).toEqual({ error: "Unauthorized" });
      });
    });

    describe("concurrent calls", () => {
      it("should handle multiple concurrent checkAuth calls", async () => {
        // Arrange - alternating authenticated and unauthenticated
        mockedAuth
          .mockResolvedValueOnce({
            user: { id: "user-1", email: "user1@test.com", name: "User 1" },
          })
          .mockResolvedValueOnce(null)
          .mockResolvedValueOnce({
            user: { id: "user-2", email: "user2@test.com", name: "User 2" },
          });

        // Act
        const results = await Promise.all([
          checkAuth(),
          checkAuth(),
          checkAuth(),
        ]);

        // Assert
        expect(results[0]).toBeNull(); // authenticated
        expect(results[1]).toBeInstanceOf(NextResponse); // not authenticated
        expect(results[2]).toBeNull(); // authenticated
        expect(mockedAuth).toHaveBeenCalledTimes(3);
      });
    });
  });
});
