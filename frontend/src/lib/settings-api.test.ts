import { describe, it, expect, vi, beforeEach } from "vitest";

const mockSessionFetch =
  vi.fn<(url: string, init?: RequestInit) => Promise<Response>>();

vi.mock("./session-cache", () => ({
  sessionFetch: (...args: Parameters<typeof mockSessionFetch>) =>
    mockSessionFetch(...args),
}));

// Import after mocking
const {
  fetchSettingsSchema,
  setSettingValue,
  resetSettingValue,
  revealSettingValue,
  applyOptimisticSchemaUpdate,
  getSettingValue,
  SETTINGS_SCHEMA_SWR_KEY,
} = await import("./settings-api");

type SchemaForTest = Awaited<ReturnType<typeof fetchSettingsSchema>>;

function mockResponse(status: number, body?: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
    headers: new Headers(),
    redirected: false,
    statusText: "",
    type: "basic" as ResponseType,
    url: "",
    clone: () => mockResponse(status, body),
    body: null,
    bodyUsed: false,
    arrayBuffer: () => Promise.resolve(new ArrayBuffer(0)),
    blob: () => Promise.resolve(new Blob()),
    formData: () => Promise.resolve(new FormData()),
    text: () => Promise.resolve(""),
    bytes: () => Promise.resolve(new Uint8Array()),
  };
}

describe("fetchSettingsSchema", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns schema data on success", async () => {
    const schemaData = {
      tabs: [{ key: "operations", label: "Betrieb", categories: [] }],
    };
    mockSessionFetch.mockResolvedValue(
      mockResponse(200, { status: "success", data: schemaData }),
    );

    const result = await fetchSettingsSchema();

    expect(result).toEqual(schemaData);
    expect(mockSessionFetch).toHaveBeenCalledWith("/api/settings/schema", {
      method: "GET",
    });
  });

  it("returns null on 401", async () => {
    mockSessionFetch.mockResolvedValue(mockResponse(401));

    const result = await fetchSettingsSchema();

    expect(result).toBeNull();
  });

  it("returns null on 403", async () => {
    mockSessionFetch.mockResolvedValue(mockResponse(403));

    const result = await fetchSettingsSchema();

    expect(result).toBeNull();
  });

  it("throws on 500 (server error)", async () => {
    mockSessionFetch.mockResolvedValue(mockResponse(500));

    await expect(fetchSettingsSchema()).rejects.toThrow(
      "Einstellungen konnten nicht geladen werden (500)",
    );
  });

  it("returns null when no auth token available", async () => {
    mockSessionFetch.mockRejectedValue(
      new Error("No authentication token available"),
    );

    const result = await fetchSettingsSchema();

    expect(result).toBeNull();
  });

  it("returns null on network error", async () => {
    mockSessionFetch.mockRejectedValue(new Error("fetch failed"));

    const result = await fetchSettingsSchema();

    expect(result).toBeNull();
  });
});

describe("setSettingValue", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns null on success", async () => {
    mockSessionFetch.mockResolvedValue(
      mockResponse(200, { status: "success" }),
    );

    const result = await setSettingValue("test.key", "value");

    expect(result).toBeNull();
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/settings/values/test.key",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ value: "value" }),
      }),
    );
  });

  it("returns German error on 404", async () => {
    mockSessionFetch.mockResolvedValue(
      mockResponse(404, { error: "setting not found" }),
    );

    const result = await setSettingValue("bad.key", "value");

    expect(result).toBe("Einstellung nicht gefunden.");
  });

  it("returns validation error on 400", async () => {
    mockSessionFetch.mockResolvedValue(
      mockResponse(400, {
        error: "invalid value for setting test.key: below minimum 10",
      }),
    );

    const result = await setSettingValue("test.key", 5);

    expect(result).toContain("Minimum");
  });

  it("returns generic error on 500", async () => {
    mockSessionFetch.mockResolvedValue(
      mockResponse(500, { error: "internal server error" }),
    );

    const result = await setSettingValue("test.key", "value");

    expect(result).toBe("Einstellung konnte nicht gespeichert werden.");
  });

  it("returns network error message on fetch failure", async () => {
    mockSessionFetch.mockRejectedValue(new Error("network error"));

    const result = await setSettingValue("test.key", "value");

    expect(result).toContain("Netzwerkfehler");
  });
});

describe("resetSettingValue", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns null on 204", async () => {
    mockSessionFetch.mockResolvedValue(mockResponse(204));

    const result = await resetSettingValue("test.key");

    expect(result).toBeNull();
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/settings/values/test.key",
      { method: "DELETE" },
    );
  });

  it("returns null on 200", async () => {
    mockSessionFetch.mockResolvedValue(mockResponse(200));

    const result = await resetSettingValue("test.key");

    expect(result).toBeNull();
  });

  it("returns error on 500", async () => {
    mockSessionFetch.mockResolvedValue(mockResponse(500));

    const result = await resetSettingValue("test.key");

    expect(result).toContain("zurückgesetzt");
  });

  it("returns network error on fetch failure", async () => {
    mockSessionFetch.mockRejectedValue(new Error("network error"));

    const result = await resetSettingValue("test.key");

    expect(result).toContain("Netzwerkfehler");
  });
});

describe("SETTINGS_SCHEMA_SWR_KEY", () => {
  it("is the literal string used by every SWR consumer", () => {
    // The shared base key is tenant-prefixed by useSWRAuth/useTenantMutate.
    // Locking the value keeps all readers and invalidation paths in sync.
    expect(SETTINGS_SCHEMA_SWR_KEY).toBe("settings-schema");
  });
});

describe("getSettingValue", () => {
  const schema = {
    tabs: [
      {
        categories: [
          {
            items: [
              { key: "timetable.enabled", value: false },
              { key: "operations.meal_plan_enabled", value: true },
            ],
          },
        ],
      },
    ],
  } as NonNullable<SchemaForTest>;

  it("returns true and false values without treating false as missing", () => {
    expect(getSettingValue(schema, "timetable.enabled")).toBe(false);
    expect(getSettingValue(schema, "operations.meal_plan_enabled")).toBe(true);
  });

  it("returns undefined for missing schemas and keys", () => {
    expect(getSettingValue(null, "timetable.enabled")).toBeUndefined();
    expect(getSettingValue(schema, "missing.key")).toBeUndefined();
  });

  it("ignores malformed nested API collections", () => {
    const malformedSchema = {
      tabs: [{ categories: null }, { categories: [{ items: null }] }],
    } as unknown as NonNullable<SchemaForTest>;

    expect(
      getSettingValue(malformedSchema, "timetable.enabled"),
    ).toBeUndefined();
  });
});

describe("applyOptimisticSchemaUpdate", () => {
  function makeItem(
    key: string,
    overrides: Partial<{
      type: "boolean" | "number" | "time" | "text" | "password" | "select";
      value: unknown;
      default: unknown;
      depends_on:
        { key: string; condition: string; value: unknown } | null | undefined;
    }> = {},
  ) {
    return {
      key,
      label: key,
      description: "",
      type: overrides.type ?? "boolean",
      default: overrides.default ?? false,
      value: overrides.value ?? false,
      is_default: true,
      writable: true,
      visible: true,
      sort_order: 0,
      access_policy: "shared" as const,
      depends_on: overrides.depends_on ?? undefined,
    };
  }

  function makeSchema(
    items: ReturnType<typeof makeItem>[],
  ): NonNullable<SchemaForTest> {
    return {
      tabs: [
        {
          key: "tab",
          label: "Tab",
          categories: [{ key: "cat", label: "Cat", items }],
        },
      ],
    };
  }

  it("updates the targeted item's value and clears is_default", () => {
    const schema = makeSchema([
      makeItem("a", { type: "number", value: 5 }),
      makeItem("b", { type: "number", value: 10 }),
    ]);

    const next = applyOptimisticSchemaUpdate(schema, "a", 99);

    const items = next.tabs[0]!.categories[0]!.items;
    expect(items[0]!.value).toBe(99);
    expect(items[0]!.is_default).toBe(false);
    // Untouched item stays as-is.
    expect(items[1]!.value).toBe(10);
    expect(items[1]!.is_default).toBe(true);
  });

  it("keeps is_default when the new value equals the registry default", () => {
    // Boolean toggled away from default (false) to true, then back to false.
    // Toggling back to the default must restore the "Standard" badge without
    // waiting for a refetch (issue #1680).
    const schema = makeSchema([
      makeItem("a", { type: "boolean", value: true }),
    ]);

    const next = applyOptimisticSchemaUpdate(schema, "a", false);

    const item = next.tabs[0]!.categories[0]!.items[0]!;
    expect(item.value).toBe(false);
    expect(item.is_default).toBe(true);
  });

  it("keeps a resettable setting non-default even when saved at the default value", () => {
    // A number set to its registry default still wrote an override row, so the
    // reset button must stay available (is_default false). The value-based
    // badge is boolean-only (issue #1680).
    const schema = makeSchema([
      makeItem("a", { type: "number", value: 99, default: 30 }),
    ]);

    const next = applyOptimisticSchemaUpdate(schema, "a", 30);

    const item = next.tabs[0]!.categories[0]!.items[0]!;
    expect(item.value).toBe(30);
    expect(item.is_default).toBe(false);
  });

  it("masks password fields with bullets instead of leaking the cleartext", () => {
    const schema = makeSchema([
      makeItem("pw", { type: "password", value: "" }),
    ]);

    const next = applyOptimisticSchemaUpdate(schema, "pw", "supersecret");

    expect(next.tabs[0]!.categories[0]!.items[0]!.value).toBe("••••••");
  });

  it("does not mutate the input schema", () => {
    const schema = makeSchema([makeItem("a", { type: "number", value: 5 })]);
    const before = JSON.stringify(schema);

    applyOptimisticSchemaUpdate(schema, "a", 42);

    expect(JSON.stringify(schema)).toBe(before);
  });

  it("re-evaluates depends_on with `eq` against the new parent value", () => {
    const schema = makeSchema([
      makeItem("parent", { type: "boolean", value: false }),
      makeItem("child", {
        type: "number",
        value: 1,
        depends_on: { key: "parent", condition: "eq", value: true },
      }),
    ]);

    const next = applyOptimisticSchemaUpdate(schema, "parent", true);

    const child = next.tabs[0]!.categories[0]!.items[1]!;
    expect(child.visible).toBe(true);
  });

  it("hides depends_on `eq` children when the parent no longer matches", () => {
    const schema = makeSchema([
      makeItem("parent", { type: "boolean", value: true }),
      makeItem("child", {
        type: "number",
        value: 1,
        depends_on: { key: "parent", condition: "eq", value: true },
      }),
    ]);

    const next = applyOptimisticSchemaUpdate(schema, "parent", false);

    expect(next.tabs[0]!.categories[0]!.items[1]!.visible).toBe(false);
  });

  it("hides nested children when their direct parent is hidden", () => {
    const schema = makeSchema([
      makeItem("root", { type: "boolean", value: false }),
      makeItem("child", {
        type: "boolean",
        value: true,
        depends_on: { key: "root", condition: "eq", value: true },
      }),
      makeItem("grandchild", {
        type: "number",
        value: 15,
        depends_on: { key: "child", condition: "eq", value: true },
      }),
    ]);

    const next = applyOptimisticSchemaUpdate(schema, "root", false);

    expect(next.tabs[0]!.categories[0]!.items[1]!.visible).toBe(false);
    expect(next.tabs[0]!.categories[0]!.items[2]!.visible).toBe(false);
  });

  it("supports the `neq` depends_on condition", () => {
    const schema = makeSchema([
      makeItem("parent", { type: "boolean", value: true }),
      makeItem("child", {
        type: "number",
        value: 1,
        depends_on: { key: "parent", condition: "neq", value: false },
      }),
    ]);

    const next = applyOptimisticSchemaUpdate(schema, "parent", false);

    expect(next.tabs[0]!.categories[0]!.items[1]!.visible).toBe(false);
  });

  it("supports the `not_empty` depends_on condition", () => {
    const schema = makeSchema([
      makeItem("parent", { type: "text", value: "filled" }),
      makeItem("child", {
        type: "number",
        value: 1,
        depends_on: { key: "parent", condition: "not_empty", value: null },
      }),
    ]);

    const cleared = applyOptimisticSchemaUpdate(schema, "parent", "");
    expect(cleared.tabs[0]!.categories[0]!.items[1]!.visible).toBe(false);

    const filled = applyOptimisticSchemaUpdate(schema, "parent", "still here");
    expect(filled.tabs[0]!.categories[0]!.items[1]!.visible).toBe(true);
  });

  it("leaves items without depends_on visibility logic untouched", () => {
    const schema = makeSchema([
      makeItem("a", { type: "number", value: 1 }),
      makeItem("b", { type: "number", value: 2 }),
    ]);

    const next = applyOptimisticSchemaUpdate(schema, "a", 99);

    // No depends_on rule on either item: visibility stays true.
    for (const item of next.tabs[0]!.categories[0]!.items) {
      expect(item.visible).toBe(true);
    }
  });
});

describe("revealSettingValue", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns revealed value on success (nested data)", async () => {
    mockSessionFetch.mockResolvedValue(
      mockResponse(200, { data: { data: { value: "1234" } } }),
    );

    const result = await revealSettingValue("security.ogs_device_pin");

    expect(result).toBe("1234");
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/settings/values/security.ogs_device_pin/reveal",
      { method: "GET" },
    );
  });

  it("returns revealed value on success (flat data)", async () => {
    mockSessionFetch.mockResolvedValue(
      mockResponse(200, { data: { value: "5678" } }),
    );

    const result = await revealSettingValue("security.ogs_device_pin");

    expect(result).toBe("5678");
  });

  it("returns null when value is not a string", async () => {
    mockSessionFetch.mockResolvedValue(
      mockResponse(200, { data: { value: 1234 } }),
    );

    const result = await revealSettingValue("security.ogs_device_pin");

    expect(result).toBeNull();
  });

  it("returns null on non-ok response", async () => {
    mockSessionFetch.mockResolvedValue(mockResponse(403));

    const result = await revealSettingValue("security.ogs_device_pin");

    expect(result).toBeNull();
  });

  it("returns null on network error", async () => {
    mockSessionFetch.mockRejectedValue(new Error("network error"));

    const result = await revealSettingValue("security.ogs_device_pin");

    expect(result).toBeNull();
  });
});
