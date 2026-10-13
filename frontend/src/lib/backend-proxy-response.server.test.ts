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
          detail: "database at db.internal refused the connection",
          code: "general.server",
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
      code: "general.server",
    });
    expect(body).not.toContain("db.internal");
    expect(body).not.toContain("refused the connection");
  });

  it("keeps a no-content response bodyless", async () => {
    const response = forwardBackendResponse(
      new Response(null, { status: 204 }),
    );
    expect(response.status).toBe(204);
    expect(await response.text()).toBe("");
  });
});
