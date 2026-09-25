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

  it("keeps a no-content response bodyless", async () => {
    const response = forwardBackendResponse(
      new Response(null, { status: 204 }),
    );
    expect(response.status).toBe(204);
    expect(await response.text()).toBe("");
  });
});
