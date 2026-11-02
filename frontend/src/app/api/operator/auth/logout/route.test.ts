import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

type MockSession = { user: { token: string } } | null;

const { mockAuth, mockUncachedAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<MockSession>>(),
  mockUncachedAuth: vi.fn<() => Promise<MockSession>>(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockUncachedAuth,
  withOperatorAuth: (handler: unknown) => handler,
}));

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

const { POST } = await import("./route");

const request = () =>
  new NextRequest("http://localhost:3000/api/operator/auth/logout", {
    method: "POST",
  });
const refused = () =>
  Response.json({ status: 401, code: "general.permission" }, { status: 401 });

// #5194: the browser takes a 401 for "nothing left to revoke", so the
// operator logout answers 401 only once the session is known to be over.
describe("POST /api/operator/auth/logout", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
    mockAuth.mockResolvedValue({ user: { token: "access-old" } });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("confirms the revocation with 204", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const response = await POST(request());

    expect(response.status).toBe(204);
    expect(mockUncachedAuth).not.toHaveBeenCalled();
  });

  it("revokes with the renewed token when the old one was refused", async () => {
    fetchMock
      .mockResolvedValueOnce(refused())
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    mockUncachedAuth.mockResolvedValue({ user: { token: "access-new" } });

    const response = await POST(request());

    expect(response.status).toBe(204);
    const [, retry] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect((retry.headers as Record<string, string>).Authorization).toBe(
      "Bearer access-new",
    );
  });

  it("answers 503, not 401, while the renewal fails for a while", async () => {
    fetchMock.mockResolvedValueOnce(refused());
    mockUncachedAuth.mockResolvedValue({ user: { token: "access-old" } });

    const response = await POST(request());

    expect(response.status).toBe(503);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("passes the 401 once the renewal ended the session", async () => {
    fetchMock.mockResolvedValueOnce(refused());
    mockUncachedAuth.mockResolvedValue({ user: { token: "" } });

    const response = await POST(request());

    expect(response.status).toBe(401);
  });
});
