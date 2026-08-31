import { describe, it, expect } from "vitest";
import type { Session } from "next-auth";
import {
  getSmartRedirectPath,
  TAGESPLAN_PATH,
  useSmartRedirectPath,
  type SupervisionState,
} from "./redirect-utils";

// Seit #2383 ist der Tages-Betreuungsplan der Standard-Einstieg für
// Betreuungskräfte im detaillierten Modus. Die früheren Ziele (/ogs-groups,
// /active-supervisions, /students/search bei offener Betreuung) gelten nur
// noch als Fallback, wenn die Schule den Betreuungsplan abgeschaltet hat
// (timetable.enabled = false) — diese Fälle setzen unten explizit
// timetableEnabled=false.
describe("redirect-utils", () => {
  describe("getSmartRedirectPath", () => {
    const createSession = (roles: string[]): Session => ({
      user: {
        id: "1",
        email: "test@example.com",
        roles,
        token: "token",
      },
      expires: "2024-12-31",
    });

    const idle: SupervisionState = {
      hasGroups: false,
      isLoadingGroups: false,
      isSupervising: false,
      isLoadingSupervision: false,
    };

    it("sends detailed-mode caregivers to the Tagesplan (#2383)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(session, idle);
      expect(result).toBe(TAGESPLAN_PATH);
    });

    it("sends caregivers with groups to the Tagesplan when the Betreuungsplan is enabled (#2383)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(session, {
        ...idle,
        hasGroups: true,
      });
      expect(result).toBe(TAGESPLAN_PATH);
    });

    it("sends supervising caregivers to the Tagesplan when the Betreuungsplan is enabled (#2383)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(session, {
        ...idle,
        isSupervising: true,
      });
      expect(result).toBe(TAGESPLAN_PATH);
    });

    it("sends open-care caregivers to the Tagesplan when the Betreuungsplan is enabled (#2383)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(session, idle, "detailed", true);
      expect(result).toBe(TAGESPLAN_PATH);
    });

    it("keeps binary-mode caregivers on /students/search even with the Betreuungsplan enabled (#2383)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(
        session,
        { ...idle, isSupervising: true },
        "binary",
      );
      expect(result).toBe("/students/search");
    });

    it("should return /ogs-groups when groups are loading (Betreuungsplan disabled)", () => {
      const session = createSession(["user"]);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: true,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      const result = getSmartRedirectPath(
        session,
        supervisionState,
        "detailed",
        false,
        false,
      );
      expect(result).toBe("/ogs-groups");
    });

    it("should return /ogs-groups when supervision is loading (Betreuungsplan disabled)", () => {
      const session = createSession(["user"]);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: true,
      };

      const result = getSmartRedirectPath(
        session,
        supervisionState,
        "detailed",
        false,
        false,
      );
      expect(result).toBe("/ogs-groups");
    });

    it("should return /dashboard for admin users", () => {
      const session = createSession(["admin"]);
      const result = getSmartRedirectPath(session, idle);
      expect(result).toBe("/dashboard");
    });

    it("hands an existing school-only session to moto schule", () => {
      const session = createSession(["lehrkraft"]);
      const result = getSmartRedirectPath(session, idle);
      expect(result).toBe("/school/login");
    });

    it("keeps caregiver flows for dual-role lehrkraft accounts", () => {
      const session = createSession(["lehrkraft", "user"]);
      const result = getSmartRedirectPath(session, {
        ...idle,
        hasGroups: true,
      });
      expect(result).toBe(TAGESPLAN_PATH);
    });

    it("should return /ogs-groups for users with groups (Betreuungsplan disabled)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(
        session,
        { ...idle, hasGroups: true },
        "detailed",
        false,
        false,
      );
      expect(result).toBe("/ogs-groups");
    });

    it("should return /active-supervisions for users actively supervising (Betreuungsplan disabled)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(
        session,
        { ...idle, isSupervising: true },
        "detailed",
        false,
        false,
      );
      expect(result).toBe("/active-supervisions");
    });

    it("should return /students/search for binary-mode caregivers", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(
        session,
        { ...idle, isSupervising: true },
        "binary",
      );
      expect(result).toBe("/students/search");
    });

    it("should return /students/search for binary-mode caregivers with groups", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(
        session,
        { ...idle, hasGroups: true },
        "binary",
      );
      expect(result).toBe("/students/search");
    });

    it("should return /students/search for open-care caregivers when the Betreuungsplan is disabled (#1544)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(
        session,
        { ...idle, isSupervising: true },
        "detailed",
        true,
        false,
      );
      expect(result).toBe("/students/search");
    });

    it("should return /students/search for open-care caregivers with groups when the Betreuungsplan is disabled (#1544)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(
        session,
        { ...idle, hasGroups: true },
        "detailed",
        true,
        false,
      );
      expect(result).toBe("/students/search");
    });

    it("should return /ogs-groups as default for regular users (Betreuungsplan disabled)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(
        session,
        idle,
        "detailed",
        false,
        false,
      );
      expect(result).toBe("/ogs-groups");
    });

    it("should treat teacher-only accounts as caregiver users", () => {
      const session = createSession(["teacher"]);
      const result = getSmartRedirectPath(session, {
        ...idle,
        isSupervising: true,
      });
      expect(result).toBe(TAGESPLAN_PATH);
    });

    it("should prioritize caregiver access over admin when both roles are present", () => {
      const session = createSession(["admin", "user"]);
      const result = getSmartRedirectPath(session, {
        ...idle,
        hasGroups: true,
      });
      expect(result).toBe(TAGESPLAN_PATH);
    });

    it("should prioritize groups over supervision (Betreuungsplan disabled)", () => {
      const session = createSession(["user"]);
      const result = getSmartRedirectPath(
        session,
        { ...idle, hasGroups: true, isSupervising: true },
        "detailed",
        false,
        false,
      );
      expect(result).toBe("/ogs-groups");
    });

    it("should handle null session", () => {
      const result = getSmartRedirectPath(null, idle);
      expect(result).toBe("/dashboard");
    });

    it("should return caregiver loading fallback for dual-role users (Betreuungsplan disabled)", () => {
      const session = createSession(["admin", "user"]);
      const supervisionState: SupervisionState = {
        hasGroups: true,
        isLoadingGroups: true,
        isSupervising: true,
        isLoadingSupervision: false,
      };

      const result = getSmartRedirectPath(
        session,
        supervisionState,
        "detailed",
        false,
        false,
      );
      expect(result).toBe("/ogs-groups");
    });
  });

  describe("useSmartRedirectPath", () => {
    const createSession = (roles: string[]): Session => ({
      user: {
        id: "1",
        email: "test@example.com",
        roles,
        token: "token",
      },
      expires: "2024-12-31",
    });

    it("should return isReady false when groups are loading", () => {
      const session = createSession(["user"]);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: true,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(false);
      expect(result.redirectPath).toBe(TAGESPLAN_PATH);
    });

    it("should return isReady false when supervision is loading", () => {
      const session = createSession(["user"]);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: true,
      };

      const result = useSmartRedirectPath(
        session,
        supervisionState,
        "detailed",
        false,
        false,
      );

      expect(result.isReady).toBe(false);
      expect(result.redirectPath).toBe("/ogs-groups");
    });

    it("should return isReady true when nothing is loading", () => {
      const session = createSession(["user"]);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe(TAGESPLAN_PATH);
    });

    it("should return correct path for admin when ready", () => {
      const session = createSession(["admin"]);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe("/dashboard");
    });

    it("should return correct path for user with groups when ready (Betreuungsplan disabled)", () => {
      const session = createSession(["user"]);
      const supervisionState: SupervisionState = {
        hasGroups: true,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      const result = useSmartRedirectPath(
        session,
        supervisionState,
        "detailed",
        false,
        false,
      );

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe("/ogs-groups");
    });

    it("should return supervising path when ready (Betreuungsplan disabled)", () => {
      const session = createSession(["user"]);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: true,
        isLoadingSupervision: false,
      };

      const result = useSmartRedirectPath(
        session,
        supervisionState,
        "detailed",
        false,
        false,
      );

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe("/active-supervisions");
    });

    it("should return students search for binary-mode caregiver when ready", () => {
      const session = createSession(["user"]);
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: true,
        isLoadingSupervision: false,
      };

      const result = useSmartRedirectPath(session, supervisionState, "binary");

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe("/students/search");
    });

    it("should return the Tagesplan for teacher-only accounts when ready (#2383)", () => {
      const session = createSession(["teacher"]);
      const supervisionState: SupervisionState = {
        hasGroups: true,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe(TAGESPLAN_PATH);
    });

    it("should handle null session", () => {
      const supervisionState: SupervisionState = {
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      const result = useSmartRedirectPath(null, supervisionState);

      expect(result.isReady).toBe(true);
      expect(result.redirectPath).toBe("/dashboard");
    });

    it("should always return both redirectPath and isReady", () => {
      const session = createSession(["user"]);
      const supervisionState: SupervisionState = {
        hasGroups: true,
        isLoadingGroups: false,
        isSupervising: false,
        isLoadingSupervision: false,
      };

      const result = useSmartRedirectPath(session, supervisionState);

      expect(result).toHaveProperty("redirectPath");
      expect(result).toHaveProperty("isReady");
      expect(typeof result.redirectPath).toBe("string");
      expect(typeof result.isReady).toBe("boolean");
    });
  });
});
