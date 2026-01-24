/**
 * Tests for auth components barrel export
 */

import { describe, it, expect } from "vitest";
import * as authExports from "./index";

describe("auth components index", () => {
  it("exports RolePermissionManagementModal", () => {
    expect(authExports.RolePermissionManagementModal).toBeDefined();
    expect(typeof authExports.RolePermissionManagementModal).toBe("function");
  });

  it("exports SignupForm", () => {
    expect(authExports.SignupForm).toBeDefined();
    expect(typeof authExports.SignupForm).toBe("function");
  });

  it("exports all expected components", () => {
    const exportedKeys = Object.keys(authExports);

    // Should have exactly these exports
    expect(exportedKeys).toContain("RolePermissionManagementModal");
    expect(exportedKeys).toContain("SignupForm");
    expect(exportedKeys.length).toBe(2);
  });
});
