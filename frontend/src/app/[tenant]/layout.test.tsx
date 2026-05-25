import { describe, expect, it, vi } from "vitest";

vi.mock("~/env", () => ({
  env: {
    NODE_ENV: "development",
    TENANT_DOMAIN: "localhost",
  },
}));

const { bareTenantHost } = await import("./layout");

describe("bareTenantHost", () => {
  it("strips an invalid local tenant subdomain and keeps the port", () => {
    expect(bareTenantHost("asld.localhost:3000")).toBe("localhost:3000");
  });

  it("keeps the bare tenant domain unchanged", () => {
    expect(bareTenantHost("localhost:3000")).toBe("localhost:3000");
  });
});
