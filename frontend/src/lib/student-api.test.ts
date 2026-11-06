import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { mockSessionFetch } = vi.hoisted(() => ({
  mockSessionFetch:
    vi.fn<(url: string, init?: RequestInit) => Promise<Response>>(),
}));

vi.mock("./session-cache", () => ({ sessionFetch: mockSessionFetch }));

import { ApiError } from "./api-error";
import { buildStudent } from "~/test/fixtures/students";
import { subscribeStudentCompanionsChanged } from "./student-companion-api";
import {
  deleteStudentPhoto,
  deleteStudentWithData,
  fetchPrivacyConsentDefaults,
  fetchSchoolClasses,
  fetchStudent,
  fetchStudentDeletionImpact,
  fetchStudentEnrollmentExtraFields,
  fetchStudentPrivacyConsent,
  fetchStudents,
  updateStudent,
  uploadStudentPhoto,
} from "./student-api";
import {
  CompanionPlanConflictError,
  isCompanionDepartureRefusal,
  isCompanionsChanged,
  isPrivacyConsentSaved,
} from "./student-update-errors";

/** A route-wrapper success answer. */
function ok(data: unknown, status = 200): Response {
  return new Response(JSON.stringify({ success: true, message: "", data }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function problem(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/problem+json" },
  });
}

function calledUrl(index = 0): string {
  return mockSessionFetch.mock.calls[index]![0];
}

function calledInit(index = 0): RequestInit {
  return mockSessionFetch.mock.calls[index]![1] ?? {};
}

const page = {
  current_page: 1,
  page_size: 50,
  total_pages: 3,
  total_records: 120,
};

beforeEach(() => {
  mockSessionFetch.mockReset();
});

describe("fetchStudents", () => {
  it("answers the mapped students and the page of the route", async () => {
    const student = buildStudent({ id: "1", name: "Max Mustermann" });
    mockSessionFetch.mockResolvedValueOnce(
      ok({ data: [student], pagination: page }),
    );

    await expect(fetchStudents()).resolves.toEqual({
      students: [student],
      pagination: page,
    });
    expect(calledUrl()).toBe("/api/students");
  });

  it("sends every filter under its backend name", async () => {
    mockSessionFetch.mockResolvedValueOnce(ok({ data: [], pagination: page }));

    await fetchStudents({
      search: "Max",
      inHouse: false,
      groupId: ["5", "a,b"],
      schoolClass: "3a",
      gradeLevel: ["1", "2"],
      roomId: "42",
      locationState: "present",
      dayStatus: "comes_today",
      date: "2026-09-10",
      bus: "yes",
      photoConsent: "no",
      pickupStatus: "self",
      careOfferingId: "9",
      page: 2,
      pageSize: 200,
      includePickupTimes: true,
      includeArrivalTimes: true,
      includeCompanions: true,
      view: "slim",
    });

    const params = new URL(calledUrl(), "http://x").searchParams;
    expect(Object.fromEntries(params)).toEqual({
      search: "Max",
      // false is a filter of its own on in_house.
      in_house: "false",
      group_id: "5,a\\,b",
      school_class: "3a",
      grade_level: "1,2",
      room_id: "42",
      location_state: "present",
      day_status: "comes_today",
      date: "2026-09-10",
      bus: "yes",
      photo_consent: "no",
      pickup_status: "self",
      care_offering_id: "9",
      page: "2",
      page_size: "200",
      include_pickup_times: "true",
      include_arrival_times: "true",
      include_companions: "true",
      view: "slim",
    });
  });

  it("leaves unset and empty filters out", async () => {
    mockSessionFetch.mockResolvedValueOnce(ok({ data: [], pagination: page }));

    await fetchStudents({
      search: "",
      groupId: [],
      includePickupTimes: false,
      page: 0,
    });

    expect(calledUrl()).toBe("/api/students");
  });

  it("keeps the code of a refused list for the shared error path", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      problem(403, { status: 403, code: "general.permission" }),
    );

    await expect(fetchStudents()).rejects.toMatchObject({
      status: 403,
      code: "general.permission",
    });
  });
});

describe("single reads", () => {
  it("reads the school classes", async () => {
    mockSessionFetch.mockResolvedValueOnce(ok(["1a", "2b"]));

    await expect(fetchSchoolClasses()).resolves.toEqual(["1a", "2b"]);
    expect(calledUrl()).toBe("/api/students/school-classes");
  });

  it("reads one child under its encoded id", async () => {
    const student = buildStudent({ id: "7", has_write_access: true });
    mockSessionFetch.mockResolvedValueOnce(ok(student));

    await expect(fetchStudent("7/x")).resolves.toEqual(student);
    expect(calledUrl()).toBe("/api/students/7%2Fx");
  });

  it("reads the enrollment extra fields of a child", async () => {
    const groups = [
      { request_id: "1", phase_name: "2026", submitted_at: "", fields: [] },
    ];
    mockSessionFetch.mockResolvedValueOnce(ok(groups));

    await expect(fetchStudentEnrollmentExtraFields("7")).resolves.toEqual(
      groups,
    );
    expect(calledUrl()).toBe("/api/students/7/enrollment-extra-fields");
  });
});

describe("updateStudent", () => {
  let listener: ReturnType<typeof vi.fn<() => void>>;
  let unsubscribe: () => void;

  beforeEach(() => {
    listener = vi.fn<() => void>();
    unsubscribe = subscribeStudentCompanionsChanged(listener);
  });

  afterEach(() => unsubscribe());

  it("sends the update and answers the route's child unchanged", async () => {
    // The route already maps the child; mapping it again lost the surname
    // and the tag id.
    const routeStudent = buildStudent({
      id: "1",
      name: "Max Mustermann",
      first_name: "Max",
      second_name: "Mustermann",
      studentId: "TAG001",
      sick: true,
    });
    mockSessionFetch.mockResolvedValueOnce(ok(routeStudent));

    const result = await updateStudent("1", { sick: true });

    expect(result).toEqual(routeStudent);
    expect(calledUrl()).toBe("/api/students/1");
    expect(calledInit()).toMatchObject({ method: "PUT" });
    expect(JSON.parse(String(calledInit().body))).toEqual({ sick: true });
  });

  // An open Laufgemeinschaft form answers the announcement by discarding its
  // draft, so only a write that changed the links announces one.
  it("follows the route's verdict on the companion links", async () => {
    mockSessionFetch
      .mockResolvedValueOnce(
        ok({ ...buildStudent(), companions_changed: false }),
      )
      .mockResolvedValueOnce(
        ok({ ...buildStudent(), companions_changed: true }),
      );

    await updateStudent("1", { departure_days: { mon: "accompanied" } });
    expect(listener).not.toHaveBeenCalled();

    await updateStudent("1", { first_name: "Updated" });
    expect(listener).toHaveBeenCalledTimes(1);
  });

  // The student PUT answers 409 for several unrelated reasons. Only the one
  // carrying a conflict list is the question the caller can answer with
  // extend_companion_plans.
  it("types only a 409 carrying conflicts as the companion question", async () => {
    mockSessionFetch
      .mockResolvedValueOnce(
        problem(409, {
          detail: "linked child may not walk along",
          conflicts: [{ student_id: 42, weekdays: ["mon"] }],
        }),
      )
      .mockResolvedValueOnce(
        problem(409, {
          detail: "a student cannot be both sick and excused",
          code: "students.sick_excused_conflict",
        }),
      );

    const conflict = await updateStudent("1", { first_name: "A" }).catch(
      (err: unknown) => err,
    );
    expect(conflict).toBeInstanceOf(CompanionPlanConflictError);
    expect(conflict).toMatchObject({
      status: 409,
      conflicts: [{ companion_student_id: "42", weekdays: ["mon"] }],
    });

    const sickExcused = await updateStudent("1", { first_name: "A" }).catch(
      (err: unknown) => err,
    );
    expect(sickExcused).not.toBeInstanceOf(CompanionPlanConflictError);
    expect(sickExcused).toMatchObject({
      status: 409,
      code: "students.sick_excused_conflict",
    });
  });

  it("recognises the stranded-companion and stale-list refusals by code", async () => {
    mockSessionFetch
      .mockResolvedValueOnce(
        problem(400, {
          status: 400,
          detail: "removing the link would strand a linked companion",
          code: "students.companion_would_lose_departure",
        }),
      )
      .mockResolvedValueOnce(
        problem(400, { status: 400, detail: "invalid", code: "general.input" }),
      )
      .mockResolvedValueOnce(
        problem(409, {
          status: 409,
          detail: "companion list changed since it was loaded",
          code: "students.companions_changed",
        }),
      );

    const stranded = await updateStudent("1", {}).catch((err: unknown) => err);
    const otherBadRequest = await updateStudent("1", {}).catch(
      (err: unknown) => err,
    );
    const stale = await updateStudent("1", {}).catch((err: unknown) => err);

    expect(stranded).toBeInstanceOf(ApiError);
    expect(isCompanionDepartureRefusal(stranded)).toBe(true);
    expect(isCompanionDepartureRefusal(otherBadRequest)).toBe(false);
    expect(stale).not.toBeInstanceOf(CompanionPlanConflictError);
    expect(isCompanionsChanged(stale)).toBe(true);
  });

  // The proxy stamps the marker onto exactly these refusals when the consent
  // half of the same request already committed.
  it("keeps the half-saved consent marker on every refusal", async () => {
    const details = { privacy_consent_saved: true };
    mockSessionFetch
      .mockResolvedValueOnce(
        problem(409, {
          detail: "companion may not walk along on this day",
          conflicts: [{ student_id: 7, weekdays: ["mon"] }],
          details,
        }),
      )
      .mockResolvedValueOnce(
        problem(409, { code: "students.companions_changed", details }),
      )
      .mockResolvedValueOnce(
        problem(409, { code: "students.companions_changed" }),
      );

    const conflict = await updateStudent("1", {}).catch((err: unknown) => err);
    const stale = await updateStudent("1", {}).catch((err: unknown) => err);
    const unmarked = await updateStudent("1", {}).catch((err: unknown) => err);

    expect(conflict).toBeInstanceOf(CompanionPlanConflictError);
    expect(isPrivacyConsentSaved(conflict)).toBe(true);
    expect(isPrivacyConsentSaved(stale)).toBe(true);
    expect(isPrivacyConsentSaved(unmarked)).toBe(false);
  });
});

describe("guarded student deletion", () => {
  const input = {
    expected_fingerprint: "abc",
    confirmation_name: "Mia Muster",
    reason: "test_data" as const,
    acknowledged: true as const,
  };

  it("loads the deletion impact under the encoded id", async () => {
    const impact = { fingerprint: "abc", total: 0 };
    mockSessionFetch.mockResolvedValueOnce(ok(impact));

    await expect(fetchStudentDeletionImpact("42/unsafe")).resolves.toEqual(
      impact,
    );
    expect(calledUrl()).toBe("/api/students/42%2Funsafe/delete-impact");
  });

  it("uses the completion-scoped deletion endpoints", async () => {
    mockSessionFetch
      .mockResolvedValueOnce(ok({ fingerprint: "abc" }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    await fetchStudentDeletionImpact("42", "73");
    await deleteStudentWithData("42", input, "73");

    expect(calledUrl(0)).toBe(
      "/api/students/care-withdrawals/73/deletion-impact",
    );
    expect(calledUrl(1)).toBe("/api/students/care-withdrawals/73");
  });

  it("sends every confirmation field in the DELETE body", async () => {
    mockSessionFetch.mockResolvedValueOnce(ok({ success: true }));

    await deleteStudentWithData("42", input);

    expect(calledUrl()).toBe("/api/students/42");
    expect(calledInit()).toMatchObject({
      method: "DELETE",
      body: JSON.stringify(input),
    });
  });

  it("keeps the conflict code and diagnostics of a stale preview", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      problem(409, {
        status: 409,
        detail: "Vorschau veraltet",
        code: "students.deletion_preview_changed",
      }),
    );

    await expect(deleteStudentWithData("42", input)).rejects.toMatchObject({
      status: 409,
      detail: "Vorschau veraltet",
      code: "students.deletion_preview_changed",
    });
  });
});

describe("student photo", () => {
  it("posts the photo as multipart form data and answers its URL", async () => {
    mockSessionFetch.mockResolvedValueOnce(ok({ photo_url: "/uploads/p.jpg" }));

    const result = await uploadStudentPhoto(
      "7",
      new Blob(["x"], { type: "image/png" }),
      { consentAcknowledged: true },
    );

    expect(result).toEqual({ photoUrl: "/uploads/p.jpg" });
    expect(calledUrl()).toBe("/api/students/7/photo");
    const body = calledInit().body as FormData;
    expect(body).toBeInstanceOf(FormData);
    expect((body.get("photo") as File).type).toBe("image/png");
    expect(body.get("consent_acknowledged")).toBe("true");
  });

  it("keeps the file name of a File and sends no consent unasked", async () => {
    mockSessionFetch.mockResolvedValueOnce(ok({ photo_url: "/uploads/p.jpg" }));

    await uploadStudentPhoto(
      "7",
      new File(["x"], "mia.jpg", { type: "image/jpeg" }),
    );

    const body = calledInit().body as FormData;
    expect((body.get("photo") as File).name).toBe("mia.jpg");
    expect(body.has("consent_acknowledged")).toBe(false);
  });

  it("keeps code and request ID of a failed upload", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      problem(400, {
        status: 400,
        code: "general.input",
        instance: "req-1",
      }),
    );

    await expect(
      uploadStudentPhoto("7", new Blob(["x"])),
    ).rejects.toMatchObject({
      status: 400,
      code: "general.input",
      instance: "req-1",
    });
  });

  it("deletes the photo", async () => {
    mockSessionFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(deleteStudentPhoto("7")).resolves.toBeUndefined();
    expect(calledUrl()).toBe("/api/students/7/photo");
    expect(calledInit()).toMatchObject({ method: "DELETE" });
  });
});

describe("privacy consent", () => {
  it("maps the stored consent", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      ok({
        id: 1,
        student_id: 7,
        policy_version: "1.0",
        accepted: true,
        accepted_at: "2026-01-15T10:00:00Z",
        renewal_required: false,
        data_retention_days: 21,
        created_at: "2026-01-15T10:00:00Z",
        updated_at: "2026-01-15T10:00:00Z",
      }),
    );

    const consent = await fetchStudentPrivacyConsent("7");

    expect(calledUrl()).toBe("/api/students/7/privacy-consent");
    expect(consent).toEqual({ accepted: true, dataRetentionDays: 21 });
  });

  it("maps the school's defaults of a child without a stored consent", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      ok({
        policy_version: "1.0",
        accepted: false,
        renewal_required: true,
        data_retention_days: 14,
      }),
    );

    await expect(fetchStudentPrivacyConsent("7")).resolves.toMatchObject({
      accepted: false,
      dataRetentionDays: 14,
    });
  });

  it("reads the defaults a new child starts with", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      ok({ policy_version: "1.0", accepted: false, data_retention_days: 12 }),
    );

    await expect(fetchPrivacyConsentDefaults()).resolves.toEqual({
      accepted: false,
      dataRetentionDays: 12,
    });
    expect(calledUrl()).toBe("/api/students/privacy-consent/defaults");
  });
});
