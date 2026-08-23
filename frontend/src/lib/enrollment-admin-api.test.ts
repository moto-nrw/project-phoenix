import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  approveEnrollmentChangeRequest,
  askEnrollmentChangeRequestQuestion,
  correctAdminChildData,
  decideAdminChild,
  exportStudentEnrollmentRequests,
  getAdminEnrollmentChangeRequest,
  getAdminRequest,
  listAdminChildOfferingAdjustments,
  listAdminEnrollmentChangeRequests,
  listAdminRequests,
  listStudentEnrollmentRequests,
  rejectEnrollmentChangeRequest,
  updateAdminChildOfferings,
  type AdminEnrollmentChangeRequest,
  type AdminOfferingAdjustment,
  type AdminRequestChild,
  type AdminRequestDetail,
  type AdminRequestSummary,
} from "./enrollment-admin-api";

const mockFetch = vi.fn<typeof fetch>();
const originalFetch = global.fetch;
const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;

let anchor: {
  href: string;
  download: string;
  click: ReturnType<typeof vi.fn>;
  remove: ReturnType<typeof vi.fn>;
};

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

function child(overrides: Partial<AdminRequestChild> = {}): AdminRequestChild {
  return {
    id: "child-1",
    first_name: "Lina",
    last_name: "Muster",
    date_of_birth: "2018-04-15",
    target_grade_level: 2,
    status: "submitted",
    activation_mode: "manual",
    ...overrides,
  };
}

function summary(
  overrides: Partial<AdminRequestSummary> = {},
): AdminRequestSummary {
  return {
    id: "request-1",
    phase_id: "phase-1",
    phase_name: "Schuljahr 2026",
    guardian_first_name: "Mara",
    guardian_last_name: "Muster",
    guardian_email: "mara@example.test",
    guardian_phone: "+491234",
    submitted_at: "2026-01-01T00:00:00Z",
    children: [child()],
    ...overrides,
    care_offering_selection_mode:
      overrides.care_offering_selection_mode ?? "optional",
  };
}

function detail(
  overrides: Partial<AdminRequestDetail> = {},
): AdminRequestDetail {
  return {
    ...summary(),
    status_token: "status-token",
    ...overrides,
  };
}

function adjustment(
  overrides: Partial<AdminOfferingAdjustment> = {},
): AdminOfferingAdjustment {
  return {
    id: "adjustment-1",
    request_id: "request-1",
    request_child_id: "child-1",
    student_id: "student-1",
    actor_account_id: "account-1",
    actor_role: "admin",
    reason: "Korrektur",
    before: [],
    after: [],
    changed_at: "2026-01-02T00:00:00Z",
    ...overrides,
  };
}

function changeRequest(
  overrides: Partial<AdminEnrollmentChangeRequest> = {},
): AdminEnrollmentChangeRequest {
  return {
    id: "change-1",
    request_id: "request-1",
    origin: "parent",
    status: "pending_review",
    parent_note: "Bitte prüfen",
    base_snapshot: {},
    proposed_snapshot: {},
    diff: {},
    created_at: "2026-01-03T00:00:00Z",
    updated_at: "2026-01-03T00:00:00Z",
    ...overrides,
  };
}

function responseError(status = 500) {
  return new Response("not json", { status });
}

describe("enrollment-admin-api", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    global.fetch = mockFetch;
    anchor = {
      href: "",
      download: "",
      click: vi.fn(),
      remove: vi.fn(),
    };
    vi.spyOn(document, "createElement").mockReturnValue(
      anchor as unknown as HTMLAnchorElement,
    );
    vi.spyOn(document.body, "append").mockImplementation(() => undefined);
    URL.createObjectURL = vi.fn(() => "blob:student-enrollment-export");
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
    URL.createObjectURL = originalCreateObjectURL;
    URL.revokeObjectURL = originalRevokeObjectURL;
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("lists admin requests with filters and normalizes non-array payloads", async () => {
    const request = summary();
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: [request] }));

    await expect(
      listAdminRequests({ phaseId: "phase/1", childStatus: "approved" }),
    ).resolves.toEqual([request]);
    expect(mockFetch).toHaveBeenLastCalledWith(
      "/api/enrollment/admin/requests?phase_id=phase%2F1&child_status=approved",
      { cache: "no-store" },
    );

    mockFetch.mockResolvedValueOnce(jsonResponse({ data: { id: "bad" } }));
    await expect(listAdminRequests()).resolves.toEqual([]);
    expect(mockFetch).toHaveBeenLastCalledWith(
      "/api/enrollment/admin/requests",
      { cache: "no-store" },
    );
  });

  it("builds list URLs without relying on a browser window", async () => {
    vi.stubGlobal("window", undefined);
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: [] }));

    await expect(listAdminRequests()).resolves.toEqual([]);

    expect(mockFetch).toHaveBeenCalledWith("/api/enrollment/admin/requests", {
      cache: "no-store",
    });
  });

  it("loads admin request details and student request history", async () => {
    const requestDetail = detail({ id: "request/1" });
    const studentRequests = [summary({ id: "request-2" })];
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: requestDetail }));
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: studentRequests }));

    await expect(getAdminRequest("request/1")).resolves.toEqual(requestDetail);
    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "/api/enrollment/admin/requests/request%2F1",
      { cache: "no-store" },
    );

    await expect(listStudentEnrollmentRequests("student/1")).resolves.toEqual(
      studentRequests,
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "/api/enrollment/admin/students/student%2F1/requests",
      { cache: "no-store" },
    );

    mockFetch.mockResolvedValueOnce(jsonResponse({ data: null }));
    await expect(listStudentEnrollmentRequests("student-1")).resolves.toEqual(
      [],
    );
  });

  it("downloads student enrollment exports with server and fallback filenames", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(new Blob(["PDF"]), {
        headers: {
          "content-disposition": 'attachment; filename="student-requests.pdf"',
        },
      }),
    );

    await exportStudentEnrollmentRequests("student/1", "pdf");

    expect(mockFetch).toHaveBeenLastCalledWith(
      "/api/enrollment/admin/students/student%2F1/requests/export",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ format: "pdf" }),
      },
    );
    expect(anchor.href).toBe("blob:student-enrollment-export");
    expect(anchor.download).toBe("student-requests.pdf");
    expect(document.body.append).toHaveBeenCalledWith(anchor);
    expect(anchor.click).toHaveBeenCalledTimes(1);
    expect(anchor.remove).toHaveBeenCalledTimes(1);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith(
      "blob:student-enrollment-export",
    );

    mockFetch.mockResolvedValueOnce(new Response(new Blob(["XLSX"])));
    await exportStudentEnrollmentRequests("student-1", "xlsx");
    expect(anchor.download).toBe("anmeldungen.xlsx");
  });

  it("decides children and defaults missing reasons to an empty string", async () => {
    const approvedChild = child({ status: "approved" });
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: approvedChild }));

    await expect(
      decideAdminChild("request-1", "child-1", "approved"),
    ).resolves.toEqual(approvedChild);
    expect(mockFetch).toHaveBeenLastCalledWith("/api/enrollment/admin/decide", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        request_id: "request-1",
        child_id: "child-1",
        status: "approved",
        reason: "",
      }),
    });

    mockFetch.mockResolvedValueOnce(
      jsonResponse(child({ status: "rejected" })),
    );
    await decideAdminChild("request-1", "child-1", "rejected", "Doppelung");
    expect(
      JSON.parse(mockFetch.mock.calls.at(-1)?.[1]?.body as string),
    ).toEqual({
      request_id: "request-1",
      child_id: "child-1",
      status: "rejected",
      reason: "Doppelung",
    });
  });

  it("updates child offerings and lists offering adjustments", async () => {
    const updatedChild = child({
      offerings: [
        {
          offering_id: "offering-1",
          offering_name: "Nachmittag",
          days_of_week_mode: "parent_choice",
          selected_days: ["mon"],
        },
      ],
    });
    const history = [adjustment()];
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: updatedChild }));
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: history }));

    await expect(
      updateAdminChildOfferings("request-1", "child-1", {
        offerings: [{ offering_id: "offering-1", selected_days: ["mon"] }],
        reason: "Elternwunsch",
      }),
    ).resolves.toEqual(updatedChild);
    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "/api/enrollment/admin/offerings",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          request_id: "request-1",
          child_id: "child-1",
          offerings: [{ offering_id: "offering-1", selected_days: ["mon"] }],
          reason: "Elternwunsch",
        }),
      },
    );

    await expect(
      listAdminChildOfferingAdjustments("request/1", "child/1"),
    ).resolves.toEqual(history);
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "/api/enrollment/admin/offering-adjustments?request_id=request%2F1&child_id=child%2F1",
      { cache: "no-store" },
    );

    mockFetch.mockResolvedValueOnce(jsonResponse({ data: { id: "bad" } }));
    await expect(
      listAdminChildOfferingAdjustments("request-1", "child-1"),
    ).resolves.toEqual([]);
  });

  it("corrects approved child data through the flattened admin route", async () => {
    const audit = changeRequest({ origin: "admin", status: "approved" });
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: audit }));

    await expect(
      correctAdminChildData("request-1", "child-1", {
        first_name: "Lina",
        last_name: "Richtig",
        date_of_birth: "2018-05-16",
        target_grade_level: 2,
        target_school_class: "2a",
        reason: "Nach Rücksprache korrigiert",
      }),
    ).resolves.toEqual(audit);
    expect(mockFetch).toHaveBeenLastCalledWith(
      "/api/enrollment/admin/data-correction",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          request_id: "request-1",
          child_id: "child-1",
          first_name: "Lina",
          last_name: "Richtig",
          date_of_birth: "2018-05-16",
          target_grade_level: 2,
          target_school_class: "2a",
          reason: "Nach Rücksprache korrigiert",
        }),
      },
    );
  });

  it("lists and loads admin enrollment change requests", async () => {
    const request = changeRequest();
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: [request] }));
    mockFetch.mockResolvedValueOnce(jsonResponse(request));

    await expect(
      listAdminEnrollmentChangeRequests({
        requestId: "request/1",
        status: "needs_parent_response",
      }),
    ).resolves.toEqual([request]);
    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "/api/enrollment/admin/change-requests?request_id=request%2F1&status=needs_parent_response",
      { cache: "no-store" },
    );

    await expect(getAdminEnrollmentChangeRequest("change/1")).resolves.toEqual(
      request,
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "/api/enrollment/admin/change-requests/change%2F1",
      { cache: "no-store" },
    );

    mockFetch.mockResolvedValueOnce(jsonResponse({ data: null }));
    await expect(listAdminEnrollmentChangeRequests()).resolves.toEqual([]);
  });

  it("posts admin change-request decisions", async () => {
    const asked = changeRequest({ status: "needs_parent_response" });
    const approved = changeRequest({ status: "approved" });
    const rejected = changeRequest({ status: "rejected" });
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: asked }));
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: approved }));
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: rejected }));

    await expect(
      askEnrollmentChangeRequestQuestion(
        "change/1",
        "Bitte Nachweis hochladen",
      ),
    ).resolves.toEqual(asked);
    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "/api/enrollment/admin/change-requests/change%2F1/question",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ body: "Bitte Nachweis hochladen" }),
      },
    );

    await expect(
      approveEnrollmentChangeRequest("change/1", "Passt"),
    ).resolves.toEqual(approved);
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "/api/enrollment/admin/change-requests/change%2F1/approve",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ note: "Passt" }),
      },
    );

    await expect(
      rejectEnrollmentChangeRequest("change/1", "Unvollständig"),
    ).resolves.toEqual(rejected);
    expect(mockFetch).toHaveBeenNthCalledWith(
      3,
      "/api/enrollment/admin/change-requests/change%2F1/reject",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ note: "Unvollständig" }),
      },
    );
  });

  it("surfaces endpoint-specific fallback messages on admin failures", async () => {
    const failingCalls: Array<[() => Promise<unknown>, string]> = [
      [() => listAdminRequests(), "Anmeldungen konnten nicht geladen werden"],
      [
        () => getAdminRequest("request-1"),
        "Anmeldung konnte nicht geladen werden",
      ],
      [
        () => listStudentEnrollmentRequests("student-1"),
        "Anmeldungen konnten nicht geladen werden",
      ],
      [
        () => exportStudentEnrollmentRequests("student-1", "pdf"),
        "Export konnte nicht erstellt werden",
      ],
      [
        () => decideAdminChild("request-1", "child-1", "approved"),
        "Entscheidung konnte nicht gespeichert werden",
      ],
      [
        () =>
          updateAdminChildOfferings("request-1", "child-1", {
            offerings: [],
            reason: "Korrektur",
          }),
        "Betreuungsangebote konnten nicht gespeichert werden",
      ],
      [
        () => listAdminChildOfferingAdjustments("request-1", "child-1"),
        "Änderungshistorie konnte nicht geladen werden",
      ],
      [
        () => listAdminEnrollmentChangeRequests(),
        "Änderungsanfragen konnten nicht geladen werden",
      ],
      [
        () => getAdminEnrollmentChangeRequest("change-1"),
        "Änderungsanfrage konnte nicht geladen werden",
      ],
      [
        () => askEnrollmentChangeRequestQuestion("change-1", "Frage"),
        "Rückfrage konnte nicht gesendet werden",
      ],
      [
        () => approveEnrollmentChangeRequest("change-1", "OK"),
        "Änderungsanfrage konnte nicht freigegeben werden",
      ],
      [
        () => rejectEnrollmentChangeRequest("change-1", "Nein"),
        "Änderungsanfrage konnte nicht abgelehnt werden",
      ],
    ];

    expect.assertions(failingCalls.length);
    for (const [call, message] of failingCalls) {
      mockFetch.mockResolvedValueOnce(responseError());
      await expect(call()).rejects.toThrow(message);
    }
  });
});
