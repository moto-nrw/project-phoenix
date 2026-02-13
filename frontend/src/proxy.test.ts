import { describe, it, expect, vi, beforeEach } from "vitest";
import type { NextRequest } from "next/server";
import { proxy } from "./proxy";

// Use vi.hoisted for mock values referenced in vi.mock
const { mockNext, mockRedirect } = vi.hoisted(() => ({
  mockNext: vi.fn(() => ({ status: 200, type: "next" })),
  mockRedirect: vi.fn((url: URL) => ({ status: 307, url, type: "redirect" })),
}));

vi.mock("next/server", async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return {
    ...actual,
    NextResponse: {
      next: mockNext,
      redirect: mockRedirect,
    },
  };
});

describe("proxy", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const createRequest = (
    pathname: string,
    cookieValue?: string,
  ): NextRequest => {
    const url = `http://localhost:3000${pathname}`;
    return {
      nextUrl: { pathname },
      url,
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

    proxy(request);

    expect(mockNext).toHaveBeenCalled();
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("redirects to login when accessing operator route without token", () => {
    const request = createRequest("/operator/suggestions");

    proxy(request);

    expect(mockRedirect).toHaveBeenCalledWith(
      new URL("/operator/login", "http://localhost:3000/operator/suggestions"),
    );
  });

  it("allows access to operator route with valid token", () => {
    const request = createRequest("/operator/suggestions", "valid-token");

    proxy(request);

    expect(mockNext).toHaveBeenCalled();
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("allows access to non-operator routes without token", () => {
    const request = createRequest("/dashboard");

    proxy(request);

    expect(mockNext).toHaveBeenCalled();
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("redirects to login for operator settings without token", () => {
    const request = createRequest("/operator/settings");

    proxy(request);

    expect(mockRedirect).toHaveBeenCalled();
  });

  it("allows access to operator settings with token", () => {
    const request = createRequest("/operator/settings", "token-123");

    proxy(request);

    expect(mockNext).toHaveBeenCalled();
  });

  it("does not redirect for operator login with token", () => {
    const request = createRequest("/operator/login", "existing-token");

    proxy(request);

    expect(mockNext).toHaveBeenCalled();
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("redirects for nested operator routes without token", () => {
    const request = createRequest("/operator/suggestions/123");

    proxy(request);

    expect(mockRedirect).toHaveBeenCalled();
  });

  it("allows nested operator routes with token", () => {
    const request = createRequest("/operator/suggestions/123", "token");

    proxy(request);

    expect(mockNext).toHaveBeenCalled();
  });

  it("checks for cookie value existence, not just cookie object", () => {
    const request = {
      nextUrl: { pathname: "/operator/dashboard" },
      url: "http://localhost:3000/operator/dashboard",
      cookies: {
        get: vi.fn(() => ({ value: "" })), // Empty value
      },
    } as unknown as NextRequest;

    proxy(request);

    expect(mockRedirect).toHaveBeenCalled();
  });
});
