import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  decideMasterDataChangeRequest,
  listMasterDataChangeRequestHistory,
  listMasterDataChangeRequests,
} from "./master-data-review-api";

const originalFetch = globalThis.fetch;

function mockFetch(
  impl: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>,
) {
  globalThis.fetch = vi.fn(impl) as typeof globalThis.fetch;
}

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

beforeEach(() => {
  globalThis.fetch = originalFetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("master-data review API", () => {
  it("lists pending change requests from the staff queue", async () => {
    let seenURL = "";
    let seenMethod = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenMethod = init?.method ?? "";
      return jsonResponse({
        data: [
          {
            id: "100",
            student_id: "42",
            first_name: "Lara",
            last_name: "Beispiel",
            target: "person",
            field_key: "birthday",
            old_value: "2018-01-01",
            new_value: "2018-01-02",
            status: "pending",
            created_at: "2026-06-24T12:00:00Z",
          },
        ],
      });
    });

    const out = await listMasterDataChangeRequests();

    expect(seenURL).toBe("/api/students/master-data-change-requests");
    expect(seenMethod).toBe("GET");
    expect(out).toHaveLength(1);
    expect(out[0]!.field_key).toBe("birthday");
  });

  it("posts approve decisions with an encoded request id", async () => {
    let seenURL = "";
    let seenMethod = "";
    let seenBody = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenMethod = init?.method ?? "";
      seenBody = String(init?.body ?? "");
      return jsonResponse({
        data: {
          id: "id/with space",
          student_id: "42",
          first_name: "Lara",
          last_name: "Beispiel",
          target: "person",
          field_key: "first_name",
          new_value: "Lea",
          status: "approved",
          created_at: "2026-06-24T12:00:00Z",
          reviewed_at: "2026-06-24T12:05:00Z",
        },
      });
    });

    const out = await decideMasterDataChangeRequest(
      "id/with space",
      true,
      "passt",
    );

    expect(seenURL).toBe(
      "/api/students/master-data-change-requests/id%2Fwith%20space/decide",
    );
    expect(seenMethod).toBe("POST");
    expect(seenBody).toBe(JSON.stringify({ approve: true, reason: "passt" }));
    expect(out.status).toBe("approved");
  });

  it("sends an empty reason for rejected decisions without a reason", async () => {
    let seenBody = "";
    mockFetch(async (_input, init) => {
      seenBody = String(init?.body ?? "");
      return jsonResponse({
        data: {
          id: "100",
          student_id: "42",
          first_name: "Lara",
          last_name: "Beispiel",
          target: "person",
          field_key: "last_name",
          new_value: "Muster",
          status: "rejected",
          created_at: "2026-06-24T12:00:00Z",
        },
      });
    });

    await decideMasterDataChangeRequest("100", false);

    expect(seenBody).toBe(JSON.stringify({ approve: false, reason: "" }));
  });

  it("throws backend-provided messages on failed requests", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "already decided" }, { status: 409 }),
    );

    await expect(decideMasterDataChangeRequest("100", true)).rejects.toThrow(
      /already decided/,
    );
  });

  it("falls back to a generic list error when the body is not JSON", async () => {
    mockFetch(async () => new Response("nope", { status: 500 }));

    await expect(listMasterDataChangeRequests()).rejects.toThrow(
      /Änderungsanfragen konnten nicht geladen werden/,
    );
  });

  it("lädt die Historie mit URL-kodiertem Cursor", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({
        data: { items: [], next_cursor: "abc" },
      });
    });

    const out = await listMasterDataChangeRequestHistory("cur+1");

    expect(seenURL).toBe(
      "/api/students/master-data-change-requests/history?cursor=cur%2B1",
    );
    expect(out.next_cursor).toBe("abc");
  });
});
