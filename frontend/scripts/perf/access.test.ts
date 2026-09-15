import { afterEach, describe, expect, it, vi } from "vitest";

import { perfPort } from "./access";

describe("perfPort", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("returns a valid configured port", () => {
    vi.stubEnv("PERF_PORT", "3100");

    expect(perfPort()).toBe("3100");
  });

  it.each(["", "abc", "0", "65536"])("rejects the invalid port %s", (port) => {
    vi.stubEnv("PERF_PORT", port);

    expect(perfPort).toThrow(
      "PERF_PORT must be an integer between 1 and 65535 for the performance harness.",
    );
  });
});
