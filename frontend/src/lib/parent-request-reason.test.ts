import { describe, expect, it } from "vitest";

import { requiresGuardianReason } from "./parent-request-reason";

describe("requiresGuardianReason", () => {
  // Policy "nobody" (and "staff") reaches the parents portal as
  // reason_required=false: the note becomes optional.
  it("makes the note optional when the school requires no reason", () => {
    expect(requiresGuardianReason({ reason_required: false })).toBe(false);
  });

  // Policy "guardians" and "both" reach the portal as reason_required=true.
  it("keeps the note mandatory when the school requires a reason", () => {
    expect(requiresGuardianReason({ reason_required: true })).toBe(true);
  });

  // Strictest reading when the flag is missing: an old backend or a failed
  // features fetch must not let a request through that the server rejects.
  it("keeps the note mandatory when the flag is missing", () => {
    expect(requiresGuardianReason({})).toBe(true);
    expect(requiresGuardianReason()).toBe(true);
  });
});
