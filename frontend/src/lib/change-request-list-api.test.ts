import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  fetchPendingEnrollmentChangeRequestCount,
  listAggregatedOpenRequests,
  listAggregatedRequestHistory,
  listEnrollmentChangeRequests,
} from "./change-request-list-api";

describe("change request list API", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads open requests with trimmed search and type filters", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { items: [] } }), { status: 200 }),
      );

    await expect(
      listAggregatedOpenRequests({
        search: "  Emma  ",
        types: ["master_data", "excused"],
      }),
    ).resolves.toEqual({ items: [] });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/change-requests?view=open&search=Emma&types=master_data%2Cexcused",
      { cache: "no-store" },
    );
  });

  it("loads history with status, date, and cursor filters", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({ data: { items: [], next_cursor: "next" } }),
        {
          status: 200,
        },
      ),
    );

    await expect(
      listAggregatedRequestHistory({
        statuses: ["approved"],
        from: "2026-08-01",
        to: "2026-08-19",
        cursor: "current",
      }),
    ).resolves.toEqual({ items: [], next_cursor: "next" });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/change-requests?view=history&status=approved&from=2026-08-01&to=2026-08-19&cursor=current",
      { cache: "no-store" },
    );
  });

  it("uses the response error message when loading fails", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ error: "Keine Berechtigung" }), {
        status: 403,
      }),
    );

    await expect(listAggregatedOpenRequests()).rejects.toThrow(
      "Keine Berechtigung",
    );
  });

  it("keeps the generic message when the error body carries no text", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({}), { status: 500 }),
    );

    await expect(listAggregatedOpenRequests()).rejects.toThrow(
      "Anfragen konnten nicht geladen werden.",
    );
  });

  it("uses the generic message for non-JSON failures", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("upstream error", { status: 502 }),
    );

    await expect(listAggregatedRequestHistory()).rejects.toThrow(
      "Anfragen konnten nicht geladen werden.",
    );
  });
});

describe("pending enrollment change request count", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("reads the count from the envelope", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ data: { pending_count: 3 } }), {
        status: 200,
      }),
    );

    await expect(fetchPendingEnrollmentChangeRequestCount()).resolves.toBe(3);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/enrollment/admin/change-requests/pending-count",
      { cache: "no-store" },
    );
  });

  it("falls back to 0 when the envelope carries no count", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ data: {} }), { status: 200 }),
    );

    await expect(fetchPendingEnrollmentChangeRequestCount()).resolves.toBe(0);
  });

  it("falls back to 0 when the envelope has no data at all", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({}), { status: 200 }),
    );

    await expect(fetchPendingEnrollmentChangeRequestCount()).resolves.toBe(0);
  });

  it("falls back to 0 on an error response", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("nope", { status: 403 }),
    );

    await expect(fetchPendingEnrollmentChangeRequestCount()).resolves.toBe(0);
  });

  it("falls back to 0 when the request itself fails", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("offline"));

    await expect(fetchPendingEnrollmentChangeRequestCount()).resolves.toBe(0);
  });
});

describe("enrollment change request list", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("drops the type filter and keeps the remaining filters", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response(
          JSON.stringify({ data: { items: [], next_cursor: "next" } }),
          { status: 200 },
        ),
      );

    await expect(
      listEnrollmentChangeRequests("history", {
        types: ["enrollment"],
        search: "  Mia  ",
        statuses: ["approved", "rejected"],
        from: "2026-08-01",
        to: "2026-08-19",
        cursor: "current",
      }),
    ).resolves.toEqual({ items: [], next_cursor: "next" });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/enrollment/admin/change-requests/list?view=history&search=Mia&status=approved%2Crejected&from=2026-08-01&to=2026-08-19&cursor=current",
      { cache: "no-store" },
    );
  });

  it("omits empty filters on the open view", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { items: [] } }), { status: 200 }),
      );

    await expect(listEnrollmentChangeRequests("open")).resolves.toEqual({
      items: [],
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/enrollment/admin/change-requests/list?view=open",
      { cache: "no-store" },
    );
  });

  it("ignores a blank search and empty filter arrays", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { items: [] } }), { status: 200 }),
      );

    await listEnrollmentChangeRequests("open", {
      search: "   ",
      types: [],
      statuses: [],
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/enrollment/admin/change-requests/list?view=open",
      { cache: "no-store" },
    );
  });

  it("throws a German message when the list fails", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("nope", { status: 500 }),
    );

    await expect(listEnrollmentChangeRequests("open")).rejects.toThrow(
      "Anmeldungsänderungen konnten nicht geladen werden.",
    );
  });
});
