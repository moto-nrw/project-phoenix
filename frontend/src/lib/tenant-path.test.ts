import { describe, expect, it } from "vitest";

import {
  normalizeTenantPathname,
  resolveDetailReferrer,
  tenantAwarePath,
} from "./tenant-path";

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

describe("resolveDetailReferrer", () => {
  const fallback = "/students/search";
  const allowed = ["/students/search", "/database/students", "/rooms"];

  it("keeps an allowed collection path including its filters", () => {
    expect(
      resolveDetailReferrer(
        "/database/students?groupBy=group",
        fallback,
        allowed,
      ),
    ).toBe("/database/students?groupBy=group");
  });

  it("keeps an allowed detail path", () => {
    expect(resolveDetailReferrer("/rooms/7", fallback, allowed)).toBe(
      "/rooms/7",
    );
  });

  it.each(["//attacker.example", "/\\attacker.example", "/my-room"])(
    "falls back for an unapproved or external path %s",
    (candidate) => {
      expect(resolveDetailReferrer(candidate, fallback, allowed)).toBe(
        fallback,
      );
    },
  );

  it.each(["/rooms/../settings", "/rooms/%2e%2e/settings"])(
    "rejects a dot-segment escape from an allowed prefix: %s",
    (candidate) => {
      expect(resolveDetailReferrer(candidate, fallback, allowed)).toBe(
        fallback,
      );
    },
  );
});
