import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GET as handoff } from "./route";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://server:8080",
}));

const fetchMock = vi.fn();

function request(host: string, role: string, token?: string) {
  const headers = new Headers({ "x-forwarded-proto": "https" });
  if (token) headers.set("cookie", `moto-demo-token=${token}`);
  return new NextRequest(
    `http://${host}/api/demo/access/handoff?role=${role}`,
    { headers },
  );
}

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.demo.example");
});

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

// The banner switches between the OGS app and the parents app (#3468). The
// same token opens both; it travels from the cookie into the fragment of a
// redirect, so no script of the page reads it and no log records it.
describe("demo handoff route", () => {
  it("sends the OGS app's visitor to the parents app's entry page", async () => {
    const response = await handoff(
      request("ogs-beispiel.demo.example", "parent", "secret-token"),
    );

    expect(response.status).toBe(303);
    expect(response.headers.get("Location")).toBe(
      "https://eltern.demo.example/demo#token=secret-token&role=parent&switched=1",
    );
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("sends the parents app's visitor back to the demo school in the chosen role", async () => {
    fetchMock.mockResolvedValue(
      Response.json({
        status: "ready",
        school_url: "https://ogs-beispiel.demo.example",
      }),
    );

    const response = await handoff(
      request("eltern.demo.example", "lead", "secret-token"),
    );

    expect(response.headers.get("Location")).toBe(
      "https://ogs-beispiel.demo.example/demo#token=secret-token&role=lead&switched=1",
    );
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://server:8080/demo/access/status");
    expect((init.headers as Record<string, string>).Authorization).toBe(
      "Bearer secret-token",
    );
  });

  it.each([
    ["without a token", "lead", undefined],
    ["for an unknown role", "operator", "secret-token"],
  ])("leads to this host's entry page %s", async (_case, role, token) => {
    const response = await handoff(request("eltern.demo.example", role, token));

    expect(response.headers.get("Location")).toBe("/demo");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("leads to this host's entry page when the link has expired", async () => {
    fetchMock.mockResolvedValue(
      Response.json({ code: "demo_access_expired" }, { status: 410 }),
    );

    const response = await handoff(
      request("eltern.demo.example", "caregiver", "secret-token"),
    );

    expect(response.headers.get("Location")).toBe("/demo");
  });
});
