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
        display_name: "Safe Name",
      },
      entries: [{ confirm_password: "confirmed-password", count: 2 }],
    };

    const result = redactSensitiveLogData(input);

    expect(result).toEqual({
      password: REDACTED_LOG_VALUE,
      profile: {
        device_pin: REDACTED_LOG_VALUE,
        accessToken: REDACTED_LOG_VALUE,
        clientSecret: REDACTED_LOG_VALUE,
        display_name: "Safe Name",
      },
      entries: [{ confirm_password: REDACTED_LOG_VALUE, count: 2 }],
    });
    expect(input.password).toBe("plain-password");
    expect(input.profile.accessToken).toBe("access-token-value");
  });

  it("does not redact unrelated keys that merely contain the same letters", () => {
    expect(
      redactSensitiveLogData({ mapping: "kept", shipping: "kept" }),
    ).toEqual({ mapping: "kept", shipping: "kept" });
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
        'GET /parents/enroll/status/route-token/edit?access_token=query-token&late_invite=invite-token Authorization: Bearer bearer-token {"password":"plain-password"}',
      ),
    ).toBe(
      'GET /parents/enroll/status/[REDACTED]/edit?access_token=[REDACTED]&late_invite=[REDACTED] Authorization: Bearer [REDACTED] {"password":"[REDACTED]"}',
    );
  });
});
