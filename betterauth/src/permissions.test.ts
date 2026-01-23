/**
 * Comprehensive tests for betterauth/src/permissions.ts
 *
 * Tests the GDPR-compliant role-based access control system for Project Phoenix.
 * Verifies that:
 * 1. The access controller is correctly configured with all resources
 * 2. Each role has the correct permissions
 * 3. GDPR compliance: location permissions are properly restricted
 * 4. The authorize() method works correctly for all roles
 */
import { describe, it, expect } from "vitest";
import {
  ac,
  supervisor,
  ogsAdmin,
  bueroAdmin,
  traegerAdmin,
} from "./permissions.js";

// Type helper to access any property on statements for testing
type AnyStatements = Record<string, readonly string[] | undefined>;

// Type for authorize to allow testing invalid inputs
type AnyAuthorize = (
  request: Record<string, string[]>,
  connector?: "AND" | "OR"
) => { success: boolean; error?: string };

// ============================================
// ACCESS CONTROLLER TESTS
// ============================================

describe("Access Controller (ac)", () => {
  it("exports an access controller with statements", () => {
    expect(ac).toBeDefined();
    expect(ac.statements).toBeDefined();
    expect(typeof ac.newRole).toBe("function");
  });

  it("defines all required resources", () => {
    const expectedResources = [
      "student",
      "group",
      "room",
      "attendance",
      "location",
      "staff",
      "ogs",
      "config",
      "schedule",
      "feedback",
      "substitution",
      "person",
    ];

    const statements = ac.statements as AnyStatements;
    for (const resource of expectedResources) {
      expect(statements[resource]).toBeDefined();
    }
  });

  it("defines correct actions for student resource", () => {
    expect(ac.statements.student).toEqual(["read", "create", "update", "delete"]);
  });

  it("defines correct actions for group resource including assign", () => {
    expect(ac.statements.group).toEqual(["read", "create", "update", "delete", "assign"]);
  });

  it("defines correct actions for room resource", () => {
    expect(ac.statements.room).toEqual(["read", "create", "update", "delete"]);
  });

  it("defines correct actions for attendance resource", () => {
    expect(ac.statements.attendance).toEqual(["read", "checkin", "checkout"]);
  });

  it("defines location resource with only read action (GDPR sensitive)", () => {
    expect(ac.statements.location).toEqual(["read"]);
  });

  it("defines correct actions for staff resource including invite", () => {
    expect(ac.statements.staff).toEqual(["read", "create", "update", "delete", "invite"]);
  });

  it("defines correct actions for ogs resource", () => {
    expect(ac.statements.ogs).toEqual(["read", "update"]);
  });

  it("defines correct actions for config resource including manage", () => {
    expect(ac.statements.config).toEqual(["read", "create", "update", "manage"]);
  });

  it("defines correct actions for schedule resource", () => {
    expect(ac.statements.schedule).toEqual(["read", "create", "update", "delete"]);
  });

  it("defines correct actions for feedback resource", () => {
    expect(ac.statements.feedback).toEqual(["read", "create", "delete"]);
  });

  it("defines correct actions for substitution resource", () => {
    expect(ac.statements.substitution).toEqual(["read", "create", "update", "delete"]);
  });

  it("defines correct actions for person resource", () => {
    expect(ac.statements.person).toEqual(["read", "create", "update", "delete"]);
  });
});

// ============================================
// SUPERVISOR ROLE TESTS
// ============================================

describe("Supervisor Role", () => {
  // Cast to AnyStatements for checking undefined properties
  const statements = supervisor.statements as AnyStatements;
  // Wrap in arrow function to avoid unbound-method lint error
  const authorize: AnyAuthorize = (req, conn) => supervisor.authorize(req as Parameters<typeof supervisor.authorize>[0], conn);

  it("exports a supervisor role", () => {
    expect(supervisor).toBeDefined();
    expect(supervisor.statements).toBeDefined();
    expect(typeof supervisor.authorize).toBe("function");
  });

  describe("permissions", () => {
    it("has read and update permissions for student", () => {
      expect(supervisor.statements.student).toEqual(["read", "update"]);
    });

    it("has read-only permission for group", () => {
      expect(supervisor.statements.group).toEqual(["read"]);
    });

    it("has full attendance permissions (read, checkin, checkout)", () => {
      expect(supervisor.statements.attendance).toEqual(["read", "checkin", "checkout"]);
    });

    it("has location:read permission (operational need)", () => {
      expect(supervisor.statements.location).toEqual(["read"]);
    });

    it("has read-only permission for schedule", () => {
      expect(supervisor.statements.schedule).toEqual(["read"]);
    });

    it("has read and create permissions for feedback", () => {
      expect(supervisor.statements.feedback).toEqual(["read", "create"]);
    });

    it("has read-only permission for substitution", () => {
      expect(supervisor.statements.substitution).toEqual(["read"]);
    });

    it("does NOT have room permissions", () => {
      expect(statements.room).toBeUndefined();
    });

    it("does NOT have staff permissions", () => {
      expect(statements.staff).toBeUndefined();
    });

    it("does NOT have ogs permissions", () => {
      expect(statements.ogs).toBeUndefined();
    });

    it("does NOT have config permissions", () => {
      expect(statements.config).toBeUndefined();
    });

    it("does NOT have person permissions", () => {
      expect(statements.person).toBeUndefined();
    });
  });

  describe("authorize()", () => {
    it("authorizes reading students", () => {
      const result = supervisor.authorize({ student: ["read"] });
      expect(result.success).toBe(true);
    });

    it("authorizes updating students", () => {
      const result = supervisor.authorize({ student: ["update"] });
      expect(result.success).toBe(true);
    });

    it("authorizes reading and updating students together", () => {
      const result = supervisor.authorize({ student: ["read", "update"] });
      expect(result.success).toBe(true);
    });

    it("denies creating students", () => {
      const result = authorize({ student: ["create"] });
      expect(result.success).toBe(false);
    });

    it("denies deleting students", () => {
      const result = authorize({ student: ["delete"] });
      expect(result.success).toBe(false);
    });

    it("authorizes check-in operations", () => {
      const result = supervisor.authorize({ attendance: ["checkin"] });
      expect(result.success).toBe(true);
    });

    it("authorizes checkout operations", () => {
      const result = supervisor.authorize({ attendance: ["checkout"] });
      expect(result.success).toBe(true);
    });

    it("authorizes reading location (operational need)", () => {
      const result = supervisor.authorize({ location: ["read"] });
      expect(result.success).toBe(true);
    });

    it("denies accessing room resource", () => {
      const result = authorize({ room: ["read"] });
      expect(result.success).toBe(false);
    });

    it("denies managing staff", () => {
      const result = authorize({ staff: ["read"] });
      expect(result.success).toBe(false);
    });

    it("authorizes reading feedback", () => {
      const result = supervisor.authorize({ feedback: ["read"] });
      expect(result.success).toBe(true);
    });

    it("authorizes creating feedback", () => {
      const result = supervisor.authorize({ feedback: ["create"] });
      expect(result.success).toBe(true);
    });

    it("denies deleting feedback", () => {
      const result = authorize({ feedback: ["delete"] });
      expect(result.success).toBe(false);
    });
  });
});

// ============================================
// OGS ADMIN ROLE TESTS
// ============================================

describe("OGS Admin Role", () => {
  // Wrap in arrow function to avoid unbound-method lint error
  const authorize: AnyAuthorize = (req, conn) => ogsAdmin.authorize(req as Parameters<typeof ogsAdmin.authorize>[0], conn);

  it("exports an ogsAdmin role", () => {
    expect(ogsAdmin).toBeDefined();
    expect(ogsAdmin.statements).toBeDefined();
    expect(typeof ogsAdmin.authorize).toBe("function");
  });

  describe("permissions", () => {
    it("has full student permissions", () => {
      expect(ogsAdmin.statements.student).toEqual(["read", "create", "update", "delete"]);
    });

    it("has full group permissions including assign", () => {
      expect(ogsAdmin.statements.group).toEqual(["read", "create", "update", "delete", "assign"]);
    });

    it("has full room permissions", () => {
      expect(ogsAdmin.statements.room).toEqual(["read", "create", "update", "delete"]);
    });

    it("has full attendance permissions", () => {
      expect(ogsAdmin.statements.attendance).toEqual(["read", "checkin", "checkout"]);
    });

    it("has location:read permission (operational OGS management)", () => {
      expect(ogsAdmin.statements.location).toEqual(["read"]);
    });

    it("has staff permissions without delete", () => {
      expect(ogsAdmin.statements.staff).toEqual(["read", "create", "update", "invite"]);
    });

    it("has ogs read and update permissions", () => {
      expect(ogsAdmin.statements.ogs).toEqual(["read", "update"]);
    });

    it("has full config permissions", () => {
      expect(ogsAdmin.statements.config).toEqual(["read", "create", "update", "manage"]);
    });

    it("has full schedule permissions", () => {
      expect(ogsAdmin.statements.schedule).toEqual(["read", "create", "update", "delete"]);
    });

    it("has full feedback permissions", () => {
      expect(ogsAdmin.statements.feedback).toEqual(["read", "create", "delete"]);
    });

    it("has full substitution permissions", () => {
      expect(ogsAdmin.statements.substitution).toEqual(["read", "create", "update", "delete"]);
    });

    it("has full person permissions", () => {
      expect(ogsAdmin.statements.person).toEqual(["read", "create", "update", "delete"]);
    });
  });

  describe("authorize()", () => {
    it("authorizes full student CRUD", () => {
      const result = ogsAdmin.authorize({ student: ["read", "create", "update", "delete"] });
      expect(result.success).toBe(true);
    });

    it("authorizes group assignment", () => {
      const result = ogsAdmin.authorize({ group: ["assign"] });
      expect(result.success).toBe(true);
    });

    it("authorizes reading location (operational need)", () => {
      const result = ogsAdmin.authorize({ location: ["read"] });
      expect(result.success).toBe(true);
    });

    it("authorizes inviting staff", () => {
      const result = ogsAdmin.authorize({ staff: ["invite"] });
      expect(result.success).toBe(true);
    });

    it("denies deleting staff", () => {
      const result = authorize({ staff: ["delete"] });
      expect(result.success).toBe(false);
    });

    it("authorizes managing config", () => {
      const result = ogsAdmin.authorize({ config: ["manage"] });
      expect(result.success).toBe(true);
    });

    it("authorizes multiple resource access", () => {
      const result = ogsAdmin.authorize({
        student: ["read"],
        room: ["update"],
        schedule: ["create"],
      });
      expect(result.success).toBe(true);
    });
  });
});

// ============================================
// BURO ADMIN ROLE TESTS (GDPR RESTRICTED)
// ============================================

describe("Buro Admin Role (GDPR Restricted)", () => {
  // Cast to AnyStatements for checking undefined properties
  const statements = bueroAdmin.statements as AnyStatements;
  // Wrap in arrow function to avoid unbound-method lint error
  const authorize: AnyAuthorize = (req, conn) => bueroAdmin.authorize(req as Parameters<typeof bueroAdmin.authorize>[0], conn);

  it("exports a bueroAdmin role", () => {
    expect(bueroAdmin).toBeDefined();
    expect(bueroAdmin.statements).toBeDefined();
    expect(typeof bueroAdmin.authorize).toBe("function");
  });

  describe("permissions", () => {
    it("has full student permissions", () => {
      expect(bueroAdmin.statements.student).toEqual(["read", "create", "update", "delete"]);
    });

    it("has group permissions WITHOUT assign (not operational)", () => {
      expect(bueroAdmin.statements.group).toEqual(["read", "create", "update", "delete"]);
      expect(bueroAdmin.statements.group).not.toContain("assign");
    });

    it("does NOT have room permissions", () => {
      expect(statements.room).toBeUndefined();
    });

    it("has attendance read-only permission", () => {
      expect(bueroAdmin.statements.attendance).toEqual(["read"]);
    });

    it("does NOT have location permission (GDPR CRITICAL)", () => {
      expect(statements.location).toBeUndefined();
    });

    it("has full staff permissions including delete", () => {
      expect(bueroAdmin.statements.staff).toEqual(["read", "create", "update", "delete", "invite"]);
    });

    it("has ogs read and update permissions", () => {
      expect(bueroAdmin.statements.ogs).toEqual(["read", "update"]);
    });

    it("has full config permissions", () => {
      expect(bueroAdmin.statements.config).toEqual(["read", "create", "update", "manage"]);
    });

    it("has full schedule permissions", () => {
      expect(bueroAdmin.statements.schedule).toEqual(["read", "create", "update", "delete"]);
    });

    it("has full feedback permissions", () => {
      expect(bueroAdmin.statements.feedback).toEqual(["read", "create", "delete"]);
    });

    it("has full substitution permissions", () => {
      expect(bueroAdmin.statements.substitution).toEqual(["read", "create", "update", "delete"]);
    });

    it("has full person permissions", () => {
      expect(bueroAdmin.statements.person).toEqual(["read", "create", "update", "delete"]);
    });
  });

  describe("authorize()", () => {
    it("authorizes full student CRUD", () => {
      const result = bueroAdmin.authorize({ student: ["read", "create", "update", "delete"] });
      expect(result.success).toBe(true);
    });

    it("DENIES location access (GDPR CRITICAL)", () => {
      const result = authorize({ location: ["read"] });
      expect(result.success).toBe(false);
      expect(result.error).toContain("location");
    });

    it("authorizes deleting staff (higher privilege than ogsAdmin)", () => {
      const result = bueroAdmin.authorize({ staff: ["delete"] });
      expect(result.success).toBe(true);
    });

    it("denies group assignment (not operational)", () => {
      const result = authorize({ group: ["assign"] });
      expect(result.success).toBe(false);
    });

    it("authorizes attendance read but not checkin", () => {
      const resultRead = bueroAdmin.authorize({ attendance: ["read"] });
      expect(resultRead.success).toBe(true);

      const resultCheckin = authorize({ attendance: ["checkin"] });
      expect(resultCheckin.success).toBe(false);
    });

    it("denies room access", () => {
      const result = authorize({ room: ["read"] });
      expect(result.success).toBe(false);
    });
  });
});

// ============================================
// TRAEGER ADMIN ROLE TESTS (GDPR RESTRICTED)
// ============================================

describe("Traeger Admin Role (GDPR Restricted)", () => {
  // Cast to AnyStatements for checking undefined properties
  const statements = traegerAdmin.statements as AnyStatements;
  // Wrap in arrow function to avoid unbound-method lint error
  const authorize: AnyAuthorize = (req, conn) => traegerAdmin.authorize(req as Parameters<typeof traegerAdmin.authorize>[0], conn);

  it("exports a traegerAdmin role", () => {
    expect(traegerAdmin).toBeDefined();
    expect(traegerAdmin.statements).toBeDefined();
    expect(typeof traegerAdmin.authorize).toBe("function");
  });

  describe("permissions", () => {
    it("has full student permissions", () => {
      expect(traegerAdmin.statements.student).toEqual(["read", "create", "update", "delete"]);
    });

    it("has group permissions WITHOUT assign (not operational)", () => {
      expect(traegerAdmin.statements.group).toEqual(["read", "create", "update", "delete"]);
      expect(traegerAdmin.statements.group).not.toContain("assign");
    });

    it("does NOT have room permissions", () => {
      expect(statements.room).toBeUndefined();
    });

    it("has attendance read-only permission", () => {
      expect(traegerAdmin.statements.attendance).toEqual(["read"]);
    });

    it("does NOT have location permission (GDPR CRITICAL)", () => {
      expect(statements.location).toBeUndefined();
    });

    it("has full staff permissions including delete", () => {
      expect(traegerAdmin.statements.staff).toEqual(["read", "create", "update", "delete", "invite"]);
    });

    it("has ogs read and update permissions", () => {
      expect(traegerAdmin.statements.ogs).toEqual(["read", "update"]);
    });

    it("has full config permissions", () => {
      expect(traegerAdmin.statements.config).toEqual(["read", "create", "update", "manage"]);
    });

    it("has full schedule permissions", () => {
      expect(traegerAdmin.statements.schedule).toEqual(["read", "create", "update", "delete"]);
    });

    it("has full feedback permissions", () => {
      expect(traegerAdmin.statements.feedback).toEqual(["read", "create", "delete"]);
    });

    it("has full substitution permissions", () => {
      expect(traegerAdmin.statements.substitution).toEqual(["read", "create", "update", "delete"]);
    });

    it("has full person permissions", () => {
      expect(traegerAdmin.statements.person).toEqual(["read", "create", "update", "delete"]);
    });
  });

  describe("authorize()", () => {
    it("authorizes full student CRUD", () => {
      const result = traegerAdmin.authorize({ student: ["read", "create", "update", "delete"] });
      expect(result.success).toBe(true);
    });

    it("DENIES location access (GDPR CRITICAL)", () => {
      const result = authorize({ location: ["read"] });
      expect(result.success).toBe(false);
      expect(result.error).toContain("location");
    });

    it("authorizes deleting staff", () => {
      const result = traegerAdmin.authorize({ staff: ["delete"] });
      expect(result.success).toBe(true);
    });

    it("denies group assignment (not operational)", () => {
      const result = authorize({ group: ["assign"] });
      expect(result.success).toBe(false);
    });

    it("authorizes attendance read but not checkin or checkout", () => {
      const resultRead = traegerAdmin.authorize({ attendance: ["read"] });
      expect(resultRead.success).toBe(true);

      const resultCheckin = authorize({ attendance: ["checkin"] });
      expect(resultCheckin.success).toBe(false);

      const resultCheckout = authorize({ attendance: ["checkout"] });
      expect(resultCheckout.success).toBe(false);
    });

    it("denies room access", () => {
      const result = authorize({ room: ["read"] });
      expect(result.success).toBe(false);
    });
  });
});

// ============================================
// GDPR COMPLIANCE TESTS
// ============================================

describe("GDPR Compliance", () => {
  // Cast to AnyStatements for checking undefined properties
  const bueroStatements = bueroAdmin.statements as AnyStatements;
  const traegerStatements = traegerAdmin.statements as AnyStatements;
  // Wrap in arrow functions to avoid unbound-method lint error
  const bueroAuthorize: AnyAuthorize = (req, conn) => bueroAdmin.authorize(req as Parameters<typeof bueroAdmin.authorize>[0], conn);
  const traegerAuthorize: AnyAuthorize = (req, conn) => traegerAdmin.authorize(req as Parameters<typeof traegerAdmin.authorize>[0], conn);

  describe("Location Permission Restrictions", () => {
    it("supervisor CAN access location (front-line operational need)", () => {
      expect(supervisor.statements.location).toEqual(["read"]);
      expect(supervisor.authorize({ location: ["read"] }).success).toBe(true);
    });

    it("ogsAdmin CAN access location (runs OGS operations)", () => {
      expect(ogsAdmin.statements.location).toEqual(["read"]);
      expect(ogsAdmin.authorize({ location: ["read"] }).success).toBe(true);
    });

    it("bueroAdmin CANNOT access location (manages remotely)", () => {
      expect(bueroStatements.location).toBeUndefined();
      expect(bueroAuthorize({ location: ["read"] }).success).toBe(false);
    });

    it("traegerAdmin CANNOT access location (highest admin, manages remotely)", () => {
      expect(traegerStatements.location).toBeUndefined();
      expect(traegerAuthorize({ location: ["read"] }).success).toBe(false);
    });
  });

  describe("Operational vs Administrative Permissions", () => {
    it("only operational roles have group:assign", () => {
      expect(supervisor.statements.group).not.toContain("assign");
      expect(ogsAdmin.statements.group).toContain("assign");
      expect(bueroAdmin.statements.group).not.toContain("assign");
      expect(traegerAdmin.statements.group).not.toContain("assign");
    });

    it("only operational roles have attendance:checkin/checkout", () => {
      expect(supervisor.statements.attendance).toContain("checkin");
      expect(supervisor.statements.attendance).toContain("checkout");
      expect(ogsAdmin.statements.attendance).toContain("checkin");
      expect(ogsAdmin.statements.attendance).toContain("checkout");
      expect(bueroAdmin.statements.attendance).not.toContain("checkin");
      expect(bueroAdmin.statements.attendance).not.toContain("checkout");
      expect(traegerAdmin.statements.attendance).not.toContain("checkin");
      expect(traegerAdmin.statements.attendance).not.toContain("checkout");
    });
  });
});

// ============================================
// ROLE HIERARCHY TESTS
// ============================================

describe("Role Hierarchy", () => {
  describe("supervisor < ogsAdmin", () => {
    it("ogsAdmin has all supervisor student permissions", () => {
      for (const action of supervisor.statements.student ?? []) {
        expect(ogsAdmin.statements.student).toContain(action);
      }
    });

    it("ogsAdmin has more student permissions than supervisor", () => {
      const supervisorActions = supervisor.statements.student?.length ?? 0;
      const ogsAdminActions = ogsAdmin.statements.student?.length ?? 0;
      expect(ogsAdminActions).toBeGreaterThan(supervisorActions);
    });
  });

  describe("ogsAdmin < bueroAdmin for staff management", () => {
    it("bueroAdmin can delete staff, ogsAdmin cannot", () => {
      expect(ogsAdmin.statements.staff).not.toContain("delete");
      expect(bueroAdmin.statements.staff).toContain("delete");
    });
  });

  describe("bueroAdmin = traegerAdmin for permissions", () => {
    it("traegerAdmin has same permissions as bueroAdmin", () => {
      // Both have same permissions, just different scope (traeger > buro)
      expect(traegerAdmin.statements.student).toEqual(bueroAdmin.statements.student);
      expect(traegerAdmin.statements.group).toEqual(bueroAdmin.statements.group);
      expect(traegerAdmin.statements.staff).toEqual(bueroAdmin.statements.staff);
      expect(traegerAdmin.statements.attendance).toEqual(bueroAdmin.statements.attendance);

      // Cast to check undefined properties
      const bueroStatements = bueroAdmin.statements as AnyStatements;
      const traegerStatements = traegerAdmin.statements as AnyStatements;
      expect(traegerStatements.location).toEqual(bueroStatements.location);
    });
  });
});

// ============================================
// AUTHORIZE CONNECTOR TESTS
// ============================================

describe("authorize() connector behavior", () => {
  // Wrap in arrow function to avoid unbound-method lint error
  const supervisorAuthorize: AnyAuthorize = (req, conn) => supervisor.authorize(req as Parameters<typeof supervisor.authorize>[0], conn);

  describe("AND connector (default)", () => {
    it("fails if any permission is missing", () => {
      const result = supervisorAuthorize({ student: ["read", "delete"] }, "AND");
      expect(result.success).toBe(false);
    });

    it("succeeds only if all permissions are present", () => {
      const result = supervisor.authorize({ student: ["read", "update"] }, "AND");
      expect(result.success).toBe(true);
    });
  });

  describe("OR connector", () => {
    it("succeeds if at least one resource permission passes", () => {
      // supervisor has student:read
      const result = supervisor.authorize({ student: ["read"] }, "OR");
      expect(result.success).toBe(true);
    });
  });

  describe("multiple resources", () => {
    it("fails with AND if one resource fails", () => {
      const result = supervisorAuthorize(
        {
          student: ["read"],
          room: ["read"], // supervisor doesn't have room access
        },
        "AND"
      );
      expect(result.success).toBe(false);
    });

    it("ogsAdmin can access multiple resources", () => {
      const result = ogsAdmin.authorize(
        {
          student: ["read"],
          room: ["read"],
          schedule: ["create"],
        },
        "AND"
      );
      expect(result.success).toBe(true);
    });
  });
});

// ============================================
// EDGE CASES
// ============================================

describe("Edge Cases", () => {
  // Wrap in arrow function to avoid unbound-method lint error
  const supervisorAuthorize: AnyAuthorize = (req, conn) => supervisor.authorize(req as Parameters<typeof supervisor.authorize>[0], conn);

  it("authorize() with empty request object", () => {
    // Empty request should not grant access (no permissions checked = no success)
    const result = supervisorAuthorize({});
    expect(result.success).toBe(false);
  });

  it("authorize() with non-existent resource", () => {
    const result = supervisorAuthorize({ nonExistent: ["read"] });
    expect(result.success).toBe(false);
    expect(result.error).toContain("nonExistent");
  });

  it("newRole can create additional custom roles", () => {
    const customRole = ac.newRole({
      student: ["read"],
      location: ["read"],
    });
    expect(customRole.statements.student).toEqual(["read"]);
    expect(customRole.statements.location).toEqual(["read"]);
    expect(customRole.authorize({ student: ["read"] }).success).toBe(true);
  });
});
