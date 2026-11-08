import { describe, expect, it } from "vitest";

import { withLateInviteQuery } from "./enrollment-http";

describe("withLateInviteQuery", () => {
  it("leaves the URL unchanged without a token", () => {
    expect(withLateInviteQuery("/api/x", undefined)).toBe("/api/x");
    expect(withLateInviteQuery("/api/x", "  ")).toBe("/api/x");
    expect(withLateInviteQuery("/api/x", null)).toBe("/api/x");
  });

  it("appends the trimmed, encoded token as the first or a further parameter", () => {
    expect(withLateInviteQuery("/api/x", " to ken/1 ")).toBe(
      "/api/x?late_invite=to%20ken%2F1",
    );
    expect(
      withLateInviteQuery("https://demo.example/anmeldung/5?ref=mail", "abc"),
    ).toBe("https://demo.example/anmeldung/5?ref=mail&late_invite=abc");
  });
});
