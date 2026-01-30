import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { GET } from "./route";

// ============================================================================
// Types
// ============================================================================

type ExtendedSession = Omit<Session, "user" | "error"> & {
  user: Session["user"] & { token?: string };
  error?: string;
};

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

// ============================================================================
// Test Helpers
// ============================================================================

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/update-session", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 401 when no session found", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error: string };
    expect(json.error).toBe("No session found");
  });

  it("returns success with valid session", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const response = await GET();

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success: boolean;
      hasToken: boolean;
    };
    expect(json.success).toBe(true);
    expect(json.hasToken).toBe(true);
  });

  it("indicates when session has a token", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const response = await GET();
    const json = (await response.json()) as { hasToken: boolean };

    expect(json.hasToken).toBe(true);
  });

  it("indicates when session lacks a token", async () => {
    const sessionWithoutToken: ExtendedSession = {
      user: { id: "1", name: "Test User" },
      expires: "2099-01-01",
    };
    mockAuth.mockResolvedValueOnce(sessionWithoutToken);

    const response = await GET();
    const json = (await response.json()) as { hasToken: boolean };

    expect(json.hasToken).toBe(false);
  });

  it("includes token error if present", async () => {
    const sessionWithError: ExtendedSession = {
      user: { id: "1", token: "test-token", name: "Test User" },
      expires: "2099-01-01",
      error: "RefreshAccessTokenError",
    };
    mockAuth.mockResolvedValueOnce(sessionWithError);

    const response = await GET();
    const json = (await response.json()) as { tokenError: string };

    expect(json.tokenError).toBe("RefreshAccessTokenError");
  });

  it("returns undefined tokenError when no error", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const response = await GET();
    const json = (await response.json()) as { tokenError?: string };

    expect(json.tokenError).toBeUndefined();
  });

  it("handles auth function errors gracefully", async () => {
    mockAuth.mockRejectedValueOnce(new Error("Auth service unavailable"));

    const response = await GET();

    expect(response.status).toBe(500);
    const json = (await response.json()) as { error: string };
    expect(json.error).toBe("Failed to update session");
  });

  it("triggers session refresh via auth() call", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    await GET();

    expect(mockAuth).toHaveBeenCalledTimes(1);
  });
});
