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

  // The banner switches the demo role later (#3467). The token stays in a
  // cookie no script can read, sent to the demo routes only.
  it("keeps a redeemed token in an httpOnly cookie for later role switches", async () => {
    fetchMock.mockResolvedValue(
      Response.json({
        access_token: "access",
        refresh_token: "refresh",
        demo: { access_id: "4711", role: "lead" },
      }),
    );

    const response = await sessions(
      request({ token: "secret-token", role: "lead" }),
    );

    expect(response.status).toBe(200);
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.body).toBe(
      JSON.stringify({ token: "secret-token", role: "lead" }),
    );
    const cookie = response.cookies.get("moto-demo-token");
    expect(cookie).toMatchObject({
      value: "secret-token",
      httpOnly: true,
      sameSite: "strict",
      path: "/api/demo",
      maxAge: 14 * 24 * 60 * 60,
    });
    expect(await response.json()).not.toHaveProperty("token");
  });

  it("switches the role with the token from the cookie", async () => {
    fetchMock.mockResolvedValue(
      Response.json({ access_token: "access", refresh_token: "refresh" }),
    );
    const switchRequest = request({ role: "caregiver" });
    switchRequest.cookies.set("moto-demo-token", "secret-token");

    const response = await sessions(switchRequest);

    expect(response.status).toBe(200);
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.body).toBe(
      JSON.stringify({ token: "secret-token", role: "caregiver" }),
    );
  });

  it("sets no cookie when the backend refuses the token", async () => {
    fetchMock.mockResolvedValue(
      Response.json({ code: "demo_access_unknown" }, { status: 404 }),
    );

    const response = await sessions(request({ token: "wrong-token" }));

    expect(response.status).toBe(404);
    expect(response.cookies.get("moto-demo-token")).toBeUndefined();
  });

  it("answers 404 without a token and never calls the backend", async () => {
    const response = await sessions(request({}));

    expect(response.status).toBe(404);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
