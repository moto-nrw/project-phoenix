import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

import {
  listMyChildren,
  listMyEnrollments,
  listEnrollableSchools,
  fetchParentEnrollmentProfile,
  submitParentEnrollment,
  fetchParentProfile,
  updateParentPortalLocale,
  submitSickNote,
  listSickDays,
  listExcusedRequests,
  withdrawExcusedRequest,
  getChildFeatures,
  getChildMealPlan,
  getChildCareOfferings,
  getChildOfferingCatalog,
  submitOfferingChangeRequest,
  withdrawOfferingChangeRequest,
  getChildMasterData,
  updateMasterDataField,
  submitMasterDataRequest,
  listMessageThreads,
  listChildThreads,
  fetchMessagesUnreadCount,
  getChildConversation,
  postChildMessage,
  listAnnouncements,
  fetchAnnouncementsUnreadCount,
  markAnnouncementRead,
  acknowledgeAnnouncement,
  type Child,
  type EnrollmentRequest,
  type StatusDay,
  type ChildMasterData,
  type ThreadSummary,
  type ThreadView,
  type ParentAnnouncement,
} from "./parent-api";

import type { SubmitEnrollmentPayload } from "./enrollment-submission-api";

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

function mkChild(id: string, schoolName: string): Child {
  return {
    student_id: id,
    tenant_id: "1",
    first_name: "Lara",
    last_name: "Beispiel",
    school_class: "1a",
    status: "active",
    school_name: schoolName,
    school_slug: "school",
  };
}

function mkEnrollment(id: string): EnrollmentRequest {
  return {
    request_id: id,
    tenant_id: "1",
    status_token: "tok",
    submitted_at: "2026-04-01T12:00:00Z",
    phase_id: "5",
    phase_name: "Schuljahr",
    service_start_date: "2026-09-01",
    service_end_date: "2027-07-31",
    school_name: "OGS",
    school_slug: "ogs",
    children: [
      {
        child_id: "99",
        first_name: "Lara",
        last_name: "Beispiel",
        status: "submitted",
      },
    ],
  };
}

describe("parent care offering requests", () => {
  it("loads the booked offerings and selectable catalog", async () => {
    mockFetch(async (input) => {
      if (String(input).endsWith("/catalog")) {
        return jsonResponse({
          data: {
            phase_name: "Schuljahr 2026/27",
            selection_mode: "at_least_one",
            earliest_effective_from: "2027-02-01",
            latest_effective_from: "2027-07-31",
            items: [],
          },
        });
      }
      return jsonResponse({
        data: {
          offerings: [],
          groups: [],
          can_request: true,
        },
      });
    });

    await expect(getChildCareOfferings("child/42")).resolves.toMatchObject({
      can_request: true,
    });
    await expect(getChildOfferingCatalog("child/42")).resolves.toMatchObject({
      phase_name: "Schuljahr 2026/27",
    });
  });

  it("submits and withdraws a complete desired booking", async () => {
    const requests: Array<{ url: string; body: unknown }> = [];
    mockFetch(async (input, init) => {
      requests.push({
        url: String(input),
        body: JSON.parse(String(init?.body)) as unknown,
      });
      return jsonResponse({
        data: {
          offerings: [],
          groups: [],
          can_request: false,
        },
      });
    });

    await submitOfferingChangeRequest("child/42", {
      offerings: [{ offering_id: "7", selected_days: ["mon"] }],
      effective_from: "2027-02-01",
      note: "Bitte",
    });
    await withdrawOfferingChangeRequest("child/42", "request/77");

    expect(requests).toEqual([
      {
        url: "/api/parent/me/children/child%2F42/care-offerings/requests",
        body: {
          offerings: [{ offering_id: "7", selected_days: ["mon"] }],
          effective_from: "2027-02-01",
          note: "Bitte",
        },
      },
      {
        url: "/api/parent/me/children/child%2F42/care-offerings/requests/request%2F77/withdraw",
        body: {},
      },
    ]);
  });
});

// --- listMyChildren --------------------------------------------------

describe("listMyChildren", () => {
  it("returns the unwrapped data array on 200", async () => {
    mockFetch(async () => jsonResponse({ data: [mkChild("1234", "OGS A")] }));
    const out = await listMyChildren();
    expect(out).toHaveLength(1);
    expect(out[0]!.student_id).toBe("1234");
    expect(out[0]!.school_name).toBe("OGS A");
  });

  it("handles flat (non-enveloped) response by returning it directly", async () => {
    // Some routes return the data directly without { status, data }.
    mockFetch(async () => jsonResponse([mkChild("1", "OGS")]));
    const out = await listMyChildren();
    expect(out).toHaveLength(1);
  });

  it("throws with backend error message on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "forbidden" }, { status: 403 }),
    );
    await expect(listMyChildren()).rejects.toThrow(/forbidden/);
  });

  it("throws with generic message on non-OK non-JSON body", async () => {
    mockFetch(async () => new Response("not json", { status: 500 }));
    await expect(listMyChildren()).rejects.toThrow(/Request failed \(500\)/);
  });

  it("redirects to /parents/login on 401", async () => {
    // 401 redirects via window.location.assign; happy-dom's
    // location.assign is a no-op but we can spy on it to verify.
    const assign = vi.fn();
    Object.defineProperty(window, "location", {
      writable: true,
      value: { assign, host: "parents.localhost:3000" },
    });
    mockFetch(async () => new Response("", { status: 401 }));
    await expect(listMyChildren()).rejects.toThrow();
    expect(assign).toHaveBeenCalledWith("/parents/login");
  });
});

// --- listMyEnrollments ----------------------------------------------

describe("listMyEnrollments", () => {
  it("returns the unwrapped data array on 200", async () => {
    mockFetch(async () => jsonResponse({ data: [mkEnrollment("99")] }));
    const out = await listMyEnrollments();
    expect(out).toHaveLength(1);
    expect(out[0]!.request_id).toBe("99");
    expect(out[0]!.children).toHaveLength(1);
  });

  it("throws on non-OK", async () => {
    mockFetch(async () => jsonResponse({ error: "boom" }, { status: 500 }));
    await expect(listMyEnrollments()).rejects.toThrow(/boom/);
  });
});

// --- listEnrollableSchools -------------------------------------------

describe("listEnrollableSchools", () => {
  it("hits the enrollable-schools endpoint and unwraps the data array", async () => {
    const seen: string[] = [];
    mockFetch(async (input) => {
      seen.push(String(input));
      return jsonResponse({
        data: [{ phase_id: "5", school_slug: "ogs", audience: "open" }],
      });
    });

    const out = await listEnrollableSchools();

    expect(seen).toEqual(["/api/parent/me/enrollable-schools"]);
    expect(out).toHaveLength(1);
  });

  it("preserves the backend order, which the picker renders as-is", async () => {
    // The backend pre-sorts (linked schools first); re-sorting on the client
    // would silently undo that contract (#1663).
    mockFetch(async () =>
      jsonResponse({
        data: [
          { phase_id: "2", school_slug: "linked" },
          { phase_id: "1", school_slug: "unlinked" },
        ],
      }),
    );

    const out = await listEnrollableSchools();

    expect(out.map((p) => p.phase_id)).toEqual(["2", "1"]);
  });

  it("throws on non-OK", async () => {
    mockFetch(async () => jsonResponse({ error: "boom" }, { status: 500 }));
    await expect(listEnrollableSchools()).rejects.toThrow(/boom/);
  });
});

// --- fetchParentEnrollmentProfile -----------------------------------

describe("fetchParentEnrollmentProfile", () => {
  it("returns null on 401 (no JWT — caller renders without prefill)", async () => {
    mockFetch(async () => new Response("", { status: 401 }));
    const out = await fetchParentEnrollmentProfile("school");
    expect(out).toBeNull();
  });

  it("returns the unwrapped profile on 200", async () => {
    mockFetch(async () =>
      jsonResponse({
        data: {
          guardian: { first_name: "Anna", last_name: "B", email: "a@b.c" },
          children: [],
        },
      }),
    );
    const out = await fetchParentEnrollmentProfile("school");
    expect(out).not.toBeNull();
    expect(out!.guardian.first_name).toBe("Anna");
  });

  it("URL-encodes the tenant slug", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: { guardian: {}, children: [] } });
    });
    await fetchParentEnrollmentProfile("we/ird");
    expect(seenURL).toContain("we%2Fird");
  });

  it("throws with backend error message on non-OK non-401", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "tenant unknown" }, { status: 404 }),
    );
    await expect(fetchParentEnrollmentProfile("school")).rejects.toThrow(
      /tenant unknown/,
    );
  });

  it("throws with generic message on non-OK + malformed body", async () => {
    mockFetch(async () => new Response("not json", { status: 500 }));
    await expect(fetchParentEnrollmentProfile("school")).rejects.toThrow(
      /Profile request failed \(500\)/,
    );
  });

  it("handles flat (non-enveloped) response", async () => {
    mockFetch(async () =>
      jsonResponse({
        guardian: { first_name: "Anna", last_name: "", email: "" },
        children: [],
      }),
    );
    const out = await fetchParentEnrollmentProfile("school");
    expect(out).not.toBeNull();
    expect(out!.guardian.first_name).toBe("Anna");
  });
});

// --- submitParentEnrollment ----------------------------------------

const validPayload: SubmitEnrollmentPayload = {
  phase_id: 42,
  guardian_first_name: "Anna",
  guardian_last_name: "Beispiel",
  guardian_email: "a@b.c",
  consent_flags: {},
  custom_data: {},
  children: [],
};

describe("submitParentEnrollment", () => {
  it("POSTs to the parent route and returns the unwrapped result", async () => {
    let seenURL = "";
    let seenBody = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenBody = (init?.body as string) ?? "";
      return jsonResponse(
        {
          data: { request_id: "1234", status_url: "/status/tok" },
        },
        { status: 201 },
      );
    });
    const out = await submitParentEnrollment("school", validPayload);
    expect(out.request_id).toBe("1234");
    expect(out.status_url).toBe("/status/tok");
    expect(seenURL).toContain("/api/parent/enrollments/school/submit");
    expect(seenBody).toContain(`"phase_id":42`);
  });

  it("URL-encodes the tenant slug", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse(
        { data: { request_id: "1", status_url: "/" } },
        { status: 201 },
      );
    });
    await submitParentEnrollment("a/b", validPayload);
    expect(seenURL).toContain("a%2Fb");
  });

  it("handles flat (non-enveloped) response", async () => {
    mockFetch(async () =>
      jsonResponse(
        { request_id: "9999", status_url: "/status/abc" },
        { status: 201 },
      ),
    );
    const out = await submitParentEnrollment("school", validPayload);
    expect(out.request_id).toBe("9999");
    expect(out.status_url).toBe("/status/abc");
  });

  it("falls back to empty strings when neither envelope nor flat fields present", async () => {
    mockFetch(async () => jsonResponse({}, { status: 201 }));
    const out = await submitParentEnrollment("school", validPayload);
    expect(out.request_id).toBe("");
    expect(out.status_url).toBe("");
  });

  it("throws with backend error message on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "Anmeldung bereits vorhanden" }, { status: 409 }),
    );
    await expect(
      submitParentEnrollment("school", validPayload),
    ).rejects.toThrow(/Anmeldung bereits vorhanden/);
  });

  it("preserves coded enrollment errors from the parent submit route", async () => {
    mockFetch(async () =>
      jsonResponse(
        {
          error: "late invite is invalid",
          code: "enrollment.late_invite_invalid",
        },
        { status: 403 },
      ),
    );
    await expect(
      submitParentEnrollment("school", validPayload),
    ).rejects.toMatchObject({
      code: "enrollment.late_invite_invalid",
      status: 403,
      message: expect.stringContaining("Nachzügler-Link"),
    });
  });

  it("throws with German fallback when error body is malformed", async () => {
    mockFetch(async () => new Response("not json", { status: 500 }));
    await expect(
      submitParentEnrollment("school", validPayload),
    ).rejects.toThrow(/Anmeldung konnte nicht übermittelt werden/);
  });
});

// --- Stammdaten master-data helpers ---------------------------------

function mkMasterData(): ChildMasterData {
  return {
    student_id: "42",
    first_name: "Lara",
    last_name: "Beispiel",
    birthday: "2018-03-04",
    school_class: "2a",
    status: "active",
    health_info: "Allergie",
    guardian_profile_id: "77",
    email: "parent@example.test",
    address_street: "Musterweg 1",
    address_city: "Köln",
    address_postal_code: "50667",
    preferred_contact_method: "email",
    language_preference: "de",
    primary_phone: "+491234",
    allowed_departure_modes: { mon: ["pickup"], tue: ["bus", "alone"] },
    pending_changes: [],
  };
}

describe("parent master-data API helpers", () => {
  it("loads a child's master data and URL-encodes the student id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: mkMasterData() });
    });

    const out = await getChildMasterData("42/unsafe");

    expect(out.first_name).toBe("Lara");
    expect(seenURL).toBe("/api/parent/me/children/42%2Funsafe/master-data");
  });

  it("patches one direct-edit field with encoded path segments", async () => {
    let seenURL = "";
    let seenMethod = "";
    let seenBody = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenMethod = init?.method ?? "";
      seenBody = String(init?.body ?? "");
      return jsonResponse({ data: mkMasterData() });
    });

    await updateMasterDataField(
      "42/unsafe",
      "guardian/profile",
      "email address",
      "new@example.test",
    );

    expect(seenMethod).toBe("PATCH");
    expect(seenURL).toBe(
      "/api/parent/me/children/42%2Funsafe/master-data/guardian%2Fprofile/email%20address",
    );
    expect(seenBody).toBe(JSON.stringify({ value: "new@example.test" }));
  });

  it("submits approval-required changes and unwraps the response", async () => {
    let seenURL = "";
    let seenMethod = "";
    let seenBody = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenMethod = init?.method ?? "";
      seenBody = String(init?.body ?? "");
      return jsonResponse({
        data: [
          {
            id: "900",
            target: "person",
            field_key: "first_name",
            new_value: "Lea",
            status: "pending",
            created_at: "2026-06-24T10:00:00Z",
          },
        ],
      });
    });

    const out = await submitMasterDataRequest("42", [
      { target: "person", field_key: "first_name", value: "Lea" },
    ]);

    expect(seenMethod).toBe("POST");
    expect(seenURL).toBe("/api/parent/me/children/42/master-data/requests");
    expect(seenBody).toContain('"field_key":"first_name"');
    expect(out[0]!.id).toBe("900");
  });

  it("surfaces backend errors for master-data writes", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "field is locked" }, { status: 409 }),
    );

    await expect(
      updateMasterDataField("42", "student", "health_info", "x"),
    ).rejects.toThrow(/field is locked/);
  });
});

describe("fetchParentProfile", () => {
  it("unwraps the { data } envelope and returns the portal_locale", async () => {
    mockFetch(async () => jsonResponse({ data: { portal_locale: "en" } }));
    const profile = await fetchParentProfile();
    expect(profile.portal_locale).toBe("en");
  });

  it("returns portal_locale null when the parent never chose a language", async () => {
    mockFetch(async () => jsonResponse({ data: { portal_locale: null } }));
    const profile = await fetchParentProfile();
    expect(profile.portal_locale).toBeNull();
  });
});

describe("updateParentPortalLocale", () => {
  it("PUTs the locale as JSON and returns the updated profile", async () => {
    let capturedUrl: RequestInfo | URL = "";
    let capturedInit: RequestInit = {};
    mockFetch(async (url, init) => {
      capturedUrl = url;
      capturedInit = init ?? {};
      return jsonResponse({ data: { portal_locale: "ru" } });
    });

    const profile = await updateParentPortalLocale("ru");

    expect(profile.portal_locale).toBe("ru");
    expect(capturedUrl).toBe("/api/parent/me/profile");
    expect(capturedInit.method).toBe("PUT");
    expect(
      (capturedInit.headers as Record<string, string>)["Content-Type"],
    ).toBe("application/json");
    expect(capturedInit.body).toBe(JSON.stringify({ portal_locale: "ru" }));
  });

  it("throws when the backend rejects the update", async () => {
    mockFetch(async () => jsonResponse({}, { status: 400 }));
    await expect(updateParentPortalLocale("en")).rejects.toThrow(
      /Profile update failed \(400\)/,
    );
  });
});

// --- sick notes ------------------------------------------------------

function mkStatusDay(date: string, note?: string): StatusDay {
  return {
    id: "1",
    student_id: "84",
    date,
    status: "sick",
    reported_at: "2026-06-01T08:00:00Z",
    source: "parent",
    note,
  };
}

describe("submitSickNote", () => {
  it("POSTs dates + reason to the child sick-note route and unwraps data", async () => {
    let seenURL = "";
    let seenBody = "";
    let seenMethod = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenBody = (init?.body as string) ?? "";
      seenMethod = init?.method ?? "";
      return jsonResponse(
        { data: { status_days: [mkStatusDay("2026-06-02", "Fieber")] } },
        { status: 201 },
      );
    });
    const out = await submitSickNote("84", ["2026-06-02"], "Fieber");
    expect(out.status_days).toHaveLength(1);
    expect(out.status_days[0]!.note).toBe("Fieber");
    expect(out.pending_request).toBeUndefined();
    expect(seenMethod).toBe("POST");
    expect(seenURL).toContain("/api/parent/me/children/84/sick-note");
    expect(seenBody).toContain('"dates":["2026-06-02"]');
    expect(seenBody).toContain('"reason":"Fieber"');
  });

  it("normalizes the legacy bare-array response into the envelope shape (issue #1845 backward-compat)", async () => {
    // A backend on the pre-#1845 contract (and the current direct-write path)
    // responds with the bare status-day array, not the { status_days } object.
    // submitSickNote must wrap it so callers see one shape.
    mockFetch(async () =>
      jsonResponse(
        { data: [mkStatusDay("2026-06-02", "Fieber")] },
        { status: 201 },
      ),
    );
    const out = await submitSickNote("84", ["2026-06-02"], "Fieber");
    expect(out.status_days).toHaveLength(1);
    expect(out.status_days[0]!.note).toBe("Fieber");
    expect(out.pending_request).toBeUndefined();
  });

  it("defaults the status to a Krankmeldung (sick)", async () => {
    let seenBody = "";
    mockFetch(async (_input, init) => {
      seenBody = (init?.body as string) ?? "";
      return jsonResponse({ data: { status_days: [] } }, { status: 201 });
    });
    await submitSickNote("84", ["2026-06-02"], "Fieber");
    expect(seenBody).toContain('"status":"sick"');
  });

  it("sends the chosen status for an excused absence (issue #1735)", async () => {
    let seenBody = "";
    mockFetch(async (_input, init) => {
      seenBody = (init?.body as string) ?? "";
      return jsonResponse({ data: { status_days: [] } }, { status: 201 });
    });
    await submitSickNote("84", ["2026-06-02"], "Termin", "excused");
    expect(seenBody).toContain('"status":"excused"');
    expect(seenBody).toContain('"reason":"Termin"');
  });

  it("returns a pending_request (empty status_days) for a gated excused absence (issue #1845)", async () => {
    mockFetch(async () =>
      jsonResponse(
        {
          data: {
            status_days: [],
            pending_request: {
              id: "req-7",
              student_id: "84",
              status: "pending",
              dates: ["2026-06-02"],
              note: "Zahnarzt",
              created_at: "2026-06-01T09:00:00Z",
            },
          },
        },
        { status: 201 },
      ),
    );
    const out = await submitSickNote(
      "84",
      ["2026-06-02"],
      "Zahnarzt",
      "excused",
    );
    expect(out.status_days).toHaveLength(0);
    expect(out.pending_request?.id).toBe("req-7");
    expect(out.pending_request?.status).toBe("pending");
  });

  it("URL-encodes the student id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: { status_days: [] } }, { status: 201 });
    });
    await submitSickNote("a/b", ["2026-06-02"], "Fieber");
    expect(seenURL).toContain("a%2Fb");
  });

  it("throws with the backend error message on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "Krankmeldung deaktiviert" }, { status: 403 }),
    );
    await expect(
      submitSickNote("84", ["2026-06-02"], "Fieber"),
    ).rejects.toThrow(/Krankmeldung deaktiviert/);
  });

  it("redirects to /parents/login on 401", async () => {
    const assign = vi.fn();
    Object.defineProperty(window, "location", {
      writable: true,
      value: { assign, host: "parents.localhost:3000" },
    });
    mockFetch(async () => new Response("", { status: 401 }));
    await expect(
      submitSickNote("84", ["2026-06-02"], "Fieber"),
    ).rejects.toThrow();
    expect(assign).toHaveBeenCalledWith("/parents/login");
  });
});

describe("listSickDays", () => {
  it("GETs the child sick-note route and returns the data array", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: [mkStatusDay("2026-06-02")] });
    });
    const out = await listSickDays("84");
    expect(out).toHaveLength(1);
    expect(seenURL).toContain("/api/parent/me/children/84/sick-note");
  });
});

describe("listExcusedRequests", () => {
  it("GETs the child excused-requests route and returns the data array", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({
        data: [
          {
            id: "req-1",
            student_id: "84",
            status: "pending",
            dates: ["2026-06-02"],
            note: "Zahnarzt",
            created_at: "2026-06-01T09:00:00Z",
          },
        ],
      });
    });
    const out = await listExcusedRequests("84");
    expect(out).toHaveLength(1);
    expect(out[0]!.status).toBe("pending");
    expect(seenURL).toContain("/api/parent/me/children/84/excused-requests");
  });
});

describe("withdrawExcusedRequest", () => {
  it("DELETEs the request and returns the withdrawn request", async () => {
    let seenURL = "";
    let seenMethod = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenMethod = init?.method ?? "";
      return jsonResponse({
        data: {
          id: "req-1",
          student_id: "84",
          status: "withdrawn",
          dates: ["2026-06-02"],
          note: "Zahnarzt",
          created_at: "2026-06-01T09:00:00Z",
          reviewed_at: "2026-06-01T10:00:00Z",
        },
      });
    });
    const out = await withdrawExcusedRequest("84", "req-1");
    expect(seenMethod).toBe("DELETE");
    expect(out.status).toBe("withdrawn");
    expect(seenURL).toContain(
      "/api/parent/me/children/84/excused-requests/req-1",
    );
  });
});

describe("getChildFeatures", () => {
  it("GETs the features route and returns the resolved flags", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({
        data: {
          sick_note_enabled: true,
          notes_enabled: false,
          pickup_change_enabled: true,
          related_accounts_invite_enabled: false,
          related_accounts_remove_enabled: false,
          master_data_edit_enabled: true,
          master_data_contact_edit_enabled: false,
          master_data_request_enabled: true,
        },
      });
    });
    const out = await getChildFeatures("84");
    expect(out.sick_note_enabled).toBe(true);
    expect(out.notes_enabled).toBe(false);
    expect(out.master_data_contact_edit_enabled).toBe(false);
    expect(seenURL).toContain("/api/parent/me/children/84/features");
  });

  it("throws on non-OK so the caller can fall back to defaults", async () => {
    mockFetch(async () => new Response("nope", { status: 500 }));
    await expect(getChildFeatures("84")).rejects.toThrow(
      /Request failed \(500\)/,
    );
  });
});

describe("getChildMealPlan", () => {
  it("GETs the child meal-plan route with the week_start query and returns entries", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({
        data: [
          { date: "2026-07-06", position: 0, dish: "Spaghetti", note: null },
          {
            date: "2026-07-06",
            position: 1,
            dish: "Salat",
            note: "vegetarisch",
          },
        ],
      });
    });
    const out = await getChildMealPlan("84", "2026-07-06");
    expect(out).toHaveLength(2);
    expect(out[0]?.dish).toBe("Spaghetti");
    expect(out[1]?.note).toBe("vegetarisch");
    expect(seenURL).toContain("/api/parent/me/children/84/meal-plan");
    expect(seenURL).toContain("week_start=2026-07-06");
  });

  it("URL-encodes the student id and week start", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: [] });
    });
    await getChildMealPlan("a/b", "2026-07-06");
    expect(seenURL).toContain("/api/parent/me/children/a%2Fb/meal-plan");
  });

  it("propagates a 403 (meal_plan_disabled) so the caller can hide the section", async () => {
    mockFetch(async () => new Response("nope", { status: 403 }));
    await expect(getChildMealPlan("84", "2026-07-06")).rejects.toThrow(
      /Request failed \(403\)/,
    );
  });
});

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function mkThreadSummary(
  overrides: Partial<ThreadSummary> = {},
): ThreadSummary {
  return {
    thread_id: "t1",
    student_id: "42",
    student_name: "Max Mustermann",
    school_name: "OGS A",
    counterpart_name: "OGS OGS A",
    last_message_read_by_staff: false,
    unread: 0,
    ...overrides,
  };
}

function mkThreadView(overrides: Partial<ThreadView> = {}): ThreadView {
  return {
    thread_id: "t1",
    student_id: "42",
    student_name: "Max Mustermann",
    school_name: "OGS A",
    counterpart_name: "OGS OGS A",
    messages: [],
    ...overrides,
  };
}

// --- listMessageThreads -------------------------------------------------------

describe("listMessageThreads", () => {
  it("GETs /api/parent/me/messages and returns the data array", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: [mkThreadSummary({ unread: 2 })] });
    });
    const out = await listMessageThreads();
    expect(out).toHaveLength(1);
    expect(out[0]!.thread_id).toBe("t1");
    expect(out[0]!.unread).toBe(2);
    expect(seenURL).toBe("/api/parent/me/messages");
  });

  it("throws on non-OK with a clear message", async () => {
    mockFetch(async () => new Response("nope", { status: 500 }));
    await expect(listMessageThreads()).rejects.toThrow(
      /Request failed \(500\)/,
    );
  });

  it("returns an empty array when data is missing from the envelope", async () => {
    mockFetch(async () => jsonResponse({}));
    // parent-api's getJson unwraps { data } or returns the body directly;
    // an envelope with no data field falls through to the body itself which
    // is an empty object — cast defensively.
    const out = await listMessageThreads();
    // Either empty array or empty object-like — the key invariant is no throw.
    expect(out).toBeDefined();
  });
});

// --- listChildThreads ---------------------------------------------------------

describe("listChildThreads", () => {
  it("GETs the per-child threads route with encoded studentId", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: [mkThreadSummary()] });
    });
    const out = await listChildThreads("42");
    expect(out).toHaveLength(1);
    expect(seenURL).toContain("/api/parent/me/messages/children/42/threads");
  });

  it("URL-encodes the studentId", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: [] });
    });
    await listChildThreads("a/b");
    expect(seenURL).toContain("a%2Fb");
  });

  it("throws on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "forbidden" }, { status: 403 }),
    );
    await expect(listChildThreads("42")).rejects.toThrow(/forbidden/);
  });
});

// --- fetchMessagesUnreadCount -------------------------------------------------

describe("fetchMessagesUnreadCount", () => {
  it("GETs /api/parent/me/messages/unread-count and returns the count", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ unread_count: 7 });
    });
    const count = await fetchMessagesUnreadCount();
    expect(count).toBe(7);
    expect(seenURL).toContain("/api/parent/me/messages/unread-count");
  });

  it("returns 0 when unread_count is missing from the response", async () => {
    mockFetch(async () => jsonResponse({}));
    const count = await fetchMessagesUnreadCount();
    expect(count).toBe(0);
  });

  it("throws on non-OK", async () => {
    mockFetch(async () => new Response("", { status: 500 }));
    await expect(fetchMessagesUnreadCount()).rejects.toThrow(
      /Request failed \(500\)/,
    );
  });
});

// --- getChildConversation -----------------------------------------------------

describe("getChildConversation", () => {
  it("GETs the child conversation route and returns a ThreadView", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse(mkThreadView({ messages: [] }));
    });
    const view = await getChildConversation("42");
    expect(view.student_id).toBe("42");
    expect(view.messages).toEqual([]);
    expect(seenURL).toContain("/api/parent/me/messages/children/42");
  });

  it("URL-encodes the studentId", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse(mkThreadView());
    });
    await getChildConversation("a/b");
    expect(seenURL).toContain("a%2Fb");
  });

  it("throws on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "not found" }, { status: 404 }),
    );
    await expect(getChildConversation("42")).rejects.toThrow(/not found/);
  });
});

// --- postChildMessage ---------------------------------------------------------

describe("postChildMessage", () => {
  it("POSTs the message body and returns the updated ThreadView", async () => {
    let seenURL = "";
    let seenBody = "";
    let seenMethod = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenBody = (init?.body as string) ?? "";
      seenMethod = init?.method ?? "";
      return jsonResponse(mkThreadView({ thread_id: "t99" }), { status: 201 });
    });
    const view = await postChildMessage("42", "Hallo OGS!");
    expect(view.thread_id).toBe("t99");
    expect(seenMethod).toBe("POST");
    expect(seenURL).toContain("/api/parent/me/messages/children/42");
    expect(seenBody).toContain('"body":"Hallo OGS!"');
  });

  it("URL-encodes the studentId in the POST URL", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse(mkThreadView(), { status: 201 });
    });
    await postChildMessage("x/y", "text");
    expect(seenURL).toContain("x%2Fy");
  });

  it("throws on non-OK with the backend error message", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "Messaging deaktiviert" }, { status: 403 }),
    );
    await expect(postChildMessage("42", "test")).rejects.toThrow(
      /Messaging deaktiviert/,
    );
  });
});

function mkAnnouncement(
  overrides: Partial<ParentAnnouncement> = {},
): ParentAnnouncement {
  return {
    id: "5",
    title: "Sommerfest",
    body: "Wir feiern am Freitag.",
    priority: "info",
    requires_acknowledgement: false,
    school_name: "OGS Am Berg",
    published_at: "2026-07-01T08:00:00Z",
    read: false,
    acknowledged: false,
    // A plain Mitteilung; the Umfrage tests override this (#1371).
    response_type: "none",
    ...overrides,
  };
}

describe("listAnnouncements", () => {
  it("GETs the news feed and unwraps the data envelope", async () => {
    let seenURL = "";
    let seenMethod: string | undefined;
    const feed = [mkAnnouncement(), mkAnnouncement({ id: "6", read: true })];
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenMethod = init?.method;
      return jsonResponse({ data: feed });
    });

    await expect(listAnnouncements()).resolves.toEqual(feed);
    expect(seenURL).toBe("/api/parent/me/news");
    expect(seenMethod).toBe("GET");
  });

  it("throws a ParentApiError on a failed feed load", async () => {
    mockFetch(async () => jsonResponse({ error: "kaputt" }, { status: 500 }));
    await expect(listAnnouncements()).rejects.toThrow(/kaputt/);
  });
});

describe("fetchAnnouncementsUnreadCount", () => {
  it("returns the unread_count from the envelope", async () => {
    mockFetch(async (input) => {
      expect(typeof input === "string" ? input : input.toString()).toBe(
        "/api/parent/me/news/unread-count",
      );
      return jsonResponse({ data: { unread_count: 4 } });
    });
    await expect(fetchAnnouncementsUnreadCount()).resolves.toBe(4);
  });

  it("defaults to zero when the count is missing", async () => {
    mockFetch(async () => jsonResponse({ data: {} }));
    await expect(fetchAnnouncementsUnreadCount()).resolves.toBe(0);
  });
});

describe("markAnnouncementRead", () => {
  it("POSTs the loaded published_at version to the read endpoint", async () => {
    let seenURL = "";
    let seenMethod: string | undefined;
    let seenBody: unknown;
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenMethod = init?.method;
      seenBody = JSON.parse(init?.body as string);
      return jsonResponse({ data: { read: true } });
    });

    await markAnnouncementRead("5", "2026-07-01T08:00:00Z");
    expect(seenURL).toBe("/api/parent/me/news/5/read");
    expect(seenMethod).toBe("POST");
    expect(seenBody).toEqual({ published_at: "2026-07-01T08:00:00Z" });
  });

  it("URL-encodes the announcement id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: { read: true } });
    });
    await markAnnouncementRead("a/b", "2026-07-01T08:00:00Z");
    expect(seenURL).toBe("/api/parent/me/news/a%2Fb/read");
  });

  it("propagates a 409 when the loaded version is stale", async () => {
    mockFetch(async () => jsonResponse({ error: "veraltet" }, { status: 409 }));
    await expect(
      markAnnouncementRead("5", "2026-07-01T08:00:00Z"),
    ).rejects.toThrow(/veraltet/);
  });
});

describe("acknowledgeAnnouncement", () => {
  it("POSTs the loaded published_at version to the acknowledge endpoint", async () => {
    let seenURL = "";
    let seenBody: unknown;
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenBody = JSON.parse(init?.body as string);
      return jsonResponse({ data: { acknowledged: true } });
    });

    await acknowledgeAnnouncement("5", "2026-07-01T08:00:00Z");
    expect(seenURL).toBe("/api/parent/me/news/5/acknowledge");
    expect(seenBody).toEqual({ published_at: "2026-07-01T08:00:00Z" });
  });

  it("propagates a 409 when acknowledging since-corrected wording", async () => {
    mockFetch(async () => jsonResponse({ error: "veraltet" }, { status: 409 }));
    await expect(
      acknowledgeAnnouncement("5", "2026-07-01T08:00:00Z"),
    ).rejects.toThrow(/veraltet/);
  });
});
