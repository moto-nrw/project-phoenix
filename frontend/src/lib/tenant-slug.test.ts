import { describe, expect, it } from "vitest";
import { isValidTenantSlug } from "./tenant-slug";

describe("isValidTenantSlug", () => {
  it.each(["school-a", "a", "1", "abc-123-def", "a".repeat(63)])(
    "accepts the valid tenant slug %s",
    (slug) => {
      expect(isValidTenantSlug(slug)).toBe(true);
    },
  );

  it.each([
    "",
    "wp-trackback.php",
    "__status",
    "School-a",
    "-school-a",
    "school-a-",
    "müller",
    "a".repeat(64),
  ])("rejects the invalid tenant slug %s", (slug) => {
    expect(isValidTenantSlug(slug)).toBe(false);
  });
});
