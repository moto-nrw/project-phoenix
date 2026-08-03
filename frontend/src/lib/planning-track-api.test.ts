import { beforeEach, describe, expect, it, vi } from "vitest";

const { sessionFetch } = vi.hoisted(() => ({ sessionFetch: vi.fn() }));

vi.mock("./session-cache", () => ({ sessionFetch }));

import { planningTrackService } from "./planning-track-api";

function response(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("planningTrackService", () => {
  beforeEach(() => vi.clearAllMocks());

  it("maps list results and keeps archived tracks available", async () => {
    sessionFetch.mockResolvedValueOnce(
      response({
        data: [
          { id: 4, name: "Früh", color: "#5080D8", sort_order: 0 },
          {
            id: 5,
            name: "Alt",
            color: "#83CD2D",
            sort_order: 1,
            archived_at: "2026-08-03T10:00:00Z",
          },
        ],
      }),
    );

    await expect(planningTrackService.list()).resolves.toEqual([
      {
        id: "4",
        name: "Früh",
        color: "#5080D8",
        sortOrder: 0,
        archivedAt: undefined,
      },
      {
        id: "5",
        name: "Alt",
        color: "#83CD2D",
        sortOrder: 1,
        archivedAt: "2026-08-03T10:00:00Z",
      },
    ]);
  });

  it("serializes the complete order as numeric ids", async () => {
    sessionFetch.mockResolvedValueOnce(response({ data: [] }));

    await planningTrackService.reorder(["12", "7"]);

    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/timetable/planning-tracks/order",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ ids: [12, 7] }),
      }),
    );
  });

  it("accepts an empty successful archive response from the delete proxy", async () => {
    sessionFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(planningTrackService.archive("5")).resolves.toBeUndefined();
    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/timetable/planning-tracks/5",
      { method: "DELETE" },
    );
  });
});
