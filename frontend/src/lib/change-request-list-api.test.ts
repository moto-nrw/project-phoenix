import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api-error";
import {
  bulkApproveParentRequests,
  ChangeRequestStaleError,
  fetchPendingEnrollmentChangeRequestCount,
  getFamilyProtection,
  listAggregatedOpenRequests,
  listAggregatedRequestHistory,
  listEnrollmentChangeRequests,
  markRequestDone,
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

  it("scopes the history to one child via the student filter", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { items: [] } }), { status: 200 }),
      );

    await expect(
      listAggregatedRequestHistory({ studentId: "42" }),
    ).resolves.toEqual({ items: [] });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/change-requests?view=history&student_id=42",
      { cache: "no-store" },
    );
  });

  it("throws an ApiError with the wire code when loading fails", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          error: "absence read required",
          code: "students.absence_read_required",
          instance: "req-403",
        }),
        { status: 403 },
      ),
    );

    const error = await listAggregatedOpenRequests().catch((err) => err);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 403,
      code: "students.absence_read_required",
      requestId: "req-403",
    });
  });

  it("keeps the status class when the error body carries no code", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({}), { status: 500 }),
    );

    await expect(listAggregatedOpenRequests()).rejects.toMatchObject({
      status: 500,
      code: "general.server",
    });
  });

  it("classifies non-JSON failures by status", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("upstream error", { status: 502 }),
    );

    await expect(listAggregatedRequestHistory()).rejects.toMatchObject({
      status: 502,
      code: "general.unavailable",
    });
  });

  it("turns a stale write into ChangeRequestStaleError with its code", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ code: "students.change_request_stale" }), {
        status: 409,
      }),
    );

    const error = (await markRequestDone("excused", "7", "v1").catch(
      (err: unknown) => err,
    )) as ApiError;
    expect(error).toBeInstanceOf(ChangeRequestStaleError);
    expect(error).toBeInstanceOf(ApiError);
    expect(error.code).toBe("students.change_request_stale");
  });

  it("keeps the code of any other failed write", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({ code: "students.bulk_approval_ineligible" }),
        { status: 409 },
      ),
    );

    await expect(
      bulkApproveParentRequests(
        [{ kind: "excused", id: "1", expected_version: "v" }],
        "",
      ),
    ).rejects.toMatchObject({
      status: 409,
      code: "students.bulk_approval_ineligible",
    });
  });

  it("throws an ApiError when the family protection cannot be loaded", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("nope", { status: 503 }),
    );

    await expect(getFamilyProtection("5")).rejects.toMatchObject({
      status: 503,
      code: "general.unavailable",
    });
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

  it("drops the student filter, which this endpoint does not know", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { items: [] } }), { status: 200 }),
      );

    await listEnrollmentChangeRequests("open", {
      studentId: "42",
      search: "Mia",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/enrollment/admin/change-requests/list?view=open&search=Mia",
      { cache: "no-store" },
    );
  });

  it("throws an ApiError when the list fails", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("nope", { status: 500 }),
    );

    const error = await listEnrollmentChangeRequests("open").catch(
      (err) => err,
    );
    expect(error).toBeInstanceOf(ApiError);
    expect(error.code).toBe("general.server");
  });
});
