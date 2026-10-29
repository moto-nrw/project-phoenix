import { describe, expect, it } from "vitest";
import {
  withPrivateDownloadNoStore,
  withPrivateNoStore,
} from "./private-no-store";

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

describe("withPrivateDownloadNoStore (#5119)", () => {
  it("keeps status, filename, length, cookies and the unread stream", async () => {
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new Uint8Array([0, 255]));
        controller.close();
      },
    });
    const original = new Response(stream, {
      status: 206,
      headers: {
        "Content-Type": "image/png",
        "Content-Disposition": 'attachment; filename="private.png"',
        "Content-Length": "2",
        "Cache-Control": "public, max-age=60",
        "Set-Cookie": "session=rotated; Path=/; HttpOnly",
      },
    });
    const response = await withPrivateDownloadNoStore(() => original)();
    expect(response).toBe(original);
    expect(response.status).toBe(206);
    expect(response.body).toBe(stream);
    expect(response.bodyUsed).toBe(false);
    expect(Object.fromEntries(response.headers)).toMatchObject({
      "cache-control": "private, no-store",
      "content-type": "image/png",
      "content-disposition": 'attachment; filename="private.png"',
      "content-length": "2",
      "set-cookie": "session=rotated; Path=/; HttpOnly",
    });
    expect(new Uint8Array(await response.arrayBuffer())).toEqual(
      new Uint8Array([0, 255]),
    );
  });

  it("copies immutable headers without reading the body", async () => {
    const original = new Response(new Uint8Array([1, 2]), {
      headers: { "Content-Type": "application/pdf" },
    });
    // The fetch header guard rejects writes; exercise that branch without I/O.
    Object.defineProperty(original.headers, "set", {
      value: () => {
        throw new TypeError("immutable");
      },
    });
    const response = await withPrivateDownloadNoStore(() => original)();
    expect(response.status).toBe(200);
    expect(response.body).toBe(original.body);
    expect(response.bodyUsed).toBe(false);
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    expect(response.headers.get("Content-Type")).toBe("application/pdf");
    expect(new Uint8Array(await response.arrayBuffer())).toEqual(
      new Uint8Array([1, 2]),
    );
  });

  it.each(["image/png", "text/calendar", "text/event-stream"])(
    "leaves ordinary %s responses outside the download policy",
    async (contentType) => {
      const original = new Response("resource", {
        headers: {
          "Content-Type": contentType,
          "Cache-Control": "private, max-age=60",
        },
      });
      const response = await withPrivateNoStore(() => original)();
      expect(response).toBe(original);
      expect(response.headers.get("Cache-Control")).toBe("private, max-age=60");
      expect(await response.text()).toBe("resource");
    },
  );
});
