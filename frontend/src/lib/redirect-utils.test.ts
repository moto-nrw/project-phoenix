import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  checkSaasAdminStatus,
  getSmartRedirectPath,
  useSmartRedirectPath,
  type SupervisionState,
  type SaasAdminState,
} from "./redirect-utils";
import type { BetterAuthSession } from "./auth-utils";

describe("redirect-utils", () => {
  describe("checkSaasAdminStatus", () => {
    const originalFetch = globalThis.fetch;
    let mockFetch: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      mockFetch = vi.fn();
      globalThis.fetch = mockFetch as typeof fetch;
    });

    afterEach(() => {
      globalThis.fetch = originalFetch;
    });

    it("returns true when API returns isSaasAdmin: true", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ isSaasAdmin: true }),
      });

      const result = await checkSaasAdminStatus();

      expect(result).toBe(true);
      expect(mockFetch).toHaveBeenCalledWith("/api/auth/check-saas-admin", {
        method: "GET",
        credentials: "include",
      });
    });

    it("returns false when API returns isSaasAdmin: false", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ isSaasAdmin: false }),
      });

      const result = await checkSaasAdminStatus();

      expect(result).toBe(false);
    });

    it("returns false when response is not ok", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
      });

      const result = await checkSaasAdminStatus();

      expect(result).toBe(false);
    });

    it("returns false when fetch throws an error", async () => {
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});
      const error = new Error("Network error");
      mockFetch.mockRejectedValueOnce(error);

      const result = await checkSaasAdminStatus();

      expect(result).toBe(false);
      expect(consoleSpy).toHaveBeenCalledWith(
        "Failed to check SaaS admin status:",
        error,
      );
      consoleSpy.mockRestore();
    });

    it("returns false when response is 500 server error", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      const result = await checkSaasAdminStatus();

      expect(result).toBe(false);
    });
  });

  describe("getSmartRedirectPath", () => {
    const createSession = (
      overrides: Partial<BetterAuthSession["user"]> = {},
    ): BetterAuthSession => ({
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
        isAdmin: false,
        ...overrides,
      },
    });

    const defaultSupervisionState: SupervisionState = {
      hasGroups: false,
      isLoadingGroups: false,
      isSupervising: false,
      isLoadingSupervision: false,
    };

    it("returns /ogs-groups when loading groups", () => {
      const session = createSession();
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        isLoadingGroups: true,
      };

      const result = getSmartRedirectPath(session, supervisionState);

      expect(result).toBe("/ogs-groups");
    });

    it("returns /ogs-groups when loading supervision", () => {
      const session = createSession();
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        isLoadingSupervision: true,
      };

      const result = getSmartRedirectPath(session, supervisionState);

      expect(result).toBe("/ogs-groups");
    });

    it("returns /console when user is SaaS admin and not loading", () => {
      const session = createSession();
      const supervisionState: SupervisionState = defaultSupervisionState;
      const saasAdminState: SaasAdminState = {
        isSaasAdmin: true,
        isLoading: false,
      };

      const result = getSmartRedirectPath(
        session,
        supervisionState,
        saasAdminState,
      );

      expect(result).toBe("/console");
    });

    it("does not return /console when SaaS admin state is still loading", () => {
      const session = createSession();
      const supervisionState: SupervisionState = defaultSupervisionState;
      const saasAdminState: SaasAdminState = {
        isSaasAdmin: true,
        isLoading: true,
      };

      const result = getSmartRedirectPath(
        session,
        supervisionState,
        saasAdminState,
      );

      // Should fall through to default since loading
      expect(result).toBe("/ogs-groups");
    });

    it("returns /dashboard when user is org admin", () => {
      const session = createSession({ isAdmin: true });
      const supervisionState: SupervisionState = defaultSupervisionState;

      const result = getSmartRedirectPath(session, supervisionState);

      expect(result).toBe("/dashboard");
    });

    it("returns /dashboard for admin even if has groups", () => {
      const session = createSession({ isAdmin: true });
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        hasGroups: true,
      };

      const result = getSmartRedirectPath(session, supervisionState);

      expect(result).toBe("/dashboard");
    });

    it("returns /ogs-groups when user has groups", () => {
      const session = createSession();
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        hasGroups: true,
      };

      const result = getSmartRedirectPath(session, supervisionState);

      expect(result).toBe("/ogs-groups");
    });

    it("returns /active-supervisions when user is supervising", () => {
      const session = createSession();
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        isSupervising: true,
      };

      const result = getSmartRedirectPath(session, supervisionState);

      expect(result).toBe("/active-supervisions");
    });

    it("returns /ogs-groups when user is supervising and has groups (groups takes priority)", () => {
      const session = createSession();
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        hasGroups: true,
        isSupervising: true,
      };

      const result = getSmartRedirectPath(session, supervisionState);

      expect(result).toBe("/ogs-groups");
    });

    it("returns /ogs-groups as default for regular users", () => {
      const session = createSession();
      const supervisionState: SupervisionState = defaultSupervisionState;

      const result = getSmartRedirectPath(session, supervisionState);

      expect(result).toBe("/ogs-groups");
    });

    it("returns /ogs-groups when session is null", () => {
      const supervisionState: SupervisionState = defaultSupervisionState;

      const result = getSmartRedirectPath(null, supervisionState);

      expect(result).toBe("/ogs-groups");
    });

    it("returns /ogs-groups when saasAdminState is undefined", () => {
      const session = createSession();
      const supervisionState: SupervisionState = defaultSupervisionState;

      const result = getSmartRedirectPath(session, supervisionState, undefined);

      expect(result).toBe("/ogs-groups");
    });

    it("prioritizes SaaS admin over org admin", () => {
      const session = createSession({ isAdmin: true });
      const supervisionState: SupervisionState = defaultSupervisionState;
      const saasAdminState: SaasAdminState = {
        isSaasAdmin: true,
        isLoading: false,
      };

      const result = getSmartRedirectPath(
        session,
        supervisionState,
        saasAdminState,
      );

      expect(result).toBe("/console");
    });

    it("prioritizes org admin over groups", () => {
      const session = createSession({ isAdmin: true });
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        hasGroups: true,
        isSupervising: true,
      };

      const result = getSmartRedirectPath(session, supervisionState);

      expect(result).toBe("/dashboard");
    });
  });

  describe("useSmartRedirectPath", () => {
    const createSession = (
      overrides: Partial<BetterAuthSession["user"]> = {},
    ): BetterAuthSession => ({
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
        isAdmin: false,
        ...overrides,
      },
    });

    const defaultSupervisionState: SupervisionState = {
      hasGroups: false,
      isLoadingGroups: false,
      isSupervising: false,
      isLoadingSupervision: false,
    };

    it("returns isReady=false when loading groups", () => {
      const session = createSession();
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        isLoadingGroups: true,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(false);
      expect(result.redirectPath).toBe("/ogs-groups");
    });

    it("returns isReady=false when loading supervision", () => {
      const session = createSession();
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        isLoadingSupervision: true,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(false);
    });

    it("returns isReady=false when saasAdminState is loading", () => {
      const session = createSession();
      const supervisionState: SupervisionState = defaultSupervisionState;
      const saasAdminState: SaasAdminState = {
        isSaasAdmin: true,
        isLoading: true,
      };

      const result = useSmartRedirectPath(
        session,
        supervisionState,
        saasAdminState,
      );

      expect(result.isReady).toBe(false);
    });

    it("returns isReady=true when all loading is complete", () => {
      const session = createSession();
      const supervisionState: SupervisionState = defaultSupervisionState;
      const saasAdminState: SaasAdminState = {
        isSaasAdmin: false,
        isLoading: false,
      };

      const result = useSmartRedirectPath(
        session,
        supervisionState,
        saasAdminState,
      );

      expect(result.isReady).toBe(true);
    });

    it("returns isReady=true when saasAdminState is undefined", () => {
      const session = createSession();
      const supervisionState: SupervisionState = defaultSupervisionState;

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(true);
    });

    it("returns correct redirect path for admin", () => {
      const session = createSession({ isAdmin: true });
      const supervisionState: SupervisionState = defaultSupervisionState;

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.redirectPath).toBe("/dashboard");
      expect(result.isReady).toBe(true);
    });

    it("returns correct redirect path for SaaS admin", () => {
      const session = createSession();
      const supervisionState: SupervisionState = defaultSupervisionState;
      const saasAdminState: SaasAdminState = {
        isSaasAdmin: true,
        isLoading: false,
      };

      const result = useSmartRedirectPath(
        session,
        supervisionState,
        saasAdminState,
      );

      expect(result.redirectPath).toBe("/console");
      expect(result.isReady).toBe(true);
    });

    it("returns correct redirect path for user with groups", () => {
      const session = createSession();
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        hasGroups: true,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.redirectPath).toBe("/ogs-groups");
      expect(result.isReady).toBe(true);
    });

    it("returns correct redirect path for supervising user", () => {
      const session = createSession();
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        isSupervising: true,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.redirectPath).toBe("/active-supervisions");
      expect(result.isReady).toBe(true);
    });

    it("handles null session", () => {
      const supervisionState: SupervisionState = defaultSupervisionState;

      const result = useSmartRedirectPath(null, supervisionState);

      expect(result.redirectPath).toBe("/ogs-groups");
      expect(result.isReady).toBe(true);
    });

    it("handles all loading states together", () => {
      const session = createSession();
      const supervisionState: SupervisionState = {
        ...defaultSupervisionState,
        isLoadingGroups: true,
        isLoadingSupervision: true,
      };
      const saasAdminState: SaasAdminState = {
        isSaasAdmin: false,
        isLoading: true,
      };

      const result = useSmartRedirectPath(
        session,
        supervisionState,
        saasAdminState,
      );

      expect(result.isReady).toBe(false);
      expect(result.redirectPath).toBe("/ogs-groups");
    });
  });
});
