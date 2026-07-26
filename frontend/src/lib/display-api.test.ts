import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

import {
  DisplayDashboardError,
  fetchDisplayDashboard,
  listDisplays,
  createDisplay,
  updateDisplay,
  regenerateDisplayToken,
  deleteDisplay,
} from "./display-api";
import type { BackendDisplay } from "./display-helpers";

const originalFetch = globalThis.fetch;

function mockFetch(
  impl: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>,
) {
  const fn = vi.fn(impl);
  globalThis.fetch = fn as typeof globalThis.fetch;
  return fn;
}

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

function mkBackendDisplay(id: number): BackendDisplay {
  return {
    id,
    name: `Display ${id}`,
    is_active: true,
    created_at: "2026-07-01T08:00:00Z",
    updated_at: "2026-07-01T08:00:00Z",
  };
}

beforeEach(() => {
  globalThis.fetch = originalFetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("fetchDisplayDashboard", () => {
  it("sends the token via X-Display-Token header and returns the payload", async () => {
    const payload = { status: "active", school_name: "Testschule" };
    const fetchMock = mockFetch(() => Promise.resolve(jsonResponse(payload)));

    const result = await fetchDisplayDashboard("raw-token");

    expect(result).toEqual(payload);
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/display/dashboard");
    expect(new Headers(init?.headers).get("X-Display-Token")).toBe("raw-token");
    expect(init?.cache).toBe("no-store");
  });

  it("throws DisplayDashboardError with the status on non-OK responses", async () => {
    mockFetch(() => Promise.resolve(jsonResponse({}, { status: 404 })));

    await expect(fetchDisplayDashboard("revoked")).rejects.toThrow(
      DisplayDashboardError,
    );
    mockFetch(() => Promise.resolve(jsonResponse({}, { status: 404 })));
    await expect(fetchDisplayDashboard("revoked")).rejects.toMatchObject({
      status: 404,
    });
  });
});

describe("listDisplays", () => {
  it("unwraps the envelope and maps backend displays", async () => {
    mockFetch(() =>
      Promise.resolve(
        jsonResponse({ status: "success", data: [mkBackendDisplay(7)] }),
      ),
    );

    const displays = await listDisplays();

    expect(displays).toEqual([
      {
        id: "7",
        name: "Display 7",
        isActive: true,
        createdAt: "2026-07-01T08:00:00Z",
      },
    ]);
  });

  it("returns an empty list when data is missing", async () => {
    mockFetch(() => Promise.resolve(jsonResponse({ status: "success" })));

    await expect(listDisplays()).resolves.toEqual([]);
  });

  it("throws the backend error message on failure", async () => {
    mockFetch(() =>
      Promise.resolve(jsonResponse({ error: "kaputt" }, { status: 500 })),
    );

    await expect(listDisplays()).rejects.toThrow("kaputt");
  });

  it("falls back to a generic message when the error body is not JSON", async () => {
    mockFetch(() => Promise.resolve(new Response("nope", { status: 502 })));

    await expect(listDisplays()).rejects.toThrow("request failed (502)");
  });
});

describe("createDisplay", () => {
  it("POSTs the name and returns display plus raw token", async () => {
    const fetchMock = mockFetch(() =>
      Promise.resolve(
        jsonResponse({
          data: { display: mkBackendDisplay(3), token: "secret-token" },
        }),
      ),
    );

    const result = await createDisplay("Foyer");

    expect(result.token).toBe("secret-token");
    expect(result.display.id).toBe("3");
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/displays");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(init?.body as string)).toEqual({ name: "Foyer" });
  });
});

describe("updateDisplay", () => {
  it("PATCHes only the provided fields with snake_case keys", async () => {
    const fetchMock = mockFetch(() =>
      Promise.resolve(jsonResponse({ data: { display: mkBackendDisplay(4) } })),
    );

    const result = await updateDisplay("4", { isActive: false });

    expect(result.id).toBe("4");
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/displays/4");
    expect(init?.method).toBe("PATCH");
    expect(JSON.parse(init?.body as string)).toEqual({ is_active: false });
  });

  it("includes the name when renaming", async () => {
    const fetchMock = mockFetch(() =>
      Promise.resolve(jsonResponse({ data: { display: mkBackendDisplay(4) } })),
    );

    await updateDisplay("4", { name: "Mensa" });

    const [, init] = fetchMock.mock.calls[0]!;
    expect(JSON.parse(init?.body as string)).toEqual({ name: "Mensa" });
  });
});

describe("regenerateDisplayToken", () => {
  it("POSTs to the regenerate endpoint and returns the new token", async () => {
    const fetchMock = mockFetch(() =>
      Promise.resolve(jsonResponse({ data: { token: "fresh-token" } })),
    );

    await expect(regenerateDisplayToken("9")).resolves.toBe("fresh-token");
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/displays/9/regenerate");
    expect(init?.method).toBe("POST");
  });
});

describe("deleteDisplay", () => {
  it("DELETEs the display and resolves on success", async () => {
    const fetchMock = mockFetch(() =>
      Promise.resolve(new Response(null, { status: 204 })),
    );

    await expect(deleteDisplay("5")).resolves.toBeUndefined();
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/displays/5");
    expect(init?.method).toBe("DELETE");
  });

  it("throws the backend error message on failure", async () => {
    mockFetch(() =>
      Promise.resolve(jsonResponse({ error: "verboten" }, { status: 403 })),
    );

    await expect(deleteDisplay("5")).rejects.toThrow("verboten");
  });

  it("falls back to a generic message when the error body is not JSON", async () => {
    mockFetch(() => Promise.resolve(new Response("nope", { status: 500 })));

    await expect(deleteDisplay("5")).rejects.toThrow("delete failed (500)");
  });
});
