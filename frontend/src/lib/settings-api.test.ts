import { describe, it, expect, vi, beforeEach } from "vitest";

const mockSessionFetch =
  vi.fn<(url: string, init?: RequestInit) => Promise<Response>>();

vi.mock("./session-cache", () => ({
  sessionFetch: (...args: Parameters<typeof mockSessionFetch>) =>
    mockSessionFetch(...args),
}));

// Import after mocking
const { fetchSettingsSchema, setSettingValue, resetSettingValue } =
  await import("./settings-api");

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
