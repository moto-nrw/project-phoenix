import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const { PUT, PATCH } = await import("./route");

function putRequest(body: unknown): NextRequest {
  return new NextRequest(
    new URL("http://localhost:3000/api/enrollment/schema/7"),
    {
      method: "PUT",
      body: JSON.stringify(body),
      headers: { "Content-Type": "application/json" },
    },
  );
}

function context() {
  return { params: Promise.resolve({ id: "7" }) };
}

const legalBlocks = [
  {
    key: "foto_homepage",
    kind: "consent",
    title: "Fotoeinwilligung – Homepage",
    label: "Fotos dürfen auf der Homepage veröffentlicht werden.",
    text: "",
    required: false,
    enabled: true,
    sort_order: 10,
    source: "custom",
  },
];

describe("PUT /api/enrollment/schema/[id]", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    mockAuth.mockReset();
    mockAuth.mockResolvedValue({
      user: { id: "1", token: "test-token" },
      expires: "2099-01-01",
    });
    fetchMock.mockReset();
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ data: {} }), { status: 201 }),
    );
    vi.stubGlobal("fetch", fetchMock);
  });

  it("forwards legal_blocks to the backend", async () => {
    // Regression guard: the proxy rebuilds the body field-by-field, so a
    // key it does not copy is silently dropped on every template update.
    await PUT(
      putRequest({
        fields: [],
        core_requirements: {},
        legal_blocks: legalBlocks,
      }),
      context(),
    );

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const forwarded = JSON.parse(init.body as string) as Record<
      string,
      unknown
    >;
    expect(forwarded.legal_blocks).toEqual(legalBlocks);
  });

  it("omits legal_blocks when the client did not send them", async () => {
    // Absent must stay absent: the backend preserves the existing blocks
    // for a missing key, while [] would clear them.
    await PUT(putRequest({ fields: [] }), context());

    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const forwarded = JSON.parse(init.body as string) as Record<
      string,
      unknown
    >;
    expect("legal_blocks" in forwarded).toBe(false);
  });

  it("forwards name for a combined rename + edit save", async () => {
    // The name rides along on the PUT so the backend renames + publishes in
    // one transaction; the proxy must not drop it.
    await PUT(putRequest({ name: "Ferienprogramm", fields: [] }), context());

    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const forwarded = JSON.parse(init.body as string) as Record<
      string,
      unknown
    >;
    expect(forwarded.name).toBe("Ferienprogramm");
  });

  it("omits name on a content-only save", async () => {
    // Absent name = pure content edit; the backend keeps the lineage name.
    await PUT(putRequest({ fields: [] }), context());

    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const forwarded = JSON.parse(init.body as string) as Record<
      string,
      unknown
    >;
    expect("name" in forwarded).toBe(false);
  });
});

describe("PATCH /api/enrollment/schema/[id]", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    mockAuth.mockReset();
    mockAuth.mockResolvedValue({
      user: { id: "1", token: "test-token" },
      expires: "2099-01-01",
    });
    fetchMock.mockReset();
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ data: {} }), { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);
  });

  function patchRequest(body: unknown): NextRequest {
    return new NextRequest(
      new URL("http://localhost:3000/api/enrollment/schema/7"),
      {
        method: "PATCH",
        body: JSON.stringify(body),
        headers: { "Content-Type": "application/json" },
      },
    );
  }

  it("forwards the rename to the backend with PATCH + the name", async () => {
    await PATCH(patchRequest({ name: "Ferienprogramm" }), context());

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://backend.test/api/enrollment/schema/7");
    expect(init.method).toBe("PATCH");
    const forwarded = JSON.parse(init.body as string) as Record<
      string,
      unknown
    >;
    expect(forwarded.name).toBe("Ferienprogramm");
  });

  it("rejects an unauthenticated request with 401", async () => {
    mockAuth.mockResolvedValue(null);
    const response = await PATCH(patchRequest({ name: "X" }), context());
    expect(response.status).toBe(401);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects a missing/blank id with 400 before any backend call", async () => {
    const response = await PATCH(patchRequest({ name: "X" }), {
      params: Promise.resolve({ id: "" }),
    });
    expect(response.status).toBe(400);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("returns 500 when the upstream backend call throws", async () => {
    fetchMock.mockRejectedValue(new Error("ECONNREFUSED"));
    const response = await PATCH(patchRequest({ name: "X" }), context());
    expect(response.status).toBe(500);
  });
});
