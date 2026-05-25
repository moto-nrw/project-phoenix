import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  confirmRenewal,
  fetchMyEnrollmentProfile,
  fetchPublicCareOfferings,
  fetchPublicPhases,
  fetchStatus,
  patchStatus,
  submitEnrollment,
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
          care_required: true,
        },
      }),
    );

    await expect(
      fetchPublicCareOfferings("test tenant", "phase/5"),
    ).resolves.toMatchObject({
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
      careRequired: false,
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
          },
        ],
      }),
    );
    await expect(fetchPublicPhases("demo")).resolves.toHaveLength(1);

    mockFetch.mockResolvedValueOnce(jsonResponse({ data: { id: "bad" } }));
    await expect(fetchPublicPhases("demo")).resolves.toEqual([]);
  });

  it("returns null for unauthenticated profile requests", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ error: "unauthorized" }, { status: 401 }),
    );

    await expect(fetchMyEnrollmentProfile()).resolves.toBeNull();
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

  it("confirms renewals and defaults missing counters to zero", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ confirmed: 3 }));
    await expect(confirmRenewal("tok")).resolves.toBe(3);

    mockFetch.mockResolvedValueOnce(jsonResponse({}));
    await expect(confirmRenewal("tok")).resolves.toBe(0);
  });

  it("surfaces fallback messages when status mutations fail", async () => {
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
