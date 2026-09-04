import { afterEach, describe, expect, it, vi } from "vitest";
import { getServerApiUrl } from "./server-api-url";

describe("getServerApiUrl", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("returns the explicitly configured internal URL", () => {
    vi.stubEnv("API_URL", "http://server:8080");

    expect(getServerApiUrl()).toBe("http://server:8080");
  });

  it("fails fast when API_URL is missing", () => {
    vi.stubEnv("API_URL", "");

    expect(() => getServerApiUrl()).toThrow("API_URL is not set");
  });
});
