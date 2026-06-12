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

const { PUT } = await import("./route");

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
});
