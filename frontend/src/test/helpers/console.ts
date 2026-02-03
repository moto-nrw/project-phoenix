import { afterEach, beforeEach, type MockInstance, vi } from "vitest";

type ConsoleMethod = "log" | "warn" | "error";

// eslint-disable-next-line @typescript-eslint/no-empty-function
const noop = () => {};

/**
 * Suppresses specified console methods during tests and automatically
 * restores them after each test. Returns the spies for assertion use.
 *
 * Usage:
 *   const spies = suppressConsole("error", "warn");
 *   // Later in test: expect(spies.error).toHaveBeenCalledWith(...)
 */
export function suppressConsole(...methods: ConsoleMethod[]) {
  const spies: Partial<Record<ConsoleMethod, MockInstance>> = {};

  beforeEach(() => {
    for (const method of methods) {
      spies[method] = vi.spyOn(console, method).mockImplementation(noop);
    }
  });

  afterEach(() => {
    for (const method of methods) {
      spies[method]?.mockRestore();
    }
  });

  return spies as Record<ConsoleMethod, MockInstance>;
}
