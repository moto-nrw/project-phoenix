import { describe, it, expect } from "vitest";
import { NextRequest } from "next/server";
import {
  buildQueryString,
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
  createUnauthorizedResponse,
  isStringParam,
  type RouteContext,
} from "./route-wrapper-utils";

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const init: { method: string; body?: string; headers?: HeadersInit } = {
    method: options.method ?? "GET",
  };
  if (options.body) {
    init.body = JSON.stringify(options.body);
    init.headers = { "Content-Type": "application/json" };
  }
  return new NextRequest(url, init);
}

describe("buildQueryString", () => {
  it("returns empty string for no query params", () => {
    const request = createMockRequest("/api/test");
    expect(buildQueryString(request)).toBe("");
  });

  it("returns query string with single param", () => {
    const request = createMockRequest("/api/test?status=active");
    expect(buildQueryString(request)).toBe("?status=active");
  });

  it("returns query string with multiple params", () => {
    const request = createMockRequest("/api/test?status=active&page=2");
    const result = buildQueryString(request);
    expect(result).toContain("status=active");
    expect(result).toContain("page=2");
    expect(result.startsWith("?")).toBe(true);
  });

  it("handles special characters in query params", () => {
    const request = createMockRequest("/api/test?search=hello%20world");
    expect(buildQueryString(request)).toBe("?search=hello+world");
  });
});

describe("extractParams", () => {
  it("extracts params from context", async () => {
    const request = createMockRequest("/api/groups/123");
    const context: RouteContext = {
      params: Promise.resolve({ id: "123", name: "test" }),
    };
    const params = await extractParams(request, context);
    expect(params.id).toBe("123");
    expect(params.name).toBe("test");
  });

  it("skips undefined context params", async () => {
    const request = createMockRequest("/api/groups/123");
    const context: RouteContext = {
      params: Promise.resolve({ id: "123", optional: undefined }),
    };
    const params = await extractParams(request, context);
    expect(params.id).toBe("123");
    expect(params.optional).toBeUndefined();
  });

  it("extracts ID from URL path when not in context", async () => {
    const request = createMockRequest("/api/groups/456/members");
    const context: RouteContext = { params: Promise.resolve({}) };
    const params = await extractParams(request, context);
    expect(params.id).toBe("456");
  });

  it("uses context ID over URL path ID", async () => {
    const request = createMockRequest("/api/groups/456/members");
    const context: RouteContext = { params: Promise.resolve({ id: "789" }) };
    const params = await extractParams(request, context);
    expect(params.id).toBe("789");
  });

  it("extracts query params from URL", async () => {
    const request = createMockRequest("/api/groups?status=active&limit=10");
    const context: RouteContext = { params: Promise.resolve({}) };
    const params = await extractParams(request, context);
    expect(params.status).toBe("active");
    expect(params.limit).toBe("10");
  });

  it("merges context params and query params", async () => {
    const request = createMockRequest("/api/groups/123?status=active");
    const context: RouteContext = { params: Promise.resolve({ id: "123" }) };
    const params = await extractParams(request, context);
    expect(params.id).toBe("123");
    expect(params.status).toBe("active");
  });

  it("extracts last numeric segment as ID when multiple exist", async () => {
    const request = createMockRequest("/api/groups/123/members/456");
    const context: RouteContext = { params: Promise.resolve({}) };
    const params = await extractParams(request, context);
    expect(params.id).toBe("456");
  });
});

describe("parseRequestBody", () => {
  it("parses valid JSON body", async () => {
    const body = { name: "Test", value: 123 };
    const request = createMockRequest("/api/test", { method: "POST", body });
    const result = await parseRequestBody<typeof body>(request);
    expect(result).toEqual(body);
  });

  it("returns empty object for empty body", async () => {
    const request = new NextRequest("http://localhost:3000/api/test", {
      method: "POST",
    });
    const result = await parseRequestBody<Record<string, unknown>>(request);
    expect(result).toEqual({});
  });

  it("returns empty object for invalid JSON", async () => {
    const request = new NextRequest("http://localhost:3000/api/test", {
      method: "POST",
      body: "invalid json{",
      headers: { "Content-Type": "application/json" },
    });
    const result = await parseRequestBody<Record<string, unknown>>(request);
    expect(result).toEqual({});
  });

  it("handles nested objects", async () => {
    const body = {
      user: { name: "Test", role: "admin" },
      meta: { version: 1 },
    };
    const request = createMockRequest("/api/test", { method: "POST", body });
    const result = await parseRequestBody<typeof body>(request);
    expect(result).toEqual(body);
  });
});

describe("wrapInApiResponse", () => {
  it("wraps plain data in ApiResponse", () => {
    const data = { id: "1", name: "Test" };
    const response = wrapInApiResponse(data);
    expect(response).toEqual({ success: true, message: "Success", data });
  });

  it("returns already wrapped data unchanged", () => {
    // wrapInApiResponse checks if data itself has "success" in it
    const alreadyWrapped = { success: true, data: { id: "1" } };
    const response = wrapInApiResponse(alreadyWrapped);
    expect(response).toBe(alreadyWrapped);
  });

  it("wraps null data", () => {
    const response = wrapInApiResponse(null);
    expect(response).toEqual({ success: true, message: "Success", data: null });
  });

  it("wraps array data", () => {
    const data = [1, 2, 3];
    const response = wrapInApiResponse(data);
    expect(response).toEqual({ success: true, message: "Success", data });
  });

  it("wraps primitive data", () => {
    const response = wrapInApiResponse("test string");
    expect(response).toEqual({
      success: true,
      message: "Success",
      data: "test string",
    });
  });
});

describe("createUnauthorizedResponse", () => {
  it("creates 401 response with error message", async () => {
    const response = createUnauthorizedResponse();
    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json).toEqual({ error: "Unauthorized" });
  });
});

describe("isStringParam", () => {
  it("returns true for string param", () => {
    expect(isStringParam("test")).toBe(true);
    expect(isStringParam("123")).toBe(true);
    expect(isStringParam("")).toBe(true);
  });

  it("returns false for non-string param", () => {
    expect(isStringParam(123)).toBe(false);
    expect(isStringParam(null)).toBe(false);
    expect(isStringParam(undefined)).toBe(false);
    expect(isStringParam({})).toBe(false);
    expect(isStringParam([])).toBe(false);
  });
});
