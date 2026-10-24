import { describe, expect, it } from "vitest";
import { forwardBackendResponse } from "./backend-proxy-response.server";

describe("forwardBackendResponse", () => {
  it("preserves a structured backend conflict byte-for-byte", async () => {
    const body =
      '{"code":"item_conflict","details":{"id":"42"},"errors":[{"field":"name"}],"instance":"/api/items/42"}';
    const response = forwardBackendResponse(
      new Response(body, {
        status: 409,
        headers: {
          "Content-Type": "application/problem+json",
          "Retry-After": "17",
        },
      }),
    );

    expect(response.status).toBe(409);
    expect(response.headers.get("Content-Type")).toBe(
      "application/problem+json",
    );
    expect(response.headers.get("Retry-After")).toBe("17");
    expect(await response.text()).toBe(body);
  });

  it("replaces a backend 5xx body with a safe problem envelope", async () => {
    const response = forwardBackendResponse(
      new Response(
        JSON.stringify({
          type: "about:blank",
          title: "Internal Server Error",
          status: 500,
          detail: "database at db.internal refused the connection",
          code: "iot.schulhof_not_configured",
          details: { host: "db.internal" },
        }),
        {
          status: 500,
          headers: { "Content-Type": "application/problem+json" },
        },
      ),
    );

    expect(response.status).toBe(500);
    const body = await response.text();
    expect(JSON.parse(body)).toMatchObject({
      status: 500,
      detail: "Internal Server Error",
      code: "iot.schulhof_not_configured",
    });
    expect(body).not.toContain("db.internal");
    expect(body).not.toContain("refused the connection");
  });

  it("adds the request ID to a sanitized backend error", async () => {
    const response = forwardBackendResponse(
      new Response("upstream diagnostics", { status: 502 }),
      new Request("https://frontend.test/api/example", {
        headers: { "X-Request-ID": "request-502" },
      }),
    );

    expect(await response.json()).toMatchObject({
      status: 502,
      instance: "request-502",
    });
  });

  it("replaces an unregistered backend code with the status class", async () => {
    const response = forwardBackendResponse(
      new Response(
        JSON.stringify({
          type: "about:blank",
          title: "Internal Server Error",
          status: 500,
          detail: "database at db.internal refused the connection",
          code: "upstream.unknown_code",
        }),
        { status: 500 },
      ),
    );

    expect(await response.json()).toMatchObject({
      status: 500,
      detail: "Internal Server Error",
      code: "general.server",
    });
  });

  it("keeps a no-content response bodyless", async () => {
    const response = forwardBackendResponse(
      new Response(null, { status: 204 }),
    );
    expect(response.status).toBe(204);
    expect(await response.text()).toBe("");
  });
});
