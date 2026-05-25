import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

// parent-url has a module-level _isParents cache. Each test sets the
// window.location.host + env BEFORE importing the module to control
// what gets cached on first call. vi.resetModules() guarantees a fresh
// module instance per test.

const ORIGINAL_HOSTNAME = process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;

function setLocation(host: string) {
  Object.defineProperty(window, "location", {
    writable: true,
    value: { host, origin: `https://${host}` },
  });
}

beforeEach(() => {
  vi.resetModules();
  process.env.NEXT_PUBLIC_PARENTS_HOSTNAME = "parents.localhost:3000";
});

afterEach(() => {
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

// The `typeof window === "undefined"` throw path for parentAbsoluteUrl
// can't be exercised under happy-dom (a window always exists). The
// branch is enforced by static analysis — the explicit throw makes the
// SSR-misuse failure mode loud and obvious.
