import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchPickupExtensions } from "./pickup-extension-api";

// Shape the BFF route actually sends: proxyGet unwraps the backend envelope
// and the GET wrapper puts its own around the result (#3776). An earlier
// fixture without this envelope hid that the browser always read no tasks.
function bffResponse(tasks: unknown[]): Response {
  return new Response(
    JSON.stringify({ success: true, message: "Success", data: { tasks } }),
  );
}

describe("fetchPickupExtensions", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("reads the task list from the BFF envelope", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        bffResponse([
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
        ]),
      ),
    );

    await expect(fetchPickupExtensions()).resolves.toMatchObject([
      { effectiveFrom: "2026-09-15" },
    ]);
  });

  it("maps a day task with its blocks for one child", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      bffResponse([
        {
          id: 3,
          student_id: 19,
          student_name: "David Krause",
          kind: "day",
          date: "2026-10-07",
          previous_pickup_time: "14:00",
          pickup_time: "17:00",
          blocks: [
            {
              id: 29,
              title: "Freies Spiel",
              start_time: "14:45",
              end_time: "17:00",
            },
          ],
        },
      ]),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(fetchPickupExtensions("19")).resolves.toEqual([
      {
        id: "3",
        studentId: "19",
        studentName: "David Krause",
        kind: "day",
        date: "2026-10-07",
        previousPickupTime: "14:00",
        pickupTime: "17:00",
        blocks: [
          {
            id: "29",
            title: "Freies Spiel",
            startTime: "14:45",
            endTime: "17:00",
          },
        ],
      },
    ]);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/pickup-extensions?student_id=19",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("returns no tasks when the BFF sends none", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(bffResponse([])));

    await expect(fetchPickupExtensions()).resolves.toEqual([]);
  });
});
