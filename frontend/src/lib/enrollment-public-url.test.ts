import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { buildEnrollmentPublicUrl } from "./enrollment-public-url";

const originalLocation = window.location;

function setLocation(origin: string, hostname: string) {
  Object.defineProperty(window, "location", {
    configurable: true,
    value: { origin, hostname },
  });
}

beforeEach(() => {
  setLocation("https://demo.localhost:3000", "demo.localhost");
});

afterEach(() => {
  Object.defineProperty(window, "location", {
    configurable: true,
    value: originalLocation,
  });
});

describe("buildEnrollmentPublicUrl", () => {
  it("builds the phase picker URL in subdomain mode", () => {
    expect(buildEnrollmentPublicUrl({ tenantSlug: "demo" })).toBe(
      "https://demo.localhost:3000/anmeldung",
    );
  });

  it("builds a per-phase URL in subdomain mode", () => {
    expect(
      buildEnrollmentPublicUrl({ tenantSlug: "demo", phaseId: "phase 1" }),
    ).toBe("https://demo.localhost:3000/anmeldung/phase%201");
  });

  it("keeps the tenant slug in path mode", () => {
    setLocation("https://localhost:3000", "localhost");
    expect(
      buildEnrollmentPublicUrl({ tenantSlug: "demo", phaseId: "phase 1" }),
    ).toBe("https://localhost:3000/demo/anmeldung/phase%201");
  });

  it("returns an empty string without a tenant slug", () => {
    expect(buildEnrollmentPublicUrl({ tenantSlug: null })).toBe("");
  });
});
