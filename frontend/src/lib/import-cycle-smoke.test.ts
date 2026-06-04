import { describe, expect, it } from "vitest";

describe("frontend API module imports", () => {
  it("loads the former circular dependency modules", async () => {
    await expect(import("./api")).resolves.toBeDefined();
    await expect(import("./api-helpers")).resolves.toBeDefined();
    await expect(import("./auth-api")).resolves.toBeDefined();
    await expect(import("./auth-service")).resolves.toBeDefined();
  });
});
