import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchChildQuota } from "./child-quota-api";

function answer(status: number, body: unknown) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status })),
  );
}

describe("fetchChildQuota", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("reads the Kinderkontingent and the Kontingentzahl", async () => {
    answer(200, {
      data: { limited: true, booked_places: 50, occupied_places: 48 },
    });

    await expect(fetchChildQuota()).resolves.toEqual({
      booked: 50,
      occupied: 48,
    });
  });

  it("returns null for a school without Kinderkontingent", async () => {
    answer(200, { data: { limited: false } });

    await expect(fetchChildQuota()).resolves.toBeNull();
  });

  it("fails on a refused request so the line stays hidden", async () => {
    answer(403, { error: "forbidden" });

    await expect(fetchChildQuota()).rejects.toThrow("403");
  });
});
