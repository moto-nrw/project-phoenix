import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { ServerSession } from "~/server/auth/shared";
import type { ServerAuthRequest } from "~/server/auth/route-handler";

// withTenantAuth is a pass-through in tests (setup-common); the request's
// `auth` stands for the session the wrapper reads from the cookie.
const { POST } = await import("./route");

const mockFetch = vi.fn();

beforeEach(() => {
  mockFetch.mockReset();
  mockFetch.mockResolvedValue(new Response(null, { status: 204 }));
  vi.stubGlobal("fetch", mockFetch);
});

function endRequest(
  auth: ServerSession | null,
  body: Record<string, unknown> = {},
): ServerAuthRequest {
  const request = new NextRequest(
    "http://localhost:3000/api/auth/staff-preview/end",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  ) as ServerAuthRequest;
  request.auth = auth;
  return request;
}

function session(user: Partial<ServerSession["user"]>): ServerSession {
  return {
    user: { id: "42", ...user },
    expires: "2099-01-01T00:00:00.000Z",
  };
}

function forwardedBody(): unknown {
  const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
  return JSON.parse(init.body as string);
}

const context = { params: Promise.resolve({}) };

describe("POST /api/auth/staff-preview/end (#5005)", () => {
  // The browser no longer holds the preview token: it ends a running preview
  // with an empty body, and the route reads the token from the session.
  it("records the running preview with the token of the session", async () => {
    const response = await POST(
      endRequest(session({ isPreview: true, token: "preview-token" })),
      context,
    );

    expect(response.status).toBe(204);
    expect(mockFetch).toHaveBeenCalledOnce();
    expect(mockFetch.mock.calls[0]?.[0]).toBe(
      "http://server:8080/auth/staff-preview/end",
    );
    expect(forwardedBody()).toEqual({ preview_token: "preview-token" });
  });

  // A start whose session swap failed presents the token it was handed.
  it("records a presented token although the session holds no preview", async () => {
    await POST(
      endRequest(session({ token: "admin-token" }), {
        preview_token: "minted-preview-token",
      }),
      context,
    );

    expect(forwardedBody()).toEqual({ preview_token: "minted-preview-token" });
  });

  it("never sends the admin's own token as a preview token", async () => {
    const response = await POST(
      endRequest(session({ token: "admin-token" })),
      context,
    );

    expect(response.status).toBe(204);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("asks nothing when no session is left", async () => {
    const response = await POST(endRequest(null), context);

    expect(response.status).toBe(204);
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
