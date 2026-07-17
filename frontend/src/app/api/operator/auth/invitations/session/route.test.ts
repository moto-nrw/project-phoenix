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

    expect(response.status).toBe(204);
    const cookie = response.headers.get("set-cookie");
    expect(cookie).toContain("operator.invitation-token=invite-token");
    expect(cookie).toContain("HttpOnly");
    expect(cookie).toContain("SameSite=strict");
    expect(cookie).toContain("Path=/api/operator/auth/invitations");
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
