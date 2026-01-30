import { describe, it, expect, vi } from "vitest";

// Mock next-auth
vi.mock("next-auth", () => ({
  default: vi.fn(() => ({
    auth: vi.fn(),
    handlers: { GET: vi.fn(), POST: vi.fn() },
    signIn: vi.fn(),
  })),
}));

// Mock react cache
vi.mock("react", () => ({
  cache: vi.fn((fn: unknown) => fn),
}));

// Mock the config
vi.mock("./config", () => ({
  authConfig: {
    providers: [],
    callbacks: {},
    pages: { signIn: "/" },
    session: { strategy: "jwt" as const },
  },
}));

describe("auth/index", () => {
  it("should export auth function", async () => {
    const { auth } = await import("./index");
    expect(auth).toBeDefined();
    expect(typeof auth).toBe("function");
  });

  it("should export handlers object", async () => {
    const { handlers } = await import("./index");
    expect(handlers).toBeDefined();
    expect(typeof handlers).toBe("object");
  });

  it("should export signIn function", async () => {
    const { signIn } = await import("./index");
    expect(signIn).toBeDefined();
    expect(typeof signIn).toBe("function");
  });

  it("should cache auth function with React.cache", async () => {
    const { cache } = await import("react");
    const { auth } = await import("./index");

    // Verify cache was called
    expect(cache).toHaveBeenCalled();

    // Verify auth is a function
    expect(typeof auth).toBe("function");
  });
});
