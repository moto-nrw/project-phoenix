import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  createAnnouncement,
  deleteAnnouncement,
  fetchAnnouncementRecipients,
  fetchAnnouncementStats,
  fetchAnnouncements,
  fetchPollChildren,
  fetchPollResults,
  isPoll,
  publishAnnouncement,
  remindUnanswered,
  unpublishAnnouncement,
  updateAnnouncement,
  type Announcement,
  type AnnouncementInput,
} from "./parent-announcements-api";

const originalFetch = globalThis.fetch;

function announcement(overrides: Partial<Announcement> = {}): Announcement {
  return {
    id: "7",
    title: "Sommerfest",
    body: "Wir feiern am Freitag.",
    priority: "info",
    requires_acknowledgement: false,
    send_email: false,
    status: "draft",
    active: true,
    created_at: "2026-07-01T08:00:00Z",
    updated_at: "2026-07-01T08:00:00Z",
    targets: [{ target_type: "school_all" }],
    // A plain Mitteilung: the backend always sends these two, polls override
    // them (#1371).
    response_type: "none",
    options: [],
    ...overrides,
  };
}

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

function mockFetch(
  impl: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>,
) {
  globalThis.fetch = vi.fn(impl) as typeof globalThis.fetch;
  return globalThis.fetch as ReturnType<typeof vi.fn>;
}

const validInput: AnnouncementInput = {
  title: "Sommerfest",
  body: "Wir feiern am Freitag.",
  priority: "important",
  link_url: "https://schule.example/fest",
  requires_acknowledgement: true,
  send_email: true,
  expires_at: "2026-07-31T00:00:00Z",
  targets: [
    { target_type: "class", ref_id: "3", ref_text: "Klasse 3b" },
    { target_type: "student", ref_id: "42" },
  ],
};

beforeEach(() => {
  globalThis.fetch = originalFetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  vi.restoreAllMocks();
});

describe("fetchAnnouncements", () => {
  it("requests the include_inactive variant by default and unwraps the envelope", async () => {
    let seenURL = "";
    let seenInit: RequestInit | undefined;
    const expected = [announcement(), announcement({ id: "8" })];
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenInit = init;
      return jsonResponse({ data: expected });
    });

    await expect(fetchAnnouncements()).resolves.toStrictEqual(expected);
    expect(seenURL).toBe("/api/parent-announcements?include_inactive=true");
    // A GET carries no init.
    expect(seenInit).toBeUndefined();
  });

  it("omits the query string when inactive announcements are excluded", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: [] });
    });

    await fetchAnnouncements(false);
    expect(seenURL).toBe("/api/parent-announcements");
  });

  it("returns an empty list when the backend omits the data field", async () => {
    mockFetch(async () => jsonResponse({}));
    await expect(fetchAnnouncements()).resolves.toEqual([]);
  });
});

describe("createAnnouncement", () => {
  it("POSTs the input as JSON and returns the created announcement", async () => {
    const created = announcement({ id: "99", status: "draft" });
    const fetchFn = mockFetch(async () => jsonResponse({ data: created }));

    await expect(createAnnouncement(validInput)).resolves.toStrictEqual(
      created,
    );

    const [url, init] = fetchFn.mock.calls[0]!;
    expect(url).toBe("/api/parent-announcements");
    expect(init).toMatchObject({
      method: "POST",
      headers: { "Content-Type": "application/json" },
    });
    expect(JSON.parse((init as RequestInit).body as string)).toEqual(
      validInput,
    );
  });

  it("throws the German fallback when the backend returns no data", async () => {
    mockFetch(async () => jsonResponse({ status: "ok" }));
    await expect(createAnnouncement(validInput)).rejects.toThrow(
      "Elternmitteilung konnte nicht erstellt werden",
    );
  });
});

describe("updateAnnouncement", () => {
  it("PUTs to the id-scoped path and encodes the id", async () => {
    const updated = announcement({ id: "a/b", title: "Neu" });
    const fetchFn = mockFetch(async () => jsonResponse({ data: updated }));

    await expect(updateAnnouncement("a/b", validInput)).resolves.toStrictEqual(
      updated,
    );

    const [url, init] = fetchFn.mock.calls[0]!;
    expect(url).toBe("/api/parent-announcements/a%2Fb");
    expect((init as RequestInit).method).toBe("PUT");
    expect(JSON.parse((init as RequestInit).body as string)).toEqual(
      validInput,
    );
  });

  it("throws the save fallback when data is missing", async () => {
    mockFetch(async () => jsonResponse({}));
    await expect(updateAnnouncement("7", validInput)).rejects.toThrow(
      "Elternmitteilung konnte nicht gespeichert werden",
    );
  });
});

describe("deleteAnnouncement", () => {
  it("sends a DELETE and resolves without a value on success", async () => {
    const fetchFn = mockFetch(async () => jsonResponse({ status: "ok" }));

    await expect(deleteAnnouncement("7")).resolves.toBeUndefined();
    const [url, init] = fetchFn.mock.calls[0]!;
    expect(url).toBe("/api/parent-announcements/7");
    expect((init as RequestInit).method).toBe("DELETE");
  });

  it("surfaces the backend error message on a failed delete", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "veröffentlichte Mitteilung" }, { status: 409 }),
    );
    await expect(deleteAnnouncement("7")).rejects.toThrow(
      "veröffentlichte Mitteilung",
    );
  });
});

describe("publishAnnouncement", () => {
  it("POSTs an empty body to the publish path", async () => {
    const published = announcement({ status: "published" });
    const fetchFn = mockFetch(async () => jsonResponse({ data: published }));

    await expect(publishAnnouncement("7")).resolves.toStrictEqual(published);
    const [url, init] = fetchFn.mock.calls[0]!;
    expect(url).toBe("/api/parent-announcements/7/publish");
    expect(init).toMatchObject({
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    });
  });

  it("throws the publish fallback when the backend returns no announcement", async () => {
    mockFetch(async () => jsonResponse({}));
    await expect(publishAnnouncement("7")).rejects.toThrow(
      "Elternmitteilung konnte nicht veröffentlicht werden",
    );
  });
});

describe("unpublishAnnouncement", () => {
  it("POSTs an empty body to the unpublish path", async () => {
    const retracted = announcement({ status: "draft" });
    const fetchFn = mockFetch(async () => jsonResponse({ data: retracted }));

    await expect(unpublishAnnouncement("7")).resolves.toStrictEqual(retracted);
    const [url, init] = fetchFn.mock.calls[0]!;
    expect(url).toBe("/api/parent-announcements/7/unpublish");
    expect((init as RequestInit).body).toBe("{}");
  });

  it("throws the retract fallback when data is missing", async () => {
    mockFetch(async () => jsonResponse({}));
    await expect(unpublishAnnouncement("7")).rejects.toThrow(
      "Elternmitteilung konnte nicht zurückgezogen werden",
    );
  });
});

describe("fetchAnnouncementStats", () => {
  it("returns the parsed stats payload", async () => {
    mockFetch(async () =>
      jsonResponse({
        data: { target_count: 12, read_count: 5, acknowledged_count: 2 },
      }),
    );

    await expect(fetchAnnouncementStats("7")).resolves.toEqual({
      target_count: 12,
      read_count: 5,
      acknowledged_count: 2,
    });
  });

  it("falls back to a zeroed stats object when data is absent", async () => {
    mockFetch(async () => jsonResponse({}));
    await expect(fetchAnnouncementStats("7")).resolves.toEqual({
      target_count: 0,
      read_count: 0,
      acknowledged_count: 0,
    });
  });
});

describe("fetchAnnouncementRecipients", () => {
  it("returns the recipient list from the envelope", async () => {
    const recipients = [
      {
        account_id: "1",
        first_name: "Ada",
        last_name: "Lovelace",
        status: "read" as const,
        read_at: "2026-07-02T09:00:00Z",
      },
    ];
    mockFetch(async () => jsonResponse({ data: recipients }));
    await expect(fetchAnnouncementRecipients("7")).resolves.toEqual(recipients);
  });

  it("returns an empty list when the backend omits recipients", async () => {
    mockFetch(async () => jsonResponse({}));
    await expect(fetchAnnouncementRecipients("7")).resolves.toEqual([]);
  });
});

describe("error handling", () => {
  it("prefers the backend error message over the German fallback", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "Titel fehlt" }, { status: 400 }),
    );
    await expect(fetchAnnouncements()).rejects.toThrow("Titel fehlt");
  });

  it("keeps the German fallback when the error body is not JSON", async () => {
    mockFetch(
      async () =>
        new Response("<html>502</html>", {
          status: 502,
          headers: { "Content-Type": "text/html" },
        }),
    );
    await expect(fetchAnnouncements()).rejects.toThrow(
      "Elternmitteilungen konnten nicht geladen werden",
    );
  });

  it("keeps the German fallback when the JSON error body has no error field", async () => {
    mockFetch(async () => jsonResponse({ message: "nope" }, { status: 500 }));
    await expect(fetchAnnouncementStats("7")).rejects.toThrow(
      "Statistik konnte nicht geladen werden",
    );
  });
});

// --- Umfragen (#1371) -------------------------------------------------------

describe("poll endpoints", () => {
  it("creates a poll with its options and deadline", async () => {
    let seenBody: unknown;
    mockFetch(async (_input, init) => {
      seenBody = JSON.parse(String(init?.body));
      return jsonResponse({
        data: announcement({
          response_type: "single_choice",
          options: [
            { id: "1", label: "Ja" },
            { id: "2", label: "Nein" },
          ],
        }),
      });
    });

    const created = await createAnnouncement({
      ...validInput,
      response_type: "single_choice",
      response_deadline: "2026-07-20T21:59:59Z",
      options: ["Ja", "Nein"],
    });

    expect(seenBody).toMatchObject({
      response_type: "single_choice",
      response_deadline: "2026-07-20T21:59:59Z",
      options: ["Ja", "Nein"],
    });
    expect(isPoll(created)).toBe(true);
  });

  it("treats an announcement without a response type as a plain Mitteilung", () => {
    expect(isPoll(announcement())).toBe(false);
  });

  it("fetches the tally and falls back to empty results", async () => {
    const expected = {
      child_count: 12,
      answered_count: 5,
      options: [{ option_id: "1", label: "Ja", count: 4 }],
    };
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: expected });
    });

    await expect(fetchPollResults("7")).resolves.toStrictEqual(expected);
    expect(seenURL).toBe("/api/parent-announcements/7/poll-results");

    mockFetch(async () => jsonResponse({}));
    await expect(fetchPollResults("7")).resolves.toStrictEqual({
      child_count: 0,
      answered_count: 0,
      options: [],
    });
  });

  it("fetches the per-child answer list", async () => {
    const expected = [
      {
        student_id: "10",
        first_name: "Felix",
        last_name: "Schneider",
        school_class: "1a",
        answer_labels: ["Ja"],
        responded_at: "2026-07-02T10:00:00Z",
      },
    ];
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: expected });
    });

    await expect(fetchPollChildren("7")).resolves.toStrictEqual(expected);
    expect(seenURL).toBe("/api/parent-announcements/7/poll-children");
  });

  it("POSTs the reminder and returns how many guardians it reached", async () => {
    let seenURL = "";
    let seenMethod: string | undefined;
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenMethod = init?.method;
      return jsonResponse({ data: { reminded_count: 8 } });
    });

    await expect(remindUnanswered("7")).resolves.toBe(8);
    expect(seenURL).toBe("/api/parent-announcements/7/remind");
    expect(seenMethod).toBe("POST");
  });

  it("surfaces the German fallback when the reminder fails", async () => {
    mockFetch(async () => jsonResponse({}, { status: 409 }));
    await expect(remindUnanswered("7")).rejects.toThrow(
      "Erinnerung konnte nicht gesendet werden",
    );
  });
});
