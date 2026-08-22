import { describe, expect, it } from "vitest";
import { ParentApiError } from "./parent-api";
import { parentMessageError } from "./parent-message-error";

const messages = {
  sendError: "send failed",
  sessionExpired: "session expired",
  permissionDenied: "permission denied",
  invalidRequest: "invalid request",
};

describe("parentMessageError", () => {
  it.each([
    [401, "sessionExpired"],
    [403, "permissionDenied"],
    [400, "invalidRequest"],
    [500, "sendError"],
  ] as const)("maps HTTP %s to translated copy", (status, key) => {
    const error = new ParentApiError("raw backend text", status);

    expect(
      parentMessageError(error, (messageKey) => messages[messageKey]),
    ).toBe(messages[key]);
  });

  it("does not expose an unknown raw error", () => {
    expect(
      parentMessageError(new Error("internal details"), (key) => messages[key]),
    ).toBe("send failed");
  });
});
