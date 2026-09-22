import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { POST as reset } from "./route";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://server:8080",
}));

const fetchMock = vi.fn();

function request(body: unknown, token?: string) {
  const headers = new Headers({ "Content-Type": "application/json" });
  if (token) headers.set("cookie", `moto-demo-token=${token}`);
  return new NextRequest(
    "http://ogs-beispiel.demo.example/api/demo/access/reset",
    {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    },
  );
}

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

// „Demo neu anfangen" (#3470): the banner sends its role, the token comes
// from the cookie, and the answer is the waiting room with the token in the
// fragment, the role and the restart mark.
describe("demo reset route", () => {
  it("asks the backend for a fresh school and answers with the waiting room", async () => {
    fetchMock.mockResolvedValue(
      Response.json(
        {
          status: "preparing",
          entry_url: "https://demo.example/demo#token=secret-token",
        },
        { status: 202 },
      ),
    );

    const response = await reset(request({ role: "lead" }, "secret-token"));

    expect(response.status).toBe(202);
    expect(await response.json()).toEqual({
      entry_url:
        "https://demo.example/demo#token=secret-token&restarted=1&role=lead",
    });
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://server:8080/demo/access/reset");
    expect(init.method).toBe("POST");
    expect(init.body).toBe(JSON.stringify({ token: "secret-token" }));
  });

  it("leaves an unknown role out of the fragment", async () => {
    fetchMock.mockResolvedValue(
      Response.json(
        { entry_url: "https://demo.example/demo#token=secret-token" },
        { status: 202 },
      ),
    );

    const response = await reset(request({ role: "hacker" }, "secret-token"));

    expect(await response.json()).toEqual({
      entry_url: "https://demo.example/demo#token=secret-token&restarted=1",
    });
  });

  it("answers 404 without a token and asks the backend nothing", async () => {
    const response = await reset(request({ role: "lead" }));

    expect(response.status).toBe(404);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("passes the backend's refusal through", async () => {
    fetchMock.mockResolvedValue(
      Response.json(
        { status: "error", code: "demo_access_expired" },
        { status: 410 },
      ),
    );

    const response = await reset(request({ role: "lead" }, "secret-token"));

    expect(response.status).toBe(410);
    expect(await response.json()).toMatchObject({
      code: "demo_access_expired",
    });
  });

  it("answers 500 when the backend cannot be reached", async () => {
    fetchMock.mockRejectedValue(new Error("connection refused"));

    const response = await reset(request({ role: "lead" }, "secret-token"));

    expect(response.status).toBe(500);
  });
});
