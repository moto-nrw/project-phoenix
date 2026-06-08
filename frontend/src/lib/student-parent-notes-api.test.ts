import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

import {
  fetchStudentParentNotes,
  type StudentParentNote,
} from "./student-parent-notes-api";

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

function mkNote(body: string): StudentParentNote {
  return {
    id: "7",
    student_id: "84",
    body,
    created_at: "2026-06-01T09:00:00Z",
  };
}

describe("fetchStudentParentNotes", () => {
  it("GETs the student parent-notes route and returns the data array", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: [mkNote("Hallo Team")] });
    });
    const out = await fetchStudentParentNotes("84");
    expect(out).toHaveLength(1);
    expect(out[0]!.body).toBe("Hallo Team");
    expect(seenURL).toContain("/api/students/84/parent-notes");
  });

  it("returns an empty array when the envelope has no data", async () => {
    mockFetch(async () => jsonResponse({ status: "success" }));
    const out = await fetchStudentParentNotes("84");
    expect(out).toEqual([]);
  });

  it("throws a German error message on non-OK", async () => {
    mockFetch(async () => new Response("nope", { status: 500 }));
    await expect(fetchStudentParentNotes("84")).rejects.toThrow(
      /Elternnachrichten konnten nicht geladen werden/,
    );
  });
});
