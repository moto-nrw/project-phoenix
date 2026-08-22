import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

import {
  listPhases,
  createPhase,
  updatePhase,
  deletePhase,
  getPhaseDeleteImpact,
  createRollover,
  fetchRolloverPreview,
  listRolloverReview,
  decideRolloverReview,
  phaseToInput,
  setPhaseCalendarPeriod,
  type Phase,
  type PhaseDeleteImpact,
  type PhaseInput,
  type RolloverInput,
  type RolloverResult,
} from "./enrollment-phase-api";

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

function mkPhase(id: string, name: string): Phase {
  return {
    id,
    name,
    kind: "school_year",
    service_start_date: "2026-09-01",
    service_end_date: "2027-07-31",
    enrollment_open_at: null,
    enrollment_close_at: null,
    form_schema_id: "7",
    show_status_reason_to_parent: false,
    care_overflow_mode: "waitlist",
    care_offering_selection_mode: "optional",
    is_active: true,
    created_at: "2026-04-01T12:00:00Z",
    updated_at: "2026-04-01T12:00:00Z",
  };
}

const validInput: PhaseInput = {
  name: "Schuljahr 2026",
  kind: "school_year",
  service_start_date: "2026-09-01",
  service_end_date: "2027-07-31",
  show_status_reason_to_parent: false,
  care_overflow_mode: "waitlist",
  care_offering_selection_mode: "optional",
  is_active: true,
};

// --- listPhases -------------------------------------------------------

describe("listPhases", () => {
  it("returns the unwrapped data on 200", async () => {
    mockFetch(async () => jsonResponse({ data: [mkPhase("1", "X")] }));
    const out = await listPhases();
    expect(out).toHaveLength(1);
    expect(out[0]!.id).toBe("1");
  });

  it("accepts a bare array response", async () => {
    mockFetch(async () => jsonResponse([mkPhase("1", "X")]));
    const out = await listPhases();
    expect(out.map((phase) => phase.id)).toEqual(["1"]);
  });

  it("returns [] when the backend returns non-array data", async () => {
    mockFetch(async () => jsonResponse({ data: null }));
    expect(await listPhases()).toEqual([]);
  });

  it("uses cache: 'no-store'", async () => {
    let seenInit: RequestInit | undefined;
    mockFetch(async (_, init) => {
      seenInit = init;
      return jsonResponse({ data: [] });
    });
    await listPhases();
    expect(seenInit?.cache).toBe("no-store");
  });

  it("throws with the German fallback message on non-OK + non-JSON body", async () => {
    // Non-JSON body → readError's try/catch falls through with the
    // operation-specific fallback string (no HTTP suffix because
    // the suffix is appended inside the try-block only).
    mockFetch(async () => new Response("nope", { status: 500 }));
    await expect(listPhases()).rejects.toThrow(
      /Phasen konnten nicht geladen werden/,
    );
  });
});

// --- phase calendar period helpers ------------------------------------

describe("phase calendar period helpers", () => {
  it("maps a phase to the full update input and preserves explicit values", () => {
    const phase: Phase = {
      ...mkPhase("1234", "Sommerferien"),
      kind: "holiday",
      enrollment_open_at: "2026-03-01T08:00:00Z",
      enrollment_close_at: "2026-03-15T18:00:00Z",
      form_schema_id: "44",
      calendar_period_id: "55",
      show_status_reason_to_parent: true,
      care_overflow_mode: "reject",
      care_offering_selection_mode: "exactly_one",
      is_active: false,
    };

    expect(phaseToInput(phase)).toEqual({
      name: "Sommerferien",
      kind: "holiday",
      service_start_date: "2026-09-01",
      service_end_date: "2027-07-31",
      enrollment_open_at: "2026-03-01T08:00:00Z",
      enrollment_close_at: "2026-03-15T18:00:00Z",
      form_schema_id: "44",
      calendar_period_id: "55",
      show_status_reason_to_parent: true,
      care_overflow_mode: "reject",
      care_offering_selection_mode: "exactly_one",
      is_active: false,
      available_school_classes: [],
      require_school_class: false,
      // Legacy phase (mkPhase omits audience/eligible_school_classes) defaults
      // to the "open"/no-restriction eligibility, so a round-trip save never
      // silently changes it (#1663).
      audience: "open",
      eligible_school_classes: [],
    });
  });

  it("normalizes omitted nullable phase fields before a full update", () => {
    const phase = {
      ...mkPhase("1234", "Schuljahr"),
      enrollment_open_at: undefined,
      enrollment_close_at: undefined,
      form_schema_id: undefined,
      calendar_period_id: undefined,
      care_offering_selection_mode: undefined,
    } as unknown as Phase;

    expect(phaseToInput(phase)).toMatchObject({
      enrollment_open_at: null,
      enrollment_close_at: null,
      form_schema_id: null,
      calendar_period_id: null,
      care_offering_selection_mode: "optional",
    });
  });

  it("links a phase via refetch-then-PUT, writing from the fresh server state", async () => {
    // The caller holds a stale snapshot; the server has a newer name. The
    // PUT body must come from the refetched state, not the stale argument.
    const stale = mkPhase("phase/1", "Alter Name");
    const fresh = mkPhase("phase/1", "Neuer Name");
    const calls: { url: string; method: string; body?: PhaseInput }[] = [];
    mockFetch(async (input, init) => {
      const url = typeof input === "string" ? input : input.toString();
      const method = init?.method ?? "GET";
      calls.push({
        url,
        method,
        body: init?.body
          ? (JSON.parse(init.body as string) as PhaseInput)
          : undefined,
      });
      if (method === "GET") return jsonResponse({ data: fresh });
      return jsonResponse({
        data: { ...fresh, calendar_period_id: "period/2" },
      });
    });

    const out = await setPhaseCalendarPeriod(stale, "period/2");

    expect(calls.map((c) => c.method)).toEqual(["GET", "PUT"]);
    expect(calls[0]!.url).toContain("/phase%2F1");
    expect(calls[1]!.url).toContain("/phase%2F1");
    expect(calls[1]!.body).toEqual({
      ...phaseToInput(fresh),
      calendar_period_id: "period/2",
    });
    expect(out.calendar_period_id).toBe("period/2");
  });

  it("unlinks a phase calendar period with null", async () => {
    const phase = {
      ...mkPhase("phase-1", "Schuljahr"),
      calendar_period_id: "7",
    };
    let seenBody: PhaseInput | undefined;
    mockFetch(async (_, init) => {
      if (!init?.method || init.method === "GET") {
        return jsonResponse({ data: phase });
      }
      seenBody = JSON.parse((init?.body as string) ?? "{}") as PhaseInput;
      return jsonResponse({ data: { ...phase, calendar_period_id: null } });
    });

    const out = await setPhaseCalendarPeriod(phase, null);

    expect(seenBody?.calendar_period_id).toBeNull();
    expect(out.calendar_period_id).toBeNull();
  });

  it("does not PUT when the refetch fails", async () => {
    const methods: string[] = [];
    mockFetch(async (_, init) => {
      methods.push(init?.method ?? "GET");
      return jsonResponse({ error: "boom" }, { status: 500 });
    });

    await expect(
      setPhaseCalendarPeriod(mkPhase("1", "X"), "2"),
    ).rejects.toThrow();
    expect(methods).toEqual(["GET"]);
  });
});

// --- createPhase ------------------------------------------------------

describe("createPhase", () => {
  it("POSTs the body and returns the unwrapped phase", async () => {
    let seenBody = "";
    mockFetch(async (_, init) => {
      seenBody = (init?.body as string) ?? "";
      return jsonResponse(
        { data: mkPhase("1234", "Schuljahr 2026") },
        { status: 201 },
      );
    });
    const out = await createPhase(validInput);
    expect(out.id).toBe("1234");
    expect(seenBody).toContain(`"name":"Schuljahr 2026"`);
  });

  it("throws on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "validation failed" }, { status: 400 }),
    );
    await expect(createPhase(validInput)).rejects.toThrow(
      /Phase konnte nicht erstellt werden/,
    );
  });

  it("translates the backend phase name validation error to German", async () => {
    mockFetch(async () =>
      jsonResponse(
        { error: "phase validation: phase name is required" },
        { status: 400 },
      ),
    );
    await expect(createPhase(validInput)).rejects.toThrow(
      /Bitte gib einen Namen für die Anmeldephase ein/,
    );
  });
});

// --- rollover preview --------------------------------------------------

describe("fetchRolloverPreview", () => {
  it("GETs the grade bump choice and unwraps the preview", async () => {
    let seenURL = "";
    let seenInit: RequestInit | undefined;
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenInit = init;
      return jsonResponse({
        data: {
          carry_candidate_count: 3,
          carried_count: 2,
          review_count: 1,
          review_by_reason: { grade: 1 },
          excluded_count: 0,
          excluded_by_status: {},
          request_count: 2,
        },
      });
    });

    const preview = await fetchRolloverPreview("phase/1", true);

    expect(seenURL).toContain("/phase%2F1/rollover-preview?bumps_grade=true");
    expect(seenInit?.cache).toBe("no-store");
    expect(preview.carried_count).toBe(2);
  });

  it("throws the preview fallback when the request fails", async () => {
    mockFetch(async () => new Response("nope", { status: 500 }));

    await expect(fetchRolloverPreview("1", false)).rejects.toThrow(
      /Vorschau konnte nicht geladen werden/,
    );
  });
});

// --- updatePhase ------------------------------------------------------

describe("updatePhase", () => {
  it("PUTs to the id-bearing URL and returns the unwrapped phase", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: mkPhase("1234", "Updated") });
    });
    const out = await updatePhase("1234", { ...validInput, name: "Updated" });
    expect(out.name).toBe("Updated");
    expect(seenURL).toContain("/1234");
  });

  it("URL-encodes the id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: mkPhase("a/b", "X") });
    });
    await updatePhase("a/b", validInput);
    expect(seenURL).toContain("a%2Fb");
  });

  it("translates validation error 'service_end_date must be on or after service_start_date' to German", async () => {
    mockFetch(async () =>
      jsonResponse(
        { error: "service_end_date must be on or after service_start_date" },
        { status: 400 },
      ),
    );
    await expect(updatePhase("1", validInput)).rejects.toThrow(
      /Das Ende des Betreuungszeitraums muss am gleichen Tag oder nach dem Beginn liegen/,
    );
  });

  it("translates 'enrollment_close_at must be after enrollment_open_at' to German", async () => {
    mockFetch(async () =>
      jsonResponse(
        { error: "enrollment_close_at must be after enrollment_open_at" },
        { status: 400 },
      ),
    );
    await expect(updatePhase("1", validInput)).rejects.toThrow(
      /Die Schließung des Anmeldefensters muss nach der Öffnung liegen/,
    );
  });

  it("uses the German fallback for unmatched English validation messages", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "some other backend error" }, { status: 400 }),
    );
    await expect(updatePhase("1", validInput)).rejects.toThrow(
      /Phase konnte nicht gespeichert werden/,
    );
  });
});

// --- deletePhase ------------------------------------------------------

describe("deletePhase", () => {
  it("resolves on 204", async () => {
    mockFetch(async () => new Response(null, { status: 204 }));
    await expect(deletePhase("1234")).resolves.toBeUndefined();
  });

  it("URL-encodes the id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return new Response(null, { status: 204 });
    });
    await deletePhase("a/b");
    expect(seenURL).toContain("a%2Fb");
  });

  // The "has requests/offerings" delete guard was removed — a phase is
  // always deletable now. A non-204 response surfaces the backend's error
  // text directly (no special code translation).
  it("translates known backend not-found text on a non-OK response", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "phase not found" }, { status: 404 }),
    );
    await expect(deletePhase("1234")).rejects.toThrow(
      /Die Anmeldephase wurde nicht gefunden/,
    );
  });

  it("falls back to the German operation fallback when body is malformed", async () => {
    mockFetch(async () => new Response("not json", { status: 500 }));
    await expect(deletePhase("1234")).rejects.toThrow(
      /Phase konnte nicht gelöscht werden/,
    );
  });
});

// --- getPhaseDeleteImpact ---------------------------------------------

describe("getPhaseDeleteImpact", () => {
  it("returns the impact counts on a successful response", async () => {
    const impact: PhaseDeleteImpact = {
      requests: 3,
      care_offerings: 2,
      students_kept: 5,
    };
    mockFetch(async () => jsonResponse({ data: impact }));
    await expect(getPhaseDeleteImpact("1234")).resolves.toEqual(impact);
  });

  it("requests the delete-impact path with the URL-encoded id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({
        data: { requests: 0, care_offerings: 0, students_kept: 0 },
      });
    });
    await getPhaseDeleteImpact("a/b");
    expect(seenURL).toContain("a%2Fb/delete-impact");
  });

  it("throws the German fallback when the response is not OK", async () => {
    mockFetch(async () => new Response("boom", { status: 500 }));
    await expect(getPhaseDeleteImpact("1234")).rejects.toThrow(
      /Löschvorschau konnte nicht geladen werden/,
    );
  });
});

// --- createRollover ---------------------------------------------------

const validRolloverInput: RolloverInput = {
  name: "Schuljahr 2027",
  kind: "school_year",
  service_start_date: "2027-09-01",
  service_end_date: "2028-07-31",
  rollover_mode: "opt_out",
  rollover_auto_approve: false,
  rollover_deadline: "2027-07-15T18:00:00Z",
};

describe("createRollover", () => {
  it("POSTs the body and returns the unwrapped RolloverResult", async () => {
    const result: RolloverResult = {
      phase: mkPhase("99", "Schuljahr 2027"),
      summary: {
        source_child_count: 5,
        rolled_count: 4,
        review_count: 1,
        review_by_reason: { grade_above_max: 1 },
        request_count: 3,
        enqueued_emails: 3,
        skipped_empty_email: 0,
      },
    };
    mockFetch(async () => jsonResponse({ data: result }, { status: 201 }));
    const out = await createRollover("42", validRolloverInput);
    expect(out.phase.id).toBe("99");
    expect(out.summary.rolled_count).toBe(4);
  });

  it("URL-encodes the source phase id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse(
        { data: { phase: mkPhase("1", "X"), summary: {} } },
        { status: 201 },
      );
    });
    await createRollover("a/b", validRolloverInput);
    expect(seenURL).toContain("a%2Fb/rollover");
  });

  it("translates rollover.source_not_found code", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "rollover.source_not_found", error: "source not found" },
        { status: 404 },
      ),
    );
    await expect(createRollover("42", validRolloverInput)).rejects.toThrow(
      /Quellphase wurde nicht gefunden/,
    );
  });

  it("translates rollover.source_already_rolled code", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "rollover.source_already_rolled", error: "already rolled" },
        { status: 409 },
      ),
    );
    await expect(createRollover("42", validRolloverInput)).rejects.toThrow(
      /bereits eine Anschlussphase/,
    );
  });

  it("translates rollover.invalid_request code", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "rollover.invalid_request", error: "invalid" },
        { status: 400 },
      ),
    );
    await expect(createRollover("42", validRolloverInput)).rejects.toThrow(
      /unvollständig oder ungültig/,
    );
  });

  it("translates rollover.duplicate_name code", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "rollover.duplicate_name", error: "duplicate name" },
        { status: 409 },
      ),
    );
    await expect(createRollover("42", validRolloverInput)).rejects.toThrow(
      /bereits eine Phase mit diesem Namen/,
    );
  });

  it("uses the German fallback for unknown English rollover codes", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "rollover.something.else", error: "weird backend error" },
        { status: 500 },
      ),
    );
    await expect(createRollover("42", validRolloverInput)).rejects.toThrow(
      /Rollover konnte nicht erstellt werden/,
    );
  });
});

// --- listRolloverReview ----------------------------------------------

describe("listRolloverReview", () => {
  it("returns the unwrapped list on 200", async () => {
    mockFetch(async () =>
      jsonResponse({
        data: [
          {
            child_id: "1",
            request_id: "2",
            first_name: "Lara",
            last_name: "Beispiel",
            guardian_first_name: "Anna",
            guardian_last_name: "Beispiel",
            guardian_email: "a@b.c",
            status_token: "tok",
          },
        ],
      }),
    );
    const out = await listRolloverReview("42");
    expect(out).toHaveLength(1);
    expect(out[0]!.first_name).toBe("Lara");
  });

  it("returns [] when backend returns non-array data", async () => {
    mockFetch(async () => jsonResponse({ data: null }));
    expect(await listRolloverReview("42")).toEqual([]);
  });

  it("throws with the German fallback on non-OK + non-JSON body", async () => {
    mockFetch(async () => new Response("nope", { status: 500 }));
    await expect(listRolloverReview("42")).rejects.toThrow(
      /Prüfliste konnte nicht geladen werden/,
    );
  });
});

// --- decideRolloverReview -------------------------------------------

describe("decideRolloverReview", () => {
  it("POSTs to the request-children URL with the decision body", async () => {
    let seenURL = "";
    let seenBody = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenBody = (init?.body as string) ?? "";
      return new Response(null, { status: 204 });
    });
    await decideRolloverReview("99", { decision: "keep", new_grade_level: 2 });
    expect(seenURL).toContain(
      "/api/enrollment/admin/request-children/99/rollover-review",
    );
    expect(seenBody).toContain(`"decision":"keep"`);
    expect(seenBody).toContain(`"new_grade_level":2`);
  });

  it("translates rollover.review_invalid code", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "rollover.review_invalid", error: "invalid" },
        { status: 400 },
      ),
    );
    await expect(
      decideRolloverReview("99", { decision: "keep" }),
    ).rejects.toThrow(/in diesem Zustand nicht erlaubt/);
  });

  it("translates rollover.review_not_found code", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "rollover.review_not_found", error: "not found" },
        { status: 404 },
      ),
    );
    await expect(
      decideRolloverReview("99", { decision: "drop" }),
    ).rejects.toThrow(/Eintrag in der Prüfliste existiert nicht mehr/);
  });
});
