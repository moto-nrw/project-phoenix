import { describe, expect, it, vi } from "vitest";

/**
 * #5399: the session-end barrier comes with `<SessionEndBarrier />`, which
 * the tenant shell's protected layout and the other portals' providers
 * mount. Public tenant pages do not load it; there the fetch observer sends
 * unchanged.
 */
describe("SessionEndBarrier", () => {
  it("installs the barrier when its module loads, and only then", async () => {
    vi.resetModules();
    const state = await import("~/lib/session-end-state");
    expect(state.sessionEndBarrier()).toBeNull();

    await import("./session-end-barrier");

    const barrier = await import("~/lib/session-end-barrier");
    expect(state.sessionEndBarrier()).toBe(barrier);
  });

  it("is mounted by the tenant shell and every other portal", async () => {
    const { readFileSync } = await import("node:fs");
    for (const file of [
      "src/app/[tenant]/(protected)/layout.tsx",
      "src/app/parents/providers.tsx",
      "src/app/school/providers.tsx",
      "src/app/operator/providers.tsx",
      "src/app/caterer/providers.tsx",
      "src/app/traeger/providers.tsx",
    ]) {
      expect(readFileSync(file, "utf8"), file).toContain(
        "<SessionEndBarrier />",
      );
    }
  });
});
