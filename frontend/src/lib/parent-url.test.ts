import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

// parent-url has a module-level _isParents cache. Each test sets the
// window.location.host + env BEFORE importing the module to control
// what gets cached on first call. vi.resetModules() guarantees a fresh
// module instance per test.

const ORIGINAL_HOSTNAME = process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;

function setLocation(host: string, protocol = "https:") {
  Object.defineProperty(window, "location", {
    writable: true,
    value: { host, protocol, origin: `${protocol}//${host}` },
  });
}

beforeEach(() => {
  vi.resetModules();
  process.env.NEXT_PUBLIC_PARENTS_HOSTNAME = "parents.localhost:3000";
});

afterEach(() => {
  vi.unstubAllGlobals();
  if (ORIGINAL_HOSTNAME !== undefined) {
    process.env.NEXT_PUBLIC_PARENTS_HOSTNAME = ORIGINAL_HOSTNAME;
  } else {
    delete process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
  }
});

// --- parentPath ---------------------------------------------------------

describe("parentPath on parents subdomain", () => {
  beforeEach(() => {
    setLocation("parents.localhost:3000");
  });

  it("strips the /parents prefix", async () => {
    const { parentPath } = await import("./parent-url");
    expect(parentPath("/parents/children/123")).toBe("/children/123");
  });

  it("returns '/' for the bare /parents path", async () => {
    const { parentPath } = await import("./parent-url");
    expect(parentPath("/parents")).toBe("/");
  });

  it("handles a non-prefixed path by stripping (no-op + or-fallback)", async () => {
    // The implementation calls path.replace(/^\/parents/, "") || "/".
    // For a path that already has no /parents prefix it just returns
    // the path unchanged.
    const { parentPath } = await import("./parent-url");
    expect(parentPath("/children/123")).toBe("/children/123");
  });
});

describe("parentPath on a non-parents host", () => {
  beforeEach(() => {
    setLocation("school.localhost:3000");
  });

  it("prefixes a bare path with /parents", async () => {
    const { parentPath } = await import("./parent-url");
    expect(parentPath("/children/123")).toBe("/parents/children/123");
  });

  it("does not double-prefix an already-prefixed path", async () => {
    const { parentPath } = await import("./parent-url");
    expect(parentPath("/parents/children/123")).toBe("/parents/children/123");
  });
});

describe("parentPath missing-env guard", () => {
  beforeEach(() => {
    setLocation("anything.localhost:3000");
    delete process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
  });

  it("throws a friendly error when NEXT_PUBLIC_PARENTS_HOSTNAME is unset", async () => {
    const { parentPath } = await import("./parent-url");
    expect(() => parentPath("/foo")).toThrow(/NEXT_PUBLIC_PARENTS_HOSTNAME/);
  });
});

// --- parentsPortalLoginUrl ----------------------------------------------

describe("parentsPortalLoginUrl", () => {
  it("points at the parents host, not the current one", async () => {
    setLocation("school.localhost:3000");
    const { parentsPortalLoginUrl } = await import("./parent-url");
    expect(parentsPortalLoginUrl()).toBe(
      "https://parents.localhost:3000/login",
    );
  });

  it("follows the scheme of the current page", async () => {
    setLocation("school.localhost:3000", "http:");
    const { parentsPortalLoginUrl } = await import("./parent-url");
    expect(parentsPortalLoginUrl()).toBe("http://parents.localhost:3000/login");
  });

  it("appends the search string when given", async () => {
    setLocation("school.localhost:3000");
    const { parentsPortalLoginUrl } = await import("./parent-url");
    expect(parentsPortalLoginUrl("?from=staff")).toBe(
      "https://parents.localhost:3000/login?from=staff",
    );
  });

  it("does not append a port — the env var already carries one", async () => {
    setLocation("school.moto-app.de");
    process.env.NEXT_PUBLIC_PARENTS_HOSTNAME = "eltern.moto-app.de";
    const { parentsPortalLoginUrl } = await import("./parent-url");
    expect(parentsPortalLoginUrl()).toBe("https://eltern.moto-app.de/login");
  });

  it("throws when NEXT_PUBLIC_PARENTS_HOSTNAME is unset", async () => {
    setLocation("school.localhost:3000");
    delete process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
    const { parentsPortalLoginUrl } = await import("./parent-url");
    expect(() => parentsPortalLoginUrl()).toThrow(
      /NEXT_PUBLIC_PARENTS_HOSTNAME/,
    );
  });

  it("throws when called on the server", async () => {
    setLocation("school.localhost:3000");
    const { parentsPortalLoginUrl } = await import("./parent-url");
    vi.stubGlobal("window", undefined);
    expect(() => parentsPortalLoginUrl()).toThrow(/client-only/);
  });
});

// --- parentAbsoluteUrl --------------------------------------------------

describe("parentAbsoluteUrl on parents subdomain", () => {
  beforeEach(() => {
    setLocation("parents.localhost:3000");
  });

  it("returns origin + clean (prefix-stripped) path", async () => {
    const { parentAbsoluteUrl } = await import("./parent-url");
    expect(parentAbsoluteUrl("/parents/children/123")).toBe(
      "https://parents.localhost:3000/children/123",
    );
  });
});

describe("parentAbsoluteUrl on a non-parents host", () => {
  beforeEach(() => {
    setLocation("school.localhost:3000");
  });

  it("returns origin + /parents prefix", async () => {
    const { parentAbsoluteUrl } = await import("./parent-url");
    expect(parentAbsoluteUrl("/children/123")).toBe(
      "https://school.localhost:3000/parents/children/123",
    );
  });

  it("preserves an already-prefixed /parents path", async () => {
    const { parentAbsoluteUrl } = await import("./parent-url");
    expect(parentAbsoluteUrl("/parents/login")).toBe(
      "https://school.localhost:3000/parents/login",
    );
  });
});

describe("parentAbsoluteUrl on the server", () => {
  it("throws instead of guessing an origin", async () => {
    setLocation("school.localhost:3000");
    const { parentAbsoluteUrl } = await import("./parent-url");
    vi.stubGlobal("window", undefined);
    expect(() => parentAbsoluteUrl("/children/123")).toThrow(/client-only/);
  });
});
