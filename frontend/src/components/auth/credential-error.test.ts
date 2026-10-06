import { describe, expect, it } from "vitest";

import { ApiError } from "~/lib/api-error";
import { presentError } from "~/lib/error-presentation";
import { credentialError } from "./credential-error";

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
    const shown = credentialError(refused());

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
      presentError(credentialError(refused()), "die Anmeldung").requiresLogin,
    ).toBe(false);
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

    expect(credentialError(forbidden)).toBe(forbidden);
    expect(credentialError(crash)).toBe(crash);
  });
});
