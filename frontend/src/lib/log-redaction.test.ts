import { describe, expect, it } from "vitest";
import {
  REDACTED_LOG_VALUE,
  redactSensitiveLogData,
  redactSensitiveLogString,
} from "./log-redaction";

describe("redactSensitiveLogData", () => {
  it("redacts sensitive keys recursively without mutating the input", () => {
    const input = {
      password: "plain-password",
      profile: {
        device_pin: "1234",
        accessToken: "access-token-value",
        clientSecret: "client-secret-value",
        api_key: "api-key-value",
        jwt: "jwt-value",
        JWTToken: "compound-jwt-token-value",
        PINCode: "compound-pin-value",
        xStaffId: "header-staff-id-value",
        display_name: "Safe Name",
      },
      entries: [
        {
          confirm_password: "confirmed-password",
          apiKey: "camel-api-key-value",
          count: 2,
        },
      ],
      headers: {
        authorization: "Basic authorization-value",
        Cookie: "session=cookie-value",
        "X-API-Key": "header-api-key-value",
        "X-Device-Key": "header-device-key-value",
        "X-Staff-PIN": "header-staff-pin-value",
      },
    };

    const result = redactSensitiveLogData(input);

    expect(result).toEqual({
      password: REDACTED_LOG_VALUE,
      profile: {
        device_pin: REDACTED_LOG_VALUE,
        accessToken: REDACTED_LOG_VALUE,
        clientSecret: REDACTED_LOG_VALUE,
        api_key: REDACTED_LOG_VALUE,
        jwt: REDACTED_LOG_VALUE,
        JWTToken: REDACTED_LOG_VALUE,
        PINCode: REDACTED_LOG_VALUE,
        xStaffId: REDACTED_LOG_VALUE,
        display_name: "Safe Name",
      },
      entries: [
        {
          confirm_password: REDACTED_LOG_VALUE,
          apiKey: REDACTED_LOG_VALUE,
          count: 2,
        },
      ],
      headers: {
        authorization: REDACTED_LOG_VALUE,
        Cookie: REDACTED_LOG_VALUE,
        "X-API-Key": REDACTED_LOG_VALUE,
        "X-Device-Key": REDACTED_LOG_VALUE,
        "X-Staff-PIN": REDACTED_LOG_VALUE,
      },
    });
    expect(input.password).toBe("plain-password");
    expect(input.profile.accessToken).toBe("access-token-value");
  });

  it("does not redact unrelated keys that merely contain the same letters", () => {
    expect(
      redactSensitiveLogData({
        mapping: "kept",
        shipping: "kept",
        staff_id: "kept",
        authorization_status: "kept",
        cookie_consent: "kept",
      }),
    ).toEqual({
      mapping: "kept",
      shipping: "kept",
      staff_id: "kept",
      authorization_status: "kept",
      cookie_consent: "kept",
    });
    expect(
      redactSensitiveLogString(
        "?mapping=kept&shipping=kept&authorization_status=kept&cookie_consent=kept\nmapping=kept, shipping=kept, staff_id=kept",
      ),
    ).toBe(
      "?mapping=kept&shipping=kept&authorization_status=kept&cookie_consent=kept\nmapping=kept, shipping=kept, staff_id=kept",
    );
    expect(
      redactSensitiveLogString("POST /operator/auth/invitations/validate"),
    ).toBe("POST /operator/auth/invitations/validate");
  });

  it("normalizes long uppercase keys in linear time", () => {
    const longSafeKey = "A".repeat(80_000);
    const longSensitiveKey = `${longSafeKey}Token`;
    const startedAt = performance.now();

    const result = redactSensitiveLogData({
      [longSafeKey]: "kept",
      [longSensitiveKey]: "credential-value",
    }) as Record<string, unknown>;

    const elapsedMs = performance.now() - startedAt;

    expect(result[longSafeKey]).toBe("kept");
    expect(result[longSensitiveKey]).toBe(REDACTED_LOG_VALUE);
    expect(elapsedMs).toBeLessThan(1_000);
  });

  it("preserves typed token-state metadata while redacting credentials", () => {
    const tokenExpiry = new Date("2026-08-03T12:00:00.000Z");

    expect(
      redactSensitiveLogData({
        has_token: true,
        has_refresh_token: false,
        has_tokens: true,
        token_expiry: "2026-08-03T12:00:00.000Z",
        access_token_expiry: "15m",
        refresh_token_expires_at: tokenExpiry,
        unsafe_has_token: "credential-value",
        unsafe_token_expiry: "credential-value",
        token_value: "credential-value",
      }),
    ).toEqual({
      has_token: true,
      has_refresh_token: false,
      has_tokens: true,
      token_expiry: "2026-08-03T12:00:00.000Z",
      access_token_expiry: "15m",
      refresh_token_expires_at: tokenExpiry,
      unsafe_has_token: REDACTED_LOG_VALUE,
      unsafe_token_expiry: REDACTED_LOG_VALUE,
      token_value: REDACTED_LOG_VALUE,
    });
  });

  it("marks circular references instead of throwing", () => {
    const input: Record<string, unknown> = { safe: true };
    input.self = input;

    expect(redactSensitiveLogData(input)).toEqual({
      safe: true,
      self: "[Circular]",
    });
  });

  it("redacts credentials embedded in routes, query strings, and text", () => {
    expect(
      redactSensitiveLogString(
        'GET /parents/enroll/status/route-token/edit?access_token=query-token&status_token=query-status-token&statusToken=query-camel-status-token&token_value=query-token-value&staffPIN=query-staff-pin&PINCode=query-pin-code&jwt=query-jwt&apiKey=query-api-key&authorization=query-authorization&cookie=query-cookie&late_invite=invite-token\nproxy=http://server:8080/public/calendar/backend-calendar-token?format=ics\ninvite=http://server:8080/auth/invitations/standard-invite-token/accept\nAuthorization: Basic basic-credential\nX-API-Key: raw-api-key\nX-Device-Key: raw-device-key\nX-Staff-ID: 42\nX-Staff-PIN: 1234\nX-Staff-Auth-PIN: 5678\nCookie: session=cookie-value; csrf=csrf-value\n{"password":"plain-password","status_token":"serialized-status-token","statusToken":"serialized-camel-status-token","token_value":"serialized-token-value","staffPIN":"serialized-staff-pin","PINCode":"serialized-pin-code","jwt":"serialized-jwt","api_key":"serialized-api-key","X-API-Key":"serialized-header-key","x_staff_id":"serialized-staff-id","X-Staff-PIN":"serialized-staff-pin","authorization":"raw-authorization","cookie":"session=serialized-cookie"}',
      ),
    ).toBe(
      'GET /parents/enroll/status/[REDACTED]/edit?access_token=[REDACTED]&status_token=[REDACTED]&statusToken=[REDACTED]&token_value=[REDACTED]&staffPIN=[REDACTED]&PINCode=[REDACTED]&jwt=[REDACTED]&apiKey=[REDACTED]&authorization=[REDACTED]&cookie=[REDACTED]&late_invite=[REDACTED]\nproxy=http://server:8080/public/calendar/[REDACTED]?format=ics\ninvite=http://server:8080/auth/invitations/[REDACTED]/accept\nAuthorization: [REDACTED]\nX-API-Key: [REDACTED]\nX-Device-Key: [REDACTED]\nX-Staff-ID: [REDACTED]\nX-Staff-PIN: [REDACTED]\nX-Staff-Auth-PIN: [REDACTED]\nCookie: [REDACTED]\n{"password":"[REDACTED]","status_token":"[REDACTED]","statusToken":"[REDACTED]","token_value":"[REDACTED]","staffPIN":"[REDACTED]","PINCode":"[REDACTED]","jwt":"[REDACTED]","api_key":"[REDACTED]","X-API-Key":"[REDACTED]","x_staff_id":"[REDACTED]","X-Staff-PIN":"[REDACTED]","authorization":"[REDACTED]","cookie":"[REDACTED]"}',
    );
  });

  it("redacts parent-request feed capability tokens", () => {
    expect(
      redactSensitiveLogString(
        "GET /api/request-feed/frontend-secret; proxy=/public/request-feed/backend-secret",
      ),
    ).toBe(
      "GET /api/request-feed/[REDACTED] proxy=/public/request-feed/[REDACTED]",
    );
  });

  it("redacts complete quoted credential values with spaces and escapes", () => {
    expect(
      redactSensitiveLogString(
        String.raw`{"password":"correct horse's \"quoted\" secret","pin":'12\' 34',"X-API-Key":"api key's \"quoted\" suffix","safe":"kept"}`,
      ),
    ).toBe(
      `{"password":"${REDACTED_LOG_VALUE}","pin":'${REDACTED_LOG_VALUE}',"X-API-Key":"${REDACTED_LOG_VALUE}","safe":"kept"}`,
    );
  });

  it("redacts credentials in escaped serialized content", () => {
    expect(
      redactSensitiveLogString(
        String.raw`payload={\"access_token\":\"escaped-secret\",\"safe\":\"kept\"}`,
      ),
    ).toBe(
      String.raw`payload={\"access_token\":\"[REDACTED]\",\"safe\":\"kept\"}`,
    );
  });

  it("redacts unquoted credential values through explicit boundaries", () => {
    expect(
      redactSensitiveLogString(
        "password=correct horse battery\npin: 12 34, status=401\nsecret=alpha beta gamma\nPINCode=1234\nstaffPIN=5678\ntoken_value=header.payload.signature\nupstream failed;token=semicolon-secret;status=502",
      ),
    ).toBe(
      `password=${REDACTED_LOG_VALUE}\npin: ${REDACTED_LOG_VALUE}, status=401\nsecret=${REDACTED_LOG_VALUE}\nPINCode=${REDACTED_LOG_VALUE}\nstaffPIN=${REDACTED_LOG_VALUE}\ntoken_value=${REDACTED_LOG_VALUE}\nupstream failed;token=${REDACTED_LOG_VALUE};status=502`,
    );
  });
});
