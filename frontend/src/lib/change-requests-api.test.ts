import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import {
  fetchChangeRequestAccess,
  fetchPendingChangeRequestCount,
} from "./change-requests-api";

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

beforeEach(() => {
  vi.clearAllMocks();
  globalThis.fetch = originalFetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("fetchPendingChangeRequestCount", () => {
  it("GETs the pending-count proxy route with no-store", async () => {
    let seenURL = "";
    let seenInit: RequestInit | undefined;
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenInit = init;
      return jsonResponse({ data: { pending_count: 3 } });
    });

    const count = await fetchPendingChangeRequestCount();

    expect(seenURL).toBe("/api/students/change-requests/pending-count");
    expect(seenInit?.method).toBe("GET");
    expect(seenInit?.cache).toBe("no-store");
    expect(count).toBe(3);
  });

  it("returns the count from the {data} envelope", async () => {
    mockFetch(async () => jsonResponse({ data: { pending_count: 12 } }));
    expect(await fetchPendingChangeRequestCount()).toBe(12);
  });

  it("returns 0 when pending_count is absent from data", async () => {
    mockFetch(async () => jsonResponse({ data: {} }));
    expect(await fetchPendingChangeRequestCount()).toBe(0);
  });

  it("returns 0 when the envelope has no data key", async () => {
    mockFetch(async () => jsonResponse({}));
    expect(await fetchPendingChangeRequestCount()).toBe(0);
  });

  it("returns 0 (never throws) on a non-OK response, so the badge can't break the sidebar", async () => {
    mockFetch(async () => jsonResponse({ error: "verboten" }, 403));
    expect(await fetchPendingChangeRequestCount()).toBe(0);
  });

  it("returns 0 on a 500 with a non-JSON body", async () => {
    mockFetch(async () => new Response("boom", { status: 500 }));
    expect(await fetchPendingChangeRequestCount()).toBe(0);
  });
});

describe("fetchChangeRequestAccess", () => {
  it("liefert die serverseitig ausgewertete Elternanfragen-Freigabe", async () => {
    mockFetch(async () =>
      jsonResponse({ data: { review_access: "group_leader" } }),
    );

    await expect(fetchChangeRequestAccess()).resolves.toBe("group_leader");
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "/api/students/change-requests/access",
      expect.objectContaining({ method: "GET", cache: "no-store" }),
    );
  });
});
