import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { REDACTED_LOG_VALUE } from "~/lib/log-redaction";

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<Session | null>>(),
}));

vi.mock("~/server/auth", () => ({ auth: mockAuth }));

const { POST } = await import("./route");

function createRequest(body: unknown): NextRequest {
  return new NextRequest("http://localhost:3000/api/logs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("POST /api/logs", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({
      user: { id: "account-42", name: "Test User" },
      expires: "2099-01-01",
    });
  });

  it("rejects unauthenticated log ingestion", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await POST(createRequest({ entries: [] }));

    expect(response.status).toBe(401);
  });

  it("redacts sensitive fields before writing client logs to stdout", async () => {
    const consoleLog = vi.spyOn(console, "log").mockImplementation(() => {});

    const response = await POST(
      createRequest({
        entries: [
          {
            level: "debug",
            msg: "X-API-Key: sentinel-header\npassword=sentinel-password phrase suffix, status=401",
            password: "sentinel-password",
            nested: {
              device_pin: "sentinel-pin",
              accessToken: "sentinel-token",
              client_secret: "sentinel-secret",
              jwt: "sentinel-jwt",
              api_key: "sentinel-api-key",
              authorization: "Basic sentinel-authorization",
              cookie: "session=sentinel-cookie",
              safe_id: "teacher-17",
            },
          },
        ],
        timestamp: "2026-08-03T00:00:00.000Z",
      }),
    );

    expect(response.status).toBe(200);
    expect(consoleLog).toHaveBeenCalledTimes(1);

    const output = JSON.parse(String(consoleLog.mock.calls[0]?.[0])) as {
      password: string;
      msg: string;
      nested: Record<string, unknown>;
      user_id: string;
    };
    expect(output).toMatchObject({
      password: REDACTED_LOG_VALUE,
      msg: `X-API-Key: ${REDACTED_LOG_VALUE}\npassword=${REDACTED_LOG_VALUE}, status=401`,
      nested: {
        device_pin: REDACTED_LOG_VALUE,
        accessToken: REDACTED_LOG_VALUE,
        client_secret: REDACTED_LOG_VALUE,
        jwt: REDACTED_LOG_VALUE,
        api_key: REDACTED_LOG_VALUE,
        authorization: REDACTED_LOG_VALUE,
        cookie: REDACTED_LOG_VALUE,
        safe_id: "teacher-17",
      },
      user_id: "account-42",
    });
    expect(JSON.stringify(output)).not.toMatch(
      /sentinel-(password|pin|token|secret|jwt|api-key|authorization|cookie|header)/,
    );

    consoleLog.mockRestore();
  });
});
