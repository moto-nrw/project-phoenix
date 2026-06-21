import { describe, expect, it } from "vitest";

import { tenantAwarePath } from "./tenant-path";

describe("tenantAwarePath", () => {
  it("prefixes the tenant slug in path mode", () => {
    expect(tenantAwarePath("/admin/enrollments", "demo", "path")).toBe(
      "/demo/admin/enrollments",
    );
  });

  it("keeps clean paths in subdomain mode", () => {
    expect(tenantAwarePath("/admin/enrollments", "demo", "subdomain")).toBe(
      "/admin/enrollments",
    );
  });

  it("normalizes paths without a leading slash", () => {
    expect(tenantAwarePath("dashboard", "demo", "path")).toBe(
      "/demo/dashboard",
    );
  });
});
