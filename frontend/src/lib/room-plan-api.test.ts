import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchRoomPlanBlocks } from "./room-plan-api";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("fetchRoomPlanBlocks (#4768)", () => {
  it("asks for the room's window and maps the blocks", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(200, {
        data: {
          instances: [
            {
              id: 41,
              date: "2026-09-09",
              start_time: "14:00",
              end_time: "15:00",
              title: "Kunst-AG",
              status: "cancelled",
              group_name: "Sonnengruppe",
              cancel_reason: "Krank",
              room_id: 3,
            },
            {
              id: 12,
              date: "2026-09-09",
              start_time: "12:00",
              end_time: "13:00",
              title: "Mittagessen",
              status: "planned",
              cancel_reason: null,
            },
          ],
        },
      }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const blocks = await fetchRoomPlanBlocks("3", "2026-09-07", "2026-09-11");

    const url = String(fetchMock.mock.calls[0]?.[0]);
    expect(url).toContain("/api/timetable/instances?");
    const query = new URLSearchParams(url.split("?")[1]);
    expect(Object.fromEntries(query)).toEqual({
      from: "2026-09-07",
      to: "2026-09-11",
      room_id: "3",
    });
    expect(blocks).toEqual([
      {
        id: "41",
        date: "2026-09-09",
        startTime: "14:00",
        endTime: "15:00",
        title: "Kunst-AG",
        status: "cancelled",
        groupName: "Sonnengruppe",
        cancelReason: "Krank",
      },
      {
        id: "12",
        date: "2026-09-09",
        startTime: "12:00",
        endTime: "13:00",
        title: "Mittagessen",
        status: "planned",
      },
    ]);
  });

  it("turns a failed request into an error", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(
        jsonResponse(403, { status: 403, code: "general.forbidden" }),
      ) as unknown as typeof fetch;

    await expect(
      fetchRoomPlanBlocks("3", "2026-09-07", "2026-09-11"),
    ).rejects.toMatchObject({ status: 403 });
  });
});
