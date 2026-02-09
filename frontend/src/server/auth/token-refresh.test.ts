import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";

// Mock auth - MUST be hoisted before imports
vi.mock("~/server/auth", () => ({
  auth: vi.fn(),
}));

// Mock env
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

// Import after mocks
import { refreshSessionTokensOnServer } from "./token-refresh";
import { auth } from "~/server/auth";

// Type the mock properly
const mockAuth = auth as unknown as ReturnType<
  typeof vi.fn<() => Promise<Session | null>>
>;

describe("refreshSessionTokensOnServer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should return null when no session", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
    expect(mockAuth).toHaveBeenCalledOnce();
  });

  it("should return null when no refresh token in session", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "123",
        email: "test@example.com",
      },
      expires: "2099-12-31",
    });

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
  });

  it("should return null when session has empty token", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "123",
        email: "test@example.com",
        token: "",
        refreshToken: "refresh-token",
      },
      expires: "2099-12-31",
    });

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
  });

  it("should return tokens from session after auth refresh", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "123",
        email: "test@example.com",
        token: "new-access-token",
        refreshToken: "new-refresh-token",
      },
      expires: "2099-12-31",
    });

    const result = await refreshSessionTokensOnServer();

    expect(result).toEqual({
      accessToken: "new-access-token",
      refreshToken: "new-refresh-token",
    });
    expect(mockAuth).toHaveBeenCalledOnce();
  });

  it("should handle auth errors gracefully", async () => {
    mockAuth.mockRejectedValueOnce(new Error("Auth failed"));

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
  });

  it("should only allow one refresh at a time", async () => {
    let resolveAuth: (value: Session | null) => void;
    const authPromise = new Promise<Session | null>((resolve) => {
      resolveAuth = resolve;
    });

    mockAuth.mockReturnValueOnce(authPromise);

    // Start first refresh
    const firstRefresh = refreshSessionTokensOnServer();

    // Start second refresh while first is in progress
    const secondRefresh = refreshSessionTokensOnServer();

    // Resolve auth
    resolveAuth!({
      user: {
        id: "123",
        email: "test@example.com",
        token: "new-access-token",
        refreshToken: "new-refresh-token",
      },
      expires: "2099-12-31",
    });

    const [result1, result2] = await Promise.all([firstRefresh, secondRefresh]);

    // Both should return the same result
    expect(result1).toEqual(result2);
    // Auth should only be called once
    expect(mockAuth).toHaveBeenCalledTimes(1);
  });

  it("should allow new refresh after previous completes", async () => {
    mockAuth.mockResolvedValue({
      user: {
        id: "123",
        email: "test@example.com",
        token: "new-access-token",
        refreshToken: "new-refresh-token",
      },
      expires: "2099-12-31",
    });

    // First refresh
    const result1 = await refreshSessionTokensOnServer();
    expect(result1).toBeDefined();

    // Second refresh after first completes
    const result2 = await refreshSessionTokensOnServer();
    expect(result2).toBeDefined();

    // Should have called auth twice
    expect(mockAuth).toHaveBeenCalledTimes(2);
  });

  it("should start fresh after a failed call (promise cleared on failure)", async () => {
    // First call: auth() returns null (failure)
    mockAuth.mockResolvedValueOnce(null);

    const result1 = await refreshSessionTokensOnServer();
    expect(result1).toBeNull();
    expect(mockAuth).toHaveBeenCalledTimes(1);

    // Second call: auth() succeeds — should NOT reuse the failed promise
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "456",
        email: "test@example.com",
        token: "recovered-access-token",
        refreshToken: "recovered-refresh-token",
      },
      expires: "2099-12-31",
    });

    const result2 = await refreshSessionTokensOnServer();
    expect(result2).toEqual({
      accessToken: "recovered-access-token",
      refreshToken: "recovered-refresh-token",
    });
    // auth() called again (promise was cleared after first failure)
    expect(mockAuth).toHaveBeenCalledTimes(2);
  });
});
