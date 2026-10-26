import { describe, expect, it } from "vitest";
import { hasSessionAccess } from "./session-access";

const expires = "2026-09-10T12:00:00.000Z";

describe("hasSessionAccess", () => {
  it("grants access to a session with an access token", () => {
    expect(
      hasSessionAccess({ user: { id: "1", token: "token" }, expires }),
    ).toBe(true);
  });

  // A portal layout's server snapshot carries no token (#4990, #4991).
  it("grants access to a server snapshot without a token", () => {
    expect(hasSessionAccess({ user: { id: "1" }, expires })).toBe(true);
  });

  it("refuses a session whose token the callback stripped", () => {
    expect(
      hasSessionAccess({
        user: { id: "1", token: "" },
        error: "RefreshTokenExpired",
        expires,
      }),
    ).toBe(false);
  });

  it("refuses a missing session", () => {
    expect(hasSessionAccess(null)).toBe(false);
    expect(hasSessionAccess(undefined)).toBe(false);
  });
});
