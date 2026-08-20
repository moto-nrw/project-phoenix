import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  listAggregatedOpenRequests,
  listAggregatedRequestHistory,
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

  it("uses the generic message for non-JSON failures", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("upstream error", { status: 502 }),
    );

    await expect(listAggregatedRequestHistory()).rejects.toThrow(
      "Anfragen konnten nicht geladen werden.",
    );
  });
});
