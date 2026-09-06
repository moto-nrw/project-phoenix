import { validateSessionToken } from "~/server/auth/token-validation";
vi.mock("~/server/auth/token-validation", () => ({
  validateSessionToken: vi.fn(),
}));
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
    vi.mocked(validateSessionToken).mockResolvedValue({
      id: "account-42",
      exp: 4102444800,
      tenant_id: 7,
    });
    mockAuth.mockResolvedValue({
      user: { id: "account-42", name: "Test User", token: "verified-access" },
      expires: "2099-01-01",
    });
  });

  it("rejects unauthenticated log ingestion", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await POST(createRequest({ entries: [] }));

    expect(response.status).toBe(401);
  });

  it("rejects a token-less session identity", async () => {
    mockAuth.mockResolvedValueOnce({
      user: { id: "account-42" },
      expires: "2099-01-01",
    });
    const response = await POST(createRequest({ entries: [] }));
    expect(response.status).toBe(401);
  });

  it("rejects stale or backend-rejected identities", async () => {
    vi.mocked(validateSessionToken).mockResolvedValueOnce(null);
    expect((await POST(createRequest({ entries: [] }))).status).toBe(401);
  });

  it("does not let client fields override verified attribution or provenance", async () => {
    const output = vi.spyOn(console, "log").mockImplementation(() => {});
    expect(
      (
        await POST(
          createRequest({
            entries: [
              {
                level: "info",
                msg: "bounded_test",
                user_id: "other",
                context: "server",
                provenance: "audit",
                via_api: false,
              },
            ],
          }),
        )
      ).status,
    ).toBe(200);
    expect(JSON.parse(String(output.mock.calls[0]?.[0]))).toMatchObject({
      user_id: "account-42",
      context: "client",
      provenance: "client_log",
      via_api: true,
    });
    output.mockRestore();
  });

  it.each([
    {
      entries: Array.from({ length: 51 }, () => ({
        level: "info",
        msg: "test",
      })),
    },
    { entries: [{ level: "info", msg: "test" }, null] },
    { entries: [{ level: "audit", msg: "test" }] },
    { entries: [{ level: "info", msg: "x".repeat(4097) }] },
  ])("rejects an invalid complete batch without logging", async (body) => {
    const output = vi.spyOn(console, "log").mockImplementation(() => {});
    expect((await POST(createRequest(body))).status).toBe(400);
    expect(output).not.toHaveBeenCalled();
    output.mockRestore();
  });

  it("bounds streamed request bytes without a content-length header", async () => {
    expect(
      (await POST(createRequest({ entries: [], padding: "x".repeat(65536) })))
        .status,
    ).toBe(413);
  });

  it("rejects malformed JSON deterministically", async () => {
    const request = new NextRequest("http://localhost/api/logs", {
      method: "POST",
      body: "{",
    });
    expect((await POST(request)).status).toBe(400);
  });

  it("limits per-actor request rate", async () => {
    vi.mocked(validateSessionToken).mockResolvedValue({
      id: "rate-test",
      exp: 4102444800,
    });
    mockAuth.mockResolvedValue({
      user: { id: "rate-test", token: "verified" },
      expires: "2099-01-01",
    });
    for (let i = 0; i < 20; i++)
      expect((await POST(createRequest({ entries: [] }))).status).toBe(200);
    const response = await POST(createRequest({ entries: [] }));
    expect(response.status).toBe(429);
    expect(response.headers.get("retry-after")).toBe("60");
  });

  it("redacts sensitive fields before writing client logs to stdout", async () => {
    const consoleLog = vi.spyOn(console, "log").mockImplementation(() => {});

    const response = await POST(
      createRequest({
        entries: [
          {
            level: "debug",
            msg: 'X-API-Key: sentinel-header\nX-Staff-ID: sentinel-staff-id\nX-Staff-PIN: sentinel-staff-pin\nX-Staff-Auth-PIN: sentinel-staff-auth-pin\nstatusToken=sentinel-status-token\nPINCode=sentinel-pin-code\nstaffPIN=sentinel-staff-pin\ntoken_value=sentinel-token-value\nproxy=http://server:8080/public/calendar/sentinel-calendar-token?format=ics\ninvite=http://server:8080/auth/invitations/sentinel-invite-token/accept\npayload={\\"access_token\\":\\"sentinel-escaped-token\\"}\npassword=sentinel-password phrase suffix, status=401',
            password: "sentinel-password",
            nested: {
              device_pin: "sentinel-pin",
              accessToken: "sentinel-token",
              client_secret: "sentinel-secret",
              jwt: "sentinel-jwt",
              JWTToken: "sentinel-compound-jwt-token",
              PINCode: "sentinel-compound-pin",
              api_key: "sentinel-api-key",
              xStaffId: "sentinel-staff-id",
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
      msg: `X-API-Key: ${REDACTED_LOG_VALUE}\nX-Staff-ID: ${REDACTED_LOG_VALUE}\nX-Staff-PIN: ${REDACTED_LOG_VALUE}\nX-Staff-Auth-PIN: ${REDACTED_LOG_VALUE}\nstatusToken=${REDACTED_LOG_VALUE}\nPINCode=${REDACTED_LOG_VALUE}\nstaffPIN=${REDACTED_LOG_VALUE}\ntoken_value=${REDACTED_LOG_VALUE}\nproxy=http://server:8080/public/calendar/${REDACTED_LOG_VALUE}?format=ics\ninvite=http://server:8080/auth/invitations/${REDACTED_LOG_VALUE}/accept\npayload={\\"access_token\\":\\"${REDACTED_LOG_VALUE}\\"}\npassword=${REDACTED_LOG_VALUE}, status=401`,
      nested: {
        device_pin: REDACTED_LOG_VALUE,
        accessToken: REDACTED_LOG_VALUE,
        client_secret: REDACTED_LOG_VALUE,
        jwt: REDACTED_LOG_VALUE,
        JWTToken: REDACTED_LOG_VALUE,
        PINCode: REDACTED_LOG_VALUE,
        api_key: REDACTED_LOG_VALUE,
        xStaffId: REDACTED_LOG_VALUE,
        authorization: REDACTED_LOG_VALUE,
        cookie: REDACTED_LOG_VALUE,
        safe_id: "teacher-17",
      },
      user_id: "account-42",
    });
    expect(JSON.stringify(output)).not.toMatch(
      /sentinel-(password|pin|token|secret|jwt|api-key|authorization|cookie|header|staff-id|staff-pin|staff-auth-pin|status-token|compound-jwt-token|compound-pin|calendar-token|invite-token|escaped-token)/,
    );

    consoleLog.mockRestore();
  });
});
