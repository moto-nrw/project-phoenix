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
        authorization_status: "kept",
        cookie_consent: "kept",
      }),
    ).toEqual({
      mapping: "kept",
      shipping: "kept",
      authorization_status: "kept",
      cookie_consent: "kept",
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
        'GET /parents/enroll/status/route-token/edit?access_token=query-token&apiKey=query-api-key&authorization=query-authorization&cookie=query-cookie&late_invite=invite-token\nAuthorization: Basic basic-credential\nCookie: session=cookie-value; csrf=csrf-value\n{"password":"plain-password","api_key":"serialized-api-key","authorization":"raw-authorization","cookie":"session=serialized-cookie"}',
      ),
    ).toBe(
      'GET /parents/enroll/status/[REDACTED]/edit?access_token=[REDACTED]&apiKey=[REDACTED]&authorization=[REDACTED]&cookie=[REDACTED]&late_invite=[REDACTED]\nAuthorization: [REDACTED]\nCookie: [REDACTED]\n{"password":"[REDACTED]","api_key":"[REDACTED]","authorization":"[REDACTED]","cookie":"[REDACTED]"}',
    );
  });
});
