import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const { mockGetRelationshipTypeLabel } = vi.hoisted(() => ({
  mockGetRelationshipTypeLabel: vi.fn((type: string) => `label:${type}`),
}));

vi.mock("~/lib/guardian-helpers", () => ({
  getRelationshipTypeLabel: mockGetRelationshipTypeLabel,
}));

import {
  relationshipLabel,
  fetchInboxWithFilters,
  fetchStudentThreads,
  fetchUnreadCount,
  fetchThread,
  postMessage,
  openThread,
  fetchGuardians,
  type InboxThread,
  type ThreadDetail,
  type Guardian,
} from "./parent-messages-api";

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

const originalFetch = globalThis.fetch;

function mockFetch(
  impl: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>,
) {
  globalThis.fetch = vi.fn(impl) as typeof globalThis.fetch;
}

function jsonOk(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function mkThread(overrides: Partial<InboxThread> = {}): InboxThread {
  return {
    thread_id: "t1",
    student_id: "42",
    student_name: "Max M.",
    guardian_name: "Anna M.",
    unread_count: 0,
    ...overrides,
  };
}

function mkDetail(overrides: Partial<ThreadDetail> = {}): ThreadDetail {
  return {
    thread_id: "t1",
    student_id: "42",
    student_name: "Max M.",
    guardian_name: "Anna M.",
    messages: [],
    ...overrides,
  };
}

function mkGuardian(overrides: Partial<Guardian> = {}): Guardian {
  return {
    account_id: "acc1",
    name: "Anna M.",
    is_primary: true,
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
// relationshipLabel
// ---------------------------------------------------------------------------

describe("relationshipLabel", () => {
  it("returns 'Bezugsperson' for undefined", () => {
    expect(relationshipLabel(undefined)).toBe("Bezugsperson");
  });

  it("returns 'Bezugsperson' for empty string", () => {
    expect(relationshipLabel("")).toBe("Bezugsperson");
  });

  it("delegates non-empty values to getRelationshipTypeLabel", () => {
    mockGetRelationshipTypeLabel.mockReturnValueOnce("Mutter");
    expect(relationshipLabel("parent")).toBe("Mutter");
    expect(mockGetRelationshipTypeLabel).toHaveBeenCalledWith("parent");
  });

  it("returns the helper's result verbatim for any known type", () => {
    mockGetRelationshipTypeLabel.mockReturnValueOnce("Elternteil");
    expect(relationshipLabel("guardian")).toBe("Elternteil");
  });
});

// ---------------------------------------------------------------------------
// fetchInboxWithFilters
// ---------------------------------------------------------------------------

describe("fetchInboxWithFilters", () => {
  it("GETs /api/messages without query params when onlyUnread is false", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonOk({ data: [mkThread()] });
    });
    const out = await fetchInboxWithFilters({ onlyUnread: false });
    expect(seenURL).toBe("/api/messages");
    expect(out).toHaveLength(1);
    expect(out[0]!.thread_id).toBe("t1");
  });

  it("appends ?unread=true when onlyUnread is true", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonOk({ data: [] });
    });
    await fetchInboxWithFilters({ onlyUnread: true });
    expect(seenURL).toBe("/api/messages?unread=true");
  });

  it("returns empty array when data field is missing", async () => {
    mockFetch(async () => jsonOk({ data: null }));
    const out = await fetchInboxWithFilters({});
    // null data falls through to ?? [] inside the wrapper
    expect(Array.isArray(out)).toBe(true);
  });

  it("throws with the backend error message on non-OK", async () => {
    mockFetch(async () => jsonOk({ error: "Messaging deaktiviert" }, 403));
    await expect(fetchInboxWithFilters({})).rejects.toThrow(
      /Messaging deaktiviert/,
    );
  });

  it("throws with German fallback when the response body is not JSON", async () => {
    mockFetch(async () => new Response("not json", { status: 500 }));
    await expect(fetchInboxWithFilters({})).rejects.toThrow(
      /Nachrichten konnten nicht geladen werden/,
    );
  });

  it("returns multiple threads", async () => {
    mockFetch(async () =>
      jsonOk({
        data: [mkThread({ thread_id: "t1" }), mkThread({ thread_id: "t2" })],
      }),
    );
    const out = await fetchInboxWithFilters({});
    expect(out).toHaveLength(2);
    expect(out[1]!.thread_id).toBe("t2");
  });
});

// ---------------------------------------------------------------------------
// fetchStudentThreads
// ---------------------------------------------------------------------------

describe("fetchStudentThreads", () => {
  it("GETs /api/messages/students/{id}/threads", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonOk({ data: [mkThread()] });
    });
    const out = await fetchStudentThreads("99");
    expect(seenURL).toBe("/api/messages/students/99/threads");
    expect(out).toHaveLength(1);
  });

  it("URL-encodes the studentId", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonOk({ data: [] });
    });
    await fetchStudentThreads("a/b");
    expect(seenURL).toContain("a%2Fb");
  });

  it("returns empty array when data is null", async () => {
    mockFetch(async () => jsonOk({ data: null }));
    const out = await fetchStudentThreads("99");
    expect(Array.isArray(out)).toBe(true);
  });

  it("throws with backend error on non-OK", async () => {
    mockFetch(async () => jsonOk({ error: "verboten" }, 403));
    await expect(fetchStudentThreads("99")).rejects.toThrow(/verboten/);
  });

  it("throws with German fallback when body is not JSON", async () => {
    mockFetch(async () => new Response("", { status: 503 }));
    await expect(fetchStudentThreads("99")).rejects.toThrow(
      /Nachrichten konnten nicht geladen werden/,
    );
  });
});

// ---------------------------------------------------------------------------
// fetchUnreadCount
// ---------------------------------------------------------------------------

describe("fetchUnreadCount", () => {
  it("GETs /api/messages/unread-count and returns the count", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonOk({ data: { unread_count: 5 } });
    });
    const count = await fetchUnreadCount();
    expect(count).toBe(5);
    expect(seenURL).toBe("/api/messages/unread-count");
  });

  it("returns 0 when data.unread_count is missing", async () => {
    mockFetch(async () => jsonOk({ data: {} }));
    const count = await fetchUnreadCount();
    expect(count).toBe(0);
  });

  it("returns 0 when data is missing entirely", async () => {
    mockFetch(async () => jsonOk({}));
    const count = await fetchUnreadCount();
    expect(count).toBe(0);
  });

  it("throws with backend error on non-OK", async () => {
    mockFetch(async () => jsonOk({ error: "Keine Berechtigung" }, 401));
    await expect(fetchUnreadCount()).rejects.toThrow(/Keine Berechtigung/);
  });

  it("throws with German fallback on non-JSON error", async () => {
    mockFetch(async () => new Response("", { status: 500 }));
    await expect(fetchUnreadCount()).rejects.toThrow(
      /Ungelesene Nachrichten konnten nicht geladen werden/,
    );
  });
});

// ---------------------------------------------------------------------------
// fetchThread
// ---------------------------------------------------------------------------

describe("fetchThread", () => {
  it("GETs /api/messages/threads/{id} and returns the ThreadDetail", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonOk({ data: mkDetail() });
    });
    const detail = await fetchThread("t1");
    expect(detail.thread_id).toBe("t1");
    expect(seenURL).toBe("/api/messages/threads/t1");
  });

  it("throws when data is missing in the envelope", async () => {
    mockFetch(async () => jsonOk({}));
    await expect(fetchThread("t1")).rejects.toThrow(
      /Nachrichtenverlauf konnte nicht geladen werden/,
    );
  });

  it("throws with backend error on non-OK", async () => {
    mockFetch(async () => jsonOk({ error: "Thread nicht gefunden" }, 404));
    await expect(fetchThread("t1")).rejects.toThrow(/Thread nicht gefunden/);
  });

  it("throws with German fallback when response is not JSON", async () => {
    mockFetch(async () => new Response("", { status: 500 }));
    await expect(fetchThread("t1")).rejects.toThrow(
      /Nachrichtenverlauf konnte nicht geladen werden/,
    );
  });

  it("includes messages in the returned detail", async () => {
    const msg = {
      id: "m1",
      sender_kind: "guardian" as const,
      sender_name: "Anna",
      body: "Hallo",
      created_at: "2026-01-01T10:00:00Z",
    };
    mockFetch(async () => jsonOk({ data: mkDetail({ messages: [msg] }) }));
    const detail = await fetchThread("t1");
    expect(detail.messages).toHaveLength(1);
    expect(detail.messages[0]!.body).toBe("Hallo");
  });
});

// ---------------------------------------------------------------------------
// postMessage
// ---------------------------------------------------------------------------

describe("postMessage", () => {
  it("POSTs body to /api/messages/threads/{id} and returns message list", async () => {
    let seenURL = "";
    let seenBody = "";
    let seenMethod = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenBody = (init?.body as string) ?? "";
      seenMethod = init?.method ?? "";
      return jsonOk({ data: [] });
    });
    const msgs = await postMessage("t1", "Guten Morgen!", "17");
    expect(seenURL).toBe("/api/messages/threads/t1");
    expect(seenMethod).toBe("POST");
    expect(seenBody).toContain('"body":"Guten Morgen!"');
    expect(seenBody).toContain('"handled_up_to_message_id":"17"');
    expect(Array.isArray(msgs)).toBe(true);
  });

  it("sets Content-Type: application/json header", async () => {
    let seenHeaders: HeadersInit | undefined;
    mockFetch(async (_input, init) => {
      seenHeaders = init?.headers;
      return jsonOk({ data: [] });
    });
    await postMessage("t1", "hi");
    expect((seenHeaders as Record<string, string>)["Content-Type"]).toBe(
      "application/json",
    );
  });

  it("returns empty array when data is missing", async () => {
    mockFetch(async () => jsonOk({}));
    const msgs = await postMessage("t1", "hi");
    expect(msgs).toEqual([]);
  });

  it("throws with backend error on non-OK", async () => {
    mockFetch(async () => jsonOk({ error: "Nachricht zu lang" }, 422));
    await expect(postMessage("t1", "hi")).rejects.toThrow(/Nachricht zu lang/);
  });

  it("throws with German fallback on non-JSON error", async () => {
    mockFetch(async () => new Response("", { status: 500 }));
    await expect(postMessage("t1", "hi")).rejects.toThrow(
      /Nachricht konnte nicht gesendet werden/,
    );
  });
});

// ---------------------------------------------------------------------------
// openThread
// ---------------------------------------------------------------------------

describe("openThread", () => {
  it("POSTs to /api/messages/threads/open and returns the ThreadDetail", async () => {
    let seenURL = "";
    let seenBody = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenBody = (init?.body as string) ?? "";
      return jsonOk({ data: mkDetail({ thread_id: "t42" }) });
    });
    const detail = await openThread({
      studentId: "42",
      guardianAccountId: "acc1",
    });
    expect(detail.thread_id).toBe("t42");
    expect(seenURL).toBe("/api/messages/threads/open");
    expect(seenBody).toContain('"student_id":"42"');
    expect(seenBody).toContain('"guardian_account_id":"acc1"');
  });

  it("throws when data is missing in the envelope", async () => {
    mockFetch(async () => jsonOk({}));
    await expect(
      openThread({ studentId: "42", guardianAccountId: "acc1" }),
    ).rejects.toThrow(/Unterhaltung konnte nicht geöffnet werden/);
  });

  it("throws with backend error on non-OK", async () => {
    mockFetch(async () => jsonOk({ error: "Schüler nicht gefunden" }, 404));
    await expect(
      openThread({ studentId: "99", guardianAccountId: "acc1" }),
    ).rejects.toThrow(/Schüler nicht gefunden/);
  });

  it("throws with German fallback on non-JSON body", async () => {
    mockFetch(async () => new Response("", { status: 500 }));
    await expect(
      openThread({ studentId: "42", guardianAccountId: "acc1" }),
    ).rejects.toThrow(/Unterhaltung konnte nicht geöffnet werden/);
  });
});

// ---------------------------------------------------------------------------
// fetchGuardians
// ---------------------------------------------------------------------------

describe("fetchGuardians", () => {
  it("GETs /api/messages/students/{id}/guardians and returns guardian list", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonOk({ data: [mkGuardian()] });
    });
    const out = await fetchGuardians("42");
    expect(seenURL).toBe("/api/messages/students/42/guardians");
    expect(out).toHaveLength(1);
    expect(out[0]!.account_id).toBe("acc1");
  });

  it("returns empty array when data is missing", async () => {
    mockFetch(async () => jsonOk({ data: null }));
    const out = await fetchGuardians("42");
    expect(Array.isArray(out)).toBe(true);
  });

  it("throws with backend error on non-OK", async () => {
    mockFetch(async () => jsonOk({ error: "kein Zugriff" }, 403));
    await expect(fetchGuardians("42")).rejects.toThrow(/kein Zugriff/);
  });

  it("throws with German fallback on non-JSON body", async () => {
    mockFetch(async () => new Response("", { status: 500 }));
    await expect(fetchGuardians("42")).rejects.toThrow(
      /Bezugspersonen konnten nicht geladen werden/,
    );
  });

  it("includes relationship_type in returned guardians", async () => {
    mockFetch(async () =>
      jsonOk({
        data: [mkGuardian({ relationship_type: "parent", is_primary: false })],
      }),
    );
    const out = await fetchGuardians("42");
    expect(out[0]!.relationship_type).toBe("parent");
    expect(out[0]!.is_primary).toBe(false);
  });
});

// #1803 removed the inbox "offene Anfragen" filter (open_requests param +
// open_request_count field) and the staff confirmRequest/rejectRequest decision
// endpoints: change requests are now decided on the Änderungsanfragen admin page
// (care-request-review-api), not inline in the chat. Their tests were removed
// with them.
