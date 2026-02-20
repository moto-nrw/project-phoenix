import { describe, it, expect, vi, beforeEach } from "vitest";
import type { NextRequest } from "next/server";
import { middleware } from "./middleware";

// Use vi.hoisted for mock values referenced in vi.mock
const { mockNext, mockRedirect } = vi.hoisted(() => {
  const mockHeaders = () => ({ set: vi.fn(), get: vi.fn(), has: vi.fn() });
  return {
    mockNext: vi.fn(() => ({
      status: 200,
      type: "next",
      headers: mockHeaders(),
    })),
    mockRedirect: vi.fn((url: URL) => ({ status: 307, url, type: "redirect" })),
  };
});

vi.mock("next/server", async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return {
    ...actual,
    NextResponse: {
      next: mockNext,
      redirect: mockRedirect,
      rewrite: vi.fn((url: URL) => ({
        status: 200,
        url,
        type: "rewrite",
        headers: { set: vi.fn(), get: vi.fn(), has: vi.fn() },
      })),
    },
  };
});

describe("middleware — operator auth guard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const createRequest = (
    pathname: string,
    cookieValue?: string,
    host = "localhost:3000",
  ): NextRequest => {
    const url = `http://${host}${pathname}`;
    return {
      nextUrl: { pathname, clone: () => ({ pathname }) },
      url,
      headers: {
        get: vi.fn((name: string) => (name === "host" ? host : null)),
      },
      cookies: {
        get: vi.fn((name: string) =>
          name === "phoenix-operator-token" && cookieValue
            ? { value: cookieValue }
            : undefined,
        ),
      },
    } as unknown as NextRequest;
  };

  it("allows access to operator login page without token", () => {
    const request = createRequest("/operator/login");

    middleware(request);

    expect(mockNext).toHaveBeenCalled();
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("redirects to login when accessing operator route without token", () => {
    const request = createRequest("/operator/suggestions");

    middleware(request);

    expect(mockRedirect).toHaveBeenCalledWith(
      new URL("/operator/login", "http://localhost:3000/operator/suggestions"),
    );
  });

  it("allows access to operator route with valid token", () => {
    const request = createRequest("/operator/suggestions", "valid-token");

    middleware(request);

    expect(mockNext).toHaveBeenCalled();
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("allows access to non-operator routes without token", () => {
    const request = createRequest("/dashboard");

    middleware(request);

    expect(mockNext).toHaveBeenCalled();
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("redirects to login for operator settings without token", () => {
    const request = createRequest("/operator/settings");

    middleware(request);

    expect(mockRedirect).toHaveBeenCalled();
  });

  it("allows access to operator settings with token", () => {
    const request = createRequest("/operator/settings", "token-123");

    middleware(request);

    expect(mockNext).toHaveBeenCalled();
  });

  it("does not redirect for operator login with token", () => {
    const request = createRequest("/operator/login", "existing-token");

    middleware(request);

    expect(mockNext).toHaveBeenCalled();
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("redirects for nested operator routes without token", () => {
    const request = createRequest("/operator/suggestions/123");

    middleware(request);

    expect(mockRedirect).toHaveBeenCalled();
  });

  it("allows nested operator routes with token", () => {
    const request = createRequest("/operator/suggestions/123", "token");

    middleware(request);

    expect(mockNext).toHaveBeenCalled();
  });

  it("checks for cookie value existence, not just cookie object", () => {
    const request = {
      nextUrl: {
        pathname: "/operator/dashboard",
        clone: () => ({ pathname: "/operator/dashboard" }),
      },
      url: "http://localhost:3000/operator/dashboard",
      headers: { get: vi.fn(() => "localhost:3000") },
      cookies: {
        get: vi.fn(() => ({ value: "" })), // Empty value
      },
    } as unknown as NextRequest;

    middleware(request);

    expect(mockRedirect).toHaveBeenCalled();
  });
});
