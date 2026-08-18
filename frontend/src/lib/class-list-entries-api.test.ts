// Tests for the Klassenlisteneinträge API client (#2382): wire mapping,
// trimming, and the German error-message passthrough.

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  assignClassListEntry,
  createClassListEntry,
  deleteClassListEntry,
  fetchClassListEntries,
  updateClassListEntry,
} from "./class-list-entries-api";

const mockFetch = vi.fn();

function jsonResponse(body: unknown, ok = true, status = 200) {
  return {
    ok,
    status,
    json: () => Promise.resolve(body),
  } as unknown as Response;
}

const wireEntry = {
  id: 12,
  first_name: "Zoe",
  last_name: "Aalders",
  school_class: "1a",
  created_at: "2026-08-18T10:00:00Z",
  matching_student_ids: [7, 8],
};

beforeEach(() => {
  mockFetch.mockReset();
  vi.stubGlobal("fetch", mockFetch);
});

describe("fetchClassListEntries", () => {
  it("maps the wire entries to string IDs", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: [wireEntry] }));

    const entries = await fetchClassListEntries();

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/class-list-entries",
      expect.objectContaining({ method: "GET", credentials: "include" }),
    );
    expect(entries).toEqual([
      {
        id: "12",
        firstName: "Zoe",
        lastName: "Aalders",
        schoolClass: "1a",
        createdAt: "2026-08-18T10:00:00Z",
        matchingStudentIds: ["7", "8"],
      },
    ]);
  });

  it("tolerates a missing data array and missing match hints", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({}));
    expect(await fetchClassListEntries()).toEqual([]);

    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: [{ ...wireEntry, matching_student_ids: undefined }],
      }),
    );
    const entries = await fetchClassListEntries();
    expect(entries[0]?.matchingStudentIds).toEqual([]);
  });

  it("surfaces the backend error field verbatim", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ error: "Ein Eintrag existiert bereits" }, false, 400),
    );
    await expect(fetchClassListEntries()).rejects.toThrow(
      "Ein Eintrag existiert bereits",
    );
  });

  it("falls back to message, then to the status code", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ message: "Nicht gefunden" }, false, 404),
    );
    await expect(fetchClassListEntries()).rejects.toThrow("Nicht gefunden");

    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error("no body")),
    } as unknown as Response);
    await expect(fetchClassListEntries()).rejects.toThrow("API error (500)");
  });
});

describe("createClassListEntry", () => {
  it("trims the input and maps the created entry", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: wireEntry }));

    const entry = await createClassListEntry({
      firstName: " Zoe ",
      lastName: " Aalders ",
      schoolClass: " 1a ",
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/class-list-entries",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          first_name: "Zoe",
          last_name: "Aalders",
          school_class: "1a",
        }),
      }),
    );
    expect(entry.id).toBe("12");
  });
});

describe("updateClassListEntry", () => {
  it("PUTs the trimmed input to the entry URL", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: wireEntry }));

    const entry = await updateClassListEntry("12", {
      firstName: "Zoe",
      lastName: "Aalders",
      schoolClass: "1b",
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/class-list-entries/12",
      expect.objectContaining({ method: "PUT" }),
    );
    expect(entry.firstName).toBe("Zoe");
  });
});

describe("deleteClassListEntry", () => {
  it("DELETEs the entry URL without a body", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: null }));

    await deleteClassListEntry("12");

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/class-list-entries/12",
      expect.objectContaining({ method: "DELETE" }),
    );
    const init = mockFetch.mock.calls[0]?.[1] as RequestInit;
    expect(init.body).toBeUndefined();
  });
});

describe("assignClassListEntry", () => {
  it("POSTs the numeric student ID to the assign URL", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: null }));

    await assignClassListEntry("12", "77");

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/class-list-entries/12/assign",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ student_id: 77 }),
      }),
    );
  });
});
