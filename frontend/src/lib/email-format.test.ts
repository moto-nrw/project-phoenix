import { describe, expect, it } from "vitest";

import { looksLikeEmail } from "./email-format";

describe("looksLikeEmail", () => {
  it.each([
    ["lehrer@schule.de", true],
    ["  lehrer@schule.de ", true],
    ["lehrer@schule", false],
    ["lehrer@schule.d", false],
    ["lehrer schule@x.de", false],
    ["", false],
  ])("reads %j as %s", (email, expected) => {
    expect(looksLikeEmail(email)).toBe(expected);
  });
});
