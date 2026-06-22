import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";

import {
  RESERVED_TARGETS,
  blankField,
  fetchEnrollmentPreviewBootstrap,
  latestSchemasByName,
  schemaToPublicFormSchema,
  listSchemas,
  fetchSchemaById,
  fetchPublicActiveSchema,
  fetchPublicLegalTexts,
  createSchema,
  deleteEnrollmentLegalDocument,
  updateSchema,
  renameSchema,
  deleteSchema,
  uploadEnrollmentLegalDocument,
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
      "student.allowed_departure_modes",
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
    expect(RESERVED_TARGETS["student.allowed_departure_modes"].type).toBe(
      "weekday_multi_mode",
    );
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

// --- schemaToPublicFormSchema ----------------------------------------

describe("schemaToPublicFormSchema", () => {
  it("preserves core requirements and legal blocks for previews", () => {
    const fullSchema: FormSchema = {
      ...mkSchema("preview", "Vorschau", 3, "2026-04-01T12:00:00Z"),
      core_requirements: { guardian_phone: true },
      legal_blocks: [
        {
          key: "custom_photo_trip",
          kind: "consent",
          title: "Fotoausflug",
          label: "Mein Kind darf beim Ausflug fotografiert werden.",
          text: "Details",
          required: true,
          enabled: true,
          sort_order: 10,
          source: "custom",
        },
      ],
    };

    expect(schemaToPublicFormSchema(fullSchema)).toEqual({
      id: "preview",
      version: 3,
      fields: fullSchema.fields,
      core_requirements: { guardian_phone: true },
      legal_blocks: fullSchema.legal_blocks,
    });
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

  it("throws with the German fallback on non-OK", async () => {
    mockFetch(async () => new Response("server boom", { status: 500 }));
    await expect(listSchemas()).rejects.toThrow(
      /Formularvorlagen konnten nicht geladen werden/,
    );
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

  it("throws with the German fallback on non-OK", async () => {
    mockFetch(async () => new Response("nope", { status: 404 }));
    await expect(fetchSchemaById("999")).rejects.toThrow(
      /Formularvorlage konnte nicht geladen werden/,
    );
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

  it("throws with the German fallback on non-OK non-404", async () => {
    mockFetch(async () => new Response("nope", { status: 500 }));
    await expect(fetchPublicActiveSchema("slug", "1")).rejects.toThrow(
      /Formular konnte nicht geladen werden/,
    );
  });
});

// --- fetchEnrollmentPreviewBootstrap ---------------------------------

describe("fetchEnrollmentPreviewBootstrap", () => {
  it("loads schema preview bootstrap and URL-encodes schema id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({
        data: {
          schema: mkSchema("12", "Ferien", 1, "2026-04-01T12:00:00Z"),
          assigned_phase_count: 2,
          active_assigned_phase_count: 1,
        },
      });
    });

    const out = await fetchEnrollmentPreviewBootstrap({ schemaId: "12/3" });

    expect(seenURL).toContain("/api/enrollment/schema/preview?");
    expect(seenURL).toContain("schemaId=12%2F3");
    expect(out.schema?.name).toBe("Ferien");
    expect(out.assigned_phase_count).toBe(2);
    expect(out.active_assigned_phase_count).toBe(1);
  });

  it("loads base preview bootstrap", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({
        data: {
          schema: null,
          assigned_phase_count: 1,
          active_assigned_phase_count: 0,
        },
      });
    });

    await fetchEnrollmentPreviewBootstrap({ base: true });

    expect(seenURL).toContain("base=1");
    expect(seenURL).not.toContain("schemaId=");
  });
});

// --- fetchPublicLegalTexts -------------------------------------------

describe("fetchPublicLegalTexts", () => {
  it("URL-encodes tenant slug and phase id when provided", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: { blocks: [] } });
    });

    await fetchPublicLegalTexts("test tenant", "phase/5");

    expect(seenURL).toBe(
      "/api/enrollment/legal/test%20tenant?phaseId=phase%2F5",
    );
  });

  it("keeps the tenant-only fallback URL when no phase id is provided", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: { blocks: [] } });
    });

    await fetchPublicLegalTexts("demo");

    expect(seenURL).toBe("/api/enrollment/legal/demo");
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

  it("POSTs template legal blocks when provided", async () => {
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

    await createSchema("X", [], {}, [
      {
        key: "custom_pool",
        kind: "consent",
        title: "Schwimmbad",
        label: "Mein Kind darf teilnehmen.",
        text: "Details",
        required: true,
        enabled: true,
        sort_order: 10,
        source: "custom",
      },
    ]);

    expect(seenBody).toContain(`"legal_blocks":[`);
    expect(seenBody).toContain(`"key":"custom_pool"`);
    expect(seenBody).toContain(`"required":true`);
  });

  it("translates duplicate-name errors on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "duplicate name" }, { status: 400 }),
    );
    await expect(createSchema("X", [])).rejects.toThrow(
      /Dieser Name ist bereits vergeben/,
    );
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

  it("PUTs template legal blocks when provided", async () => {
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

    await updateSchema("1234", [], {}, [
      {
        key: "custom_pool",
        kind: "consent",
        title: "Schwimmbad",
        label: "Mein Kind darf teilnehmen.",
        text: "Details",
        required: false,
        enabled: true,
        sort_order: 10,
        source: "custom",
      },
    ]);

    expect(seenBody).toContain(`"legal_blocks":[`);
    expect(seenBody).toContain(`"key":"custom_pool"`);
    expect(seenBody).not.toContain(`"name"`);
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

  it("includes name for a combined rename + edit save", async () => {
    // Passing newName sends the name on the PUT so the backend renames the
    // lineage and publishes in one transaction.
    let seenBody = "";
    mockFetch(async (_, init) => {
      seenBody = (init?.body as string) ?? "";
      return jsonResponse(
        { data: mkSchema("1234", "Ferienprogramm", 3, "x") },
        { status: 201 },
      );
    });
    await updateSchema(
      "1234",
      [blankField(0)],
      {},
      undefined,
      "Ferienprogramm",
    );
    expect(seenBody).toContain(`"name":"Ferienprogramm"`);
    expect(seenBody).toContain(`"fields":[`);
  });

  it("translates the name-collision code to a German message", async () => {
    // A combined save that collides on the new name comes back as the same
    // 409 + code the standalone rename returns.
    mockFetch(async () =>
      jsonResponse(
        { error: "name exists", code: "enrollment.schema_name_exists" },
        { status: 409 },
      ),
    );
    await expect(
      updateSchema("1234", [], {}, undefined, "Schon vergeben"),
    ).rejects.toThrow(/bereits ein Formular mit diesem Namen/);
  });

  it("translates schema validation errors on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "invalid schema" }, { status: 400 }),
    );
    await expect(updateSchema("1234", [])).rejects.toThrow(
      /Formularvorlage ist ungültig/,
    );
  });
});

// --- legal document upload/delete ------------------------------------

describe("uploadEnrollmentLegalDocument", () => {
  it("uploads the PDF as multipart form data and returns the document URL", async () => {
    let seenURL = "";
    let seenMethod = "";
    let seenBody: BodyInit | null | undefined;
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenMethod = init?.method ?? "";
      seenBody = init?.body;
      return jsonResponse({
        data: {
          document_url: "/uploads/enrollment-form-legal-documents/1_terms.pdf",
        },
      });
    });

    const file = new File(["%PDF-1.4"], "terms.pdf", {
      type: "application/pdf",
    });
    const documentURL = await uploadEnrollmentLegalDocument(file);

    expect(seenURL).toBe("/api/enrollment/legal-documents");
    expect(seenMethod).toBe("POST");
    expect(seenBody).toBeInstanceOf(FormData);
    expect((seenBody as FormData).get("document")).toBe(file);
    expect(documentURL).toBe(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
    );
  });

  it("maps oversized PDF responses to a German error", async () => {
    mockFetch(async () => new Response("", { status: 413 }));

    await expect(
      uploadEnrollmentLegalDocument(
        new File(["%PDF-1.4"], "large.pdf", { type: "application/pdf" }),
      ),
    ).rejects.toThrow(/maximal 10 MB/);
  });

  it("maps unsupported file responses to a German error", async () => {
    mockFetch(async () => new Response("", { status: 415 }));

    await expect(
      uploadEnrollmentLegalDocument(
        new File(["not pdf"], "terms.txt", { type: "text/plain" }),
      ),
    ).rejects.toThrow(/Bitte eine PDF-Datei hochladen/);
  });

  it("rejects successful responses without a document URL", async () => {
    mockFetch(async () => jsonResponse({ data: {} }));

    await expect(
      uploadEnrollmentLegalDocument(
        new File(["%PDF-1.4"], "terms.pdf", { type: "application/pdf" }),
      ),
    ).rejects.toThrow(/PDF-Datei konnte nicht hochgeladen werden/);
  });
});

describe("deleteEnrollmentLegalDocument", () => {
  it("deletes the encoded filename and passes keepalive through", async () => {
    let seenURL = "";
    let seenInit: RequestInit | undefined;
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenInit = init;
      return new Response(null, { status: 204 });
    });

    await deleteEnrollmentLegalDocument(
      "/uploads/enrollment-form-legal-documents/1 terms.pdf",
      { keepalive: true },
    );

    expect(seenURL).toBe("/api/enrollment/legal-documents/1%20terms.pdf");
    expect(seenInit).toMatchObject({ keepalive: true, method: "DELETE" });
  });

  it("ignores empty document URLs", async () => {
    const fetchMock = vi.fn(async () => new Response(null, { status: 204 }));
    globalThis.fetch = fetchMock as typeof globalThis.fetch;

    await deleteEnrollmentLegalDocument("");

    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("throws a German error when delete fails", async () => {
    mockFetch(async () => new Response("", { status: 500 }));

    await expect(
      deleteEnrollmentLegalDocument(
        "/uploads/enrollment-form-legal-documents/1_terms.pdf",
      ),
    ).rejects.toThrow(/PDF-Datei konnte nicht entfernt werden/);
  });
});

// --- renameSchema ----------------------------------------------------

describe("renameSchema", () => {
  it("PATCHes only the name to the id endpoint", async () => {
    let seenMethod = "";
    let seenBody = "";
    mockFetch(async (_, init) => {
      seenMethod = init?.method ?? "";
      seenBody = (init?.body as string) ?? "";
      return jsonResponse({
        data: mkSchema("1234", "Ferienprogramm", 3, "2026-04-01T12:00:00Z"),
      });
    });

    const result = await renameSchema("1234", "Ferienprogramm");

    expect(seenMethod).toBe("PATCH");
    // A rename must never carry fields/legal_blocks — name only.
    expect(JSON.parse(seenBody)).toEqual({ name: "Ferienprogramm" });
    expect(result.name).toBe("Ferienprogramm");
  });

  it("URL-encodes the id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: mkSchema("1", "X", 1, "x") });
    });
    await renameSchema("a/b", "X");
    expect(seenURL).toContain("a%2Fb");
  });

  it("translates the name-collision code to a German message", async () => {
    // The backend returns 409 + enrollment.schema_name_exists; the admin
    // must see why, not a generic failure.
    mockFetch(async () =>
      jsonResponse(
        {
          code: "enrollment.schema_name_exists",
          error: "a form schema with this name already exists",
        },
        { status: 409 },
      ),
    );
    await expect(renameSchema("1234", "Schon vergeben")).rejects.toThrow(
      /bereits ein Formular mit diesem Namen/,
    );
  });

  it("falls back to a generic message on an uncoded error", async () => {
    mockFetch(async () => jsonResponse({}, { status: 500 }));
    await expect(renameSchema("1234", "X")).rejects.toThrow(
      /konnte nicht umbenannt werden/,
    );
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

  it("uses German code messages for schema_has_phases", async () => {
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

  it("uses German code messages for schema_has_requests", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "enrollment.schema_has_requests", error: "raw English msg" },
        { status: 409 },
      ),
    );
    await expect(deleteSchema("1234")).rejects.toThrow(/Anmeldungen verwendet/);
  });

  it("uses the German fallback for unknown English codes", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "something.else", error: "weird backend error" },
        { status: 500 },
      ),
    );
    await expect(deleteSchema("1234")).rejects.toThrow(
      /Formularvorlage konnte nicht gelöscht werden/,
    );
  });

  it("falls back to HTTP status when body has neither code nor error", async () => {
    mockFetch(async () => new Response("not json at all", { status: 500 }));
    await expect(deleteSchema("1234")).rejects.toThrow(/HTTP 500/);
  });
});
