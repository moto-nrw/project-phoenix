// grade-transition-api.requests.test.ts
// Request-layer tests for the Jahrgangsstufenwechsel admin API client (#405).
// The mapper tests live in grade-transition-api.test.ts; this file covers the
// fetch plumbing: request shape (method, URL, body), the { data } unwrapping,
// and the error paths that must preserve the backend's stable error code.

import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import {
  createGradeTransition,
  updateGradeTransition,
  deleteGradeTransition,
  previewGradeTransition,
  applyGradeTransition,
  revertGradeTransition,
  fetchTransitionClasses,
  fetchSuggestedMappings,
  fetchTransitionHistory,
  purgeGraduatedStudent,
  listGradeTransitions,
  TransitionRequestError,
  GRADUATES_CHECKED_IN_CODE,
  NOT_APPLIED_CODE,
  NOT_LATEST_TRANSITION_CODE,
  PREVIEW_STALE_CODE,
  NOT_DRAFT_CODE,
  type BackendGradeTransition,
  type BackendTransitionPreview,
  type BackendTransitionResult,
  type BackendSuggestedMapping,
  type BackendTransitionHistoryEntry,
} from "./grade-transition-api";

const BASE = "/api/admin/grade-transitions";

interface FetchCall {
  url: string;
  init?: RequestInit;
}

let calls: FetchCall[] = [];

/** Stubs global fetch with a single canned response for every call. */
function stubOnce(response: Partial<Response> & { json?: () => unknown }) {
  vi.stubGlobal(
    "fetch",
    vi.fn((url: string, init?: RequestInit) => {
      calls.push({ url, init });
      return Promise.resolve(response as unknown as Response);
    }),
  );
}

/** An ok response whose body is the given value wrapped in { data }. */
const okData = (data: unknown) => ({
  ok: true,
  status: 200,
  json: () => Promise.resolve({ data }),
});

/** An ok response whose body IS the payload (no { data } envelope). */
const okBare = (body: unknown) => ({
  ok: true,
  status: 200,
  json: () => Promise.resolve(body),
});

/** A failing response carrying the backend's { error, code } shape. */
const fail = (status: number, body: unknown) => ({
  ok: false,
  status,
  json: () => Promise.resolve(body),
});

/** A failing response whose body is not JSON at all (proxy/gateway HTML). */
const failUnparseable = (status: number) => ({
  ok: false,
  status,
  json: () => Promise.reject(new SyntaxError("Unexpected token <")),
});

const parseBody = (init?: RequestInit) =>
  JSON.parse(String(init?.body ?? "null")) as Record<string, unknown>;

const transitionRow: BackendGradeTransition = {
  id: "7",
  academic_year: "2026-2027",
  status: "draft",
  created_at: "2026-07-24T09:00:00Z",
  created_by: "42",
  can_modify: true,
  can_apply: true,
  can_revert: false,
};

const resultRow: BackendTransitionResult = {
  transition_id: "7",
  status: "applied",
  students_promoted: 40,
  students_graduated: 10,
  can_revert: true,
};

beforeEach(() => {
  calls = [];
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("createGradeTransition", () => {
  it("POSTs the camelCase input as snake_case and maps the answer", async () => {
    stubOnce(okData(transitionRow));

    const created = await createGradeTransition({
      academicYear: "2026-2027",
      notes: "Sommer-Wechsel",
      mappings: [
        { fromClass: "1a", toClass: "2a" },
        { fromClass: "4a", toClass: null },
      ],
    });

    expect(created.id).toBe("7");
    expect(created.academicYear).toBe("2026-2027");

    const call = calls[0];
    expect(call?.url).toBe(BASE);
    expect(call?.init?.method).toBe("POST");
    expect(parseBody(call?.init)).toEqual({
      academic_year: "2026-2027",
      notes: "Sommer-Wechsel",
      // null to_class is the Abgang marker and must survive JSON.stringify —
      // dropping it would silently turn a graduation into an unmapped class.
      mappings: [
        { from_class: "1a", to_class: "2a" },
        { from_class: "4a", to_class: null },
      ],
    });
  });

  it("throws the backend error message on failure", async () => {
    stubOnce(fail(400, { error: "academic_year is required" }));

    await expect(
      createGradeTransition({ academicYear: "", mappings: [] }),
    ).rejects.toThrow("academic_year is required");
  });

  it("falls back to the status line when the body has no message", async () => {
    stubOnce(fail(502, {}));

    await expect(
      createGradeTransition({ academicYear: "2026-2027", mappings: [] }),
    ).rejects.toThrow("HTTP 502");
  });

  it("falls back to the status line when the body is not JSON", async () => {
    stubOnce(failUnparseable(504));

    await expect(
      createGradeTransition({ academicYear: "2026-2027", mappings: [] }),
    ).rejects.toThrow("HTTP 504");
  });

  it("uses `message` when the backend sends that key instead of `error`", async () => {
    stubOnce(fail(400, { message: "ungültiges Schuljahr" }));

    await expect(
      createGradeTransition({ academicYear: "x", mappings: [] }),
    ).rejects.toThrow("ungültiges Schuljahr");
  });
});

describe("updateGradeTransition", () => {
  it("PUTs to the escaped id and omits mappings when undefined", async () => {
    stubOnce(okData(transitionRow));

    await updateGradeTransition("7", { notes: "neu" });

    const call = calls[0];
    expect(call?.url).toBe(`${BASE}/7`);
    expect(call?.init?.method).toBe("PUT");
    // `mappings: undefined` must not appear in the payload at all — the backend
    // treats an explicit [] as "remove every mapping".
    expect(parseBody(call?.init)).toEqual({ notes: "neu" });
  });

  it("sends an explicit empty list when mappings are cleared", async () => {
    stubOnce(okData(transitionRow));

    await updateGradeTransition("7", {
      academicYear: "2027-2028",
      mappings: [],
    });

    expect(parseBody(calls[0]?.init)).toEqual({
      academic_year: "2027-2028",
      mappings: [],
    });
  });

  it("escapes the id in the URL", async () => {
    stubOnce(okData(transitionRow));

    await updateGradeTransition("a b/c", { notes: "x" });

    expect(calls[0]?.url).toBe(`${BASE}/a%20b%2Fc`);
  });

  it("keeps the not_draft code so the editor can stop offering a retry", async () => {
    stubOnce(
      fail(409, {
        error: "transition is no longer a draft",
        code: NOT_DRAFT_CODE,
      }),
    );

    await expect(
      updateGradeTransition("7", { notes: "x" }),
    ).rejects.toMatchObject({
      name: "TransitionRequestError",
      code: NOT_DRAFT_CODE,
      message: "transition is no longer a draft",
    });
  });
});

describe("deleteGradeTransition", () => {
  it("DELETEs the escaped id", async () => {
    stubOnce({ ok: true, status: 204 });

    await expect(deleteGradeTransition("7")).resolves.toBeUndefined();
    expect(calls[0]?.url).toBe(`${BASE}/7`);
    expect(calls[0]?.init?.method).toBe("DELETE");
  });

  it("keeps the stable code on a conflict", async () => {
    stubOnce(fail(409, { error: "already applied", code: NOT_DRAFT_CODE }));

    const err = await deleteGradeTransition("7").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(TransitionRequestError);
    expect((err as TransitionRequestError).code).toBe(NOT_DRAFT_CODE);
  });

  it("leaves the code undefined when the backend sends none", async () => {
    stubOnce(fail(500, { error: "boom" }));

    const err = await deleteGradeTransition("7").catch((e: unknown) => e);
    expect((err as TransitionRequestError).code).toBeUndefined();
    expect((err as TransitionRequestError).message).toBe("boom");
  });
});

describe("previewGradeTransition", () => {
  const preview: BackendTransitionPreview = {
    transition_id: "7",
    academic_year: "2026-2027",
    total_students: 50,
    to_promote: 40,
    to_graduate: 10,
    by_mapping: [
      {
        from_class: "1a",
        to_class: "2a",
        student_count: 20,
        action: "promote",
      },
    ],
    unmapped_classes: null,
    warnings: null,
    fingerprint: "sha256:abc",
  };

  it("GETs the preview uncached and carries the fingerprint through", async () => {
    stubOnce(okData(preview));

    const p = await previewGradeTransition("7");

    expect(calls[0]?.url).toBe(`${BASE}/7/preview`);
    expect(calls[0]?.init?.cache).toBe("no-store");
    expect(p.fingerprint).toBe("sha256:abc");
    expect(p.unmappedClasses).toEqual([]);
    expect(p.warnings).toEqual([]);
  });

  it("degrades a fingerprint-less response to an empty string", async () => {
    // An older backend answers without the digest. Apply then sends no
    // expected_fingerprint rather than a bogus one.
    stubOnce(okData({ ...preview, fingerprint: undefined }));

    expect((await previewGradeTransition("7")).fingerprint).toBe("");
  });

  it("accepts a bare payload without the { data } envelope", async () => {
    stubOnce(okBare(preview));

    expect((await previewGradeTransition("7")).transitionId).toBe("7");
  });

  it("throws the backend message on failure", async () => {
    stubOnce(fail(404, { error: "transition not found" }));

    await expect(previewGradeTransition("7")).rejects.toThrow(
      "transition not found",
    );
  });
});

describe("applyGradeTransition", () => {
  it("binds the call to the confirmed preview via expected_fingerprint", async () => {
    stubOnce(okData(resultRow));

    const r = await applyGradeTransition("7", "sha256:abc");

    expect(calls[0]?.url).toBe(`${BASE}/7/apply`);
    expect(calls[0]?.init?.method).toBe("POST");
    expect(parseBody(calls[0]?.init)).toEqual({
      expected_fingerprint: "sha256:abc",
    });
    expect(r.studentsPromoted).toBe(40);
    expect(r.studentsGraduated).toBe(10);
    expect(r.canRevert).toBe(true);
    expect(r.warnings).toEqual([]);
  });

  it("sends an empty body when no fingerprint is known", async () => {
    stubOnce(okData(resultRow));

    await applyGradeTransition("7");

    expect(parseBody(calls[0]?.init)).toEqual({});
  });

  it("sends an empty body for an empty-string fingerprint", async () => {
    // previewGradeTransition degrades a missing digest to "", and an empty
    // expected_fingerprint would be a claim the backend cannot match.
    stubOnce(okData(resultRow));

    await applyGradeTransition("7", "");

    expect(parseBody(calls[0]?.init)).toEqual({});
  });

  it("reads a result sent without the { data } envelope", async () => {
    stubOnce(okBare(resultRow));

    expect((await applyGradeTransition("7")).transitionId).toBe("7");
  });

  it("keeps the graduates_checked_in code on a 409", async () => {
    stubOnce(
      fail(409, {
        error: "2 graduating children are still checked in",
        code: GRADUATES_CHECKED_IN_CODE,
      }),
    );

    const err = await applyGradeTransition("7").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(TransitionRequestError);
    expect((err as TransitionRequestError).code).toBe(
      GRADUATES_CHECKED_IN_CODE,
    );
  });

  it("keeps the preview_stale code on a 409", async () => {
    stubOnce(
      fail(409, { error: "preview is stale", code: PREVIEW_STALE_CODE }),
    );

    const err = await applyGradeTransition("7", "sha256:old").catch(
      (e: unknown) => e,
    );
    expect((err as TransitionRequestError).code).toBe(PREVIEW_STALE_CODE);
  });

  it("falls back to the status line when the error body is unreadable", async () => {
    stubOnce(failUnparseable(500));

    const err = await applyGradeTransition("7").catch((e: unknown) => e);
    expect((err as TransitionRequestError).message).toBe("HTTP 500");
    expect((err as TransitionRequestError).code).toBeUndefined();
  });
});

describe("revertGradeTransition", () => {
  it("POSTs an empty body to the revert route", async () => {
    stubOnce(
      okData({ ...resultRow, status: "reverted", warnings: ["hinweis"] }),
    );

    const r = await revertGradeTransition("7");

    expect(calls[0]?.url).toBe(`${BASE}/7/revert`);
    expect(calls[0]?.init?.method).toBe("POST");
    expect(parseBody(calls[0]?.init)).toEqual({});
    expect(r.status).toBe("reverted");
    expect(r.warnings).toEqual(["hinweis"]);
  });

  it("keeps the not_latest_transition code so the caller can reload", async () => {
    stubOnce(
      fail(409, {
        error: "a newer transition was applied",
        code: NOT_LATEST_TRANSITION_CODE,
      }),
    );

    const err = await revertGradeTransition("7").catch((e: unknown) => e);
    expect((err as TransitionRequestError).code).toBe(
      NOT_LATEST_TRANSITION_CODE,
    );
  });

  // #405 review: reverting a transition another admin already reverted (or
  // that is still a draft) now answers 409 not_applied instead of 500, so the
  // manager can reload the list instead of offering a hopeless retry.
  it("keeps the not_applied code so the caller can reload", async () => {
    stubOnce(
      fail(409, {
        error: "transition has already been reverted",
        code: NOT_APPLIED_CODE,
      }),
    );

    const err = await revertGradeTransition("7").catch((e: unknown) => e);
    expect((err as TransitionRequestError).code).toBe(NOT_APPLIED_CODE);
  });
});

describe("fetchTransitionClasses", () => {
  it("returns the class list", async () => {
    stubOnce(okData(["1a", "2a", "4b"]));

    expect(await fetchTransitionClasses()).toEqual(["1a", "2a", "4b"]);
    expect(calls[0]?.url).toBe(`${BASE}/classes`);
    expect(calls[0]?.init?.cache).toBe("no-store");
  });

  it("returns an empty list when the payload is null", async () => {
    stubOnce(okData(null));

    expect(await fetchTransitionClasses()).toEqual([]);
  });

  it("throws the backend message on failure", async () => {
    stubOnce(fail(403, { error: "forbidden" }));

    await expect(fetchTransitionClasses()).rejects.toThrow("forbidden");
  });
});

describe("fetchSuggestedMappings", () => {
  it("maps every suggestion", async () => {
    const suggestions: BackendSuggestedMapping[] = [
      {
        from_class: "1a",
        to_class: "2a",
        student_count: 20,
        is_graduating: false,
      },
      {
        from_class: "Vorklasse",
        student_count: 5,
        is_graduating: false,
        ambiguous: true,
      },
    ];
    stubOnce(okData(suggestions));

    const mapped = await fetchSuggestedMappings();

    expect(calls[0]?.url).toBe(`${BASE}/suggest`);
    expect(mapped).toHaveLength(2);
    expect(mapped[0]?.toClass).toBe("2a");
    expect(mapped[1]?.toClass).toBeNull();
    expect(mapped[1]?.ambiguous).toBe(true);
  });

  it("returns an empty list when the payload is null", async () => {
    stubOnce(okData(null));

    expect(await fetchSuggestedMappings()).toEqual([]);
  });
});

describe("fetchTransitionHistory", () => {
  it("maps every ledger row for the escaped id", async () => {
    const rows: BackendTransitionHistoryEntry[] = [
      {
        id: "3",
        transition_id: "7",
        student_id: "99",
        person_name: "Mika Muster",
        from_class: "4a",
        to_class: null,
        action: "graduated",
        created_at: "2026-08-01T06:00:00Z",
        student_state: "purged",
      },
    ];
    stubOnce(okData(rows));

    const history = await fetchTransitionHistory("7");

    expect(calls[0]?.url).toBe(`${BASE}/7/history`);
    expect(calls[0]?.init?.cache).toBe("no-store");
    expect(history[0]?.studentState).toBe("purged");
  });

  it("returns an empty list when the payload is null", async () => {
    stubOnce(okData(null));

    expect(await fetchTransitionHistory("7")).toEqual([]);
  });

  it("throws the backend message on failure", async () => {
    stubOnce(fail(404, { error: "transition not found" }));

    await expect(fetchTransitionHistory("7")).rejects.toThrow(
      "transition not found",
    );
  });
});

describe("purgeGraduatedStudent", () => {
  it("DELETEs the student purge route with an escaped id", async () => {
    stubOnce({ ok: true, status: 204 });

    await expect(purgeGraduatedStudent("99")).resolves.toBeUndefined();
    expect(calls[0]?.url).toBe("/api/students/99/purge");
    expect(calls[0]?.init?.method).toBe("DELETE");
  });

  it("throws the backend message on failure", async () => {
    stubOnce(fail(409, { error: "child is not an alumnus" }));

    await expect(purgeGraduatedStudent("99")).rejects.toThrow(
      "child is not an alumnus",
    );
  });

  it("falls back to the status line when the body is unreadable", async () => {
    stubOnce(failUnparseable(500));

    await expect(purgeGraduatedStudent("99")).rejects.toThrow("HTTP 500");
  });
});

describe("listGradeTransitions request handling", () => {
  it("requests uncached pages of 100", async () => {
    stubOnce(okData([transitionRow]));

    await listGradeTransitions();

    expect(calls[0]?.url).toBe(`${BASE}?page=1&page_size=100`);
    expect(calls[0]?.init?.cache).toBe("no-store");
  });

  it("tolerates a null payload", async () => {
    stubOnce(okData(null));

    expect(await listGradeTransitions()).toEqual([]);
    expect(calls).toHaveLength(1);
  });

  it("stops at the page limit against a backend that ignores `page`", async () => {
    // A backend that returns the same full page forever would otherwise spin
    // this loop without end; the 100-page bound is the safety stop.
    const fullPage = Array.from({ length: 100 }, (_, i) => ({
      ...transitionRow,
      id: String(i + 1),
    }));
    stubOnce(okData(fullPage));

    const result = await listGradeTransitions();

    expect(calls).toHaveLength(100);
    expect(result).toHaveLength(100);
  });

  it("propagates a failing page", async () => {
    stubOnce(fail(500, { error: "database unavailable" }));

    await expect(listGradeTransitions()).rejects.toThrow(
      "database unavailable",
    );
  });
});
