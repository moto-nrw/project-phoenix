import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { authStepProxy } from "./auth-proxy.server";

const { captureException } = vi.hoisted(() => ({ captureException: vi.fn() }));
vi.mock("@sentry/nextjs", () => ({ captureException }));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const fetchMock = vi.fn<typeof fetch>();
vi.stubGlobal("fetch", fetchMock);

function step(opts: { body?: string; headers?: Record<string, string> } = {}) {
  return new NextRequest("http://app.test/api/auth/mfa/resend", {
    method: "POST",
    headers: {
      "x-forwarded-for": "203.0.113.10",
      "user-agent": "test-agent",
      ...opts.headers,
    },
    body: opts.body,
  });
}

const sent = () => {
  const [url, init] = fetchMock.mock.calls[0]!;
  return { url, init: init!, headers: new Headers(init!.headers) };
};

const resend = authStepProxy("/auth/mfa/resend", "optional-json");

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue(Response.json({ ok: true }));
});

describe("authStepProxy", () => {
  it("forwards the body, the step's proof and the client identity, but no cookie", async () => {
    const response = await resend(
      step({
        body: JSON.stringify({ challenge_token: "c" }),
        headers: {
          authorization: "Bearer challenge-proof",
          cookie: "authjs.session-token=secret; mfa_trust_device=d",
        },
      }),
    );

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ ok: true });
    expect(sent().url).toBe("http://backend.test/auth/mfa/resend");
    expect(sent().init).toMatchObject({
      method: "POST",
      body: JSON.stringify({ challenge_token: "c" }),
    });
    expect(sent().headers.get("authorization")).toBe("Bearer challenge-proof");
    expect(sent().headers.get("x-forwarded-for")).toBe("203.0.113.10");
    expect(sent().headers.get("x-moto-frontend-origin")).toBe(
      "http://app.test",
    );
    expect(sent().headers.get("cookie")).toBeNull();
  });

  it("sends {} for an empty body and no Authorization without a proof", async () => {
    await resend(step());

    expect(sent().init.body).toBe("{}");
    expect(sent().headers.get("authorization")).toBeNull();
  });

  it("sends no body for a step without one", async () => {
    await authStepProxy("/auth/mfa/enroll/start", "none")(step());

    expect(sent().init.body).toBeUndefined();
  });

  it("passes a backend refusal through with its status and body", async () => {
    fetchMock.mockResolvedValueOnce(
      Response.json(
        { status: 429, title: "Too Many Requests", code: "general.rate_limit" },
        { status: 429, headers: { "Retry-After": "60" } },
      ),
    );

    const response = await resend(step());

    expect(response.status).toBe(429);
    expect(response.headers.get("retry-after")).toBe("60");
    await expect(response.json()).resolves.toMatchObject({
      code: "general.rate_limit",
    });
  });

  it("answers a backend 204 without a body", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const response = await resend(step());

    expect(response.status).toBe(204);
    expect(await response.text()).toBe("");
  });

  it.each([
    ["a non-JSON body", "upstream exploded at host-12"],
    ["malformed JSON", '{"detail": "db at 10.0.0.5'],
  ])("replaces a backend 5xx with %s by a safe envelope", async (_c, body) => {
    fetchMock.mockResolvedValueOnce(new Response(body, { status: 502 }));

    const response = await resend(step());

    expect(response.status).toBe(502);
    const text = await response.text();
    expect(text).not.toContain("10.0.0.5");
    expect(text).not.toContain("host-12");
    expect(JSON.parse(text)).toMatchObject({ status: 502 });
  });

  it("answers an unreachable backend with 503 and reports it once", async () => {
    fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));

    const response = await resend(step());

    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toMatchObject({
      code: "general.unavailable",
    });
    expect(captureException).toHaveBeenCalledOnce();
  });

  it("gives up on a backend that does not answer within the auth-step budget", async () => {
    vi.useFakeTimers();
    fetchMock.mockImplementationOnce(
      (_url, init) =>
        new Promise((_resolve, reject) =>
          init?.signal?.addEventListener("abort", () =>
            reject(init.signal?.reason),
          ),
        ),
    );
    let settled = false;
    const pending = resend(step()).finally(() => {
      settled = true;
    });

    await vi.advanceTimersByTimeAsync(9_999);
    expect(settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);

    expect((await pending).status).toBe(503);
  });
});
