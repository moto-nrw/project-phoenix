import { beforeEach, describe, expect, it, vi } from "vitest";

const proxyPutMock = vi.hoisted(() => vi.fn());

vi.mock("~/lib/route-proxy.server", () => ({
  proxyPut: proxyPutMock,
}));

vi.mock("~/lib/route-wrapper-utils.server", () => ({
  requirePathSegmentParam: (
    params: Record<string, string>,
    key: string,
  ): string => params[key] ?? "",
}));

describe("staff shift move proxy route", () => {
  beforeEach(() => {
    vi.resetModules();
    proxyPutMock.mockReset();
    proxyPutMock.mockImplementation((pathBuilder) => pathBuilder);
  });

  it("proxies the shift id to the backend atomic move endpoint", async () => {
    const route = await import("./route");
    const pathBuilder = route.PUT as unknown as (
      params: Record<string, string>,
    ) => string;

    expect(pathBuilder({ shiftId: "42" })).toBe("/api/staff-shifts/42/move");
  });
});
