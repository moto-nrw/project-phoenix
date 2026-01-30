import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("../server/auth", () => ({
  auth: mockAuth,
}));

// Import after mocks are set up
const { checkAuth } = await import("./api-helpers.server");

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

describe("api-helpers.server", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("checkAuth", () => {
    it("returns null when authenticated with token", async () => {
      mockAuth.mockResolvedValueOnce(defaultSession);

      const result = await checkAuth();

      expect(result).toBeNull();
      expect(mockAuth).toHaveBeenCalledTimes(1);
    });

    it("returns 401 response when not authenticated", async () => {
      mockAuth.mockResolvedValueOnce(null);

      const result = await checkAuth();

      expect(result).not.toBeNull();
      expect(result?.status).toBe(401);

      const json = (await result?.json()) as { error: string };
      expect(json).toEqual({ error: "Unauthorized" });
    });

    it("returns 401 response when session has no token", async () => {
      mockAuth.mockResolvedValueOnce({
        user: { id: "1", name: "Test User" },
        expires: "2099-01-01",
      });

      const result = await checkAuth();

      expect(result).not.toBeNull();
      expect(result?.status).toBe(401);

      const json = (await result?.json()) as { error: string };
      expect(json).toEqual({ error: "Unauthorized" });
    });

    it("returns 401 response when user is missing", async () => {
      mockAuth.mockResolvedValueOnce({
        expires: "2099-01-01",
      } as ExtendedSession);

      const result = await checkAuth();

      expect(result).not.toBeNull();
      expect(result?.status).toBe(401);
    });
  });
});
