import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api-error";
import { decideMasterDataChangeRequest } from "./master-data-review-api";

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

  it("throws an ApiError with the wire code on failed requests", async () => {
    mockFetch(async () =>
      jsonResponse(
        {
          error: "already decided",
          code: "students.change_request_not_pending",
          instance: "req-1",
        },
        { status: 409 },
      ),
    );

    const error = await decideMasterDataChangeRequest("100", true).catch(
      (err) => err,
    );
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 409,
      code: "students.change_request_not_pending",
      requestId: "req-1",
    });
  });

  it("classifies a non-JSON failure by status", async () => {
    mockFetch(async () => new Response("nope", { status: 500 }));

    await expect(
      decideMasterDataChangeRequest("100", true),
    ).rejects.toMatchObject({
      status: 500,
      code: "general.server",
    });
  });
});
