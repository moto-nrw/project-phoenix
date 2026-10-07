import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchPlannerGroups, fetchPlannerRooms } from "./planner-reference-api";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("planner reference lists", () => {
  it("unwraps the envelope of a successful answer", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: [{ id: "1", name: "Mensa" }] }), {
        status: 200,
      }),
    ) as unknown as typeof fetch;

    await expect(fetchPlannerRooms()).resolves.toEqual([
      { id: "1", name: "Mensa" },
    ]);
  });

  it("keeps code and request ID of a failed answer", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "error",
          error: "boom",
          code: "general.server",
          instance: "req-3",
        }),
        { status: 500 },
      ),
    ) as unknown as typeof fetch;

    await expect(fetchPlannerGroups()).rejects.toMatchObject({
      name: "ApiError",
      status: 500,
      code: "general.server",
      requestId: "req-3",
    });
  });

  it("turns a request that never reached the API into general.unavailable", async () => {
    globalThis.fetch = vi
      .fn()
      .mockRejectedValue(
        new TypeError("Failed to fetch"),
      ) as unknown as typeof fetch;

    await expect(fetchPlannerRooms()).rejects.toMatchObject({
      code: "general.unavailable",
    });
  });
});
