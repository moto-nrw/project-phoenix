import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import {
  getSmartRedirectPath,
  useSmartRedirectPath,
  type SupervisionState,
} from "./redirect-utils";

// Mock the auth-utils module
vi.mock("~/lib/auth-utils", () => ({
  isAdmin: vi.fn(),
}));

import { isAdmin } from "~/lib/auth-utils";

describe("redirect-utils", () => {
  beforeEach(() => {
    // Reset mocks before each test
    vi.clearAllMocks();
  });

  describe("getSmartRedirectPath", () => {
    const createSession = (isAdminUser: boolean): Session => ({
      user: {
        id: "1",
        email: "test@example.com",
        isAdmin: isAdminUser,
        isTeacher: !isAdminUser,
        token: "token",
      },
      expires: "2024-12-31",
    });

    it("should return /ogs-groups when groups are loading", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: true,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/ogs-groups");
    });

    it("should return /ogs-groups when supervision is loading", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: true,
      };

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/ogs-groups");
    });

    it("should return /ogs-groups when both are loading", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: true,
        isSupervising: false,
        isLoadingSupervision: true,
      };

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/ogs-groups");
    });

    it("should return /dashboard for admin users", () => {
      const session = createSession(true);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(true);

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/dashboard");
      expect(isAdmin).toHaveBeenCalledWith(session);
    });

    it("should return /ogs-groups for users with groups", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: true,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(false);

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/ogs-groups");
    });

    it("should return /active-supervisions for users actively supervising", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: true,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(false);

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/active-supervisions");
    });

    it("should return /ogs-groups as default for regular users", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(false);

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/ogs-groups");
    });

    it("should prioritize admin over groups", () => {
      const session = createSession(true);
      const supervisionState: SupervisionState = {
        hasGroups: true,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(true);

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/dashboard");
    });

    it("should prioritize admin over supervision", () => {
      const session = createSession(true);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: true,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(true);

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/dashboard");
    });

    it("should prioritize groups over supervision", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: true,
        isLoadingGroups: false,
        isSupervising: true,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(false);

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/ogs-groups");
    });

    it("should handle null session", () => {
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(false);

      const result = getSmartRedirectPath(null, supervisionState);
      expect(result).toBe("/ogs-groups");
    });

    it("should return loading fallback even if other conditions are true", () => {
      const session = createSession(true);
      const supervisionState: SupervisionState = {
        hasGroups: true,
        isLoadingGroups: true,
        isSupervising: true,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(true);

      const result = getSmartRedirectPath(session, supervisionState);
      expect(result).toBe("/ogs-groups");
    });
  });

  describe("useSmartRedirectPath", () => {
    const createSession = (isAdminUser: boolean): Session => ({
      user: {
        id: "1",
        email: "test@example.com",
        isAdmin: isAdminUser,
        isTeacher: !isAdminUser,
        token: "token",
      },
      expires: "2024-12-31",
    });

    it("should return isReady false when groups are loading", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: true,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(false);
      expect(result.redirectPath).toBe("/ogs-groups");
    });

    it("should return isReady false when supervision is loading", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: true,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(false);
      expect(result.redirectPath).toBe("/ogs-groups");
    });

    it("should return isReady false when both are loading", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: true,
        isSupervising: false,
        isLoadingSupervision: true,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(false);
      expect(result.redirectPath).toBe("/ogs-groups");
    });

    it("should return isReady true when nothing is loading", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(false);

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe("/ogs-groups");
    });

    it("should return correct path for admin when ready", () => {
      const session = createSession(true);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(true);

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe("/dashboard");
    });

    it("should return correct path for user with groups when ready", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: true,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(false);

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe("/ogs-groups");
    });

    it("should return correct path for supervising user when ready", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: true,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(false);

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe("/active-supervisions");
    });

    it("should handle null session", () => {
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(false);

      const result = useSmartRedirectPath(null, supervisionState);

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe("/ogs-groups");
    });

    it("should always return both redirectPath and isReady", () => {
      const session = createSession(false);
      const supervisionState: SupervisionState = {
        hasGroups: true,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      vi.mocked(isAdmin).mockReturnValue(false);

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result).toHaveProperty("redirectPath");
      expect(result).toHaveProperty("isReady");
      expect(typeof result.redirectPath).toBe("string");
      expect(typeof result.isReady).toBe("boolean");
    });
  });
});
