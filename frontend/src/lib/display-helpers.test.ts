import { describe, expect, it, vi, afterEach } from "vitest";
import { mapDisplayResponse, buildDisplayUrl } from "./display-helpers";

describe("mapDisplayResponse", () => {
  it("maps backend snake_case to camelCase with string id", () => {
    const mapped = mapDisplayResponse({
      id: 42,
      name: "Eingangsbereich",
      is_active: true,
      created_at: "2026-07-07T10:00:00Z",
      updated_at: "2026-07-07T10:00:00Z",
    });
    expect(mapped).toEqual({
      id: "42",
      name: "Eingangsbereich",
      isActive: true,
      createdAt: "2026-07-07T10:00:00Z",
    });
  });
});

describe("buildDisplayUrl", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stubLocation(hostname: string, origin: string) {
    vi.stubGlobal("window", {
      location: { hostname, origin },
    } as unknown as Window & typeof globalThis);
  }

  it("puts the token in the URL fragment in subdomain mode", () => {
    stubLocation("schule.moto-app.de", "https://schule.moto-app.de");
    expect(buildDisplayUrl("tok en", "schule")).toBe(
      "https://schule.moto-app.de/display#tok%20en",
    );
  });

  it("includes the tenant path segment in path mode", () => {
    stubLocation("localhost", "http://localhost:3000");
    expect(buildDisplayUrl("abc", "schule")).toBe(
      "http://localhost:3000/schule/display#abc",
    );
  });
});
