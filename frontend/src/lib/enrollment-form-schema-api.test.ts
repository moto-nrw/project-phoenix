import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";

import {
  RESERVED_TARGETS,
  blankField,
  latestSchemasByName,
  listSchemas,
  fetchSchemaById,
  fetchPublicActiveSchema,
  createSchema,
  updateSchema,
  deleteSchema,
  type FormSchema,
  type FormField,
} from "./enrollment-form-schema-api";

// Type-safe fetch mock. The real fetch is global; we stub it per
// test so each test controls the response shape.
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

// --- RESERVED_TARGETS pinning ----------------------------------------

describe("RESERVED_TARGETS", () => {
  it("matches the backend ReservedTargets shape", () => {
    // The keys must stay in sync with backend/models/enrollment/form_schema.go.
    // Pin both keys + per-target type so a backend rename surfaces here.
    const keys = Object.keys(RESERVED_TARGETS).sort();
    expect(keys).toEqual([
      "schedule.arrival",
      "schedule.pickup",
      // student.bus is the legacy alias; student.bus_days is the canonical
      // Buskind target (#1582). Both must resolve so older saved schemas work.
      "student.bus",
      "student.bus_days",
      "student.contacts",
      // student.departure is the unified per-weekday departure-mode target (#1610).
      "student.departure",
      "student.extra_info",
      "student.health_info",
      "student.pickup_status",
    ]);
  });

  it("declares the right field type per target", () => {
    expect(RESERVED_TARGETS["student.health_info"].type).toBe("textarea");
    expect(RESERVED_TARGETS["student.bus_days"].type).toBe("weekday_boolean");
    expect(RESERVED_TARGETS["student.bus"].type).toBe("weekday_boolean");
    expect(RESERVED_TARGETS["student.pickup_status"].type).toBe(
      "weekday_boolean",
    );
    expect(RESERVED_TARGETS["schedule.pickup"].type).toBe("weekday_schedule");
    expect(RESERVED_TARGETS["student.contacts"].type).toBe("contact_list");
  });

  it("declares every reserved target as per-child", () => {
    // Every entry in the current map is appliesToChild=true. If a new
    // target lands that's per-guardian (not per-child), this assertion
    // surfaces it so the admin form renders the field correctly.
    for (const [target, spec] of Object.entries(RESERVED_TARGETS)) {
      expect(spec.appliesToChild).toBe(true);
      expect(spec.label.length).toBeGreaterThan(0);
      expect(target.length).toBeGreaterThan(0);
    }
  });
});

// --- blankField --------------------------------------------------------

describe("blankField", () => {
  it("returns a fresh field with the given sort order", () => {
    const field = blankField(7);
    expect(field.sort_order).toBe(7);
    expect(field.type).toBe("text");
    expect(field.required).toBe(false);
    expect(field.applies_to_child).toBe(false);
    expect(field.key).toBe("");
    expect(field.label).toBe("");
  });

  it("each call returns a fresh object (no shared reference)", () => {
    const a = blankField(0);
    const b = blankField(1);
    a.label = "A";
    expect(b.label).toBe("");
  });
});

// --- latestSchemasByName ----------------------------------------------

function mkSchema(
  id: string,
  name: string,
  version: number,
  createdAt: string,
): FormSchema {
  return {
    id,
    name,
    version,
    is_active: true,
    fields: [],
    created_by: "4321",
    created_at: createdAt,
  };
}

describe("latestSchemasByName", () => {
  it("returns the highest-version row per name", () => {
    const list = [
      mkSchema("1", "A", 1, "2026-04-01T12:00:00Z"),
      mkSchema("2", "A", 2, "2026-04-02T12:00:00Z"),
      mkSchema("3", "B", 1, "2026-04-01T12:00:00Z"),
    ];
    const out = latestSchemasByName(list);
    const names = out.map((s) => s.name).sort();
    expect(names).toEqual(["A", "B"]);
    const a = out.find((s) => s.name === "A")!;
    expect(a.version).toBe(2);
  });

  it("tie-breaks equal versions by newest created_at", () => {
    const list = [
      mkSchema("1", "A", 2, "2026-04-01T12:00:00Z"),
      mkSchema("2", "A", 2, "2026-04-05T12:00:00Z"),
    ];
    const out = latestSchemasByName(list);
    expect(out).toHaveLength(1);
    expect(out[0]!.id).toBe("2");
  });

  it("sorts the result by created_at DESC", () => {
    const list = [
      mkSchema("1", "A", 1, "2026-04-01T12:00:00Z"),
      mkSchema("2", "B", 1, "2026-04-10T12:00:00Z"),
      mkSchema("3", "C", 1, "2026-04-05T12:00:00Z"),
    ];
    const out = latestSchemasByName(list);
    expect(out.map((s) => s.name)).toEqual(["B", "C", "A"]);
  });

  it("returns an empty array for an empty input", () => {
    expect(latestSchemasByName([])).toEqual([]);
  });
});

// --- listSchemas ------------------------------------------------------

describe("listSchemas", () => {
  it("returns the unwrapped data array on 200", async () => {
    mockFetch(async () =>
      jsonResponse({
        data: [mkSchema("1", "A", 1, "2026-04-01T12:00:00Z")],
      }),
    );
    const out = await listSchemas();
    expect(out).toHaveLength(1);
    expect(out[0]!.name).toBe("A");
  });

  it("returns [] when the backend returns a non-array data", async () => {
    mockFetch(async () => jsonResponse({ data: null }));
    const out = await listSchemas();
    expect(out).toEqual([]);
  });

  it("throws with the HTTP status on non-OK", async () => {
    mockFetch(async () => new Response("server boom", { status: 500 }));
    await expect(listSchemas()).rejects.toThrow(/HTTP 500/);
  });
});

// --- fetchSchemaById --------------------------------------------------

describe("fetchSchemaById", () => {
  it("returns the unwrapped schema on 200", async () => {
    mockFetch(async () =>
      jsonResponse({
        data: mkSchema("1234", "Schuljahr", 2, "2026-04-01T12:00:00Z"),
      }),
    );
    const out = await fetchSchemaById("1234");
    expect(out.id).toBe("1234");
    expect(out.version).toBe(2);
  });

  it("URL-encodes the id (no path traversal)", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ id: "x" });
    });
    await fetchSchemaById("../bad/id");
    expect(seenURL).not.toContain("../");
    expect(seenURL).toContain("..%2Fbad%2Fid");
  });

  it("throws on non-OK", async () => {
    mockFetch(async () => new Response("nope", { status: 404 }));
    await expect(fetchSchemaById("999")).rejects.toThrow(/HTTP 404/);
  });
});

// --- fetchPublicActiveSchema -----------------------------------------

describe("fetchPublicActiveSchema", () => {
  it("returns null on 404 (no schema published yet)", async () => {
    // 404 is the explicit no-schema fallback path so the form renders
    // core fields only.
    mockFetch(async () => new Response("", { status: 404 }));
    const out = await fetchPublicActiveSchema("slug", "1");
    expect(out).toBeNull();
  });

  it("returns the schema on 200", async () => {
    mockFetch(async () =>
      jsonResponse({
        data: { id: "1234", version: 1, fields: [] as FormField[] },
      }),
    );
    const out = await fetchPublicActiveSchema("slug", "1");
    expect(out).not.toBeNull();
    expect(out!.id).toBe("1234");
  });

  it("URL-encodes the tenant slug and phase id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: { id: "1", version: 1, fields: [] } });
    });
    await fetchPublicActiveSchema("we/ird", "ph/ase");
    expect(seenURL).not.toContain("we/ird");
    expect(seenURL).toContain("we%2Fird");
    expect(seenURL).toContain("ph%2Fase");
  });

  it("throws on non-OK non-404", async () => {
    mockFetch(async () => new Response("nope", { status: 500 }));
    await expect(fetchPublicActiveSchema("slug", "1")).rejects.toThrow(
      /HTTP 500/,
    );
  });
});

// --- createSchema ----------------------------------------------------

describe("createSchema", () => {
  it("POSTs name + fields and returns the unwrapped schema", async () => {
    let seenBody = "";
    mockFetch(async (_, init) => {
      seenBody = (init?.body as string) ?? "";
      return jsonResponse(
        {
          data: mkSchema("1234", "X", 1, "2026-04-01T12:00:00Z"),
        },
        { status: 201 },
      );
    });
    const out = await createSchema("X", [blankField(0)]);
    expect(out.id).toBe("1234");
    expect(seenBody).toContain(`"name":"X"`);
    expect(seenBody).toContain(`"fields":[`);
  });

  it("throws with the backend error message on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "duplicate name" }, { status: 400 }),
    );
    await expect(createSchema("X", [])).rejects.toThrow(/duplicate name/);
  });

  it("falls back to HTTP status when body has no error string", async () => {
    mockFetch(async () => new Response("not json", { status: 500 }));
    await expect(createSchema("X", [])).rejects.toThrow(/HTTP 500/);
  });
});

// --- updateSchema ----------------------------------------------------

describe("updateSchema", () => {
  it("PUTs fields without name (inherited from source schema)", async () => {
    let seenBody = "";
    mockFetch(async (_, init) => {
      seenBody = (init?.body as string) ?? "";
      return jsonResponse(
        {
          data: mkSchema("1234", "X", 2, "2026-04-01T12:00:00Z"),
        },
        { status: 201 },
      );
    });
    await updateSchema("1234", [blankField(0)]);
    expect(seenBody).toContain(`"fields":[`);
    expect(seenBody).not.toContain(`"name"`); // name comes from server-side source row
  });

  it("URL-encodes the id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse(
        { data: mkSchema("1", "X", 1, "x") },
        { status: 201 },
      );
    });
    await updateSchema("a/b", []);
    expect(seenURL).toContain("a%2Fb");
  });

  it("throws with the backend error on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "invalid schema" }, { status: 400 }),
    );
    await expect(updateSchema("1234", [])).rejects.toThrow(/invalid schema/);
  });
});

// --- deleteSchema ----------------------------------------------------

describe("deleteSchema", () => {
  it("resolves on 204", async () => {
    mockFetch(async () => new Response(null, { status: 204 }));
    await expect(deleteSchema("1234")).resolves.toBeUndefined();
  });

  it("URL-encodes the id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return new Response(null, { status: 204 });
    });
    await deleteSchema("a/b");
    expect(seenURL).toContain("a%2Fb");
  });

  it("uses SCHEMA_ERROR_MESSAGES for schema_has_phases", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "enrollment.schema_has_phases", error: "raw English msg" },
        { status: 409 },
      ),
    );
    await expect(deleteSchema("1234")).rejects.toThrow(
      /in einer Anmeldephase verwendet/,
    );
  });

  it("uses SCHEMA_ERROR_MESSAGES for schema_has_requests", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "enrollment.schema_has_requests", error: "raw English msg" },
        { status: 409 },
      ),
    );
    await expect(deleteSchema("1234")).rejects.toThrow(/Anmeldungen verwendet/);
  });

  it("falls back to backend error text for unknown codes", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "something.else", error: "weird backend error" },
        { status: 500 },
      ),
    );
    await expect(deleteSchema("1234")).rejects.toThrow(/weird backend error/);
  });

  it("falls back to HTTP status when body has neither code nor error", async () => {
    mockFetch(async () => new Response("not json at all", { status: 500 }));
    await expect(deleteSchema("1234")).rejects.toThrow(/HTTP 500/);
  });
});
