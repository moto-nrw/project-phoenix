import { describe, expect, it } from "vitest";
import { withPrivateNoStore } from "./private-no-store";

describe("withPrivateNoStore (#4999)", () => {
  it("marks a JSON answer and keeps its status and cookies", async () => {
    const route = withPrivateNoStore(async () => {
      const response = Response.json({ access_token: "a" }, { status: 201 });
      response.headers.append("Set-Cookie", "demo=1; Path=/api/demo");
      return response;
    });

    const response = await route();

    expect(response.status).toBe(201);
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    expect(response.headers.getSetCookie()).toEqual(["demo=1; Path=/api/demo"]);
  });

  // Response.redirect has immutable headers, like a fetch result.
  it("copies a redirect whose headers are immutable", async () => {
    const redirect = Response.redirect("http://localhost:3000/dashboard", 303);
    const route = withPrivateNoStore(() => redirect);

    const response = await route();

    expect(response).not.toBe(redirect);
    expect(response.status).toBe(303);
    expect(response.headers.get("Location")).toBe(
      "http://localhost:3000/dashboard",
    );
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
  });

  it("keeps a route's own policy and leaves files alone", async () => {
    const own = withPrivateNoStore(() =>
      Response.json({}, { headers: { "Cache-Control": "public, max-age=60" } }),
    );
    const file = withPrivateNoStore(
      () => new Response("img", { headers: { "Content-Type": "image/png" } }),
    );

    expect((await own()).headers.get("Cache-Control")).toBe(
      "public, max-age=60",
    );
    expect((await file()).headers.has("Cache-Control")).toBe(false);
  });
});
