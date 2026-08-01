import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  confirmRenewal,
  createEnrollmentChangeRequest,
  fetchMyEnrollmentProfile,
  fetchEnrollmentEditBootstrap,
  fetchPublicCareOfferings,
  fetchPublicEnrollmentBootstrap,
  EMPTY_SCHOOL_CLASS_CONFIG,
  fetchPublicPhases,
  fetchStatus,
  listEnrollmentChangeRequests,
  patchStatus,
  replyEnrollmentChangeRequest,
  submitEnrollment,
  updateEnrollmentRequest,
  withdrawStatus,
  type SubmitEnrollmentPayload,
} from "./enrollment-submission-api";

const mockFetch = vi.fn<typeof fetch>();
const originalFetch = global.fetch;

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

describe("enrollment-submission-api", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    global.fetch = mockFetch;
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("loads public care offerings from the backend envelope", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: {
          offerings: [
            {
              id: "11",
              name: "Flexible Betreuung",
              days_of_week_mode: "parent_choice",
              available_days: ["mon"],
              includes_holiday_care: true,
              includes_lunch: false,
              is_active: true,
            },
          ],
          care_offering_selection_mode: "at_least_one",
          care_required: true,
        },
      }),
    );

    await expect(
      fetchPublicCareOfferings("test tenant", "phase/5"),
    ).resolves.toMatchObject({
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
      offerings: [{ id: "11", name: "Flexible Betreuung" }],
    });
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/enrollment/care-offerings/public/test%20tenant/phase%2F5",
      { cache: "no-store" },
    );
  });

  it("normalizes malformed care offering payloads", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ data: { offerings: null, care_required: false } }),
    );

    await expect(fetchPublicCareOfferings("tenant", "5")).resolves.toEqual({
      offerings: [],
      careOfferingSelectionMode: "optional",
      careRequired: false,
      collectGradeLevel: true,
      careOfferingsEnabled: true,
      schoolClass: undefined,
    });

    mockFetch.mockResolvedValueOnce(
      jsonResponse({ data: { offerings: [], care_required: true } }),
    );
    await expect(fetchPublicCareOfferings("tenant", "5")).resolves.toEqual({
      offerings: [],
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
      collectGradeLevel: true,
      careOfferingsEnabled: true,
      schoolClass: undefined,
    });
  });

  it("normalizes enrollment feature flags from enabled, disabled, and missing fields", async () => {
    mockFetch
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            offerings: [],
            collect_grade_level: true,
            care_offerings_enabled: true,
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            offerings: [],
            collect_grade_level: false,
            care_offerings_enabled: false,
          },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ data: { offerings: [] } }));

    await expect(
      fetchPublicCareOfferings("tenant", "5"),
    ).resolves.toMatchObject({
      collectGradeLevel: true,
      careOfferingsEnabled: true,
    });
    await expect(
      fetchPublicCareOfferings("tenant", "5"),
    ).resolves.toMatchObject({
      collectGradeLevel: false,
      careOfferingsEnabled: false,
    });
    await expect(
      fetchPublicCareOfferings("tenant", "5"),
    ).resolves.toMatchObject({
      collectGradeLevel: true,
      careOfferingsEnabled: true,
    });
  });

  it("parses the concrete-class config when the backend sends it", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: {
          offerings: [],
          care_offering_selection_mode: "optional",
          school_class: {
            collect: true,
            require: true,
            collect_grade_1: true,
            // Non-string entries are filtered out.
            available_classes: ["2a", "2b", 3, null, "3a"],
          },
        },
      }),
    );

    await expect(
      fetchPublicCareOfferings("tenant", "5"),
    ).resolves.toMatchObject({
      schoolClass: {
        collect: true,
        require: true,
        available_classes: ["2a", "2b", "3a"],
        // Server-authoritative grade-1 collection flag (#1663).
        collect_grade_1: true,
      },
    });
  });

  it("coerces a malformed concrete-class config to safe defaults", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: {
          offerings: [],
          care_offering_selection_mode: "optional",
          // collect/require missing, available_classes not an array.
          school_class: { available_classes: "2a" },
        },
      }),
    );

    await expect(
      fetchPublicCareOfferings("tenant", "5"),
    ).resolves.toMatchObject({
      schoolClass: {
        collect: false,
        require: false,
        available_classes: [],
        collect_grade_1: false,
      },
    });
  });

  it("omits the concrete-class config when the backend does not send it", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: { offerings: [], care_offering_selection_mode: "optional" },
      }),
    );

    const result = await fetchPublicCareOfferings("tenant", "5");
    expect(result.schoolClass).toBeUndefined();
    // The disabled default consumers fall back to.
    expect(EMPTY_SCHOOL_CLASS_CONFIG).toEqual({
      collect: false,
      available_classes: [],
      require: false,
      // Grade 1 stays grade-level-only unless the backend says otherwise (#1663).
      collect_grade_1: false,
    });
  });

  it("maps known submission error codes to parent-facing messages", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse(
        { code: "enrollment.care_offering_full", error: "raw backend text" },
        { status: 409 },
      ),
    );

    await expect(fetchPublicCareOfferings("tenant", "5")).rejects.toMatchObject(
      {
        message: expect.stringContaining("bereits voll"),
        status: 409,
        code: "enrollment.care_offering_full",
      },
    );
  });

  it("loads public phases and falls back to an empty list for non-arrays", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: [
          {
            id: "5",
            name: "2026",
            kind: "school_year",
            service_start_date: "2026-08-01",
            service_end_date: "2027-07-31",
            show_status_reason_to_parent: true,
            care_offering_selection_mode: "at_least_one",
          },
        ],
      }),
    );
    await expect(fetchPublicPhases("demo")).resolves.toHaveLength(1);

    mockFetch.mockResolvedValueOnce(jsonResponse({ data: { id: "bad" } }));
    await expect(fetchPublicPhases("demo")).resolves.toEqual([]);
  });

  it("loads public enrollment bootstrap from the combined endpoint", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: {
          phase: {
            id: "5",
            name: "Ferien",
            kind: "holiday",
            service_start_date: "2026-07-01",
            service_end_date: "2026-07-31",
            show_status_reason_to_parent: false,
            care_offering_selection_mode: "exactly_one",
          },
          schema: { id: "12", version: 1, fields: [] },
          offerings: [{ id: "77", name: "Sommer", is_required: false }],
          care_offering_selection_mode: "exactly_one",
          care_required: true,
          captcha_config: { enabled: true, site_key: "site" },
          legal_texts: { blocks: [] },
        },
      }),
    );

    await expect(
      fetchPublicEnrollmentBootstrap("demo slug", "ph/5"),
    ).resolves.toMatchObject({
      phase: { id: "5", name: "Ferien" },
      schema: { id: "12" },
      offerings: [{ id: "77" }],
      care_offering_selection_mode: "exactly_one",
      captcha_config: { site_key: "site" },
      legal_texts: { blocks: [] },
    });
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/enrollment/form-bootstrap/public/demo%20slug/ph%2F5",
      { cache: "no-store" },
    );
  });

  it("omits credentials without adding a profile-enrichment query flag", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: {
          phase: { id: "5", name: "Ferien" },
          schema: null,
          offerings: [],
          care_offering_selection_mode: "optional",
          care_required: false,
          captcha_config: null,
          legal_texts: { blocks: [] },
        },
      }),
    );

    await fetchPublicEnrollmentBootstrap("demo", "5", {
      lateInviteToken: "invite token",
      omitCredentials: true,
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/enrollment/form-bootstrap/public/demo/5?late_invite=invite%20token",
      { cache: "no-store", credentials: "omit" },
    );
  });

  it("returns null for unauthenticated profile requests", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ error: "unauthorized" }, { status: 401 }),
    );

    await expect(fetchMyEnrollmentProfile()).resolves.toBeNull();
  });

  it("loads the authenticated enrollment profile", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        guardian: {
          first_name: "Mara",
          last_name: "Muster",
          email: "mara@example.test",
          phone: "+491234",
        },
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            school_class: "2a",
            grade_level: 2,
          },
        ],
      }),
    );

    await expect(fetchMyEnrollmentProfile()).resolves.toMatchObject({
      guardian: { email: "mara@example.test" },
      children: [{ id: "7", school_class: "2a" }],
    });
    expect(mockFetch).toHaveBeenCalledWith("/api/enrollment/me/profile", {
      cache: "no-store",
    });
  });

  it("submits enrollment payloads and unwraps the response", async () => {
    const payload: SubmitEnrollmentPayload = {
      phase_id: 5,
      guardian_first_name: "Mara",
      guardian_last_name: "Muster",
      guardian_email: "mara@example.test",
      children: [
        {
          first_name: "Lina",
          last_name: "Muster",
          date_of_birth: "2018-04-15",
          target_grade_level: 2,
        },
      ],
    };
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ data: { request_id: "99", status_url: "/status/abc" } }),
    );

    await expect(submitEnrollment("tenant", payload)).resolves.toEqual({
      request_id: "99",
      status_url: "/status/abc",
    });
    expect(mockFetch).toHaveBeenCalledWith("/api/enrollment/tenant/submit", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
  });

  it("throws the submission fallback when the backend sends an unreadable error", async () => {
    mockFetch.mockResolvedValueOnce(new Response("no json", { status: 500 }));

    await expect(
      submitEnrollment("tenant", {
        phase_id: 5,
        guardian_first_name: "Mara",
        guardian_last_name: "Muster",
        guardian_email: "mara@example.test",
        children: [],
      }),
    ).rejects.toThrow("Anmeldung konnte nicht übermittelt werden");
  });

  it("loads status, patches it, and withdraws whole requests or child rows", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: {
          request_id: "99",
          guardian_first_name: "Mara",
          guardian_last_name: "Muster",
          guardian_email: "mara@example.test",
          submitted_at: "2026-01-01T00:00:00Z",
          children: [
            {
              id: "7",
              first_name: "Lina",
              last_name: "Muster",
              status: "submitted",
            },
          ],
        },
      }),
    );
    await expect(fetchStatus("tok/en")).resolves.toMatchObject({
      request_id: "99",
      children: [{ id: "7" }],
    });

    mockFetch.mockResolvedValueOnce(jsonResponse({ data: {} }));
    await expect(
      patchStatus("tok", { guardian_phone: "+49" }),
    ).resolves.toBeUndefined();

    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await expect(withdrawStatus("tok")).resolves.toBeUndefined();

    mockFetch.mockResolvedValueOnce(jsonResponse({ status: "ok" }));
    await expect(withdrawStatus("tok", "7")).resolves.toBeUndefined();
    expect(mockFetch).toHaveBeenLastCalledWith(
      "/api/enrollment/requests/tok/withdraw",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ child_id: "7" }),
      },
    );
  });

  it("returns null for missing status tokens", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ error: "missing" }, { status: 404 }),
    );

    await expect(fetchStatus("missing")).resolves.toBeNull();
  });

  it("loads and updates enrollment edit drafts", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: {
          phase: {
            id: "5",
            name: "2026",
            kind: "school_year",
            service_start_date: "2026-08-01",
            service_end_date: "2027-07-31",
            show_status_reason_to_parent: true,
            care_offering_selection_mode: "optional",
          },
          schema: null,
          offerings: [],
          care_offering_selection_mode: "optional",
          care_required: false,
          grade_level_max: 13,
          legal_texts: { blocks: [] },
          draft: {
            request_id: "99",
            status_token: "tok/en",
            tenant_id: "1",
            tenant_subdomain: "demo",
            phase_id: "5",
            guardian_first_name: "Mara",
            guardian_last_name: "Muster",
            guardian_email: "mara@example.test",
            consent_flags: {},
            custom_data: {},
            children: [],
          },
        },
      }),
    );

    await expect(fetchEnrollmentEditBootstrap("tok/en")).resolves.toMatchObject(
      {
        draft: { request_id: "99", status_token: "tok/en" },
        phase: { id: "5" },
        grade_level_max: 13,
      },
    );
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/enrollment/requests/tok%2Fen/edit-bootstrap",
      { cache: "no-store" },
    );

    const payload: SubmitEnrollmentPayload = {
      phase_id: 5,
      guardian_first_name: "Mara",
      guardian_last_name: "Muster",
      guardian_email: "mara@example.test",
      children: [
        {
          first_name: "Lina",
          last_name: "Muster",
          date_of_birth: "2018-04-15",
          target_grade_level: 2,
        },
      ],
    };
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({ request_id: "99", status_url: "/status/tok" }),
    } as Response);

    await expect(updateEnrollmentRequest("tok/en", payload)).resolves.toEqual({
      request_id: "99",
      status_url: "/status/tok",
    });
    expect(mockFetch).toHaveBeenLastCalledWith(
      "/api/enrollment/requests/tok%2Fen",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      },
    );
  });

  it("creates, lists, and replies to enrollment change requests", async () => {
    const payload: SubmitEnrollmentPayload = {
      phase_id: 5,
      guardian_first_name: "Mara",
      guardian_last_name: "Muster",
      guardian_email: "mara@example.test",
      children: [
        {
          first_name: "Lina",
          last_name: "Muster",
          date_of_birth: "2018-04-15",
          target_grade_level: 2,
        },
      ],
    };
    const changeRequest = {
      id: "cr-1",
      request_id: "99",
      status: "pending_review",
      parent_note: "Bitte prüfen",
      base_snapshot: {},
      proposed_snapshot: {},
      diff: {},
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    };

    mockFetch.mockResolvedValueOnce(jsonResponse({ data: changeRequest }));
    await expect(
      createEnrollmentChangeRequest("tok/en", payload, "Bitte prüfen"),
    ).resolves.toMatchObject({ id: "cr-1", parent_note: "Bitte prüfen" });
    expect(mockFetch).toHaveBeenLastCalledWith(
      "/api/enrollment/requests/tok%2Fen/change-requests",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...payload, parent_note: "Bitte prüfen" }),
      },
    );

    mockFetch.mockResolvedValueOnce(jsonResponse({ data: [changeRequest] }));
    await expect(listEnrollmentChangeRequests("tok/en")).resolves.toEqual([
      changeRequest,
    ]);
    expect(mockFetch).toHaveBeenLastCalledWith(
      "/api/enrollment/requests/tok%2Fen/change-requests",
      { cache: "no-store" },
    );

    mockFetch.mockResolvedValueOnce(jsonResponse({ data: { nope: true } }));
    await expect(listEnrollmentChangeRequests("tok/en")).resolves.toEqual([]);

    mockFetch.mockResolvedValueOnce(jsonResponse(changeRequest));
    await expect(
      replyEnrollmentChangeRequest("tok/en", "cr/1", "Danke"),
    ).resolves.toMatchObject({ id: "cr-1" });
    expect(mockFetch).toHaveBeenLastCalledWith(
      "/api/enrollment/requests/tok%2Fen/change-requests/cr%2F1/messages",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ body: "Danke" }),
      },
    );
  });

  it("surfaces change-request errors with the endpoint fallback", async () => {
    mockFetch.mockResolvedValueOnce(new Response("no json", { status: 500 }));
    await expect(
      createEnrollmentChangeRequest(
        "tok",
        {
          phase_id: 5,
          guardian_first_name: "Mara",
          guardian_last_name: "Muster",
          guardian_email: "mara@example.test",
          children: [],
        },
        "",
      ),
    ).rejects.toThrow("Änderungsanfrage konnte nicht gespeichert werden");

    mockFetch.mockResolvedValueOnce(new Response("no json", { status: 500 }));
    await expect(listEnrollmentChangeRequests("tok")).rejects.toThrow(
      "Änderungsanfragen konnten nicht geladen werden",
    );

    mockFetch.mockResolvedValueOnce(new Response("no json", { status: 500 }));
    await expect(
      replyEnrollmentChangeRequest("tok", "cr-1", "Danke"),
    ).rejects.toThrow("Antwort konnte nicht gesendet werden");
  });

  it("confirms renewals and defaults missing counters to zero", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ confirmed: 3 }));
    await expect(confirmRenewal("tok")).resolves.toBe(3);

    mockFetch.mockResolvedValueOnce(jsonResponse({}));
    await expect(confirmRenewal("tok")).resolves.toBe(0);
  });

  it("surfaces fallback messages when status mutations fail", async () => {
    mockFetch.mockResolvedValueOnce(new Response("no json", { status: 500 }));
    await expect(fetchStatus("tok")).rejects.toThrow(
      "Status konnte nicht geladen werden",
    );

    mockFetch.mockResolvedValueOnce(
      jsonResponse({ message: "Bearbeitung gesperrt" }, { status: 423 }),
    );
    await expect(fetchEnrollmentEditBootstrap("tok")).rejects.toThrow(
      "Anmeldung kann nicht bearbeitet werden",
    );

    mockFetch.mockResolvedValueOnce(
      jsonResponse({ error: "Speichern gesperrt" }, { status: 423 }),
    );
    await expect(
      updateEnrollmentRequest("tok", {
        phase_id: 5,
        guardian_first_name: "Mara",
        guardian_last_name: "Muster",
        guardian_email: "mara@example.test",
        children: [],
      }),
    ).rejects.toThrow("Änderungen konnten nicht gespeichert werden");

    mockFetch.mockResolvedValueOnce(new Response("no json", { status: 500 }));
    await expect(
      patchStatus("tok", { guardian_first_name: "Mara" }),
    ).rejects.toThrow("Änderungen konnten nicht gespeichert werden");

    mockFetch.mockResolvedValueOnce(
      jsonResponse({ message: "Schon zurückgenommen" }, { status: 409 }),
    );
    await expect(withdrawStatus("tok")).rejects.toThrow("Schon zurückgenommen");

    mockFetch.mockResolvedValueOnce(
      jsonResponse({ error: "Abgelaufen" }, { status: 410 }),
    );
    await expect(confirmRenewal("tok")).rejects.toThrow("Abgelaufen");
  });
});
