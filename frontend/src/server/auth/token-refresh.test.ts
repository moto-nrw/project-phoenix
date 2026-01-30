import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";

// Mock auth and signIn - MUST be hoisted before imports
vi.mock("~/server/auth", () => ({
  auth: vi.fn(),
  signIn: vi.fn(),
}));

// Mock env
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

// Import after mocks
import { refreshSessionTokensOnServer } from "./token-refresh";
import { auth, signIn } from "~/server/auth";

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Type the mocks properly - auth is overloaded, we use the session getter overload
const mockAuth = auth as unknown as ReturnType<
  typeof vi.fn<() => Promise<Session | null>>
>;
const mockSignIn = signIn as unknown as ReturnType<typeof vi.fn>;

describe("refreshSessionTokensOnServer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should return null when no session", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
    expect(mockFetch).not.toHaveBeenCalled();
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("should return null when no refresh token in session", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "123",
        email: "test@example.com",
        // No refreshToken
      },
      expires: "2099-12-31",
    });

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
    expect(mockFetch).not.toHaveBeenCalled();
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("should refresh tokens successfully", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "123",
        email: "test@example.com",
        refreshToken: "old-refresh-token",
      },
      expires: "2099-12-31",
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        access_token: "new-access-token",
        refresh_token: "new-refresh-token",
      }),
    });

    mockSignIn.mockResolvedValueOnce(undefined);

    const result = await refreshSessionTokensOnServer();

    expect(result).toEqual({
      accessToken: "new-access-token",
      refreshToken: "new-refresh-token",
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/refresh",
      {
        method: "POST",
        headers: {
          Authorization: "Bearer old-refresh-token",
          "Content-Type": "application/json",
        },
      },
    );

    expect(mockSignIn).toHaveBeenCalledWith("credentials", {
      redirect: false,
      internalRefresh: "true",
      token: "new-access-token",
      refreshToken: "new-refresh-token",
    });
  });

  it("should return null when refresh API fails", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "123",
        email: "test@example.com",
        refreshToken: "old-refresh-token",
      },
      expires: "2099-12-31",
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      text: async () => "Unauthorized",
    });

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("should return null when signIn fails", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "123",
        email: "test@example.com",
        refreshToken: "old-refresh-token",
      },
      expires: "2099-12-31",
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        access_token: "new-access-token",
        refresh_token: "new-refresh-token",
      }),
    });

    mockSignIn.mockRejectedValueOnce(new Error("SignIn failed"));

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
  });

  it("should handle fetch errors gracefully", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "123",
        email: "test@example.com",
        refreshToken: "old-refresh-token",
      },
      expires: "2099-12-31",
    });

    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("should only allow one refresh at a time", async () => {
    mockAuth.mockResolvedValue({
      user: {
        id: "123",
        email: "test@example.com",
        refreshToken: "old-refresh-token",
      },
      expires: "2099-12-31",
    });

    let resolveFirstFetch: (value: unknown) => void;
    const firstFetchPromise = new Promise((resolve) => {
      resolveFirstFetch = resolve;
    });

    mockFetch.mockImplementationOnce(() => firstFetchPromise);
    mockSignIn.mockResolvedValue(undefined);

    // Start first refresh
    const firstRefresh = refreshSessionTokensOnServer();

    // Start second refresh while first is in progress
    const secondRefresh = refreshSessionTokensOnServer();

    // Resolve the fetch
    resolveFirstFetch!({
      ok: true,
      json: async () => ({
        access_token: "new-access-token",
        refresh_token: "new-refresh-token",
      }),
    });

    const [result1, result2] = await Promise.all([firstRefresh, secondRefresh]);

    // Both should return the same result
    expect(result1).toEqual(result2);
    // Fetch should only be called once
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("should allow new refresh after previous completes", async () => {
    mockAuth.mockResolvedValue({
      user: {
        id: "123",
        email: "test@example.com",
        refreshToken: "old-refresh-token",
      },
      expires: "2099-12-31",
    });

    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        access_token: "new-access-token",
        refresh_token: "new-refresh-token",
      }),
    });

    mockSignIn.mockResolvedValue(undefined);

    // First refresh
    const result1 = await refreshSessionTokensOnServer();
    expect(result1).toBeDefined();

    // Second refresh after first completes
    const result2 = await refreshSessionTokensOnServer();
    expect(result2).toBeDefined();

    // Should have called fetch twice
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it("should handle malformed JSON response", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "123",
        email: "test@example.com",
        refreshToken: "old-refresh-token",
      },
      expires: "2099-12-31",
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => {
        throw new Error("Invalid JSON");
      },
    });

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("should handle response text extraction failure", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "123",
        email: "test@example.com",
        refreshToken: "old-refresh-token",
      },
      expires: "2099-12-31",
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => {
        throw new Error("Cannot read response");
      },
    });

    const result = await refreshSessionTokensOnServer();

    expect(result).toBeNull();
  });
});
