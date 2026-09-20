import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { POST as status } from "./status/route";
import { POST as sessions } from "./sessions/route";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://server:8080",
}));

const fetchMock = vi.fn();

function request(body: unknown) {
  return new NextRequest("http://messe-demo.localhost:3000/api/demo/access/x", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});

describe("demo access routes", () => {
  it("asks the backend for the status with the token in a header, not the URL", async () => {
    fetchMock.mockResolvedValue(Response.json({ status: "ready" }));

    const response = await status(request({ token: "secret-token" }));

    expect(await response.json()).toEqual({ status: "ready" });
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://server:8080/demo/access/status");
    expect((init.headers as Record<string, string>).Authorization).toBe(
      "Bearer secret-token",
    );
  });

  it("redeems by POST and passes the backend status through", async () => {
    fetchMock.mockResolvedValue(
      Response.json({ code: "demo_access_expired" }, { status: 410 }),
    );

    const response = await sessions(request({ token: "secret-token" }));

    expect(response.status).toBe(410);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://server:8080/demo/access/sessions");
    expect(init.method).toBe("POST");
    expect(init.body).toBe(JSON.stringify({ token: "secret-token" }));
  });

  it("answers 404 without a token and never calls the backend", async () => {
    const response = await sessions(request({}));

    expect(response.status).toBe(404);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
