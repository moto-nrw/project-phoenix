import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { ExtendedSession } from "~/test/route-test-helpers";

const { mockAuth, mockUncachedAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockUncachedAuth,
    withTenantAuth: (handler: unknown) => handler,
  }),
);

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const { FileValidationError, SCAN_UPLOAD, proxyFileUpload } =
  await import("./file-upload-wrapper.server");

const SESSION_ID = "4f79f286-6f14-49cb-bb84-16fd4f906eba";

function session(token: string): ExtendedSession {
  return { user: { id: "1", token, name: "Test" }, expires: "2099-01-01" };
}

function upload(fields: Record<string, string | File>): NextRequest {
  const form = new FormData();
  for (const [name, value] of Object.entries(fields)) form.append(name, value);
  const base = new Request("http://localhost:3000/api/things/42/files", {
    method: "POST",
    body: form,
    headers: {
      "X-Forwarded-For": "203.0.113.7",
      "X-POSTHOG-SESSION-ID": SESSION_ID,
    },
  });
  return new NextRequest(base);
}

function pdf(): File {
  return new File(["%PDF-1.7"], "scan.pdf", { type: "application/pdf" });
}

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/problem+json" },
  });
}

const context = { params: Promise.resolve({ id: "42" }) };
const handler = proxyFileUpload((p) => `/api/things/${String(p.id)}/files`, {
  ...SCAN_UPLOAD,
  file: "file",
  fields: ["category"],
  optionalFields: ["note"],
});

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.clearAllMocks();
  fetchMock = vi.fn();
  global.fetch = fetchMock as unknown as typeof fetch;
  mockAuth.mockResolvedValue(session("stale"));
});

function sent(call = 0): { url: string; headers: Headers; body: FormData } {
  const [url, init] = fetchMock.mock.calls[call] as [string, RequestInit];
  return {
    url,
    headers: new Headers(init.headers),
    body: init.body as FormData,
  };
}

describe("proxyFileUpload", () => {
  it("forwards the file, the listed fields and the client identity", async () => {
    fetchMock.mockResolvedValue(json(200, { data: { id: "7" } }));

    const response = await handler(
      upload({ file: pdf(), category: "medical", note: "x", evil: "1" }),
      context,
    );

    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({ data: { id: "7" } });
    const call = sent();
    expect(call.url).toBe("http://backend.test/api/things/42/files");
    expect(call.headers.get("Authorization")).toBe("Bearer stale");
    expect(call.headers.get("X-Forwarded-For")).toBe("203.0.113.7");
    expect(call.headers.get("X-POSTHOG-SESSION-ID")).toBe(SESSION_ID);
    expect(call.body.get("file")).toBeInstanceOf(File);
    expect(call.body.get("category")).toBe("medical");
    expect(call.body.get("note")).toBe("x");
    expect(call.body.get("evil")).toBeNull();
  });

  it("repeats the upload once with a refreshed session after a 401", async () => {
    mockUncachedAuth.mockResolvedValue(session("fresh"));
    fetchMock
      .mockResolvedValueOnce(json(401, { detail: "expired" }))
      .mockResolvedValueOnce(json(200, { data: {} }));

    const response = await handler(
      upload({ file: pdf(), category: "medical" }),
      context,
    );

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(sent(1).headers.get("Authorization")).toBe("Bearer fresh");
    expect(sent(1).headers.get("X-POSTHOG-SESSION-ID")).toBe(SESSION_ID);
    expect(sent(1).body.get("file")).toBeInstanceOf(File);
  });

  it("keeps a backend error's status and problem body", async () => {
    const body = {
      type: "about:blank",
      title: "Conflict",
      status: 409,
      detail: "report closed",
      code: "general.business_rejection",
    };
    fetchMock.mockResolvedValue(json(409, body));

    const response = await handler(
      upload({ file: pdf(), category: "medical" }),
      context,
    );

    expect(response.status).toBe(409);
    expect(await response.json()).toMatchObject({
      status: 409,
      code: "general.business_rejection",
    });
  });

  it.each([
    ["the file", { category: "medical" }, "files.file_missing"],
    ["a required field", { file: pdf() }, "general.input"],
  ])("answers 400 without %s", async (_name, fields, code) => {
    const response = await handler(upload(fields), context);

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({ code });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects what the content check refuses before the backend sees it", async () => {
    const strict = proxyFileUpload("/api/things/photo", {
      ...SCAN_UPLOAD,
      file: "file",
      verify: () => {
        throw new FileValidationError(
          "bad bytes",
          415,
          "files.file_type_not_allowed",
        );
      },
    });

    const response = await strict(upload({ file: pdf() }), context);

    expect(response.status).toBe(415);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
