import { afterEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));

const { GET } = await import("./route");

// The parent's proof behind the status link (#4786): the token is the only
// authentication, so the proxy adds none and forwards the backend's answer.
describe("GET /api/enrollment/requests/[token]/signature-proof", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("asks the status read for the PDF and passes the file through", async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response("%PDF", {
          headers: {
            "content-type": "application/pdf",
            "content-disposition": 'attachment; filename="anmeldenachweis.pdf"',
          },
        }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/enrollment/requests/a%2Fb/signature-proof",
      ),
      { params: Promise.resolve({ token: "a/b" }) },
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe("application/pdf");
    expect(fetchMock).toHaveBeenCalledWith(
      "http://backend:8080/api/enrollment/requests/a%2Fb?format=pdf",
      expect.objectContaining({ method: "GET" }),
    );
    const init = fetchMock.mock.calls[0]![1] as RequestInit;
    expect(init.headers).toBeUndefined();
  });

  it("forwards a refused token unchanged", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              status: 404,
              code: "enrollment.status_link_invalid",
              detail: "x",
            }),
            {
              status: 404,
              headers: { "content-type": "application/problem+json" },
            },
          ),
      ),
    );
    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/enrollment/requests/falsch/signature-proof",
      ),
      { params: Promise.resolve({ token: "falsch" }) },
    );
    expect(response.status).toBe(404);
  });
});
