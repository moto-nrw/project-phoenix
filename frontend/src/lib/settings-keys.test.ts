import { describe, expect, it } from "vitest";
import { TENANT_RESOLVE_AFFECTING_KEYS } from "./settings-keys";

describe("TENANT_RESOLVE_AFFECTING_KEYS", () => {
  it("includes settings surfaced by tenant resolve", () => {
    expect(
      TENANT_RESOLVE_AFFECTING_KEYS.has("operations.student_photos_enabled"),
    ).toBe(true);
    expect(
      TENANT_RESOLVE_AFFECTING_KEYS.has("operations.ogs_groups_enabled"),
    ).toBe(true);
    expect(TENANT_RESOLVE_AFFECTING_KEYS.has("operations.presence_mode")).toBe(
      true,
    );
  });
});
