/**
 * Tests for Environment Configuration
 * Tests that env.js imports and validates correctly
 */
import { describe, it, expect } from "vitest";

describe("Environment Configuration", () => {
  it("imports env module without error", async () => {
    const { env } = await import("./env.js");

    expect(env).toBeDefined();
  });
});
