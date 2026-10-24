import { describe, expect, it } from "vitest";

import { ApiError } from "~/lib/api-error";
import { presentError } from "~/lib/error-presentation";
import { credentialError, LOGIN_REFUSALS } from "./credential-error";

describe("credentialError", () => {
  const refused = () => {
    const error = new ApiError("invalid credentials", 401, {
      code: "identity.invalid_credentials",
      errors: [{ field: "password", reason: "wrong" }],
      instance: "req-1",
    });
    error.retryAfterSeconds = 30;
    return error;
  };

  it("keeps code, fields and request ID but drops the 401", () => {
    const shown = credentialError(refused(), LOGIN_REFUSALS);

    expect(shown).toBeInstanceOf(ApiError);
    const error = shown as ApiError;
    expect(error.status).toBeUndefined();
    expect(error.code).toBe("identity.invalid_credentials");
    expect(error.errors).toEqual([{ field: "password", reason: "wrong" }]);
    expect(error.requestId).toBe("req-1");
    expect(error.retryAfterSeconds).toBe(30);
  });

  it("no longer sends the presentation to the login screen", () => {
    expect(presentError(refused(), "die Anmeldung").requiresLogin).toBe(true);
    expect(
      presentError(credentialError(refused(), LOGIN_REFUSALS), "die Anmeldung")
        .requiresLogin,
    ).toBe(false);
  });

  // An uncoded 401 is an expired session, also at a login step: no code is
  // invented for it (ADR 0006), so the shared path leads to the login screen.
  it("never invents a code for an uncoded 401", () => {
    const expired = new ApiError("Unauthorized", 401);
    const shown = credentialError(expired, LOGIN_REFUSALS);

    expect(shown).toBe(expired);
    expect((shown as ApiError).code).toBe("general.permission");
    expect(presentError(shown, "die Anmeldung").requiresLogin).toBe(true);
  });

  it("only rewrites the listed codes when a list is given", () => {
    const expired = new ApiError("Unauthorized", 401);

    expect(credentialError(expired, ["identity.current_password_wrong"])).toBe(
      expired,
    );
    expect(
      (credentialError(refused(), ["identity.invalid_credentials"]) as ApiError)
        .status,
    ).toBeUndefined();
  });

  it("leaves other statuses and non-API errors alone", () => {
    const forbidden = new ApiError("Forbidden", 403);
    const crash = new Error("boom");

    expect(credentialError(forbidden, LOGIN_REFUSALS)).toBe(forbidden);
    expect(credentialError(crash, LOGIN_REFUSALS)).toBe(crash);
  });
});
