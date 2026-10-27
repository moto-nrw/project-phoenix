import { describe, expect, it } from "vitest";
import { hasSessionAccess } from "./session-access";

const expires = "2026-09-10T12:00:00.000Z";

describe("hasSessionAccess", () => {
  // The browser session and the layout snapshots carry no token (#5005).
  it("grants access to a session without an error", () => {
    expect(hasSessionAccess({ user: { id: "1" }, expires })).toBe(true);
  });

  it.each(["RefreshTokenExpired", "RefreshTokenError"] as const)(
    "refuses a session the callback stripped with %s",
    (error) => {
      expect(hasSessionAccess({ user: { id: "1" }, error, expires })).toBe(
        false,
      );
    },
  );

  it("refuses a missing session", () => {
    expect(hasSessionAccess(null)).toBe(false);
    expect(hasSessionAccess(undefined)).toBe(false);
  });
});
