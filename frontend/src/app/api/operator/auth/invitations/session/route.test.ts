import { describe, expect, it } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";

describe("POST /api/operator/auth/invitations/session", () => {
  it("moves a valid invitation token into an HttpOnly cookie", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/operator/auth/invitations/session",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: "invite-token" }),
      },
    );

    const response = await POST(request);

    expect(response.status).toBe(201);
    const body = (await response.json()) as { flow_id: string };
    expect(body.flow_id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
    );
    const cookie = response.headers.get("set-cookie");
    expect(cookie).toContain(
      `operator.invitation-token.${body.flow_id}=invite-token`,
    );
    expect(cookie).toContain("HttpOnly");
    expect(cookie).toContain("SameSite=strict");
    expect(cookie).toContain("Path=/api/operator/auth/invitations");
    expect(cookie).toContain("Max-Age=604800");
  });

  it("creates distinct cookies for simultaneous invitation flows", async () => {
    const createRequest = (token: string) =>
      new NextRequest(
        "http://localhost:3000/api/operator/auth/invitations/session",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token }),
        },
      );

    const responseA = await POST(createRequest("token-a"));
    const responseB = await POST(createRequest("token-b"));
    const flowA = ((await responseA.json()) as { flow_id: string }).flow_id;
    const flowB = ((await responseB.json()) as { flow_id: string }).flow_id;

    expect(flowA).not.toBe(flowB);
    expect(responseA.headers.get("set-cookie")).toContain(
      `operator.invitation-token.${flowA}=token-a`,
    );
    expect(responseB.headers.get("set-cookie")).toContain(
      `operator.invitation-token.${flowB}=token-b`,
    );
  });

  it("rejects an empty token", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/operator/auth/invitations/session",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: "" }),
      },
    );

    const response = await POST(request);

    expect(response.status).toBe(400);
    expect(response.headers.get("set-cookie")).toBeNull();
  });
});
