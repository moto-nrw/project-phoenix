import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

import {
  listMyChildren,
  listMyEnrollments,
  fetchParentEnrollmentProfile,
  submitParentEnrollment,
  type Child,
  type EnrollmentRequest,
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

  it("throws with German fallback when error body is malformed", async () => {
    mockFetch(async () => new Response("not json", { status: 500 }));
    await expect(
      submitParentEnrollment("school", validPayload),
    ).rejects.toThrow(/Anmeldung konnte nicht übermittelt werden/);
  });
});
