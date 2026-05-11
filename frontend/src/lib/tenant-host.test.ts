import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { tenantSlugFromHost } from "./tenant-host";

const ORIGINAL_TENANT_DOMAIN = process.env.TENANT_DOMAIN;

describe("tenantSlugFromHost", () => {
  beforeAll(() => {
    process.env.TENANT_DOMAIN = "localhost";
  });

  afterAll(() => {
    if (ORIGINAL_TENANT_DOMAIN === undefined) {
      delete process.env.TENANT_DOMAIN;
    } else {
      process.env.TENANT_DOMAIN = ORIGINAL_TENANT_DOMAIN;
    }
  });

  it("returns null for null host", () => {
    expect(tenantSlugFromHost(null)).toBeNull();
  });

  it("returns null for empty string host", () => {
    expect(tenantSlugFromHost("")).toBeNull();
  });

  it("returns null when TENANT_DOMAIN is unset", () => {
    const prev = process.env.TENANT_DOMAIN;
    delete process.env.TENANT_DOMAIN;
    try {
      expect(tenantSlugFromHost("school-a.localhost:3000")).toBeNull();
    } finally {
      process.env.TENANT_DOMAIN = prev;
    }
  });

  it("returns null for the bare tenant domain (no subdomain)", () => {
    expect(tenantSlugFromHost("localhost")).toBeNull();
    expect(tenantSlugFromHost("localhost:3000")).toBeNull();
  });

  it("returns null for a hostname unrelated to the tenant domain", () => {
    expect(tenantSlugFromHost("example.com")).toBeNull();
    expect(tenantSlugFromHost("school-a.example.com")).toBeNull();
  });

  it("extracts the slug from a valid subdomain", () => {
    expect(tenantSlugFromHost("school-a.localhost")).toBe("school-a");
  });

  it("strips the port before extracting the slug", () => {
    expect(tenantSlugFromHost("school-a.localhost:3000")).toBe("school-a");
  });

  it("returns null for reserved slugs", () => {
    expect(tenantSlugFromHost("operator.localhost:3000")).toBeNull();
    expect(tenantSlugFromHost("api.localhost")).toBeNull();
    expect(tenantSlugFromHost("www.localhost")).toBeNull();
  });

  it("handles a different TENANT_DOMAIN value", () => {
    const prev = process.env.TENANT_DOMAIN;
    process.env.TENANT_DOMAIN = "moto-app.de";
    try {
      expect(tenantSlugFromHost("school-a.moto-app.de")).toBe("school-a");
      expect(tenantSlugFromHost("school-a.moto-app.de:443")).toBe("school-a");
      expect(tenantSlugFromHost("moto-app.de")).toBeNull();
      expect(tenantSlugFromHost("operator.moto-app.de")).toBeNull();
    } finally {
      process.env.TENANT_DOMAIN = prev;
    }
  });
});
