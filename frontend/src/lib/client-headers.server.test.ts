import { describe, expect, it } from "vitest";
import { NextRequest } from "next/server";
import { getClientForwardHeaders } from "./client-headers.server";

describe("getClientForwardHeaders", () => {
  it("uses the request host for the frontend origin on localhost subdomains", () => {
    const request = new NextRequest(
      "http://localhost:3000/api/auth/passkeys/login/options",
      {
        headers: {
          "x-moto-original-host": "school-a.localhost:3000",
          "user-agent": "browser",
          "x-forwarded-for": "203.0.113.10",
        },
      },
    );

    expect(getClientForwardHeaders(request)).toMatchObject({
      "X-Moto-Frontend-Origin": "http://school-a.localhost:3000",
      "X-Forwarded-For": "203.0.113.10",
      "User-Agent": "browser",
    });
  });

  it("honors x-forwarded-proto when building the frontend origin", () => {
    const request = new NextRequest(
      "http://localhost:3000/api/auth/passkeys/login/options",
      {
        headers: {
          "x-moto-original-host": "school-a.moto-app.de",
          "x-moto-original-proto": "https",
        },
      },
    );

    expect(getClientForwardHeaders(request)["X-Moto-Frontend-Origin"]).toBe(
      "https://school-a.moto-app.de",
    );
  });
});
