import { afterEach, describe, expect, it, vi } from "vitest";

const mockGetServerApiUrl = vi.hoisted(() => vi.fn(() => "http://server:8080"));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

import { resolveApiUrl } from "./api-url";

describe("resolveApiUrl", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    mockGetServerApiUrl.mockClear();
  });

  it("returns the proxy path without loading server configuration in a browser", async () => {
    await expect(resolveApiUrl("/api/students", "/students")).resolves.toBe(
      "/api/students",
    );
    expect(mockGetServerApiUrl).not.toHaveBeenCalled();
  });

  it("uses the internal API URL outside a browser", async () => {
    vi.stubGlobal("window", undefined);

    await expect(resolveApiUrl("/api/students", "/students")).resolves.toBe(
      "http://server:8080/students",
    );
    expect(mockGetServerApiUrl).toHaveBeenCalledOnce();
  });
});
