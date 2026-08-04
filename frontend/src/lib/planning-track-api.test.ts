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

  it("returns an empty list for a null payload", async () => {
    sessionFetch.mockResolvedValueOnce(response({ data: null }));

    await expect(planningTrackService.list()).resolves.toEqual([]);
  });

  it("uses the backend message when loading fails", async () => {
    sessionFetch.mockResolvedValueOnce(
      response({ error: "Lesen nicht erlaubt" }, 403),
    );

    await expect(planningTrackService.list()).rejects.toMatchObject({
      name: "PlanningTrackApiError",
      message: "Lesen nicht erlaubt",
      status: 403,
    });
  });

  it("creates and maps a planning track", async () => {
    sessionFetch.mockResolvedValueOnce(
      response({
        data: { id: 8, name: "Nord", color: "#5080D8", sort_order: 3 },
      }),
    );

    await expect(
      planningTrackService.create({
        name: "Nord",
        color: "#5080D8",
        sort_order: 3,
      }),
    ).resolves.toEqual({
      id: "8",
      name: "Nord",
      color: "#5080D8",
      sortOrder: 3,
      archivedAt: undefined,
    });
    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/timetable/planning-tracks",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          name: "Nord",
          color: "#5080D8",
          sort_order: 3,
        }),
      }),
    );
  });

  it("updates a planning track and reports a response fallback", async () => {
    sessionFetch
      .mockResolvedValueOnce(
        response({
          data: { id: 8, name: "Süd", color: "#83CD2D", sort_order: 4 },
        }),
      )
      .mockResolvedValueOnce(new Response("invalid", { status: 500 }));

    await expect(
      planningTrackService.update("8", {
        name: "Süd",
        color: "#83CD2D",
        sort_order: 4,
      }),
    ).resolves.toMatchObject({ id: "8", name: "Süd", sortOrder: 4 });
    await expect(
      planningTrackService.update("8", {
        name: "Süd",
        color: "#83CD2D",
        sort_order: 4,
      }),
    ).rejects.toMatchObject({
      message: "Planungsspur konnte nicht gespeichert werden",
      status: 500,
    });
  });

  it("serializes the complete order as numeric ids", async () => {
    sessionFetch.mockResolvedValueOnce(
      response({
        data: [{ id: 12, name: "Spät", color: "#5080D8", sort_order: 0 }],
      }),
    );

    await expect(planningTrackService.reorder(["12", "7"])).resolves.toEqual([
      {
        id: "12",
        name: "Spät",
        color: "#5080D8",
        sortOrder: 0,
        archivedAt: undefined,
      },
    ]);

    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/timetable/planning-tracks/order",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ ids: [12, 7] }),
      }),
    );
  });

  it("reports reorder failures", async () => {
    sessionFetch.mockResolvedValueOnce(response({}, 409));

    await expect(planningTrackService.reorder(["12"])).rejects.toMatchObject({
      message: "Reihenfolge konnte nicht gespeichert werden",
      status: 409,
    });
  });

  it("returns an empty order when the backend returns null", async () => {
    sessionFetch.mockResolvedValueOnce(response({ data: null }));

    await expect(planningTrackService.reorder([])).resolves.toEqual([]);
  });

  it("accepts an empty successful archive response from the delete proxy", async () => {
    sessionFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(planningTrackService.archive("5")).resolves.toBeUndefined();
    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/timetable/planning-tracks/5",
      { method: "DELETE" },
    );
  });

  it("reports archive failures", async () => {
    sessionFetch.mockResolvedValueOnce(
      new Response("invalid", { status: 502 }),
    );

    await expect(planningTrackService.archive("5")).rejects.toMatchObject({
      message: "Planungsspur konnte nicht archiviert werden",
      status: 502,
    });
  });

  it("restores and maps an archived planning track", async () => {
    sessionFetch.mockResolvedValueOnce(
      response({
        data: { id: 5, name: "Alt", color: "#83CD2D", sort_order: 1 },
      }),
    );

    await expect(planningTrackService.restore("5")).resolves.toMatchObject({
      id: "5",
      name: "Alt",
      archivedAt: undefined,
    });
    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/timetable/planning-tracks/5/restore",
      { method: "POST" },
    );
  });

  it("reports restore failures with the backend message", async () => {
    sessionFetch.mockResolvedValueOnce(
      response({ error: "Name bereits vergeben" }, 409),
    );

    await expect(planningTrackService.restore("5")).rejects.toMatchObject({
      message: "Name bereits vergeben",
      status: 409,
    });
  });
});
