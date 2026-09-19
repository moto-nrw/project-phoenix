import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchPickupExtensions } from "./pickup-extension-api";

describe("fetchPickupExtensions", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("reads the unwrapped task list returned by the BFF", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            tasks: [
              {
                id: 7,
                student_id: 42,
                student_name: "Mia Beispiel",
                kind: "weekday",
                weekday: 2,
                effective_from: "2026-09-15",
                previous_pickup_time: "14:45",
                pickup_time: "16:00",
                blocks: [],
              },
            ],
          }),
        ),
      ),
    );

    await expect(fetchPickupExtensions()).resolves.toMatchObject([
      { effectiveFrom: "2026-09-15" },
    ]);
  });
});
