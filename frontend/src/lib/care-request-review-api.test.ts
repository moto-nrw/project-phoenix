import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import {
  decideCareScheduleChangeRequest,
  CareRequestApiError,
  type StaffCareRequest,
} from "./care-request-review-api";
import type { RequestDiffEntry } from "~/lib/messaging-status";

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

const originalFetch = globalThis.fetch;

function mockFetch(
  impl: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>,
) {
  globalThis.fetch = vi.fn(impl) as typeof globalThis.fetch;
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function mkDiff(overrides: Partial<RequestDiffEntry> = {}): RequestDiffEntry {
  return { label: "Montag Ankunft", old: "07:30", new: "08:00", ...overrides };
}

function mkRequest(
  overrides: Partial<StaffCareRequest> = {},
): StaffCareRequest {
  return {
    id: "r1",
    student_id: "42",
    first_name: "Max",
    last_name: "M.",
    status: "pending",
    request_kind: "weekly_schedule",
    diff: [mkDiff()],
    affected_blocks: [],
    impact_available: true,
    impact_token: "impact-v1",
    created_at: "2026-07-01T10:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  globalThis.fetch = originalFetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

// ---------------------------------------------------------------------------
// listCareScheduleChangeRequests
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// decideCareScheduleChangeRequest
// ---------------------------------------------------------------------------

describe("decideCareScheduleChangeRequest", () => {
  it("POSTs approve=true with an empty reason string by default", async () => {
    let seenURL = "";
    let seenBody = "";
    let seenMethod = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenBody = (init?.body as string) ?? "";
      seenMethod = init?.method ?? "";
      return jsonResponse({ data: mkRequest({ status: "approved" }) });
    });

    const out = await decideCareScheduleChangeRequest(
      "r1",
      true,
      undefined,
      "impact-v1",
    );

    expect(seenMethod).toBe("POST");
    expect(seenURL).toBe(
      "/api/students/care-schedule-change-requests/r1/decide",
    );
    expect(JSON.parse(seenBody)).toEqual({
      approve: true,
      reason: "",
      impact_token: "impact-v1",
    });
    expect(out.status).toBe("approved");
  });

  it("forwards the rejection reason verbatim", async () => {
    let seenBody = "";
    mockFetch(async (_input, init) => {
      seenBody = (init?.body as string) ?? "";
      return jsonResponse({ data: mkRequest({ status: "rejected" }) });
    });

    await decideCareScheduleChangeRequest(
      "r1",
      false,
      "Zeiten kollidieren",
      "impact-v1",
    );

    expect(JSON.parse(seenBody)).toEqual({
      approve: false,
      reason: "Zeiten kollidieren",
      impact_token: "impact-v1",
    });
  });

  it("URL-encodes the request id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: mkRequest() });
    });
    await decideCareScheduleChangeRequest(
      "a/b 1",
      true,
      undefined,
      "impact-v1",
    );
    expect(seenURL).toContain("a%2Fb%201");
  });

  it("surfaces the backend code so the UI can name the recovery action", async () => {
    mockFetch(async () =>
      jsonResponse(
        {
          error: "Die Anfrage ist nicht mehr offen",
          code: "change_request_not_pending",
        },
        409,
      ),
    );

    const err = await decideCareScheduleChangeRequest(
      "r1",
      true,
      undefined,
      "impact-v1",
    ).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(CareRequestApiError);
    expect((err as CareRequestApiError).code).toBe(
      "change_request_not_pending",
    );
  });

  it("falls back to the German decision message on a non-JSON error", async () => {
    mockFetch(async () => new Response("", { status: 500 }));
    const err = await decideCareScheduleChangeRequest(
      "r1",
      false,
      undefined,
      "impact-v1",
    ).catch((e: unknown) => e);
    expect((err as CareRequestApiError).message).toBe(
      "Entscheidung konnte nicht gespeichert werden",
    );
  });
});
