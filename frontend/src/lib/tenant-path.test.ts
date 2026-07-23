import { describe, expect, it } from "vitest";

import { normalizeTenantPathname, tenantAwarePath } from "./tenant-path";

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

describe("normalizeTenantPathname", () => {
  it("strips the tenant segment in path-routing mode", () => {
    expect(
      normalizeTenantPathname("/demo/betreuungsplan", "demo", "path"),
    ).toBe("/betreuungsplan");
    expect(normalizeTenantPathname("/demo", "demo", "path")).toBe("/");
  });

  it("does not strip a slug that is a real route in subdomain mode", () => {
    expect(
      normalizeTenantPathname("/messages/thread-1", "messages", "subdomain"),
    ).toBe("/messages/thread-1");
  });

  it("leaves unrelated path-mode routes unchanged", () => {
    expect(normalizeTenantPathname("/dashboard", "demo", "path")).toBe(
      "/dashboard",
    );
  });
});
